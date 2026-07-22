package handlers

import (
	"encoding/json"
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
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/services"
)

// setupContentTestRouter builds a test router with content type routes.
func setupContentTestRouter(t *testing.T) (*gin.Engine, *gorm.DB, *auth.JWTManager) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// db = db.Debug() // uncomment for debugging
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

	ctSvc := services.NewContentTypeService(db)

	r := gin.New()
	protected := r.Group("/api/v1")
	protected.Use(mockAuthMiddleware(jwtMgr, db))
	{
		protected.GET("/content-types", NewContentTypeHandler(ctSvc).ListTypes)
		protected.GET("/content-types/:uid", NewContentTypeHandler(ctSvc).GetType)
		protected.POST("/content-types", NewContentTypeHandler(ctSvc).CreateType)
		protected.DELETE("/content-types/:uid", NewContentTypeHandler(ctSvc).DeleteType)

		protected.GET("/content/:uid", NewContentTypeHandler(ctSvc).ListEntries)
		protected.GET("/content/:uid/:documentId", NewContentTypeHandler(ctSvc).GetEntry)
		protected.POST("/content/:uid", NewContentTypeHandler(ctSvc).CreateEntry)
		protected.PUT("/content/:uid/:documentId", NewContentTypeHandler(ctSvc).UpdateEntry)
		protected.DELETE("/content/:uid/:documentId", NewContentTypeHandler(ctSvc).DeleteEntry)
	}

	return r, db, jwtMgr
}

// ─── Content Type CRUD Tests ─────────────────────────────────────────────────

