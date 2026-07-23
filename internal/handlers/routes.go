package handlers

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/backup"
	"github.com/yamovo/contentx/internal/cache"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/graphql"
	"github.com/yamovo/contentx/internal/middleware"
	"github.com/yamovo/contentx/internal/plugin"
	"github.com/yamovo/contentx/internal/services"
	"github.com/yamovo/contentx/internal/storage"
	"gorm.io/gorm"
)

// RegisterRoutes sets up all API routes. The backupMgr is created by the caller
// (cmd/server/main.go) so that the backup scheduler and the HTTP handler share
// the same Manager instance (and thus the same TryLock for concurrency control).
func RegisterRoutes(
	r *gin.Engine,
	db *gorm.DB,
	cfg *config.Config,
	jwtMgr *auth.JWTManager,
	blacklist auth.TokenStore,
	guard *auth.LoginGuard,
	cacheDriver cache.Driver,
	backupMgr *backup.Manager,
) *middleware.IPRateLimit {
	// Create services.
	articleSvc := services.NewArticleService(db, cfg.Server.BaseURL)
	authSvc := services.NewAuthService(db, jwtMgr, blacklist, guard)
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
	tokenSvc := services.NewTokenService(db)
	contentTypeSvc := services.NewContentTypeService(db).WithCache(cacheDriver, cfg.Cache.DefaultTTL)
	webhookSvc := services.NewWebhookService(db)

	// Inject webhook dispatcher into services that trigger events.
	articleSvc.SetWebhookDispatcher(webhookSvc)
	commentSvc.SetWebhookDispatcher(webhookSvc)
	mediaSvc.SetWebhookDispatcher(webhookSvc)
	userSvc.SetWebhookDispatcher(webhookSvc)

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
	articleH := NewArticleHandler(articleSvc)
	categoryH := NewCategoryHandler(categorySvc)
	tagH := NewTagHandler(tagSvc)
	commentH := NewCommentHandler(commentSvc)
	mediaH := NewMediaHandler(mediaSvc)
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
	backupH := NewBackupHandler(backupMgr, articleSvc)

	// Rate limiter for specific groups.
	rl := middleware.NewIPRateLimit()
	rl.Add("auth", 10)    // 10 req/min for auth
	rl.Add("upload", 20)  // 20 req/min for uploads
	rl.Add("comment", 30) // 30 req/min for comments

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
		}

		// Articles.
		articles := protected.Group("/articles")
		{
			articles.GET("", articleH.List)
			articles.GET("/:id", articleH.Get)
			articles.POST("", middleware.RequirePermission("articles.create"), articleH.Create)
			articles.PUT("/:id", middleware.RequirePermission("articles.edit"), articleH.Update)
			articles.DELETE("/:id", middleware.RequirePermission("articles.delete"), articleH.Delete)
			articles.POST("/bulk", middleware.RequireEditor(), articleH.BulkAction)
			articles.GET("/:id/revisions", articleH.Revisions)
			articles.POST("/:id/revisions/:revision_id/restore", articleH.RestoreRevision)
			articles.POST("/:id/like", articleH.LikeArticle)

			// Publication workflow (P2-3): single-article status transitions.
			// Reuse articles.edit for publish/unpublish/schedule/archive and
			// RequireEditor for the review-approval step.
			articles.POST("/:id/publish", middleware.RequirePermission("articles.edit"), articleH.Publish)
			articles.POST("/:id/unpublish", middleware.RequirePermission("articles.edit"), articleH.Unpublish)
			articles.POST("/:id/submit-review", middleware.RequirePermission("articles.edit"), articleH.SubmitForReview)
			articles.POST("/:id/approve", middleware.RequireEditor(), articleH.Approve)
			articles.POST("/:id/schedule", middleware.RequirePermission("articles.edit"), articleH.Schedule)
			articles.POST("/:id/archive", middleware.RequirePermission("articles.edit"), articleH.Archive)

			// i18n: article translations.
			articles.GET("/:id/translations", articleH.ListTranslations)
			articles.POST("/:id/translations", middleware.RequirePermission("articles.create"), articleH.CreateTranslation)
		}

		// Categories.
		categories := protected.Group("/categories")
		{
			categories.GET("", categoryH.List)
			categories.GET("/:id", categoryH.Get)
			categories.POST("", middleware.RequirePermission("categories.manage"), categoryH.Create)
			categories.PUT("/:id", middleware.RequirePermission("categories.manage"), categoryH.Update)
			categories.DELETE("/:id", middleware.RequirePermission("categories.manage"), categoryH.Delete)
			categories.PUT("/reorder", middleware.RequirePermission("categories.manage"), categoryH.Reorder)
		}

		// Tags.
		tags := protected.Group("/tags")
		{
			tags.GET("", tagH.List)
			tags.GET("/:id", tagH.Get)
			tags.POST("", middleware.RequirePermission("tags.manage"), tagH.Create)
			tags.PUT("/:id", middleware.RequirePermission("tags.manage"), tagH.Update)
			tags.DELETE("/:id", middleware.RequirePermission("tags.manage"), tagH.Delete)
			tags.POST("/merge", middleware.RequirePermission("tags.manage"), tagH.Merge)
		}

		// Comments.
		comments := protected.Group("/comments")
		{
			comments.GET("", commentH.List)
			comments.GET("/:id", commentH.Get)
			comments.POST("", middleware.GroupRateLimit(rl, "comment"), commentH.Create)
			comments.PUT("/:id", commentH.Update)
			comments.POST("/:id/approve", middleware.RequirePermission("comments.moderate"), commentH.Approve)
			comments.POST("/:id/spam", middleware.RequirePermission("comments.moderate"), commentH.Spam)
			comments.POST("/:id/trash", middleware.RequirePermission("comments.moderate"), commentH.Trash)
			comments.POST("/bulk", middleware.RequirePermission("comments.moderate"), commentH.BulkAction)
			comments.GET("/stats", middleware.RequirePermission("comments.view"), commentH.Stats)
		}

		// Media.
		media := protected.Group("/media")
		media.Use(middleware.GroupRateLimit(rl, "upload"))
		{
			media.GET("", mediaH.List)
			media.GET("/folders", mediaH.Folders)
			media.GET("/stats", mediaH.Stats)
			media.GET("/:id", mediaH.Get)
			media.POST("/upload", middleware.RequirePermission("media.upload"), mediaH.Upload)
			media.PUT("/:id", mediaH.Update)
			media.DELETE("/:id", middleware.RequirePermission("media.delete"), mediaH.Delete)
			media.POST("/bulk-delete", middleware.RequirePermission("media.delete"), mediaH.BulkDelete)
		}

		// Users.
		users := protected.Group("/users")
		{
			users.GET("", middleware.RequirePermission("users.view"), userH.List)
			users.GET("/:id", middleware.RequirePermission("users.view"), userH.Get)
			users.POST("", middleware.RequirePermission("users.create"), userH.Create)
			users.PUT("/:id", middleware.RequirePermission("users.edit"), userH.Update)
			users.DELETE("/:id", middleware.RequirePermission("users.delete"), userH.Delete)
			users.POST("/:id/reset-password", middleware.RequirePermission("users.edit"), userH.ResetPassword)
		}

		// Roles.
		roles := protected.Group("/roles")
		{
			roles.GET("", middleware.RequirePermission("roles.view"), roleH.List)
			roles.POST("", middleware.RequirePermission("roles.manage"), roleH.Create)
			roles.PUT("/:id", middleware.RequirePermission("roles.manage"), roleH.Update)
			roles.DELETE("/:id", middleware.RequirePermission("roles.manage"), roleH.Delete)
			roles.GET("/permissions", roleH.Permissions)
		}

		// Settings.
		settings := protected.Group("/settings")
		{
			settings.GET("", middleware.RequirePermission("settings.view"), settingsH.List)
			settings.GET("/:key", settingsH.Get)
			settings.PUT("", middleware.RequirePermission("settings.manage"), settingsH.Update)
		}

		// SEO.
		seo := protected.Group("/seo")
		{
			seo.GET("/:type/:id", seoH.GetSEOSetting)
			seo.PUT("/:type/:id", middleware.RequirePermission("seo.manage"), seoH.UpdateSEOSetting)
			seo.GET("/redirects", seoH.ListRedirects)
			seo.POST("/redirects", middleware.RequirePermission("seo.manage"), seoH.CreateRedirect)
			seo.DELETE("/redirects/:id", middleware.RequirePermission("seo.manage"), seoH.DeleteRedirect)
		}

		// Menus.
		menus := protected.Group("/menus")
		{
			menus.GET("", menuH.List)
			menus.GET("/:id", menuH.Get)
			menus.POST("", middleware.RequirePermission("menus.manage"), menuH.Create)
			menus.PUT("/:id", middleware.RequirePermission("menus.manage"), menuH.Update)
			menus.DELETE("/:id", middleware.RequirePermission("menus.manage"), menuH.Delete)
			menus.POST("/:id/items", middleware.RequirePermission("menus.manage"), menuH.AddItem)
			menus.PUT("/:id/items/:item_id", middleware.RequirePermission("menus.manage"), menuH.UpdateItem)
			menus.DELETE("/:id/items/:item_id", middleware.RequirePermission("menus.manage"), menuH.DeleteItem)
			menus.PUT("/:id/items/reorder", middleware.RequirePermission("menus.manage"), menuH.ReorderItems)
		}

		// Analytics.
		analytics := protected.Group("/analytics")
		{
			analytics.GET("/dashboard", middleware.RequirePermission("analytics.view"), analyticsH.Dashboard)
			analytics.GET("/views", middleware.RequirePermission("analytics.view"), analyticsH.ViewsOverTime)
			analytics.GET("/referrers", middleware.RequirePermission("analytics.view"), analyticsH.TopReferrers)
			analytics.GET("/devices", middleware.RequirePermission("analytics.view"), analyticsH.DeviceBreakdown)
		}

		// Plugins.
		plugins := protected.Group("/plugins")
		{
			plugins.GET("", pluginH.List)
			plugins.POST("/:id/enable", middleware.RequirePermission("plugins.manage"), pluginH.Enable)
			plugins.POST("/:id/disable", middleware.RequirePermission("plugins.manage"), pluginH.Disable)
			plugins.PUT("/:id/config", middleware.RequirePermission("plugins.manage"), pluginH.UpdateConfig)
		}

		// Themes.
		themes := protected.Group("/themes")
		{
			themes.GET("", themeH.List)
			themes.POST("/:id/activate", middleware.RequirePermission("themes.manage"), themeH.Activate)
			themes.PUT("/:id/config", middleware.RequirePermission("themes.manage"), themeH.UpdateConfig)
		}

		// System.
		system := protected.Group("/system")
		{
			system.GET("/info", systemH.Info)
			system.GET("/activity", middleware.RequirePermission("system.activity_log"), systemH.ActivityLog)

			// API Tokens (admin only).
			system.GET("/tokens", middleware.RequireAdmin(), tokenH.List)
			system.POST("/tokens", middleware.RequireAdmin(), tokenH.Create)
			system.DELETE("/tokens/:id", middleware.RequireAdmin(), tokenH.Delete)
		}

		// Content Types (admin only).
		contentTypes := protected.Group("/content-types")
		{
			contentTypes.GET("", middleware.RequireAdmin(), contentTypeH.ListTypes)
			contentTypes.GET("/:uid", middleware.RequireAdmin(), contentTypeH.GetType)
			contentTypes.POST("", middleware.RequireAdmin(), contentTypeH.CreateType)
			contentTypes.DELETE("/:uid", middleware.RequireAdmin(), contentTypeH.DeleteType)
		}

		// Content Entries (dynamic).
		content := protected.Group("/content")
		{
			content.GET("/:uid", contentTypeH.ListEntries)
			content.GET("/:uid/export", middleware.RequireAdmin(), contentTypeH.ExportEntries)
			content.POST("/:uid/import", middleware.RequireAdmin(), contentTypeH.ImportEntries)
			content.GET("/:uid/:documentId", contentTypeH.GetEntry)
			content.POST("/:uid", middleware.RequirePermission("content.create"), contentTypeH.CreateEntry)
			content.PUT("/:uid/:documentId", middleware.RequirePermission("content.update"), contentTypeH.UpdateEntry)
			content.DELETE("/:uid/:documentId", middleware.RequirePermission("content.delete"), contentTypeH.DeleteEntry)
			content.POST("/:uid/:documentId/publish", middleware.RequirePermission("content.publish"), contentTypeH.PublishEntry)
			content.POST("/:uid/:documentId/unpublish", middleware.RequirePermission("content.publish"), contentTypeH.UnpublishEntry)

			// i18n: content entry translations.
			content.GET("/:uid/:documentId/translations", contentTypeH.ListEntryTranslations)
			content.POST("/:uid/:documentId/translations", middleware.RequirePermission("content.create"), contentTypeH.CreateEntryTranslation)
		}

		// Webhooks (admin only).
		webhooks := protected.Group("/webhooks")
		{
			webhooks.GET("", middleware.RequireAdmin(), webhookH.List)
			webhooks.POST("", middleware.RequireAdmin(), webhookH.Create)
			webhooks.DELETE("/:id", middleware.RequireAdmin(), webhookH.Delete)
			webhooks.GET("/:id/logs", middleware.RequireAdmin(), webhookH.Logs)
		}

		// Search (admin: cross-status search + manual reindex).
		searchAdmin := protected.Group("/search")
		{
			searchAdmin.GET("/admin", searchH.AdminSearch)
			searchAdmin.POST("/reindex", middleware.RequireAdmin(), searchH.Reindex)
		}

		// Backup & restore (admin only).
		backupGroup := protected.Group("/admin/backup")
		{
			backupGroup.GET("", middleware.RequireAdmin(), backupH.List)
			backupGroup.POST("", middleware.RequireAdmin(), backupH.Create)
			backupGroup.GET("/:file/download", middleware.RequireAdmin(), backupH.Download)
			backupGroup.POST("/:file/restore", middleware.RequireAdmin(), backupH.Restore)
			backupGroup.DELETE("/:file", middleware.RequireAdmin(), backupH.Delete)
		}
	}

	// System health (unauthenticated).
	r.GET("/api/v1/system/health", systemH.Health)

	// Static file serving for uploads. Only relevant for the local driver
	// path; when an S3 driver is in use, files are served from object storage
	// and this route simply 404s (harmless).
	r.Static(cfg.Upload.URLPrefix, cfg.Upload.StoragePath)
	return rl
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
