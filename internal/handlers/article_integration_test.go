package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/services"
)

// setupArticleTestRouter builds a test router with article routes.
func setupArticleTestRouter(t *testing.T) (*gin.Engine, *gorm.DB, *auth.JWTManager) {
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
	cfg.JWT.RefreshTokenTTL = 86400000000000
	cfg.JWT.Issuer = "contentx-test"
	cfg.Server.BaseURL = "http://localhost:8080"

	jwtMgr := auth.NewJWTManager(cfg.JWT)

	articleSvc := services.NewArticleService(db, cfg.Server.BaseURL)

	r := gin.New()
	api := r.Group("/api/v1")
	{
		api.GET("/articles/slug/:slug", NewArticleHandler(articleSvc).GetBySlug)
	}
	protected := api.Group("")
	protected.Use(mockAuthMiddleware(jwtMgr, db))
	{
		protected.GET("/articles", NewArticleHandler(articleSvc).List)
		protected.GET("/articles/:id", NewArticleHandler(articleSvc).Get)
		protected.POST("/articles", NewArticleHandler(articleSvc).Create)
		protected.PUT("/articles/:id", NewArticleHandler(articleSvc).Update)
		protected.DELETE("/articles/:id", NewArticleHandler(articleSvc).Delete)
	}

	return r, db, jwtMgr
}

// createTestArticleDB creates a published article with revision.
func createTestArticleDB(t *testing.T, db *gorm.DB, authorID uint, title string) *models.Article {
	t.Helper()

	now := time.Now()
	article := models.Article{
		Title:       title,
		Slug:        strings.ReplaceAll(strings.ToLower(title), " ", "-"),
		Content:     "<p>Test content for " + title + "</p>",
		AuthorID:    authorID,
		Status:      models.StatusPublished,
		PublishedAt: &now,
	}
	db.Create(&article)

	revision := models.Revision{
		ArticleID: article.ID,
		Title:     article.Title,
		Content:   article.Content,
		EditorID:  authorID,
		Version:   1,
		Note:      "Initial",
	}
	db.Create(&revision)

	return &article
}

// ─── Article List Tests ──────────────────────────────────────────────────────

func TestArticle_List_Unauthenticated(t *testing.T) {
	r, _, _ := setupArticleTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// List requires auth
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestArticle_List_Empty(t *testing.T) {
	r, db, jwtMgr := setupArticleTestRouter(t)
	user := createTestUserDB(t, db, "author", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("empty list should return 200, got %d", w.Code)
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	// Meta is nested inside data as models.ListResponse
	data := resp.Data.(map[string]interface{})
	total := int64(data["total"].(float64))
	if total != 0 {
		t.Fatalf("expected 0 articles, got %d", total)
	}
}

func TestArticle_List_WithData(t *testing.T) {
	r, db, jwtMgr := setupArticleTestRouter(t)
	user := createTestUserDB(t, db, "author", "admin")
	createTestArticleDB(t, db, user.ID, "Hello World")
	createTestArticleDB(t, db, user.ID, "Second Post")
	token := generateTestJWT(t, jwtMgr, *user)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	total := int64(data["total"].(float64))
	if total != 2 {
		t.Fatalf("expected 2 articles, got %d", total)
	}
}

// ─── Article Get Tests ───────────────────────────────────────────────────────

func TestArticle_Get_NotFound(t *testing.T) {
	r, db, jwtMgr := setupArticleTestRouter(t)
	user := createTestUserDB(t, db, "author", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/99999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing article, got %d", w.Code)
	}
}

func TestArticle_Get_Success(t *testing.T) {
	r, db, jwtMgr := setupArticleTestRouter(t)
	user := createTestUserDB(t, db, "author", "admin")
	article := createTestArticleDB(t, db, user.ID, "Test Article")
	token := generateTestJWT(t, jwtMgr, *user)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/"+formatUint(article.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestArticle_GetBySlug_Success(t *testing.T) {
	r, db, _ := setupArticleTestRouter(t)
	user := createTestUserDB(t, db, "author", "admin")
	createTestArticleDB(t, db, user.ID, "Slug Test Article")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/slug/slug-test-article", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for public slug endpoint, got %d: %s", w.Code, w.Body.String())
	}
}

func TestArticle_GetBySlug_NotFound(t *testing.T) {
	r, _, _ := setupArticleTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/slug/nonexistent-slug", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent slug, got %d", w.Code)
	}
}

// ─── Article Create Tests ────────────────────────────────────────────────────

func TestArticle_Create_Success(t *testing.T) {
	r, db, jwtMgr := setupArticleTestRouter(t)
	user := createTestUserDB(t, db, "author", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	body := `{"title":"New Article","content":"<p>Content here</p>","status":"draft"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/articles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	if data["title"] != "New Article" {
		t.Fatalf("expected title 'New Article', got '%v'", data["title"])
	}
}

func TestArticle_Create_EmptyTitle(t *testing.T) {
	r, db, jwtMgr := setupArticleTestRouter(t)
	user := createTestUserDB(t, db, "author", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	body := `{"title":"","content":"<p>No title</p>"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/articles", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty title should return 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── Article Update Tests ────────────────────────────────────────────────────

func TestArticle_Update_Success(t *testing.T) {
	r, db, jwtMgr := setupArticleTestRouter(t)
	user := createTestUserDB(t, db, "author", "admin")
	article := createTestArticleDB(t, db, user.ID, "Original Title")
	token := generateTestJWT(t, jwtMgr, *user)

	body := `{"title":"Updated Title"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/articles/"+formatUint(article.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	if data["title"] != "Updated Title" {
		t.Fatalf("expected 'Updated Title', got '%v'", data["title"])
	}
}

func TestArticle_Update_NotFound(t *testing.T) {
	r, db, jwtMgr := setupArticleTestRouter(t)
	user := createTestUserDB(t, db, "author", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	body := `{"title":"Ghost"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/articles/99999", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ─── Article Delete Tests ────────────────────────────────────────────────────

func TestArticle_Delete_Success(t *testing.T) {
	r, db, jwtMgr := setupArticleTestRouter(t)
	user := createTestUserDB(t, db, "author", "admin")
	article := createTestArticleDB(t, db, user.ID, "To Delete")
	token := generateTestJWT(t, jwtMgr, *user)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/articles/"+formatUint(article.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify it's actually gone.
	var count int64
	db.Model(&models.Article{}).Where("id = ?", article.ID).Count(&count)
	if count != 0 {
		t.Fatal("article should be deleted")
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func formatUint(id uint) string {
	var buf [20]byte
	i := len(buf)
	for id >= 10 {
		i--
		buf[i] = byte(id%10) + '0'
		id /= 10
	}
	i--
	buf[i] = byte(id) + '0'
	return string(buf[i:])
}
