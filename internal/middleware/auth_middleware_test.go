package middleware

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupAuthTestDB creates an in-memory DB with user/role/permission tables
// and seeds an active admin user (ID 1) and a banned user (ID 2).
func setupAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	adminRole := models.Role{Name: "Admin", Slug: "admin"}
	viewerRole := models.Role{
		Name: "Viewer", Slug: "viewer",
		Permissions: []models.Permission{{Name: "Read Articles", Slug: "articles.read", Module: "articles"}},
	}
	db.Create(&adminRole)
	db.Create(&viewerRole)

	db.Create(&models.User{
		Username: "admin", Email: "a@x.com", Password: "x",
		RoleID: adminRole.ID, Status: models.UserStatusActive,
	})
	db.Create(&models.User{
		Username: "banned", Email: "b@x.com", Password: "x",
		RoleID: viewerRole.ID, Status: "banned",
	})
	db.Create(&models.User{
		Username: "viewer", Email: "v@x.com", Password: "x",
		RoleID: viewerRole.ID, Status: models.UserStatusActive,
	})
	return db
}

func testJWT() *auth.JWTManager {
	return auth.NewJWTManager(config.JWTConfig{
		Secret:          "test-secret-key-at-least-16-chars",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
		Issuer:          "contentx-test",
	})
}

func tokenFor(t *testing.T, m *auth.JWTManager, userID uint) string {
	t.Helper()
	pair, err := m.GenerateTokenPair(userID, "u", "e@x.com", "role", "User")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return pair.AccessToken
}

// ---------- AuthMiddleware ----------

func TestAuthMiddleware_NoToken(t *testing.T) {
	db := setupAuthTestDB(t)
	r := setupTestRouter(AuthMiddleware(testJWT(), db, nil))
	if w := doRequest(r, http.MethodGet, "/test", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	db := setupAuthTestDB(t)
	r := setupTestRouter(AuthMiddleware(testJWT(), db, nil))
	w := doRequest(r, http.MethodGet, "/test", map[string]string{"Authorization": "Bearer garbage"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", w.Code)
	}
}

func TestAuthMiddleware_RevokedToken(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	blacklist := auth.NewBlacklist()
	tok := tokenFor(t, m, 1)
	blacklist.Revoke(tok, time.Now().Add(time.Hour))

	r := setupTestRouter(AuthMiddleware(m, db, blacklist))
	w := doRequest(r, http.MethodGet, "/test", map[string]string{"Authorization": "Bearer " + tok})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for revoked token, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	tok := tokenFor(t, m, 1)

	var gotUser *models.User
	var gotClaims *auth.Claims
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(m, db, nil))
	r.GET("/test", func(c *gin.Context) {
		gotUser = GetCurrentUser(c)
		gotClaims = GetClaims(c)
		c.Status(http.StatusOK)
	})

	w := doRequest(r, http.MethodGet, "/test", map[string]string{"Authorization": "Bearer " + tok})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUser == nil || gotUser.Username != "admin" {
		t.Fatalf("user not injected: %+v", gotUser)
	}
	if gotClaims == nil || gotClaims.UserID != 1 {
		t.Fatalf("claims not injected: %+v", gotClaims)
	}
}

func TestAuthMiddleware_UserNotFound(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	tok := tokenFor(t, m, 999)
	r := setupTestRouter(AuthMiddleware(m, db, nil))
	w := doRequest(r, http.MethodGet, "/test", map[string]string{"Authorization": "Bearer " + tok})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing user, got %d", w.Code)
	}
}

func TestAuthMiddleware_InactiveUser(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	tok := tokenFor(t, m, 2) // banned user
	r := setupTestRouter(AuthMiddleware(m, db, nil))
	w := doRequest(r, http.MethodGet, "/test", map[string]string{"Authorization": "Bearer " + tok})
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for banned user, got %d", w.Code)
	}
}

func TestAuthMiddleware_TokenFromQuery(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	tok := tokenFor(t, m, 1)
	r := setupTestRouter(AuthMiddleware(m, db, nil))
	w := doRequest(r, http.MethodGet, "/test?token="+tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with query token, got %d", w.Code)
	}
}

// ---------- OptionalAuthMiddleware ----------

