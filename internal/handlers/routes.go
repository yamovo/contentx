package handlers

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/backup"
	"github.com/yamovo/contentx/internal/cache"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/graphql"
	"github.com/yamovo/contentx/internal/mcp"
	"github.com/yamovo/contentx/internal/middleware"
	"github.com/yamovo/contentx/internal/permissions"
	"github.com/yamovo/contentx/internal/plugin"
	"github.com/yamovo/contentx/internal/repository"
	"github.com/yamovo/contentx/internal/services"
	"github.com/yamovo/contentx/internal/storage"
	"gorm.io/gorm"
)

// RateLimiters bundles all background-cleanup rate limiters created by
// RegisterRoutes so the caller can shut them down together on exit.
type RateLimiters struct {
	IP       *middleware.IPRateLimit
	Register *middleware.KeyRateLimiter
}

// Shutdown stops all background cleanup goroutines. Safe to call multiple times.
func (rl *RateLimiters) Shutdown() {
	if rl == nil {
		return
	}
	if rl.IP != nil {
		rl.IP.Shutdown()
	}
	if rl.Register != nil {
		rl.Register.Stop()
	}
}

// RegisterRoutes sets up all API routes. The backupMgr is created by the caller
// (cmd/server/main.go) so that the backup scheduler and the HTTP handler share
// the same Manager instance (and thus the same TryLock for concurrency control).
func RegisterRoutes(
	r *gin.Engine,
	db *gorm.DB,
	cfg *config.Config,
	jwtMgr *auth.JWTManager,
	blacklist auth.TokenStore,
	guard auth.LoginLimiter,
	cacheDriver cache.Driver,
	backupMgr *backup.Manager,
	promCollector *middleware.PrometheusCollector,
) *RateLimiters {
	// Create services.
	articleSvc := services.NewArticleService(db, cfg.Server.BaseURL)
	authSvc := services.NewAuthService(db, jwtMgr, blacklist, guard, cfg.Auth)
	totpSvc := services.NewTOTPService(db)
	// Wire the TOTP second factor into login (checked after password, before
	// token generation).
	authSvc.SetTOTPVerifier(totpSvc)
	userSvc := services.NewUserService(db)
	roleSvc := services.NewRoleService(db)
	categorySvc := services.NewCategoryService(db)
	tagSvc := services.NewTagService(db)
	commentSvc := services.NewCommentService(db)
	mediaSvc := services.NewMediaService(db, cfg.Upload)
	settingsSvc := services.NewSettingsService(db)
	seoSvc := services.NewSEOService(db, cfg.Server.BaseURL)
	menuSvc := services.NewMenuService(db)
	analyticsSvc := services.NewAnalyticsService(db)
	pluginSvc := services.NewPluginService(db)
	themeSvc := services.NewThemeService(db)
	systemSvc := services.NewSystemService(db)
	if promCollector != nil {
		systemSvc.SetMetricsCollector(promCollector)
		promCollector.SetSnapshotter(systemSvc.SnapshotMetrics)
	}
	tokenSvc := services.NewTokenService(db)
	contentTypeSvc := services.NewContentTypeService(db).WithCache(cacheDriver, cfg.Cache.DefaultTTL)
	webhookSvc := services.NewWebhookService(db)

	// Inject webhook dispatcher into services that trigger events.
	articleSvc.SetWebhookDispatcher(webhookSvc)
	commentSvc.SetWebhookDispatcher(webhookSvc)
	mediaSvc.SetWebhookDispatcher(webhookSvc)
	userSvc.SetWebhookDispatcher(webhookSvc)

	// ─── Audit Logger: service-layer business audit with EntityID/Details.
	// Complements the HTTP-level ActivityLogger middleware (which only
	// captures method/route/IP/UA) by recording entity IDs and structured
	// details for security-sensitive operations.
	auditLogger := services.NewAuditLogger(repository.NewActivityLogRepository(db))
	articleSvc.SetAuditLogger(auditLogger)
	authSvc.SetAuditLogger(auditLogger)
	userSvc.SetAuditLogger(auditLogger)
	roleSvc.SetAuditLogger(auditLogger)
	webhookSvc.SetAuditLogger(auditLogger)
	settingsSvc.SetAuditLogger(auditLogger)

	// ─── Plugin Manager: register built-in plugins and inject into services.
	pluginMgr := plugin.NewManager(db)
	_ = pluginMgr.Register(plugin.NewWordCountPlugin())
	_ = pluginMgr.InitDB()
	articleSvc.SetPluginManager(pluginMgr)
	pluginSvc.SetPluginManager(pluginMgr)

	// ─── Search Indexer: builtin in-memory by default; MeiliSearch when
	// configured. The indexer is injected into ArticleService so every
	// create/update/delete/status-transition keeps the index in sync.
	searchIdx := buildSearchIndexer(cfg)
	articleSvc.SetSearchIndexer(searchIdx)
	articleSvc.WithCache(cacheDriver, cfg.Cache.DefaultTTL)
	if searchIdx.Name() != "noop" {
		// Warm up the index from the database on startup (best-effort;
		// failures are logged but non-fatal). Runs in a goroutine so it
		// never blocks the server from accepting traffic.
		go func() {
			n, err := articleSvc.ReindexAll(context.Background())
			if err != nil {
				slog.Warn("search index warmup failed", "error", err)
				return
			}
			slog.Info("search index warmed up", "indexed", n, "engine", searchIdx.Name())
		}()
	}

	// Build and inject the storage driver based on configuration. When the
	// driver is "local" (or unset) we keep the legacy inline disk logic in
	// MediaService (store == nil). When it is "s3" we construct an S3Driver
	// from the S3 sub-config and inject it.
	if d := buildStorageDriver(cfg); d != nil {
		mediaSvc.SetStorageDriver(d)
	}

	// Create handlers.
	authH := NewAuthHandler(authSvc)
	totpH := NewTOTPHandler(totpSvc)
	articleH := NewArticleHandler(articleSvc, cfg.Limits.MaxBulkActionSize)
	categoryH := NewCategoryHandler(categorySvc)
	tagH := NewTagHandler(tagSvc)
	commentH := NewCommentHandler(commentSvc, cfg.Limits.MaxBulkActionSize)
	mediaH := NewMediaHandler(mediaSvc, cfg.Limits.MaxBulkActionSize)
	userH := NewUserHandler(userSvc)
	roleH := NewRoleHandler(roleSvc)
	settingsH := NewSettingsHandler(settingsSvc)
	seoH := NewSEOHandler(seoSvc)
	menuH := NewMenuHandler(menuSvc)
	analyticsH := NewAnalyticsHandler(analyticsSvc)
	pluginH := NewPluginHandler(pluginSvc)
	themeH := NewThemeHandler(themeSvc)
	systemH := NewSystemHandler(systemSvc)
	tokenH := NewTokenHandler(tokenSvc)
	contentTypeH := NewContentTypeHandler(contentTypeSvc)
	webhookH := NewWebhookHandler(webhookSvc)
	searchH := NewSearchHandler(articleSvc)
	backupH := NewBackupHandler(backupMgr, articleSvc, auditLogger).WithCache(cacheDriver)

	// Rate limiter for specific groups (requests per minute).
	const (
		rateLimitAuth     = 10
		rateLimitUpload   = 20
		rateLimitComment  = 30
		rateLimitRegister = 3 // per email+IP — prevents targeted registration spam
	)
	rl := middleware.NewIPRateLimit()
	rl.Add("auth", rateLimitAuth)
	rl.Add("upload", rateLimitUpload)
	rl.Add("comment", rateLimitComment)

	// Account-dimension rate limiter for sensitive endpoints (P1-5).
	// Keyed by email+IP on the registration endpoint to complement the
	// IP-only auth group limit above.
	registerLimiter := middleware.NewKeyRateLimiter(rateLimitRegister)
	authH.SetRegisterRateLimiter(registerLimiter)

	// ─── Public API ────────────────────────────────────
	api := r.Group("/api/v1")
	{
		// Auth (rate-limited).
		authGroup := api.Group("/auth")
		authGroup.Use(middleware.GroupRateLimit(rl, "auth"))
		{
			authGroup.POST("/login", authH.Login)
			authGroup.POST("/register", authH.Register)
			authGroup.POST("/refresh", authH.RefreshToken)
		}

		// Public content.
		api.GET("/articles/slug/:slug", articleH.GetBySlug)
		api.GET("/articles/:id/comments", commentH.ArticleComments)
		api.GET("/feed", articleH.Feed)
		api.GET("/seo/sitemap", seoH.Sitemap)
		api.GET("/seo/robots.txt", seoH.RobotsTxt)
		api.GET("/settings/public", settingsH.PublicSettings)
		api.POST("/analytics/record", analyticsH.RecordView)

		// Public full-text search (forces status=published).
		api.GET("/search", searchH.Search)

		// GraphQL (read-only public endpoint). Reuses the same service
		// instances as the REST handlers; the schema exposes published
		// articles, taxonomy, approved comments, public user profiles, and
		// the RSS feed. Writes continue to go through the REST API.
		gqlSchema, gqlErr := graphql.NewSchema(graphql.Services{
			Article:  articleSvc,
			Category: categorySvc,
			Tag:      tagSvc,
			Comment:  commentSvc,
			User:     userSvc,
		})
		if gqlErr != nil {
			slog.Error("failed to build graphql schema", "error", gqlErr)
		} else {
			api.GET("/graphql", graphql.Handler(gqlSchema))
			api.POST("/graphql", graphql.Handler(gqlSchema))
		}
	}

	// ─── MCP over Streamable HTTP (opt-in; own API-token auth) ─────────────
	// Reuses the same read-only services as the stdio MCP mode. Sits outside the
	// JWT-protected group and enforces API-token auth in mcpTokenAuth.
	if cfg.MCP.HTTPEnabled {
		mountMCPHTTP(api, mcp.Deps{
			Article:       articleSvc,
			ContentType:   contentTypeSvc,
			BaseURL:       cfg.Server.BaseURL,
			IncludeDrafts: cfg.MCP.IncludeDrafts,
		}, tokenSvc)
		slog.Info("MCP HTTP endpoint enabled", "path", "/api/v1/mcp")
	}

	// ─── Protected API ─────────────────────────────────
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware(jwtMgr, db, blacklist))
	{
		// Auth (user operations).
		authP := protected.Group("/auth")
		{
			authP.POST("/logout", authH.Logout)
			authP.GET("/me", authH.Me)
			authP.PUT("/profile", authH.UpdateProfile)
			authP.PUT("/password", authH.ChangePassword)

			// TOTP two-factor authentication (current user).
			authP.GET("/totp/status", totpH.Status)
			authP.POST("/totp/setup", totpH.Setup)
			authP.POST("/totp/enable", totpH.Enable)
			authP.POST("/totp/disable", totpH.Disable)
		}

		// Articles.
		articles := protected.Group("/articles")
		{
			articles.GET("", middleware.RequirePermission(permissions.ArticlesRead), articleH.List)
			articles.GET("/:id", middleware.RequirePermission(permissions.ArticlesRead), articleH.Get)
			articles.POST("", middleware.RequirePermission(permissions.ArticlesCreate), articleH.Create)
			articles.PUT("/:id", middleware.RequirePermission(permissions.ArticlesUpdate), articleH.Update)
			articles.DELETE("/:id", middleware.RequirePermission(permissions.ArticlesDelete), articleH.Delete)
			// BulkAction selects the exact permission after parsing the action.
			articles.POST("/bulk", articleH.BulkAction)
			articles.GET("/:id/revisions", middleware.RequirePermission(permissions.ArticlesRead), articleH.Revisions)
			articles.POST("/:id/revisions/:revision_id/restore", middleware.RequirePermission(permissions.ArticlesUpdate), articleH.RestoreRevision)
			articles.POST("/:id/like", articleH.LikeArticle)

			// Publication workflow (P2-3): single-article status transitions.
			articles.POST("/:id/publish", middleware.RequirePermission(permissions.ArticlesPublish), articleH.Publish)
			articles.POST("/:id/unpublish", middleware.RequirePermission(permissions.ArticlesPublish), articleH.Unpublish)
			articles.POST("/:id/submit-review", middleware.RequirePermission(permissions.ArticlesUpdate), articleH.SubmitForReview)
			articles.POST("/:id/approve", middleware.RequirePermission(permissions.ArticlesPublish), articleH.Approve)
			articles.POST("/:id/schedule", middleware.RequirePermission(permissions.ArticlesPublish), articleH.Schedule)
			articles.POST("/:id/archive", middleware.RequirePermission(permissions.ArticlesPublish), articleH.Archive)

			// i18n: article translations.
			articles.GET("/:id/translations", middleware.RequirePermission(permissions.ArticlesRead), articleH.ListTranslations)
			articles.POST("/:id/translations", middleware.RequirePermission(permissions.ArticlesCreate), articleH.CreateTranslation)
		}

		// Categories.
		categories := protected.Group("/categories")
		{
			categories.GET("", middleware.RequirePermission(permissions.CategoriesRead), categoryH.List)
			categories.GET("/:id", middleware.RequirePermission(permissions.CategoriesRead), categoryH.Get)
			categories.POST("", middleware.RequirePermission(permissions.CategoriesCreate), categoryH.Create)
			categories.PUT("/:id", middleware.RequirePermission(permissions.CategoriesUpdate), categoryH.Update)
			categories.DELETE("/:id", middleware.RequirePermission(permissions.CategoriesDelete), categoryH.Delete)
			categories.PUT("/reorder", middleware.RequirePermission(permissions.CategoriesUpdate), categoryH.Reorder)
		}

		// Tags.
		tags := protected.Group("/tags")
		{
			tags.GET("", middleware.RequirePermission(permissions.TagsRead), tagH.List)
			tags.GET("/:id", middleware.RequirePermission(permissions.TagsRead), tagH.Get)
			tags.POST("", middleware.RequirePermission(permissions.TagsCreate), tagH.Create)
			tags.PUT("/:id", middleware.RequirePermission(permissions.TagsUpdate), tagH.Update)
			tags.DELETE("/:id", middleware.RequirePermission(permissions.TagsDelete), tagH.Delete)
			tags.POST("/merge", middleware.RequirePermission(permissions.TagsUpdate), tagH.Merge)
		}

		// Comments.
		comments := protected.Group("/comments")
		{
			comments.GET("", middleware.RequirePermission(permissions.CommentsRead), commentH.List)
			comments.GET("/:id", middleware.RequirePermission(permissions.CommentsRead), commentH.Get)
			comments.POST("", middleware.GroupRateLimit(rl, "comment"), middleware.RequirePermission(permissions.CommentsCreate), commentH.Create)
			comments.PUT("/:id", middleware.RequirePermission(permissions.CommentsUpdate), commentH.Update)
			comments.POST("/:id/approve", middleware.RequirePermission(permissions.CommentsModerate), commentH.Approve)
			comments.POST("/:id/spam", middleware.RequirePermission(permissions.CommentsModerate), commentH.Spam)
			comments.POST("/:id/trash", middleware.RequirePermission(permissions.CommentsModerate), commentH.Trash)
			comments.POST("/bulk", middleware.RequirePermission(permissions.CommentsModerate), commentH.BulkAction)
			comments.GET("/stats", middleware.RequirePermission(permissions.CommentsRead), commentH.Stats)
		}

		// Media.
		media := protected.Group("/media")
		media.Use(middleware.GroupRateLimit(rl, "upload"))
		{
			media.GET("", middleware.RequirePermission(permissions.MediaRead), mediaH.List)
			media.GET("/folders", middleware.RequirePermission(permissions.MediaRead), mediaH.Folders)
			media.GET("/stats", middleware.RequirePermission(permissions.MediaRead), mediaH.Stats)
			media.GET("/:id", middleware.RequirePermission(permissions.MediaRead), mediaH.Get)
			media.POST("/upload", middleware.RequirePermission(permissions.MediaUpload), mediaH.Upload)
			media.PUT("/:id", middleware.RequirePermission(permissions.MediaUpdate), mediaH.Update)
			media.DELETE("/:id", middleware.RequirePermission(permissions.MediaDelete), mediaH.Delete)
			media.POST("/bulk-delete", middleware.RequirePermission(permissions.MediaDelete), mediaH.BulkDelete)
		}

		// Users.
		users := protected.Group("/users")
		{
			users.GET("", middleware.RequirePermission(permissions.UsersRead), userH.List)
			users.GET("/:id", middleware.RequirePermission(permissions.UsersRead), userH.Get)
			users.POST("", middleware.RequirePermission(permissions.UsersCreate), userH.Create)
			users.PUT("/:id", middleware.RequirePermission(permissions.UsersUpdate), userH.Update)
			users.DELETE("/:id", middleware.RequirePermission(permissions.UsersDelete), userH.Delete)
			users.POST("/:id/reset-password", middleware.RequirePermission(permissions.UsersUpdate), userH.ResetPassword)
		}

		// Roles.
		roles := protected.Group("/roles")
		{
			roles.GET("", middleware.RequirePermission(permissions.RolesRead), roleH.List)
			roles.POST("", middleware.RequirePermission(permissions.RolesCreate), roleH.Create)
			roles.PUT("/:id", middleware.RequirePermission(permissions.RolesUpdate), roleH.Update)
			roles.DELETE("/:id", middleware.RequirePermission(permissions.RolesDelete), roleH.Delete)
			roles.GET("/permissions", middleware.RequirePermission(permissions.RolesRead), roleH.Permissions)
		}

		// Settings.
		settings := protected.Group("/settings")
		{
			settings.GET("", middleware.RequirePermission(permissions.SettingsRead), settingsH.List)
			settings.GET("/:key", middleware.RequirePermission(permissions.SettingsRead), settingsH.Get)
			settings.PUT("", middleware.RequirePermission(permissions.SettingsUpdate), settingsH.Update)
		}

		// SEO.
		seo := protected.Group("/seo")
		{
			seo.GET("/:type/:id", middleware.RequirePermission(permissions.SEORead), seoH.GetSEOSetting)
			seo.PUT("/:type/:id", middleware.RequirePermission(permissions.SEOUpdate), seoH.UpdateSEOSetting)
			seo.GET("/redirects", middleware.RequirePermission(permissions.SEORead), seoH.ListRedirects)
			seo.POST("/redirects", middleware.RequirePermission(permissions.SEOUpdate), seoH.CreateRedirect)
			seo.DELETE("/redirects/:id", middleware.RequirePermission(permissions.SEOUpdate), seoH.DeleteRedirect)
		}

		// Menus.
		menus := protected.Group("/menus")
		{
			menus.GET("", middleware.RequirePermission(permissions.MenusRead), menuH.List)
			menus.GET("/:id", middleware.RequirePermission(permissions.MenusRead), menuH.Get)
			menus.POST("", middleware.RequirePermission(permissions.MenusCreate), menuH.Create)
			menus.PUT("/:id", middleware.RequirePermission(permissions.MenusUpdate), menuH.Update)
			menus.DELETE("/:id", middleware.RequirePermission(permissions.MenusDelete), menuH.Delete)
			menus.POST("/:id/items", middleware.RequirePermission(permissions.MenusUpdate), menuH.AddItem)
			menus.PUT("/:id/items/:item_id", middleware.RequirePermission(permissions.MenusUpdate), menuH.UpdateItem)
			menus.DELETE("/:id/items/:item_id", middleware.RequirePermission(permissions.MenusUpdate), menuH.DeleteItem)
			menus.PUT("/:id/items/reorder", middleware.RequirePermission(permissions.MenusUpdate), menuH.ReorderItems)
		}

		// Analytics.
		analytics := protected.Group("/analytics")
		{
			analytics.GET("/dashboard", middleware.RequirePermission(permissions.AnalyticsRead), analyticsH.Dashboard)
			analytics.GET("/views", middleware.RequirePermission(permissions.AnalyticsRead), analyticsH.ViewsOverTime)
			analytics.GET("/referrers", middleware.RequirePermission(permissions.AnalyticsRead), analyticsH.TopReferrers)
			analytics.GET("/devices", middleware.RequirePermission(permissions.AnalyticsRead), analyticsH.DeviceBreakdown)
		}

		// Plugins.
		plugins := protected.Group("/plugins")
		{
			plugins.GET("", middleware.RequirePermission(permissions.PluginsRead), pluginH.List)
			plugins.POST("/:id/enable", middleware.RequirePermission(permissions.PluginsUpdate), pluginH.Enable)
			plugins.POST("/:id/disable", middleware.RequirePermission(permissions.PluginsUpdate), pluginH.Disable)
			plugins.PUT("/:id/config", middleware.RequirePermission(permissions.PluginsUpdate), pluginH.UpdateConfig)
		}

		// Themes.
		themes := protected.Group("/themes")
		{
			themes.GET("", middleware.RequirePermission(permissions.ThemesRead), themeH.List)
			themes.POST("/:id/activate", middleware.RequirePermission(permissions.ThemesUpdate), themeH.Activate)
			themes.PUT("/:id/config", middleware.RequirePermission(permissions.ThemesUpdate), themeH.UpdateConfig)
		}

		// System.
		system := protected.Group("/system")
		{
			system.GET("/info", middleware.RequirePermission(permissions.SystemRead), systemH.Info)
			system.GET("/activity", middleware.RequirePermission(permissions.SystemActivityLog), systemH.ActivityLog)

			system.GET("/tokens", middleware.RequirePermission(permissions.APITokensRead), tokenH.List)
			system.POST("/tokens", middleware.RequirePermission(permissions.APITokensCreate), tokenH.Create)
			system.DELETE("/tokens/:id", middleware.RequirePermission(permissions.APITokensDelete), tokenH.Delete)
		}

		contentTypes := protected.Group("/content-types")
		{
			contentTypes.GET("", middleware.RequirePermission(permissions.ContentTypesRead), contentTypeH.ListTypes)
			contentTypes.GET("/:uid", middleware.RequirePermission(permissions.ContentTypesRead), contentTypeH.GetType)
			contentTypes.POST("", middleware.RequirePermission(permissions.ContentTypesCreate), contentTypeH.CreateType)
			contentTypes.DELETE("/:uid", middleware.RequirePermission(permissions.ContentTypesDelete), contentTypeH.DeleteType)
		}

		// Content Entries (dynamic).
		content := protected.Group("/content")
		{
			content.GET("/:uid", middleware.RequirePermission(permissions.ContentRead), contentTypeH.ListEntries)
			content.GET("/:uid/export", middleware.RequirePermission(permissions.ContentRead), contentTypeH.ExportEntries)
			content.POST("/:uid/import", middleware.RequirePermission(permissions.ContentCreate), contentTypeH.ImportEntries)
			content.GET("/:uid/:documentId", middleware.RequirePermission(permissions.ContentRead), contentTypeH.GetEntry)
			content.POST("/:uid", middleware.RequirePermission(permissions.ContentCreate), contentTypeH.CreateEntry)
			content.PUT("/:uid/:documentId", middleware.RequirePermission(permissions.ContentUpdate), contentTypeH.UpdateEntry)
			content.DELETE("/:uid/:documentId", middleware.RequirePermission(permissions.ContentDelete), contentTypeH.DeleteEntry)
			content.POST("/:uid/:documentId/publish", middleware.RequirePermission(permissions.ContentPublish), contentTypeH.PublishEntry)
			content.POST("/:uid/:documentId/unpublish", middleware.RequirePermission(permissions.ContentPublish), contentTypeH.UnpublishEntry)

			// i18n: content entry translations.
			content.GET("/:uid/:documentId/translations", middleware.RequirePermission(permissions.ContentRead), contentTypeH.ListEntryTranslations)
			content.POST("/:uid/:documentId/translations", middleware.RequirePermission(permissions.ContentCreate), contentTypeH.CreateEntryTranslation)
		}

		webhooks := protected.Group("/webhooks")
		{
			webhooks.GET("", middleware.RequirePermission(permissions.WebhooksRead), webhookH.List)
			webhooks.POST("", middleware.RequirePermission(permissions.WebhooksCreate), webhookH.Create)
			webhooks.DELETE("/:id", middleware.RequirePermission(permissions.WebhooksDelete), webhookH.Delete)
			webhooks.GET("/:id/logs", middleware.RequirePermission(permissions.WebhooksRead), webhookH.Logs)
		}

		// Search (admin: cross-status search + manual reindex).
		searchAdmin := protected.Group("/search")
		{
			searchAdmin.GET("/admin", searchH.AdminSearch)
			searchAdmin.POST("/reindex", middleware.RequireAdmin(), searchH.Reindex)
		}

		backupGroup := protected.Group("/admin/backup")
		{
			backupGroup.GET("", middleware.RequirePermission(permissions.BackupsRead), backupH.List)
			backupGroup.POST("", middleware.RequirePermission(permissions.BackupsCreate), backupH.Create)
			backupGroup.GET("/:file/download", middleware.RequirePermission(permissions.BackupsRead), backupH.Download)
			backupGroup.POST("/:file/restore", middleware.RequirePermission(permissions.BackupsRestore), backupH.Restore)
			backupGroup.DELETE("/:file", middleware.RequirePermission(permissions.BackupsDelete), backupH.Delete)
		}
	}

	// System health (unauthenticated).
	r.GET("/api/v1/system/health", systemH.Health)

	// Static file serving for uploads. Only relevant for the local driver
	// path; when an S3 driver is in use, files are served from object storage
	// and this route simply 404s (harmless).
	//
	// Uploaded files are served with X-Content-Type-Options: nosniff, and any
	// non-image/video file (HTML, SVG, PDF, ...) is forced to download via
	// Content-Disposition: attachment so user-uploaded active content cannot
	// execute in the application's origin (defense-in-depth against stored XSS).
	uploads := r.Group(cfg.Upload.URLPrefix)
	uploads.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		if !inlineSafeUpload(c.Request.URL.Path) {
			c.Header("Content-Disposition", "attachment")
		}
	})
	uploads.Static("/", cfg.Upload.StoragePath)
	return &RateLimiters{IP: rl, Register: registerLimiter}
}

