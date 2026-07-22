package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/services"
)

// doJSONRequest performs an HTTP request with optional JSON body and bearer token.
func doJSONRequest(r *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)
	return w
}

// setupTaxonomyRouter builds a test engine with category and tag routes.
func setupTaxonomyRouter(t *testing.T) (*gin.Engine, *gorm.DB, string) {
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
	jwtMgr := auth.NewJWTManager(cfg.JWT)

	user := createTestUserDB(t, db, "taxadmin", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	categoryH := NewCategoryHandler(services.NewCategoryService(db))
	tagH := NewTagHandler(services.NewTagService(db))

	r := gin.New()
	api := r.Group("/api/v1")
	api.Use(mockAuthMiddleware(jwtMgr, db))
	{
		categories := api.Group("/categories")
		{
			categories.GET("", categoryH.List)
			categories.GET("/:id", categoryH.Get)
			categories.POST("", categoryH.Create)
			categories.PUT("/:id", categoryH.Update)
			categories.DELETE("/:id", categoryH.Delete)
			categories.PUT("/reorder", categoryH.Reorder)
		}
		tags := api.Group("/tags")
		{
			tags.GET("", tagH.List)
			tags.GET("/:id", tagH.Get)
			tags.POST("", tagH.Create)
			tags.PUT("/:id", tagH.Update)
			tags.DELETE("/:id", tagH.Delete)
			tags.POST("/merge", tagH.Merge)
		}
	}

	return r, db, token
}

// ---------- Category ----------

func TestCategory_CRUD(t *testing.T) {
	r, _, token := setupTaxonomyRouter(t)

	// Create.
	w := doJSONRequest(r, http.MethodPost, "/api/v1/categories", token,
		`{"name":"Tech","description":"Technology"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Create with missing name → 400.
	w = doJSONRequest(r, http.MethodPost, "/api/v1/categories", token, `{"description":"no name"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("create without name: expected 400, got %d", w.Code)
	}

	// List.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/categories", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	// Get.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/categories/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Get invalid ID → 400.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/categories/abc", token, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("get invalid id: expected 400, got %d", w.Code)
	}

	// Get missing → 404.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/categories/9999", token, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("get missing: expected 404, got %d", w.Code)
	}

	// Update.
	w = doJSONRequest(r, http.MethodPut, "/api/v1/categories/1", token,
		`{"name":"Tech Updated","description":"Updated"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Reorder.
	w = doJSONRequest(r, http.MethodPut, "/api/v1/categories/reorder", token,
		`{"items":[{"id":1,"sort_order":5}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("reorder: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Delete.
	w = doJSONRequest(r, http.MethodDelete, "/api/v1/categories/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", w.Code)
	}

	// Verify deleted.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/categories/1", token, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("after delete: expected 404, got %d", w.Code)
	}
}

// ---------- Tag ----------

func TestTag_CRUD(t *testing.T) {
	r, _, token := setupTaxonomyRouter(t)

	// Create.
	w := doJSONRequest(r, http.MethodPost, "/api/v1/tags", token, `{"name":"golang","color":"#00ADD8"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Create without name → 400.
	w = doJSONRequest(r, http.MethodPost, "/api/v1/tags", token, `{"color":"#fff"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("create without name: expected 400, got %d", w.Code)
	}

	// List with query params.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/tags?sort=count&limit=10&search=gol", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	// Get.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/tags/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Get invalid / missing.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/tags/xyz", token, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("get invalid: expected 400, got %d", w.Code)
	}
	w = doJSONRequest(r, http.MethodGet, "/api/v1/tags/9999", token, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("get missing: expected 404, got %d", w.Code)
	}

	// Update.
	w = doJSONRequest(r, http.MethodPut, "/api/v1/tags/1", token, `{"name":"go","color":"#007d9c"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Delete.
	w = doJSONRequest(r, http.MethodDelete, "/api/v1/tags/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", w.Code)
	}
}

func TestTag_Merge(t *testing.T) {
	r, _, token := setupTaxonomyRouter(t)

	// Create three tags.
	for i, name := range []string{"tag-a", "tag-b", "tag-c"} {
		w := doJSONRequest(r, http.MethodPost, "/api/v1/tags", token, fmt.Sprintf(`{"name":%q}`, name))
		if w.Code != http.StatusCreated {
			t.Fatalf("create tag %d: got %d", i, w.Code)
		}
	}

	// Merge tag-a + tag-b into tag-c.
	w := doJSONRequest(r, http.MethodPost, "/api/v1/tags/merge", token,
		`{"source_ids":[1,2],"target_id":3,"delete_old":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("merge: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Merge with missing fields → 400.
	w = doJSONRequest(r, http.MethodPost, "/api/v1/tags/merge", token, `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("merge invalid: expected 400, got %d", w.Code)
	}
}
