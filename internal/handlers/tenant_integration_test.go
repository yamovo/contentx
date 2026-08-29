package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/middleware"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/permissions"
	"github.com/yamovo/contentx/internal/repository"
	"github.com/yamovo/contentx/internal/services"
)

// ─── Platform tenant administration API (RFC-001 PR-5) ─────────────────────

// setupTenantAdminRouter seeds the identity DB and wires the tenant admin
// routes exactly like production (the RequirePlatformPermission middleware is
// replaced by a bare operator context; permission gating itself is covered by
// the middleware unit tests).
func setupTenantAdminRouter(t *testing.T, operatorRole string) (*gin.Engine, *gorm.DB, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	prepareHandlerTestDB(t, db)
	// Production order: migrations create the default tenant before seeding,
	// so the seeded admin membership attaches to a real tenant. Reproduce that
	// here, otherwise the seeded admin's phantom membership lands inside the
	// first tenant created by the test.
	if err := db.Create(&models.Tenant{Name: "Default", Slug: "default", Status: models.TenantStatusActive}).Error; err != nil {
		t.Fatalf("seed default tenant: %v", err)
	}
	database.Seed(db)

	cfg := &config.Config{}
	cfg.JWT.Secret = "test-jwt-secret-for-integration-testing-32chars"
	cfg.JWT.AccessTokenTTL = 3600000000000
	cfg.JWT.Issuer = "contentx-test"
	jwtMgr := auth.NewJWTManager(cfg.JWT)

	operator := createTestUserDB(t, db, "operator-"+operatorRole, operatorRole)
	pair, err := jwtMgr.GenerateTokenPair(operator.ID, operator.Username, operator.Email, operatorRole, operator.Username)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}

	svc := services.NewTenantService(repository.NewTenantRepository(db))
	svc.SetAuditLogger(services.NewAuditLogger(repository.NewActivityLogRepository(db)))
	tenantH := NewTenantHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		// Mirror the production middleware contract: authenticated user with
		// preloaded role.
		var user models.User
		if err := db.Preload("Role").First(&user, operator.ID).Error; err == nil {
			c.Set("currentUser", &user)
			c.Set("claims", auth.Claims{UserID: operator.ID})
		}
		c.Next()
	})
	api := r.Group("/api/v1")
	protected := api.Group("")
	{
		tenantGroup := protected.Group("/admin/tenants")
		tenantGroup.GET("", middleware.RequirePlatformPermission(permissions.TenantsRead), tenantH.List)
		tenantGroup.GET("/:id", middleware.RequirePlatformPermission(permissions.TenantsRead), tenantH.Get)
		tenantGroup.GET("/:id/members", middleware.RequirePlatformPermission(permissions.TenantsRead), tenantH.ListMembers)
		tenantGroup.POST("", middleware.RequirePlatformPermission(permissions.TenantsManage), tenantH.Create)
		tenantGroup.PUT("/:id", middleware.RequirePlatformPermission(permissions.TenantsManage), tenantH.Update)
		tenantGroup.POST("/:id/members", middleware.RequirePlatformPermission(permissions.TenantsManage), tenantH.AddMember)
		tenantGroup.PUT("/:id/members/:userId", middleware.RequirePlatformPermission(permissions.TenantsManage), tenantH.UpdateMemberRole)
		tenantGroup.DELETE("/:id/members/:userId", middleware.RequirePlatformPermission(permissions.TenantsManage), tenantH.RemoveMember)
	}
	return r, db, pair.AccessToken
}