// inlineSafeUpload reports whether an uploaded file at the given path may be
// served inline. Only raster images and common video containers are considered
// safe; every other type (HTML, SVG, PDF, scripts, ...) is forced to download
// so it cannot execute in the application's origin.
func inlineSafeUpload(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".mp4", ".webm":
		return true
	default:
		return false
	}
}

// buildStorageDriver constructs a storage.Driver from the application config.
// It returns nil for the "local" driver (or any unrecognized value), which
// signals MediaService to use its legacy inline local-disk logic. Only "s3"
// (and recognized aliases) produces a non-nil driver.
func buildStorageDriver(cfg *config.Config) storage.Driver {
	switch cfg.Upload.Driver {
	case "", "local":
		return nil
	case "s3", "minio", "oss":
		s3 := cfg.Upload.S3
		if s3.Endpoint == "" || s3.AccessKey == "" || s3.SecretKey == "" {
			slog.Warn("storage driver set to s3 but endpoint/access_key/secret_key missing; falling back to local disk",
				"driver", cfg.Upload.Driver, "endpoint", s3.Endpoint)
			return nil
		}
		return storage.NewS3Driver(storage.S3Config{
			Endpoint:  s3.Endpoint,
			Bucket:    s3.Bucket,
			Region:    s3.Region,
			AccessKey: s3.AccessKey,
			SecretKey: s3.SecretKey,
			PublicURL: s3.PublicURL,
			UseSSL:    s3.UseSSL,
			PathStyle: s3.PathStyle,
		})
	default:
		slog.Warn("unknown storage driver; falling back to local disk", "driver", cfg.Upload.Driver)
		return nil
	}
}

