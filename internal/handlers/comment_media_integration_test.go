package handlers

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/yamovo/contentx/internal/auth"
	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/services"
)

// setupCommentMediaRouter builds a test engine with comment and media routes.
func setupCommentMediaRouter(t *testing.T) (*gin.Engine, *gorm.DB, string) {
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

	user := createTestUserDB(t, db, "cmeditor", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	// Seed an article so comments have a valid article_id.
	createTestArticleDB(t, db, user.ID, "Commentable Post")

	commentH := NewCommentHandler(services.NewCommentService(db))
	mediaH := NewMediaHandler(services.NewMediaService(db, cfg.Upload))

	r := gin.New()
	// Public.
	r.GET("/api/v1/articles/:id/comments", commentH.ArticleComments)

	api := r.Group("/api/v1")
	api.Use(mockAuthMiddleware(jwtMgr, db))
	{
		comments := api.Group("/comments")
		{
			comments.GET("", commentH.List)
			comments.GET("/:id", commentH.Get)
			comments.POST("", commentH.Create)
			comments.PUT("/:id", commentH.Update)
			comments.POST("/:id/approve", commentH.Approve)
			comments.POST("/:id/spam", commentH.Spam)
			comments.POST("/:id/trash", commentH.Trash)
			comments.POST("/bulk", commentH.BulkAction)
			comments.GET("/stats", commentH.Stats)
		}
		media := api.Group("/media")
		{
			media.GET("", mediaH.List)
			media.GET("/folders", mediaH.Folders)
			media.GET("/stats", mediaH.Stats)
			media.GET("/:id", mediaH.Get)
			media.PUT("/:id", mediaH.Update)
			media.DELETE("/:id", mediaH.Delete)
			media.POST("/bulk-delete", mediaH.BulkDelete)
		}
	}

	return r, db, token
}

// ---------- Comment ----------

func TestComment_Handlers(t *testing.T) {
	r, _, token := setupCommentMediaRouter(t)

	// Create.
	w := doJSONRequest(r, http.MethodPost, "/api/v1/comments", token,
		`{"article_id":1,"content":"Great article!","author_name":"Reader"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create comment: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Create invalid → 400.
	w = doJSONRequest(r, http.MethodPost, "/api/v1/comments", token, `{"content":"no article"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("create invalid: expected 400, got %d", w.Code)
	}

	// List.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/comments?page=1&page_size=10", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	// Get.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/comments/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Get invalid / missing.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/comments/xyz", token, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("get invalid: expected 400, got %d", w.Code)
	}
	w = doJSONRequest(r, http.MethodGet, "/api/v1/comments/9999", token, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("get missing: expected 404, got %d", w.Code)
	}

	// Update.
	w = doJSONRequest(r, http.MethodPut, "/api/v1/comments/1", token, `{"content":"Updated text"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Update empty content → 400.
	w = doJSONRequest(r, http.MethodPut, "/api/v1/comments/1", token, `{"content":""}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("update empty: expected 400, got %d", w.Code)
	}

	// Approve / spam / trash.
	for _, action := range []string{"approve", "spam", "trash"} {
		w = doJSONRequest(r, http.MethodPost, "/api/v1/comments/1/"+action, token, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d: %s", action, w.Code, w.Body.String())
		}
	}

	// Bulk action.
	w = doJSONRequest(r, http.MethodPost, "/api/v1/comments/bulk", token,
		`{"comment_ids":[1],"action":"approve"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bulk: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Stats.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/comments/stats", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("stats: expected 200, got %d", w.Code)
	}

	// Article comments (public).
	w = doJSONRequest(r, http.MethodGet, "/api/v1/articles/1/comments", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("article comments: expected 200, got %d", w.Code)
	}
}

// ---------- Media ----------

func seedMedia(t *testing.T, db *gorm.DB, uploaderID uint) {
	t.Helper()
	m := models.Media{
		Filename:     "test-image.png",
		OriginalName: "Test Image.png",
		FilePath:     "uploads/2024/test-image.png",
		URL:          "/uploads/2024/test-image.png",
		MimeType:     "image/png",
		FileSize:     2048,
		Folder:       "/images",
		UploaderID:   uploaderID,
	}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("seed media: %v", err)
	}
}

func TestMedia_Handlers(t *testing.T) {
	r, db, token := setupCommentMediaRouter(t)
	seedMedia(t, db, 1)

	// List.
	w := doJSONRequest(r, http.MethodGet, "/api/v1/media?page=1&page_size=10", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Folders.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/media/folders", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("folders: expected 200, got %d", w.Code)
	}

	// Stats.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/media/stats", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("stats: expected 200, got %d", w.Code)
	}

	// Get.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/media/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Get invalid / missing.
	w = doJSONRequest(r, http.MethodGet, "/api/v1/media/abc", token, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("get invalid: expected 400, got %d", w.Code)
	}
	w = doJSONRequest(r, http.MethodGet, "/api/v1/media/9999", token, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("get missing: expected 404, got %d", w.Code)
	}

	// Update.
	w = doJSONRequest(r, http.MethodPut, "/api/v1/media/1", token,
		`{"alt":"Alt text","title":"New Title"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Bulk delete (second seeded record would be better, but one is fine).
	seedMedia(t, db, 1)
	w = doJSONRequest(r, http.MethodPost, "/api/v1/media/bulk-delete", token, `{"ids":[2]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bulk delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Delete.
	w = doJSONRequest(r, http.MethodDelete, "/api/v1/media/1", token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
