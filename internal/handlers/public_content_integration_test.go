package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"github.com/yamovo/contentx/internal/services"
)

// ─── RFC-002 public content delivery test matrix ───────────────────────────

// setupPublicContent seeds the default tenant (1) and tenant 2 with the same
// content type UID "products" plus a non-allowlisted type "secrets", and wires
// a gin engine with only the public delivery routes.
func setupPublicContent(t *testing.T) (*gin.Engine, *gorm.DB) {
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

	seedType := func(tenantID uint, uid string) *models.ContentType {
		t.Helper()
		ct := &models.ContentType{TenantID: tenantID, UID: uid, Name: uid, DraftPublish: true}
		if err := db.Create(ct).Error; err != nil {
			t.Fatalf("create content type %s (tenant %d): %v", uid, tenantID, err)
		}
		return ct
	}
	productsT1 := seedType(models.DefaultTenantID, "products")
	productsT2 := seedType(200, "products")
	seedType(models.DefaultTenantID, "secrets")

	now := time.Now()
	future := now.Add(24 * time.Hour)
	seedEntry := func(tenantID, typeID uint, documentID, status string, publishedAt *time.Time, title string) *models.ContentEntry {
		t.Helper()
		e := &models.ContentEntry{
			TenantID:      tenantID,
			ContentTypeID: typeID,
			DocumentID:    documentID,
			Status:        status,
			Data:          models.JSONMap{"title": title},
			Locale:        "en",
			PublishedAt:   publishedAt,
		}
		if err := db.Create(e).Error; err != nil {
			t.Fatalf("create entry %s: %v", documentID, err)
		}
		return e
	}
	seedEntry(models.DefaultTenantID, productsT1.ID, "11111111-1111-1111-1111-111111111111", models.EntryStatusPublished, &now, "T1 published")
	seedEntry(models.DefaultTenantID, productsT1.ID, "22222222-2222-2222-2222-222222222222", models.EntryStatusDraft, nil, "T1 draft")
	seedEntry(models.DefaultTenantID, productsT1.ID, "33333333-3333-3333-3333-333333333333", models.EntryStatusPublished, &future, "T1 scheduled")
	seedEntry(200, productsT2.ID, "44444444-4444-4444-4444-444444444444", models.EntryStatusPublished, &now, "T2 published")

	svc := services.NewPublicContentService(repository.NewPublicContentRepository(db))
	h := NewPublicContentHandler(svc, []string{"products"})

	r := gin.New()
	r.GET("/api/v1/public/content/:uid", h.List)
	r.GET("/api/v1/public/content/:uid/:documentId", h.Get)
	return r, db
}

