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
	if err := db.AutoMigrate(&models.User{}, &models.Role{}, &models.Permission{}, &models.Tenant{}, &models.TenantMembership{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	adminRole := models.Role{Name: "Admin", Slug: "admin"}
	viewerRole := models.Role{
		Name: "Viewer", Slug: "viewer",
		Permissions: []models.Permission{{Name: "Read Articles", Slug: "articles.read", Module: "articles"}},
	}
	db.Create(&adminRole)
	db.Create(&viewerRole)

	// Create default tenant for membership validation.
	defaultTenant := models.Tenant{Name: "Default", Slug: "default", Status: models.TenantStatusActive}
	defaultTenant.ID = models.DefaultTenantID
	db.Create(&defaultTenant)

	// Create tenant 5 for non-admin tenant-scoped token tests.
	tenant5 := models.Tenant{Name: "Tenant 5", Slug: "tenant-5", Status: models.TenantStatusActive}
	tenant5.ID = 5
	db.Create(&tenant5)

	// Create tenant 9 for admin override tests.
	tenant9 := models.Tenant{Name: "Tenant 9", Slug: "tenant-9", Status: models.TenantStatusActive}
	tenant9.ID = 9
	db.Create(&tenant9)

	// Create tenant 7 for JWT-bound platform-admin tests.
	tenant7 := models.Tenant{Name: "Tenant 7", Slug: "tenant-7", Status: models.TenantStatusActive}
	tenant7.ID = 7
	db.Create(&tenant7)

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

	// Create tenant memberships for non-admin users.
	var viewerUser models.User
	db.Where("username = ?", "viewer").First(&viewerUser)
	db.Create(&models.TenantMembership{
		TenantID: models.DefaultTenantID, UserID: viewerUser.ID, RoleSlug: models.TenantRoleEditor,
	})
	db.Create(&models.TenantMembership{
		TenantID: 5, UserID: viewerUser.ID, RoleSlug: models.TenantRoleEditor,
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

func TestAuthMiddleware_TokenFromQueryRejected(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	tok := tokenFor(t, m, 1)
	r := setupTestRouter(AuthMiddleware(m, db, nil))
	// Query parameter tokens must be rejected to prevent token leakage
	// into access logs, browser history, and Referer headers.
	w := doRequest(r, http.MethodGet, "/test?token="+tok, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for query token (security: must not accept ?token=), got %d", w.Code)
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
	return routerWithUserAndTenantRole(u, models.TenantRoleAdmin, mw...)
}

func routerWithUserAndTenantRole(u *models.User, tenantRole string, mw ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if u != nil {
			c.Set(ContextKeyUser, u)
			if tenantRole != "" {
				c.Set(ContextKeyTenantRole, tenantRole)
			}
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
	// Admin has all registered tenant permissions, but route typos fail closed.
	if w := doRequest(routerWithUser(admin, RequirePermission("articles.read")), http.MethodGet, "/test", nil); w.Code != http.StatusOK {
		t.Fatal("admin should have registered tenant permissions")
	}
	if w := doRequest(routerWithUser(admin, RequirePermission("anything.at.all")), http.MethodGet, "/test", nil); w.Code != http.StatusForbidden {
		t.Fatal("unknown permission slugs must fail closed")
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

func TestRequireTenantPermission_UsesMembershipRoleAsCeiling(t *testing.T) {
	editor := &models.User{Role: models.Role{
		Slug: "editor",
		Permissions: []models.Permission{
			{Slug: "articles.read"},
			{Slug: "articles.publish"},
		},
	}}

	if w := doRequest(routerWithUserAndTenantRole(editor, models.TenantRoleMember, RequirePermission("articles.read")), http.MethodGet, "/test", nil); w.Code != http.StatusOK {
		t.Fatalf("member should retain author-level read access, got %d", w.Code)
	}
	if w := doRequest(routerWithUserAndTenantRole(editor, models.TenantRoleMember, RequirePermission("articles.publish")), http.MethodGet, "/test", nil); w.Code != http.StatusForbidden {
		t.Fatalf("member role must cap a global editor's publish permission, got %d", w.Code)
	}
	if w := doRequest(routerWithUserAndTenantRole(editor, models.TenantRoleEditor, RequirePermission("articles.publish")), http.MethodGet, "/test", nil); w.Code != http.StatusOK {
		t.Fatalf("tenant editor should retain globally granted publish permission, got %d", w.Code)
	}
}

func TestRequirePlatformPermission_IgnoresTenantAdminRole(t *testing.T) {
	tenantAdmin := &models.User{Role: models.Role{Slug: "editor", Permissions: []models.Permission{{Slug: "users.read"}}}}
	if w := doRequest(routerWithUserAndTenantRole(tenantAdmin, models.TenantRoleAdmin, RequirePlatformPermission("users.read")), http.MethodGet, "/test", nil); w.Code != http.StatusOK {
		t.Fatalf("an explicit global platform grant should pass, got %d", w.Code)
	}
	withoutGlobalGrant := &models.User{Role: models.Role{Slug: "editor"}}
	if w := doRequest(routerWithUserAndTenantRole(withoutGlobalGrant, models.TenantRoleAdmin, RequirePlatformPermission("users.read")), http.MethodGet, "/test", nil); w.Code != http.StatusForbidden {
		t.Fatalf("tenant admin must not create a platform grant, got %d", w.Code)
	}
	if w := doRequest(routerWithUserAndTenantRole(tenantAdmin, models.TenantRoleAdmin, RequirePlatformPermission("articles.read")), http.MethodGet, "/test", nil); w.Code != http.StatusForbidden {
		t.Fatalf("tenant permissions must not be accepted by platform middleware, got %d", w.Code)
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

// ---------- userCache (LRU + TTL) ----------

func TestUserCache_Miss(t *testing.T) {
	c := newUserCache(8, time.Second)
	if _, ok := c.get(999); ok {
		t.Fatal("expected cache miss for unknown user")
	}
}

func TestUserCache_PutGet(t *testing.T) {
	c := newUserCache(8, time.Second)
	u := &models.User{BaseModel: models.BaseModel{ID: 7}, Username: "cached"}
	c.put(u)
	got, ok := c.get(7)
	if !ok {
		t.Fatal("expected cache hit after put")
	}
	if got.Username != "cached" {
		t.Fatalf("wrong user returned: %+v", got)
	}
}

func TestUserCache_TTLExpiry(t *testing.T) {
	c := newUserCache(8, 20*time.Millisecond)
	c.put(&models.User{BaseModel: models.BaseModel{ID: 1}})
	if _, ok := c.get(1); !ok {
		t.Fatal("expected hit before TTL expiry")
	}
	time.Sleep(30 * time.Millisecond)
	if _, ok := c.get(1); ok {
		t.Fatal("expected miss after TTL expiry")
	}
}

func TestUserCache_LRUEviction(t *testing.T) {
	c := newUserCache(2, time.Second)
	c.put(&models.User{BaseModel: models.BaseModel{ID: 1}})
	c.put(&models.User{BaseModel: models.BaseModel{ID: 2}})
	// Access ID 1 to make ID 2 the LRU candidate.
	if _, ok := c.get(1); !ok {
		t.Fatal("expected hit for ID 1")
	}
	// Inserting ID 3 should evict the least-recently-used (ID 2).
	c.put(&models.User{BaseModel: models.BaseModel{ID: 3}})
	if _, ok := c.get(2); ok {
		t.Fatal("ID 2 should have been evicted")
	}
	if _, ok := c.get(1); !ok {
		t.Fatal("ID 1 should still be cached (recently used)")
	}
	if _, ok := c.get(3); !ok {
		t.Fatal("ID 3 should be cached")
	}
}

func TestAuthMiddleware_RepeatedRequestReloadsUser(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	tok := tokenFor(t, m, 1)

	r := setupTestRouter(AuthMiddleware(m, db, nil))
	// Both requests load the current authorization state and remain valid.
	if w := doRequest(r, http.MethodGet, "/test", map[string]string{"Authorization": "Bearer " + tok}); w.Code != http.StatusOK {
		t.Fatalf("first request should pass, got %d", w.Code)
	}
	if w := doRequest(r, http.MethodGet, "/test", map[string]string{"Authorization": "Bearer " + tok}); w.Code != http.StatusOK {
		t.Fatalf("repeated request should pass, got %d", w.Code)
	}
}

func TestAuthMiddleware_DisabledUserRejectedImmediately(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	tok := tokenFor(t, m, 1)
	r := setupTestRouter(AuthMiddleware(m, db, nil))
	headers := map[string]string{"Authorization": "Bearer " + tok}

	if w := doRequest(r, http.MethodGet, "/test", headers); w.Code != http.StatusOK {
		t.Fatalf("first request should pass, got %d", w.Code)
	}
	if err := db.Model(&models.User{}).Where("id = ?", 1).
		Update("status", "banned").Error; err != nil {
		t.Fatalf("disable cached user: %v", err)
	}
	if w := doRequest(r, http.MethodGet, "/test", headers); w.Code != http.StatusForbidden {
		t.Fatalf("disabled user should be rejected on the next request, got %d", w.Code)
	}
}

func TestAuthMiddleware_PermissionRevocationTakesEffectImmediately(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	pair, err := m.GenerateTokenPairWithTenant(3, 5, "v", "v@x.com", "viewer", "Viewer")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	r := setupTestRouter(
		AuthMiddleware(m, db, nil),
		TenantGuard(db),
		RequireTenantPermission("articles.read"),
	)
	headers := map[string]string{"Authorization": "Bearer " + pair.AccessToken}
	if w := doRequest(r, http.MethodGet, "/test", headers); w.Code != http.StatusOK {
		t.Fatalf("initial permission check = %d, want 200", w.Code)
	}

	var viewerRole models.Role
	if err := db.Where("slug = ?", "viewer").First(&viewerRole).Error; err != nil {
		t.Fatalf("load viewer role: %v", err)
	}
	if err := db.Model(&viewerRole).Association("Permissions").Clear(); err != nil {
		t.Fatalf("revoke viewer permissions: %v", err)
	}
	if w := doRequest(r, http.MethodGet, "/test", headers); w.Code != http.StatusForbidden {
		t.Fatalf("revoked permission should be rejected on the next request, got %d", w.Code)
	}
}

func TestTenantGuard_MembershipRequiredOnlyForTenantRoutes(t *testing.T) {
	db := setupAuthTestDB(t)
	var viewerRole models.Role
	if err := db.Where("slug = ?", "viewer").First(&viewerRole).Error; err != nil {
		t.Fatalf("load viewer role: %v", err)
	}
	user := models.User{
		Username: "platform-only", Email: "platform-only@example.com", Password: "x",
		RoleID: viewerRole.ID, Status: models.UserStatusActive,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create membership-less user: %v", err)
	}

	m := testJWT()
	pair, err := m.GenerateTokenPairWithTenant(
		user.ID, models.DefaultTenantID, user.Username, user.Email, viewerRole.Slug, user.DisplayName,
	)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	headers := map[string]string{"Authorization": "Bearer " + pair.AccessToken}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	self := r.Group("/self", AuthMiddleware(m, db, nil))
	self.GET("", func(c *gin.Context) { c.Status(http.StatusOK) })
	tenant := r.Group("/tenant", AuthMiddleware(m, db, nil), TenantGuard(db))
	tenant.GET("", func(c *gin.Context) { c.Status(http.StatusOK) })

	if w := doRequest(r, http.MethodGet, "/self", headers); w.Code != http.StatusOK {
		t.Fatalf("self-service route without membership = %d, want 200", w.Code)
	}
	if w := doRequest(r, http.MethodGet, "/tenant", headers); w.Code != http.StatusForbidden {
		t.Fatalf("tenant route without membership = %d, want 403", w.Code)
	}
}

func TestGetCurrentTenant_DefaultsToDefaultTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	if got := GetCurrentTenant(c); got != models.DefaultTenantID {
		t.Fatalf("GetCurrentTenant() = %d, want %d", got, models.DefaultTenantID)
	}
}

func TestAuthMiddleware_TenantFromClaims(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	pair, err := m.GenerateTokenPairWithTenant(1, 7, "u", "e@x.com", "admin", "Admin")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	var gotTenant uint
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(m, db, nil), TenantGuard(db))
	r.GET("/test", func(c *gin.Context) {
		gotTenant = GetCurrentTenant(c)
		c.Status(http.StatusOK)
	})

	w := doRequest(r, http.MethodGet, "/test", map[string]string{"Authorization": "Bearer " + pair.AccessToken})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotTenant != 7 {
		t.Fatalf("GetCurrentTenant() = %d, want 7", gotTenant)
	}
}

func TestAuthMiddleware_TenantOverrideHeaderForAdmin(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	pair, err := m.GenerateTokenPairWithTenant(1, 7, "u", "e@x.com", "admin", "Admin")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	var gotTenant uint
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(m, db, nil), TenantGuard(db))
	r.GET("/test", func(c *gin.Context) {
		gotTenant = GetCurrentTenant(c)
		c.Status(http.StatusOK)
	})

	w := doRequest(r, http.MethodGet, "/test", map[string]string{
		"Authorization": "Bearer " + pair.AccessToken,
		"X-Tenant-ID":   "9",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotTenant != 9 {
		t.Fatalf("admin override: GetCurrentTenant() = %d, want 9", gotTenant)
	}
}

func TestAuthMiddleware_AdminOverrideRequiresExistingActiveTenant(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	pair, _ := m.GenerateTokenPairWithTenant(1, 7, "u", "e@x.com", "admin", "Admin")
	r := setupTestRouter(AuthMiddleware(m, db, nil), TenantGuard(db))

	for name, override := range map[string]string{
		"invalid":   "not-a-number",
		"missing":   "40404",
		"suspended": "12",
	} {
		t.Run(name, func(t *testing.T) {
			if name == "suspended" {
				tenant := models.Tenant{Name: "Override Suspended", Slug: "override-suspended", Status: models.TenantStatusSuspended}
				tenant.ID = 12
				if err := db.Create(&tenant).Error; err != nil {
					t.Fatalf("create suspended tenant: %v", err)
				}
			}
			w := doRequest(r, http.MethodGet, "/test", map[string]string{
				"Authorization": "Bearer " + pair.AccessToken,
				"X-Tenant-ID":   override,
			})
			if w.Code != http.StatusForbidden {
				t.Fatalf("override %q = %d, want 403", override, w.Code)
			}
		})
	}
}

func TestAuthMiddleware_TenantOverrideRejectedForNonAdmin(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	// Viewer (ID 3) with a tenant-bound token; header must be ignored.
	pair, err := m.GenerateTokenPairWithTenant(3, 5, "v", "v@x.com", "viewer", "Viewer")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	var gotTenant uint
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware(m, db, nil), TenantGuard(db))
	r.GET("/test", func(c *gin.Context) {
		gotTenant = GetCurrentTenant(c)
		c.Status(http.StatusOK)
	})

	w := doRequest(r, http.MethodGet, "/test", map[string]string{
		"Authorization": "Bearer " + pair.AccessToken,
		"X-Tenant-ID":   "9",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin override must be rejected, got %d", w.Code)
	}
	if gotTenant != 0 {
		t.Fatalf("request should not reach the handler, got tenant %d", gotTenant)
	}
}

func TestAuthMiddleware_RefreshTokenCannotBeBearerAccessToken(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()
	pair, err := m.GenerateTokenPairWithTenant(1, 7, "u", "e@x.com", "admin", "Admin")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	r := setupTestRouter(AuthMiddleware(m, db, nil))
	w := doRequest(r, http.MethodGet, "/test", map[string]string{"Authorization": "Bearer " + pair.RefreshToken})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("refresh token used as Bearer access token: got %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_RejectsSuspendedClaimTenant(t *testing.T) {
	db := setupAuthTestDB(t)
	suspended := models.Tenant{Name: "Suspended", Slug: "suspended", Status: models.TenantStatusSuspended}
	suspended.ID = 11
	if err := db.Create(&suspended).Error; err != nil {
		t.Fatalf("create suspended tenant: %v", err)
	}
	m := testJWT()
	pair, _ := m.GenerateTokenPairWithTenant(1, suspended.ID, "u", "e@x.com", "admin", "Admin")
	w := doRequest(setupTestRouter(AuthMiddleware(m, db, nil), TenantGuard(db)), http.MethodGet, "/test", map[string]string{"Authorization": "Bearer " + pair.AccessToken})
	if w.Code != http.StatusForbidden {
		t.Fatalf("suspended JWT-bound tenant should be rejected, got %d", w.Code)
	}
}

func TestAuthMiddleware_RejectsMissingOrInvalidMembership(t *testing.T) {
	db := setupAuthTestDB(t)
	m := testJWT()

	missing, _ := m.GenerateTokenPairWithTenant(3, 9, "v", "v@x.com", "viewer", "Viewer")
	w := doRequest(setupTestRouter(AuthMiddleware(m, db, nil), TenantGuard(db)), http.MethodGet, "/test", map[string]string{"Authorization": "Bearer " + missing.AccessToken})
	if w.Code != http.StatusForbidden {
		t.Fatalf("missing tenant membership should be rejected, got %d", w.Code)
	}

	if err := db.Model(&models.TenantMembership{}).
		Where("tenant_id = ? AND user_id = ?", 5, 3).
		Update("role_slug", "reviewer").Error; err != nil {
		t.Fatalf("corrupt membership role: %v", err)
	}
	invalidRole, _ := m.GenerateTokenPairWithTenant(3, 5, "v", "v@x.com", "viewer", "Viewer")
	w = doRequest(setupTestRouter(AuthMiddleware(m, db, nil), TenantGuard(db)), http.MethodGet, "/test", map[string]string{"Authorization": "Bearer " + invalidRole.AccessToken})
	if w.Code != http.StatusForbidden {
		t.Fatalf("unknown membership role should fail closed, got %d", w.Code)
	}
}

func TestAuthMiddleware_RevalidatesTenantStateOnCachedUser(t *testing.T) {
	t.Run("membership removed", func(t *testing.T) {
		db := setupAuthTestDB(t)
		m := testJWT()
		pair, _ := m.GenerateTokenPairWithTenant(3, 5, "v", "v@x.com", "viewer", "Viewer")
		r := setupTestRouter(AuthMiddleware(m, db, nil), TenantGuard(db))
		headers := map[string]string{"Authorization": "Bearer " + pair.AccessToken}

		if w := doRequest(r, http.MethodGet, "/test", headers); w.Code != http.StatusOK {
			t.Fatalf("initial request = %d, want 200", w.Code)
		}
		if err := db.Where("tenant_id = ? AND user_id = ?", 5, 3).
			Delete(&models.TenantMembership{}).Error; err != nil {
			t.Fatalf("remove membership: %v", err)
		}
		if w := doRequest(r, http.MethodGet, "/test", headers); w.Code != http.StatusForbidden {
			t.Fatalf("cached user retained removed membership: got %d, want 403", w.Code)
		}
	})

	t.Run("tenant suspended", func(t *testing.T) {
		db := setupAuthTestDB(t)
		m := testJWT()
		pair, _ := m.GenerateTokenPairWithTenant(3, 5, "v", "v@x.com", "viewer", "Viewer")
		r := setupTestRouter(AuthMiddleware(m, db, nil), TenantGuard(db))
		headers := map[string]string{"Authorization": "Bearer " + pair.AccessToken}

		if w := doRequest(r, http.MethodGet, "/test", headers); w.Code != http.StatusOK {
			t.Fatalf("initial request = %d, want 200", w.Code)
		}
		if err := db.Model(&models.Tenant{}).Where("id = ?", 5).
			Update("status", models.TenantStatusSuspended).Error; err != nil {
			t.Fatalf("suspend tenant: %v", err)
		}
		if w := doRequest(r, http.MethodGet, "/test", headers); w.Code != http.StatusForbidden {
			t.Fatalf("cached user retained suspended tenant: got %d, want 403", w.Code)
		}
	})
}

func TestAuthMiddleware_TenantDenialAuditRetainsAuthenticatedActor(t *testing.T) {
	db := setupAuthTestDB(t)
	if err := db.AutoMigrate(&models.ActivityLog{}); err != nil {
		t.Fatalf("migrate activity log: %v", err)
	}
	m := testJWT()
	pair, _ := m.GenerateTokenPairWithTenant(3, 9, "v", "v@x.com", "viewer", "Viewer")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ActivityLogger(db))
	r.Use(AuthMiddleware(m, db, nil), TenantGuard(db))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := doRequest(r, http.MethodGet, "/test", map[string]string{"Authorization": "Bearer " + pair.AccessToken})
	if w.Code != http.StatusForbidden {
		t.Fatalf("tenant denial = %d, want 403", w.Code)
	}

	var log models.ActivityLog
	if err := db.First(&log).Error; err != nil {
		t.Fatalf("read denial audit: %v", err)
	}
	if log.Action != "request.denied" || log.UserID == nil || *log.UserID != 3 {
		t.Fatalf("tenant denial was not attributed to authenticated actor: %+v", log)
	}
}