func TestContentType_List_Empty(t *testing.T) {
	r, db, jwtMgr := setupContentTestRouter(t)
	user := createTestUserDB(t, db, "ctadmin", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/content-types", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	types := resp.Data.([]interface{})
	if len(types) != 0 {
		t.Fatalf("expected 0 content types, got %d", len(types))
	}
}

func TestContentType_Create_Success(t *testing.T) {
	r, db, jwtMgr := setupContentTestRouter(t)
	user := createTestUserDB(t, db, "ctadmin", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	body := `{
		"uid": "product",
		"name": "产品",
		"description": "Product catalog",
		"fields": [
			{"name": "title", "label": "标题", "field_type": "text", "required": true},
			{"name": "price", "label": "价格", "field_type": "float", "min_value": 0},
			{"name": "status", "label": "状态", "field_type": "enum", "options": ["在售", "下架"]}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/content-types", strings.NewReader(body))
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
	if data["uid"] != "product" {
		t.Fatalf("expected uid 'product', got '%v'", data["uid"])
	}
}

func TestContentType_Create_DuplicateUID(t *testing.T) {
	r, db, jwtMgr := setupContentTestRouter(t)
	user := createTestUserDB(t, db, "ctadmin", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	// Create first type.
	body1 := `{"uid":"event","name":"活动","fields":[{"name":"title","label":"标题","field_type":"text","required":true}]}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/content-types", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer "+token)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first create should succeed, got %d: %s", w1.Code, w1.Body.String())
	}

	// Duplicate.
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/content-types", strings.NewReader(body1))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestContentType_Create_InvalidUID(t *testing.T) {
	r, db, jwtMgr := setupContentTestRouter(t)
	user := createTestUserDB(t, db, "ctadmin", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	body := `{"uid":"Invalid UID!","name":"Bad","fields":[{"name":"title","label":"Title","field_type":"text","required":true}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/content-types", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for bad UID, got %d: %s", w.Code, w.Body.String())
	}
}

func TestContentType_Get_NotFound(t *testing.T) {
	r, db, jwtMgr := setupContentTestRouter(t)
	user := createTestUserDB(t, db, "ctadmin", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/content-types/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestContentType_Get_Success(t *testing.T) {
	r, db, jwtMgr := setupContentTestRouter(t)
	user := createTestUserDB(t, db, "ctadmin", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	createBody := `{"uid":"faq","name":"FAQ","description":"Questions","fields":[{"name":"question","label":"问题","field_type":"text","required":true}]}`
	reqC := httptest.NewRequest(http.MethodPost, "/api/v1/content-types", strings.NewReader(createBody))
	reqC.Header.Set("Content-Type", "application/json")
	reqC.Header.Set("Authorization", "Bearer "+token)
	wC := httptest.NewRecorder()
	r.ServeHTTP(wC, reqC)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/content-types/faq", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	if data["uid"] != "faq" {
		t.Fatalf("expected faq, got '%v'", data["uid"])
	}
}

func TestContentType_Delete_Success(t *testing.T) {
	r, db, jwtMgr := setupContentTestRouter(t)
	user := createTestUserDB(t, db, "ctadmin", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	createBody := `{"uid":"todelete","name":"Temp","fields":[{"name":"x","label":"X","field_type":"text","required":true}]}`
	reqC := httptest.NewRequest(http.MethodPost, "/api/v1/content-types", strings.NewReader(createBody))
	reqC.Header.Set("Content-Type", "application/json")
	reqC.Header.Set("Authorization", "Bearer "+token)
	wC := httptest.NewRecorder()
	r.ServeHTTP(wC, reqC)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/content-types/todelete", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify it's gone.
	var count int64
	db.Model(&models.ContentType{}).Where("uid = ?", "todelete").Count(&count)
	if count != 0 {
		t.Fatal("content type should be deleted")
	}
}

// ─── Content Entry CRUD Tests ────────────────────────────────────────────────

func TestContentEntry_Create_Success(t *testing.T) {
	r, db, jwtMgr := setupContentTestRouter(t)
	user := createTestUserDB(t, db, "ctadmin", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	// First create a content type.
	ctBody := `{"uid":"note","name":"笔记","fields":[{"name":"title","label":"标题","field_type":"text","required":true},{"name":"body","label":"内容","field_type":"rich_text"}]}`
	reqCT := httptest.NewRequest(http.MethodPost, "/api/v1/content-types", strings.NewReader(ctBody))
	reqCT.Header.Set("Content-Type", "application/json")
	reqCT.Header.Set("Authorization", "Bearer "+token)
	wCT := httptest.NewRecorder()
	r.ServeHTTP(wCT, reqCT)
	if wCT.Code != http.StatusCreated {
		t.Fatalf("failed to create content type: %d %s", wCT.Code, wCT.Body.String())
	}

	body := `{"data": {"title":"My Note","body":"<p>Rich content</p>"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/content/note", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestContentEntry_Get_NotFound(t *testing.T) {
	r, db, jwtMgr := setupContentTestRouter(t)
	user := createTestUserDB(t, db, "ctadmin", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	// Create type first.
	ctBody := `{"uid":"page","name":"页面","fields":[{"name":"title","label":"标题","field_type":"text","required":true}]}`
	reqCT := httptest.NewRequest(http.MethodPost, "/api/v1/content-types", strings.NewReader(ctBody))
	reqCT.Header.Set("Content-Type", "application/json")
	reqCT.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(httptest.NewRecorder(), reqCT)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/content/page/nonexistent-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing entry, got %d: %s", w.Code, w.Body.String())
	}
}

func TestContentEntry_CRUD_FullCycle(t *testing.T) {
	r, db, jwtMgr := setupContentTestRouter(t)
	user := createTestUserDB(t, db, "ctadmin", "admin")
	token := generateTestJWT(t, jwtMgr, *user)

	// 1. Create content type.
	ctBody := `{"uid":"book","name":"图书","fields":[{"name":"title","label":"书名","field_type":"text","required":true},{"name":"author","label":"作者","field_type":"text"}]}`
	createCT(t, r, token, ctBody)

	// 2. Create entry.
	createBody := `{"data": {"title":"Go Programming","author":"John Doe"}}`
	docID := createEntry(t, r, token, "book", createBody)

	// 3. Get entry.
	getEntry(t, r, token, "book", docID, "Go Programming")

	// 4. List entries.
	listEntries(t, r, token, "book", 1)

	// 5. Update entry.
	updateBody := `{"data": {"title":"Advanced Go","author":"Jane Smith"}}`
	updateEntry(t, r, token, "book", docID, updateBody)

	// 6. Verify update.
	getEntry(t, r, token, "book", docID, "Advanced Go")

	// 7. Delete entry.
	deleteEntry(t, r, token, "book", docID)

	// 8. Verify empty list.
	listEntries(t, r, token, "book", 0)

	// 9. Delete content type.
	deleteCT(t, r, token, "book")

	// 10. Verify content type gone.
	var count int64
	db.Model(&models.ContentType{}).Where("uid = ?", "book").Count(&count)
	if count != 0 {
		t.Fatal("content type should be deleted")
	}
	t.Logf("full CRUD cycle passed for content type 'book'")
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func createCT(t *testing.T, r *gin.Engine, token, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/content-types", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("createCT: expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func createEntry(t *testing.T, r *gin.Engine, token, uid, body string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/content/"+uid, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("createEntry: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	return data["document_id"].(string)
}

func getEntry(t *testing.T, r *gin.Engine, token, uid, docID, expectedTitle string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/content/"+uid+"/"+docID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("getEntry: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	entry := resp.Data.(map[string]interface{})
	entryDataRaw, ok := entry["data"]
	if !ok || entryDataRaw == nil {
		t.Fatalf("getEntry: entry.data is nil or missing: %s", w.Body.String())
	}
	entryData := entryDataRaw.(map[string]interface{})
	if entryData["title"] != expectedTitle {
		t.Fatalf("getEntry: expected title '%s', got '%v'", expectedTitle, entryData["title"])
	}
}

func listEntries(t *testing.T, r *gin.Engine, token, uid string, expectedCount int64) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/content/"+uid, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("listEntries: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp APIResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	// content entries list wraps totals inside data (models.ListResponse)
	data := resp.Data.(map[string]interface{})
	total := int64(data["total"].(float64))
	if total != expectedCount {
		t.Fatalf("listEntries: expected %d entries, got %d", expectedCount, total)
	}
}

func updateEntry(t *testing.T, r *gin.Engine, token, uid, docID, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/content/"+uid+"/"+docID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("updateEntry: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func deleteEntry(t *testing.T, r *gin.Engine, token, uid, docID string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/content/"+uid+"/"+docID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("deleteEntry: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func deleteCT(t *testing.T, r *gin.Engine, token, uid string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/content-types/"+uid, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("deleteCT: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
