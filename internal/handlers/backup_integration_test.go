package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/backup"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/middleware"
)

// setupBackupRouter builds a test engine with the backup routes registered
// behind both mockAuthMiddleware (loads the user) and the real
// middleware.RequireAdmin() guard, so the 403 path can be exercised.
func setupBackupRouter(t *testing.T) (*gin.Engine, *gorm.DB, string, string, *backup.Manager) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.Seed(db)

	cfg := &config.Config{}
	cfg.JWT.Secret = "test-jwt-secret-for-integration-testing-32chars"
	cfg.JWT.AccessTokenTTL = 3600000000000
	cfg.JWT.Issuer = "contentx-test"
	cfg.Backup.Dir = t.TempDir()
	cfg.Database.Driver = "sqlite"
	cfg.Database.Name = ":memory:"
	jwtMgr := auth.NewJWTManager(cfg.JWT)

	adminUser := createTestUserDB(t, db, "backupadmin", "admin")
	adminToken := generateTestJWT(t, jwtMgr, *adminUser)
	authorUser := createTestUserDB(t, db, "backupauthor", "author")
	authorToken := generateTestJWT(t, jwtMgr, *authorUser)

	mgr := backup.NewManager(cfg.Backup, cfg.Database, "", db)
	backupH := NewBackupHandler(mgr)

	r := gin.New()
	api := r.Group("/api/v1")
	api.Use(mockAuthMiddleware(jwtMgr, db))
	{
		bg := api.Group("/admin/backup")
		bg.Use(middleware.RequireAdmin())
		{
			bg.GET("", backupH.List)
			bg.POST("", backupH.Create)
			bg.GET("/:file/download", backupH.Download)
			bg.POST("/:file/restore", backupH.Restore)
			bg.DELETE("/:file", backupH.Delete)
		}
	}
	return r, db, adminToken, authorToken, mgr
}

func TestBackupHandler_NonAdmin_Forbidden(t *testing.T) {
	r, _, _, authorToken, _ := setupBackupRouter(t)

	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/api/v1/admin/backup"},
		{http.MethodPost, "/api/v1/admin/backup"},
		{http.MethodGet, "/api/v1/admin/backup/whatever/download"},
		{http.MethodPost, "/api/v1/admin/backup/whatever/restore"},
		{http.MethodDelete, "/api/v1/admin/backup/whatever"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		req.Header.Set("Authorization", "Bearer "+authorToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s %s: expected 403 for author, got %d (body=%s)", c.method, c.path, w.Code, w.Body.String())
		}
	}
}

func TestBackupHandler_Admin_CreateListDownload(t *testing.T) {
	r, _, adminToken, _, _ := setupBackupRouter(t)

	// 1. Create db backup.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/backup?type=db", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create: expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	// Extract the db filename from the response body ({"path":"db-..."}).
	body := w.Body.String()
	idx := strings.Index(body, `"path":"`)
	if idx < 0 {
		t.Fatalf("create response missing path: %s", body)
	}
	nameStart := idx + len(`"path":"`)
	nameEnd := strings.Index(body[nameStart:], `"`)
	if nameEnd < 0 {
		t.Fatalf("create response malformed: %s", body)
	}
	filename := body[nameStart : nameStart+nameEnd]

	// 2. List — should include the created backup.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/backup", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), filename) {
		t.Fatalf("list should contain %s: %s", filename, w.Body.String())
	}

	// 3. Download the file.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/backup/"+filename+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("download: expected 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if w.Body.Len() == 0 {
		t.Fatal("download body should be non-empty")
	}
}

func TestBackupHandler_Download_PathTraversal_Rejected(t *testing.T) {
	r, _, adminToken, _, _ := setupBackupRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/backup/..%2f..%2fetc%2fpasswd/download", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Gin decodes the path param; "../etc/passwd" → filepath.Base != input → 400.
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Fatalf("expected 400 or 404 for traversal, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestBackupHandler_Download_NotFound(t *testing.T) {
	r, _, adminToken, _, _ := setupBackupRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/backup/nonexistent.db/download", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing file, got %d", w.Code)
	}
}

func TestBackupHandler_ConcurrentBackup_Returns409(t *testing.T) {
	r, _, adminToken, _, mgr := setupBackupRouter(t)

	// Hold the manager lock to simulate an in-progress backup.
	mgr.LockForTest()
	defer mgr.UnlockForTest()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/backup?type=db", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for concurrent backup, got %d (body=%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "BACKUP_IN_PROGRESS") {
		t.Fatalf("expected BACKUP_IN_PROGRESS code in body: %s", w.Body.String())
	}
}

func TestBackupHandler_Restore_NotFound(t *testing.T) {
	r, _, adminToken, _, _ := setupBackupRouter(t)

	// Use a bare filename (passes the traversal guard) but a file that
	// doesn't exist on disk — the manager should surface a 500 error.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/backup/missing.db/restore", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for restore of missing file, got %d (body=%s)", w.Code, w.Body.String())
	}
}