func doTenantJSON(r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewBuffer(b)
	} else {
		reader = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeTenant(t *testing.T, w *httptest.ResponseRecorder) models.Tenant {
	t.Helper()
	var resp struct {
		Data models.Tenant `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal tenant: %v (%s)", err, w.Body.String())
	}
	return resp.Data
}

func TestTenantAdmin_FullLifecycle(t *testing.T) {
	r, db, token := setupTenantAdminRouter(t, "admin")

	// Create.
	w := doTenantJSON(r, http.MethodPost, "/api/v1/admin/tenants", token, map[string]any{
		"name": "Acme", "slug": "acme", "max_users": 25,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	created := decodeTenant(t, w)
	if created.ID == 0 || created.Status != models.TenantStatusActive {
		t.Fatalf("unexpected created tenant: %+v", created)
	}

	// Duplicate slug conflicts.
	if w := doTenantJSON(r, http.MethodPost, "/api/v1/admin/tenants", token, map[string]any{
		"name": "Acme 2", "slug": "acme",
	}); w.Code != http.StatusConflict {
		t.Fatalf("duplicate slug should 409, got %d", w.Code)
	}

	// Update status.
	if w := doTenantJSON(r, http.MethodPut, "/api/v1/admin/tenants/"+strconv.FormatUint(uint64(created.ID), 10), token, map[string]any{
		"status": models.TenantStatusSuspended,
	}); w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}

	// Members: add, duplicate rejected, unknown user, invalid role.
	second := createTestUserDB(t, db, "tenant-member-1", "editor")
	if w := doTenantJSON(r, http.MethodPost, "/api/v1/admin/tenants/"+strconv.FormatUint(uint64(created.ID), 10)+"/members", token, map[string]any{
		"user_id": second.ID, "role_slug": "admin",
	}); w.Code != http.StatusOK {
		t.Fatalf("add member: %d %s", w.Code, w.Body.String())
	}
	if w := doTenantJSON(r, http.MethodPost, "/api/v1/admin/tenants/"+strconv.FormatUint(uint64(created.ID), 10)+"/members", token, map[string]any{
		"user_id": second.ID, "role_slug": "member",
	}); w.Code != http.StatusConflict {
		t.Fatalf("duplicate member should 409, got %d", w.Code)
	}
	if w := doTenantJSON(r, http.MethodPost, "/api/v1/admin/tenants/"+strconv.FormatUint(uint64(created.ID), 10)+"/members", token, map[string]any{
		"user_id": second.ID + 1000, "role_slug": "member",
	}); w.Code != http.StatusNotFound {
		t.Fatalf("unknown user should 404, got %d", w.Code)
	}
	if w := doTenantJSON(r, http.MethodPost, "/api/v1/admin/tenants/"+strconv.FormatUint(uint64(created.ID), 10)+"/members", token, map[string]any{
		"user_id": second.ID, "role_slug": "superadmin",
	}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid role should 400, got %d", w.Code)
	}

	// Second admin added so the first can then be removed.
	third := createTestUserDB(t, db, "tenant-member-2", "author")
	if w := doTenantJSON(r, http.MethodPost, "/api/v1/admin/tenants/"+strconv.FormatUint(uint64(created.ID), 10)+"/members", token, map[string]any{
		"user_id": third.ID, "role_slug": "admin",
	}); w.Code != http.StatusOK {
		t.Fatalf("add second admin: %d %s", w.Code, w.Body.String())
	}
	if w := doTenantJSON(r, http.MethodDelete, "/api/v1/admin/tenants/"+strconv.FormatUint(uint64(created.ID), 10)+"/members/"+strconv.FormatUint(uint64(second.ID), 10), token, nil); w.Code != http.StatusOK {
		t.Fatalf("remove admin #1: %d %s", w.Code, w.Body.String())
	}
	// Last-admin protection now applies to admin #2.
	if w := doTenantJSON(r, http.MethodDelete, "/api/v1/admin/tenants/"+strconv.FormatUint(uint64(created.ID), 10)+"/members/"+strconv.FormatUint(uint64(third.ID), 10), token, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("removing the last admin must 400, got %d", w.Code)
	}

	// Every administration step landed in the audit trail with provenance.
	var auditCount int64
	db.Model(&models.ActivityLog{}).
		Where("action LIKE ? AND source = ? AND actor_type = ? AND outcome = ?",
			"tenant.%", services.SourceREST, services.ActorUser, services.OutcomeSuccess).
		Count(&auditCount)
	if auditCount < 4 {
		t.Fatalf("expected at least 4 tenant audit events (create/update/member_add/member_remove), got %d", auditCount)
	}
}

func TestTenantAdmin_ValidationErrors(t *testing.T) {
	r, _, token := setupTenantAdminRouter(t, "admin")

	for name, body := range map[string]any{
		"missing name": map[string]any{"slug": "x"},
		"empty slug":   map[string]any{"name": "X", "slug": ""},
		"bad slug":     map[string]any{"name": "X", "slug": "Bad Slug!"},
	} {
		if w := doTenantJSON(r, http.MethodPost, "/api/v1/admin/tenants", token, body); w.Code != http.StatusBadRequest {
			t.Errorf("%s should 400, got %d", name, w.Code)
		}
	}

	if w := doTenantJSON(r, http.MethodGet, "/api/v1/admin/tenants/999999", token, nil); w.Code != http.StatusNotFound {
		t.Fatalf("unknown tenant should 404, got %d", w.Code)
	}
}

func TestTenantAdmin_PlatformPermissionGate(t *testing.T) {
	r, _, token := setupTenantAdminRouter(t, "author")

	// A non-platform role must be rejected by the platform permission guard
	// before the handler runs — tenant roles can never reach the identity plane.
	if w := doTenantJSON(r, http.MethodGet, "/api/v1/admin/tenants", token, nil); w.Code != http.StatusForbidden {
		t.Fatalf("author listing tenants must 403, got %d", w.Code)
	}
	if w := doTenantJSON(r, http.MethodPost, "/api/v1/admin/tenants", token, map[string]any{
		"name": "Nope", "slug": "nope",
	}); w.Code != http.StatusForbidden {
		t.Fatalf("author creating a tenant must 403, got %d", w.Code)
	}
}