// buildSearchIndexer constructs a SearchIndexer from the application config.
//   - "builtin" (default): in-memory inverted index with BM25 + CJK bigram
//     tokenization. Zero external dependencies; rebuilt on startup.
//   - "meilisearch": external MeiliSearch server. When the SDK or server is
//     unreachable, falls back to builtin with a warning so the app still runs.
//   - "noop"/"off"/"disabled": search disabled entirely (NoopIndexer).
func buildSearchIndexer(cfg *config.Config) services.SearchIndexer {
	switch cfg.Search.Engine {
	case "", "builtin", "memory":
		return services.NewBuiltinIndexer()
	case "noop", "off", "disabled":
		return services.NoopIndexer()
	case "meilisearch", "meili":
		// MeiliSearch SDK is not bundled to avoid pulling a network
		// dependency at build time. When the operator configures
		// SEARCH_ENGINE=meilisearch we log a notice and fall back to the
		// builtin indexer; a future task can wire the real client.
		slog.Warn("meilisearch engine requested but SDK not bundled; falling back to builtin indexer",
			"engine", cfg.Search.Engine, "meili_url", cfg.Search.MeiliURL)
		return services.NewBuiltinIndexer()
	default:
		slog.Warn("unknown search engine; falling back to builtin", "engine", cfg.Search.Engine)
		return services.NewBuiltinIndexer()
	}
}