func doGet(r *gin.Engine, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPublicContent_ListReturnsOnlyPublishedDefaultTenant(t *testing.T) {
	r, _ := setupPublicContent(t)

	w := doGet(r, "/api/v1/public/content/products", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", cc)
	}

	var resp struct {
		Data struct {
			Items []map[string]any `json:"items"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.Total != 1 || len(resp.Data.Items) != 1 {
		t.Fatalf("expected exactly 1 published entry (draft and future published_at excluded), got total=%d items=%d", resp.Data.Total, len(resp.Data.Items))
	}
	if resp.Data.Items[0]["data"].(map[string]any)["title"] != "T1 published" {
		t.Fatalf("unexpected entry: %v", resp.Data.Items[0])
	}
}

func TestPublicContent_DraftPublishUnpublishLifecycle(t *testing.T) {
	r, db := setupPublicContent(t)

	// Draft entry: 404.
	if w := doGet(r, "/api/v1/public/content/products/22222222-2222-2222-2222-222222222222", nil); w.Code != http.StatusNotFound {
		t.Fatalf("draft should be 404, got %d", w.Code)
	}

	// Publish via the dedicated flow: immediately readable.
	now := time.Now()
	if err := db.Model(&models.ContentEntry{}).Where("document_id = ?", "22222222-2222-2222-2222-222222222222").
		Updates(map[string]any{"status": models.EntryStatusPublished, "published_at": &now}).Error; err != nil {
		t.Fatalf("publish entry: %v", err)
	}
	if w := doGet(r, "/api/v1/public/content/products/22222222-2222-2222-2222-222222222222", nil); w.Code != http.StatusOK {
		t.Fatalf("published entry should be 200, got %d", w.Code)
	}

	// Unpublish: immediately 404 again.
	if err := db.Model(&models.ContentEntry{}).Where("document_id = ?", "22222222-2222-2222-2222-222222222222").
		Updates(map[string]any{"status": models.EntryStatusDraft, "published_at": nil}).Error; err != nil {
		t.Fatalf("unpublish entry: %v", err)
	}
	if w := doGet(r, "/api/v1/public/content/products/22222222-2222-2222-2222-222222222222", nil); w.Code != http.StatusNotFound {
		t.Fatalf("unpublished entry must be 404 immediately, got %d", w.Code)
	}
}

func TestPublicContent_UnknownAndNonAllowlistedTypesShare404(t *testing.T) {
	r, _ := setupPublicContent(t)

	for _, path := range []string{
		"/api/v1/public/content/does-not-exist",
		"/api/v1/public/content/secrets", // published entries exist but not allowlisted
	} {
		w := doGet(r, path, nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s should be 404, got %d", path, w.Code)
		}
	}
}

func TestPublicContent_PaginationBoundsReturn400(t *testing.T) {
	r, _ := setupPublicContent(t)

	for _, q := range []string{"?page_size=51", "?page=0", "?page=-1", "?page_size=-5", "?page=abc", "?page_size=xyz"} {
		w := doGet(r, "/api/v1/public/content/products"+q, nil)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("pagination %q should be 400, got %d", q, w.Code)
		}
	}

	// The hard cap itself is accepted.
	if w := doGet(r, "/api/v1/public/content/products?page_size=50", nil); w.Code != http.StatusOK {
		t.Fatalf("page_size=50 should be 200, got %d", w.Code)
	}
}

func TestPublicContent_DTOExposesOnlyPublicFields(t *testing.T) {
	r, _ := setupPublicContent(t)

	w := doGet(r, "/api/v1/public/content/products/11111111-1111-1111-1111-111111111111", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	entry, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected body: %v", body)
	}
	allowed := map[string]bool{"document_id": true, "data": true, "locale": true, "published_at": true, "updated_at": true}
	for key := range entry {
		if !allowed[key] {
			t.Fatalf("DTO exposed non-public field %q: %v", key, entry)
		}
	}
	for _, required := range []string{"document_id", "data", "locale", "published_at", "updated_at"} {
		if _, ok := entry[required]; !ok {
			t.Fatalf("DTO missing required field %q", required)
		}
	}
	// Data must survive the trip as the entry's public payload.
	if entry["data"].(map[string]any)["title"] != "T1 published" {
		t.Fatalf("unexpected data payload: %v", entry["data"])
	}
}

func TestPublicContent_TenantBIsInvisibleAndHeadersAreIgnored(t *testing.T) {
	r, _ := setupPublicContent(t)

	// The tenant 2 published entry (same UID, different document) must not be
	// readable anonymously.
	if w := doGet(r, "/api/v1/public/content/products/44444444-4444-4444-4444-444444444444", nil); w.Code != http.StatusNotFound {
		t.Fatalf("tenant 2 entry must be 404, got %d", w.Code)
	}

	// Anonymous tenant-switching attempts via headers are ignored.
	headers := map[string]string{"X-Tenant-ID": "200"}
	if w := doGet(r, "/api/v1/public/content/products/44444444-4444-4444-4444-444444444444", headers); w.Code != http.StatusNotFound {
		t.Fatalf("tenant header must not unlock tenant 2, got %d", w.Code)
	}
	if w := doGet(r, "/api/v1/public/content/products", headers); w.Code != http.StatusOK {
		t.Fatalf("tenant header must not break default tenant reads, got %d", w.Code)
	}
	var resp struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	_ = json.Unmarshal(doGet(r, "/api/v1/public/content/products", headers).Body.Bytes(), &resp)
	if resp.Data.Total != 1 {
		t.Fatalf("default tenant view unchanged by headers, got total=%d", resp.Data.Total)
	}
}
