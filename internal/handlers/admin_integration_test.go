package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/services"
)

// setupAdminRouter builds a test engine with settings, system, token, webhook,
// user, role, menu, analytics, plugin, theme and seo routes registered.
func setupAdminRouter(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.Seed(db)

	cfg := &config.Config{}
	cfg.JWT.Secret = "test-jwt-secret-for-integration-testing-32chars"
	cfg.JWT.AccessTokenTTL = 3600000000000
	cfg.JWT.Issuer = "contentx-test"
	cfg.Server.BaseURL = "http://localhost:8080"
	jwtMgr := auth.NewJWTManager(cfg.JWT)

	user := createTestUserDB(t, db, "adminuser", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	settingsH := NewSettingsHandler(services.NewSettingsService(db))
	seoH := NewSEOHandler(services.NewSEOService(db, cfg.Server.BaseURL))
	menuH := NewMenuHandler(services.NewMenuService(db))
	analyticsH := NewAnalyticsHandler(services.NewAnalyticsService(db))
	pluginH := NewPluginHandler(services.NewPluginService(db))
	themeH := NewThemeHandler(services.NewThemeService(db))
	systemH := NewSystemHandler(services.NewSystemService(db))
	tokenH := NewTokenHandler(services.NewTokenService(db))
	webhookH := NewWebhookHandler(services.NewWebhookService(db))
	userH := NewUserHandler(services.NewUserService(db))
	roleH := NewRoleHandler(services.NewRoleService(db))

	r := gin.New()
	// Public routes.
	r.GET("/api/v1/system/health", systemH.Health)
	r.GET("/api/v1/settings/public", settingsH.PublicSettings)
	r.GET("/api/v1/seo/sitemap", seoH.Sitemap)
	r.GET("/api/v1/seo/robots.txt", seoH.RobotsTxt)
	r.POST("/api/v1/analytics/record", analyticsH.RecordView)

	api := r.Group("/api/v1")
	api.Use(mockAuthMiddleware(jwtMgr, db))
	{
		settings := api.Group("/settings")
		{
			settings.GET("", settingsH.List)
			settings.GET("/:key", settingsH.Get)
			settings.PUT("", settingsH.Update)
		}
		seo := api.Group("/seo")
		{
			seo.GET("/redirects", seoH.ListRedirects)
			seo.POST("/redirects", seoH.CreateRedirect)
			seo.DELETE("/redirects/:id", seoH.DeleteRedirect)
			seo.GET("/:type/:id", seoH.GetSEOSetting)
			seo.PUT("/:type/:id", seoH.UpdateSEOSetting)
		}
		menus := api.Group("/menus")
		{
			menus.GET("", menuH.List)
			menus.GET("/:id", menuH.Get)
			menus.POST("", menuH.Create)
			menus.PUT("/:id", menuH.Update)
			menus.DELETE("/:id", menuH.Delete)
		}
		analytics := api.Group("/analytics")
		{
			analytics.GET("/dashboard", analyticsH.Dashboard)
			analytics.GET("/views", analyticsH.ViewsOverTime)
			analytics.GET("/referrers", analyticsH.TopReferrers)
			analytics.GET("/devices", analyticsH.DeviceBreakdown)
		}
		plugins := api.Group("/plugins")
		{
			plugins.GET("", pluginH.List)
			plugins.POST("/:id/enable", pluginH.Enable)
			plugins.POST("/:id/disable", pluginH.Disable)
			plugins.PUT("/:id/config", pluginH.UpdateConfig)
		}
		themes := api.Group("/themes")
		{
			themes.GET("", themeH.List)
			themes.POST("/:id/activate", themeH.Activate)
			themes.PUT("/:id/config", themeH.UpdateConfig)
		}
		system := api.Group("/system")
		{
			system.GET("/info", systemH.Info)
			system.GET("/activity", systemH.ActivityLog)
			system.GET("/tokens", tokenH.List)
			system.POST("/tokens", tokenH.Create)
			system.DELETE("/tokens/:id", tokenH.Delete)
		}
		webhooks := api.Group("/webhooks")
		{
			webhooks.GET("", webhookH.List)
			webhooks.POST("", webhookH.Create)
			webhooks.DELETE("/:id", webhookH.Delete)
			webhooks.GET("/:id/logs", webhookH.Logs)
		}
		users := api.Group("/users")
		{
			users.GET("", userH.List)
			users.GET("/:id", userH.Get)
			users.POST("", userH.Create)
			users.PUT("/:id", userH.Update)
			users.DELETE("/:id", userH.Delete)
			users.POST("/:id/reset-password", userH.ResetPassword)
		}
		roles := api.Group("/roles")
		{
			roles.GET("", roleH.List)
			roles.POST("", roleH.Create)
			roles.PUT("/:id", roleH.Update)
			roles.DELETE("/:id", roleH.Delete)
			roles.GET("/permissions", roleH.Permissions)
		}
	}

	return r, db, token
}

// ---------- Settings ----------

func TestSettings_Handlers(t *testing.T) {
	r, _, token := setupAdminRouter(t)

	// Update settings.
	w := doJSONRequest(r, http.MethodPut, "/api/v1/settings", token,
		`{"site_title":"My CMS","site_desc":"A test site"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update settings: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// List all.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/settings", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list settings: expected 200, got %d", w.Code)
	}

	// Get by key.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/settings/site_title", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get setting: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Public settings (no auth).
	w = doJSONRequest(r, http.MethodGet, "/api/v1/settings/public", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("public settings: expected 200, got %d", w.Code)
	}
}

// ---------- System ----------

func TestSystem_Handlers(t *testing.T) {
	r, _, token := setupAdminRouter(t)

	w := doJSONRequest(r, http.MethodGet, "/api/v1/system/info", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("system info: expected 200, got %d", w.Code)
	}

	w = doJSONRequest(r, http.MethodGet, "/api/v1/system/health", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("health: expected 200, got %d", w.Code)
	}

	w = doJSONRequest(r, http.MethodGet, "/api/v1/system/activity", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("activity: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------- API Tokens ----------

func TestToken_Handlers(t *testing.T) {
	r, _, token := setupAdminRouter(t)

	// Create.
	w := doJSONRequest(r, http.MethodPost, "/api/v1/system/tokens", token,
		`{"name":"CI Token","permissions":["articles.read"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create token: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Create invalid → 400.
	w = doJSONRequest(r, http.MethodPost, "/api/v1/system/tokens", token, `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("create invalid token: expected 400, got %d", w.Code)
	}

	// List.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/system/tokens", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list tokens: expected 200, got %d", w.Code)
	}

	// Delete.
	w = doJSONRequest(r, http.MethodDelete, "/api/v1/system/tokens/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("delete token: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Delete again → 404.
	w = doJSONRequest(r, http.MethodDelete, "/api/v1/system/tokens/1", token, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("delete missing token: expected 404, got %d", w.Code)
	}
}

// ---------- Webhooks ----------

func TestWebhook_Handlers(t *testing.T) {
	r, _, token := setupAdminRouter(t)

	// Create.
	w := doJSONRequest(r, http.MethodPost, "/api/v1/webhooks", token,
		`{"name":"Deploy","url":"https://hooks.example.com/deploy","events":["entry.publish"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create webhook: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Create invalid → 400.
	w = doJSONRequest(r, http.MethodPost, "/api/v1/webhooks", token, `{"name":"x"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("create invalid webhook: expected 400, got %d", w.Code)
	}

	// List.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/webhooks", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list webhooks: expected 200, got %d", w.Code)
	}

	// Logs.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/webhooks/1/logs", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("webhook logs: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Delete.
	w = doJSONRequest(r, http.MethodDelete, "/api/v1/webhooks/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("delete webhook: expected 200, got %d", w.Code)
	}
}

// ---------- Users & Roles ----------

func TestUser_Handlers(t *testing.T) {
	r, db, token := setupAdminRouter(t)

	// Get admin role ID.
	var roleID uint
	db.Raw("SELECT id FROM roles WHERE slug = 'admin'").Scan(&roleID)

	// Create user.
	w := doJSONRequest(r, http.MethodPost, "/api/v1/users", token,
		fmt.Sprintf(`{"username":"editor1","email":"editor1@test.com","password":"EditorPass1","role_id":%d}`, roleID))
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("create user: expected 201/200, got %d: %s", w.Code, w.Body.String())
	}

	// Create duplicate → 4xx.
	w = doJSONRequest(r, http.MethodPost, "/api/v1/users", token,
		fmt.Sprintf(`{"username":"editor1","email":"editor1@test.com","password":"EditorPass1","role_id":%d}`, roleID))
	if w.Code == http.StatusCreated || w.Code == http.StatusOK {
		t.Fatalf("duplicate user should fail, got %d", w.Code)
	}

	// List.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/users", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list users: expected 200, got %d", w.Code)
	}

	// Get.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/users/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get user: expected 200, got %d", w.Code)
	}

	// Update.
	w = doJSONRequest(r, http.MethodPut, "/api/v1/users/1", token,
		`{"display_name":"Admin Renamed"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update user: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Reset password.
	w = doJSONRequest(r, http.MethodPost, "/api/v1/users/1/reset-password", token,
		`{"new_password":"NewPass1234"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("reset password: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRole_Handlers(t *testing.T) {
	r, _, token := setupAdminRouter(t)

	w := doJSONRequest(r, http.MethodGet, "/api/v1/roles", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list roles: expected 200, got %d", w.Code)
	}

	w = doJSONRequest(r, http.MethodGet, "/api/v1/roles/permissions", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list permissions: expected 200, got %d", w.Code)
	}

	// Create role.
	w = doJSONRequest(r, http.MethodPost, "/api/v1/roles", token,
		`{"name":"Moderator","slug":"moderator","description":"Can moderate"}`)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("create role: expected 201/200, got %d: %s", w.Code, w.Body.String())
	}
}

// ---------- SEO / Menus / Analytics / Plugins / Themes ----------

func TestSEO_Handlers(t *testing.T) {
	r, _, token := setupAdminRouter(t)

	// Sitemap (public).
	w := doJSONRequest(r, http.MethodGet, "/api/v1/seo/sitemap", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("sitemap: expected 200, got %d", w.Code)
	}

	// Robots (public).
	w = doJSONRequest(r, http.MethodGet, "/api/v1/seo/robots.txt", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("robots: expected 200, got %d", w.Code)
	}

	// Update SEO setting for an article entity.
	w = doJSONRequest(r, http.MethodPut, "/api/v1/seo/article/1", token,
		`{"title":"SEO Title","desc":"SEO description"}`)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("update seo: expected 200/201, got %d: %s", w.Code, w.Body.String())
	}

	// Get SEO setting.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/seo/article/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get seo: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Create redirect.
	w = doJSONRequest(r, http.MethodPost, "/api/v1/seo/redirects", token,
		`{"from_path":"/old-page","to_path":"/new-page","status_code":301}`)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("create redirect: expected 201/200, got %d: %s", w.Code, w.Body.String())
	}

	// List redirects.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/seo/redirects", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list redirects: expected 200, got %d", w.Code)
	}

	// Delete redirect.
	w = doJSONRequest(r, http.MethodDelete, "/api/v1/seo/redirects/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("delete redirect: expected 200, got %d", w.Code)
	}
}

func TestMenu_Handlers(t *testing.T) {
	r, _, token := setupAdminRouter(t)

	// Create.
	w := doJSONRequest(r, http.MethodPost, "/api/v1/menus", token,
		`{"name":"Main Nav","slug":"main","locations":"header"}`)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("create menu: expected 201/200, got %d: %s", w.Code, w.Body.String())
	}

	// List.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/menus", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list menus: expected 200, got %d", w.Code)
	}

	// Get.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/menus/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get menu: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Update.
	w = doJSONRequest(r, http.MethodPut, "/api/v1/menus/1", token, `{"name":"Main Nav v2"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update menu: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Delete.
	w = doJSONRequest(r, http.MethodDelete, "/api/v1/menus/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("delete menu: expected 200, got %d", w.Code)
	}
}

func TestAnalytics_Handlers(t *testing.T) {
	r, _, token := setupAdminRouter(t)

	// Record a view (public).
	w := doJSONRequest(r, http.MethodPost, "/api/v1/analytics/record", "",
		`{"path":"/blog/hello-world","referrer":"https://google.com","user_agent":"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0"}`)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated && w.Code != http.StatusNoContent {
		t.Fatalf("record view: expected 2xx, got %d: %s", w.Code, w.Body.String())
	}

	// Dashboard.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/analytics/dashboard", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("dashboard: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Views over time.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/analytics/views?days=7", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("views: expected 200, got %d", w.Code)
	}

	// Top referrers.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/analytics/referrers", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("referrers: expected 200, got %d", w.Code)
	}

	// Device breakdown.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/analytics/devices", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("devices: expected 200, got %d", w.Code)
	}
}

func TestPluginTheme_Handlers(t *testing.T) {
	r, db, token := setupAdminRouter(t)

	// Seed a plugin and theme.
	db.Exec("INSERT INTO plugins (name, slug, version, description, is_enabled, config, created_at, updated_at) VALUES ('Test Plugin', 'test-plugin', '1.0.0', 'desc', 0, '{}', datetime('now'), datetime('now'))")
	db.Exec("INSERT INTO theme_configs (name, slug, is_active, config, created_at, updated_at) VALUES ('Default Theme', 'default', 0, '{}', datetime('now'), datetime('now'))")

	// Plugins list.
	w := doJSONRequest(r, http.MethodGet, "/api/v1/plugins", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list plugins: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Enable / disable plugin.
	w = doJSONRequest(r, http.MethodPost, "/api/v1/plugins/1/enable", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("enable plugin: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	w = doJSONRequest(r, http.MethodPost, "/api/v1/plugins/1/disable", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("disable plugin: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Update plugin config.
	w = doJSONRequest(r, http.MethodPut, "/api/v1/plugins/1/config", token, `{"api_key":"abc123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("plugin config: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Themes list.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/themes", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list themes: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Activate theme.
	w = doJSONRequest(r, http.MethodPost, "/api/v1/themes/1/activate", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("activate theme: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Update theme config.
	w = doJSONRequest(r, http.MethodPut, "/api/v1/themes/1/config", token, `{"primary_color":"#ff0000"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("theme config: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