func TestOptionalAuthMiddleware_NoToken(t *testing.T) {
	db := setupAuthTestDB(t)
	var gotUser *models.User
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OptionalAuthMiddleware(testJWT(), db, nil))
	r.GET("/test", func(c *gin.Context) {
		gotUser = GetCurrentUser(c)
		c.Status(http.StatusOK)
	})
	if w := doRequest(r, http.MethodGet, "/test", nil); w.Code != http.StatusOK {
		t.Fatal("optional auth should pass without token")
	}
	if gotUser != nil {
		t.Fatal("no user should be set without token")
	}
}

func TestOptionalAuthMiddleware_ValidToken(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	tok := tokenFor(t, m, 1)
	var gotUser *models.User
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OptionalAuthMiddleware(m, db, nil))
	r.GET("/test", func(c *gin.Context) {
		gotUser = GetCurrentUser(c)
		c.Status(http.StatusOK)
	})
	doRequest(r, http.MethodGet, "/test", map[string]string{"Authorization": "Bearer " + tok})
	if gotUser == nil || gotUser.Username != "admin" {
		t.Fatal("optional auth should inject user for valid token")
	}
}

// ---------- RequireRole / RequirePermission ----------

func routerWithUser(u *models.User, mw ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if u != nil {
			c.Set(ContextKeyUser, u)
		}
		c.Next()
	})
	for _, m := range mw {
		r.Use(m)
	}
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestRequireRole(t *testing.T) {
	admin := &models.User{Role: models.Role{Slug: "admin"}}
	viewer := &models.User{Role: models.Role{Slug: "viewer"}}

	// No user → 401.
	if w := doRequest(routerWithUser(nil, RequireAdmin()), http.MethodGet, "/test", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	// Admin passes RequireAdmin.
	if w := doRequest(routerWithUser(admin, RequireAdmin()), http.MethodGet, "/test", nil); w.Code != http.StatusOK {
		t.Fatalf("admin should pass RequireAdmin, got %d", w.Code)
	}
	// Viewer blocked by RequireAdmin → 403.
	if w := doRequest(routerWithUser(viewer, RequireAdmin()), http.MethodGet, "/test", nil); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	// Viewer passes RequireRole("viewer").
	if w := doRequest(routerWithUser(viewer, RequireRole("viewer")), http.MethodGet, "/test", nil); w.Code != http.StatusOK {
		t.Fatal("viewer should pass RequireRole(viewer)")
	}
	// Viewer blocked by RequireEditor; admin passes.
	if w := doRequest(routerWithUser(viewer, RequireEditor()), http.MethodGet, "/test", nil); w.Code != http.StatusForbidden {
		t.Fatal("viewer should not pass RequireEditor")
	}
	if w := doRequest(routerWithUser(admin, RequireEditor()), http.MethodGet, "/test", nil); w.Code != http.StatusOK {
		t.Fatal("admin should pass RequireEditor")
	}
}

func TestRequirePermission(t *testing.T) {
	admin := &models.User{Role: models.Role{Slug: "admin"}}
	withPerm := &models.User{Role: models.Role{
		Slug:        "viewer",
		Permissions: []models.Permission{{Slug: "articles.read"}},
	}}
	withoutPerm := &models.User{Role: models.Role{Slug: "viewer"}}

	// No user → 401.
	if w := doRequest(routerWithUser(nil, RequirePermission("articles.read")), http.MethodGet, "/test", nil); w.Code != http.StatusUnauthorized {
		t.Fatal("expected 401")
	}
	// Admin has all permissions.
	if w := doRequest(routerWithUser(admin, RequirePermission("anything.at.all")), http.MethodGet, "/test", nil); w.Code != http.StatusOK {
		t.Fatal("admin should have all permissions")
	}
	// User with the permission passes.
	if w := doRequest(routerWithUser(withPerm, RequirePermission("articles.read")), http.MethodGet, "/test", nil); w.Code != http.StatusOK {
		t.Fatal("user with permission should pass")
	}
	// User without → 403.
	if w := doRequest(routerWithUser(withoutPerm, RequirePermission("articles.read")), http.MethodGet, "/test", nil); w.Code != http.StatusForbidden {
		t.Fatal("expected 403 for missing permission")
	}
}

// ---------- Context helpers ----------

func TestGetCurrentUser_Nil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var got *models.User
	var gotClaims *auth.Claims
	r.GET("/test", func(c *gin.Context) {
		got = GetCurrentUser(c)
		gotClaims = GetClaims(c)
		c.Status(http.StatusOK)
	})
	doRequest(r, http.MethodGet, "/test", nil)
	if got != nil || gotClaims != nil {
		t.Fatal("expected nil user and claims when not authenticated")
	}
}
