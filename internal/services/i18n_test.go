package services

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// ─── Article i18n ────────────────────────────────────────────────────────────

func TestArticleService_Create_DefaultLocale(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	article, err := svc.Create(CreateArticleRequest{
		Title:   "English Title",
		Content: "<p>Hello</p>",
		Status:  "draft",
	}, user.ID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if article.Locale != "en" {
		t.Errorf("Locale = %q, want %q", article.Locale, "en")
	}
	if article.TranslationGroupID != nil {
		t.Errorf("TranslationGroupID should be nil for a fresh article, got %v", *article.TranslationGroupID)
	}
}

func TestArticleService_Create_ExplicitLocale(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	article, err := svc.Create(CreateArticleRequest{
		Title:   "中文标题",
		Content: "<p>你好</p>",
		Status:  "draft",
		Locale:  "zh",
	}, user.ID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if article.Locale != "zh" {
		t.Errorf("Locale = %q, want %q", article.Locale, "zh")
	}
}

func TestArticleService_List_FilterByLocale(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	// Create one EN and two ZH articles.
	svc.Create(CreateArticleRequest{Title: "EN One", Content: "<p>x</p>", Status: "draft", Locale: "en"}, user.ID)
	svc.Create(CreateArticleRequest{Title: "ZH One", Content: "<p>一</p>", Status: "draft", Locale: "zh"}, user.ID)
	svc.Create(CreateArticleRequest{Title: "ZH Two", Content: "<p>二</p>", Status: "draft", Locale: "zh"}, user.ID)

	// No filter — total 3.
	resp, err := svc.List(ListArticlesFilter{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if resp.Total != 3 {
		t.Errorf("Total (all) = %d, want 3", resp.Total)
	}

	// Filter by zh — total 2.
	resp, err = svc.List(ListArticlesFilter{Page: 1, PageSize: 50, Locale: "zh"})
	if err != nil {
		t.Fatalf("List zh: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("Total (zh) = %d, want 2", resp.Total)
	}

	// Filter by en — total 1.
	resp, err = svc.List(ListArticlesFilter{Page: 1, PageSize: 50, Locale: "en"})
	if err != nil {
		t.Fatalf("List en: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("Total (en) = %d, want 1", resp.Total)
	}
}

func TestArticleService_CreateTranslation_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	// Source article in EN.
	source, err := svc.Create(CreateArticleRequest{
		Title:   "Hello World",
		Content: "<p>Hello</p>",
		Status:  "published",
		Locale:  "en",
	}, user.ID)
	if err != nil {
		t.Fatalf("Create source: %v", err)
	}

	// Create a ZH translation.
	translated, err := svc.CreateTranslation(source.ID, "zh", CreateArticleRequest{
		Title:   "你好世界",
		Content: "<p>你好</p>",
	}, user.ID)
	if err != nil {
		t.Fatalf("CreateTranslation: %v", err)
	}
	if translated.Locale != "zh" {
		t.Errorf("Locale = %q, want zh", translated.Locale)
	}
	if translated.TranslationGroupID == nil {
		t.Fatal("TranslationGroupID should be set on translation")
	}
	// The group ID should be the source's own ID (since source had nil group).
	if *translated.TranslationGroupID != source.ID {
		t.Errorf("TranslationGroupID = %d, want %d", *translated.TranslationGroupID, source.ID)
	}
	if translated.Slug == "" {
		t.Error("Slug should be generated")
	}
	if translated.Slug == source.Slug {
		t.Error("Translation slug must differ from source slug (globally unique)")
	}
}

func TestArticleService_CreateTranslation_DuplicateLocale(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	source, err := svc.Create(CreateArticleRequest{
		Title:   "Hello",
		Content: "<p>Hi</p>",
		Status:  "draft",
		Locale:  "en",
	}, user.ID)
	if err != nil {
		t.Fatalf("Create source: %v", err)
	}

	// First zh translation — OK.
	if _, err := svc.CreateTranslation(source.ID, "zh", CreateArticleRequest{
		Title: "你好", Content: "<p>你好</p>",
	}, user.ID); err != nil {
		t.Fatalf("first translation: %v", err)
	}

	// Second zh translation — should fail.
	_, err = svc.CreateTranslation(source.ID, "zh", CreateArticleRequest{
		Title: "你好2", Content: "<p>你好2</p>",
	}, user.ID)
	if err == nil {
		t.Fatal("expected error for duplicate locale translation")
	}
}

func TestArticleService_CreateTranslation_MissingLocale(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	source, err := svc.Create(CreateArticleRequest{
		Title: "X", Content: "<p>x</p>", Status: "draft",
	}, user.ID)
	if err != nil {
		t.Fatalf("Create source: %v", err)
	}

	_, err = svc.CreateTranslation(source.ID, "", CreateArticleRequest{
		Title: "Y", Content: "<p>y</p>",
	}, user.ID)
	if err == nil {
		t.Fatal("expected error for empty locale")
	}
}

func TestArticleService_ListTranslations(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")
	user := createTestUser(t, db, "author1", "author")

	source, err := svc.Create(CreateArticleRequest{
		Title:   "Hello",
		Content: "<p>Hi</p>",
		Status:  "draft",
		Locale:  "en",
	}, user.ID)
	if err != nil {
		t.Fatalf("Create source: %v", err)
	}
	if _, err := svc.CreateTranslation(source.ID, "zh", CreateArticleRequest{
		Title: "你好", Content: "<p>你好</p>",
	}, user.ID); err != nil {
		t.Fatalf("translation zh: %v", err)
	}
	if _, err := svc.CreateTranslation(source.ID, "ja", CreateArticleRequest{
		Title: "こんにちは", Content: "<p>こんにちは</p>",
	}, user.ID); err != nil {
		t.Fatalf("translation ja: %v", err)
	}

	translations, err := svc.ListTranslations(source.ID)
	if err != nil {
		t.Fatalf("ListTranslations: %v", err)
	}
	if len(translations) != 2 {
		t.Errorf("got %d translations, want 2", len(translations))
	}

	// Each translation should also see its siblings (including the source).
	zhTranslations, err := svc.ListTranslations(getIDOfTranslation(t, translations, "zh"))
	if err != nil {
		t.Fatalf("ListTranslations from zh: %v", err)
	}
	if len(zhTranslations) != 2 {
		t.Errorf("from zh got %d translations, want 2 (en + ja)", len(zhTranslations))
	}
}

func TestArticleService_ListTranslations_SourceNotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewArticleService(db, "http://localhost:8080")

	_, err := svc.ListTranslations(99999)
	if err == nil {
		t.Fatal("expected error for missing source")
	}
	if err != gorm.ErrRecordNotFound {
		t.Errorf("got %v, want gorm.ErrRecordNotFound", err)
	}
}

// getIDOfTranslation returns the ID of the translation matching the locale.
func getIDOfTranslation(t *testing.T, articles []models.Article, locale string) uint {
	t.Helper()
	for _, a := range articles {
		if a.Locale == locale {
			return a.ID
		}
	}
	t.Fatalf("translation with locale %q not found", locale)
	return 0
}

// ─── ContentEntry i18n ───────────────────────────────────────────────────────

func TestContentTypeService_CreateEntry_DefaultLocale(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "author1", "author")

	seedContentType(t, svc, "products", "Product")

	entry, err := svc.CreateEntry("products", CreateEntryRequest{
		Data: map[string]interface{}{"name": "Widget"},
	}, user.ID)
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}
	if entry.Locale != "en" {
		t.Errorf("Locale = %q, want en", entry.Locale)
	}
}

func TestContentTypeService_CreateEntry_ExplicitLocale(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "author1", "author")

	seedContentType(t, svc, "products", "Product")

	entry, err := svc.CreateEntry("products", CreateEntryRequest{
		Data:   map[string]interface{}{"name": "商品"},
		Locale: "zh",
	}, user.ID)
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}
	if entry.Locale != "zh" {
		t.Errorf("Locale = %q, want zh", entry.Locale)
	}
}

func TestContentTypeService_ListEntries_FilterByLocale(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "author1", "author")

	seedContentType(t, svc, "products", "Product")

	svc.CreateEntry("products", CreateEntryRequest{Data: map[string]interface{}{"name": "A"}, Locale: "en"}, user.ID)
	svc.CreateEntry("products", CreateEntryRequest{Data: map[string]interface{}{"name": "甲"}, Locale: "zh"}, user.ID)
	svc.CreateEntry("products", CreateEntryRequest{Data: map[string]interface{}{"name": "乙"}, Locale: "zh"}, user.ID)

	// All — 3.
	respAny, err := svc.ListEntries("products", ListEntriesParams{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("ListEntries all: %v", err)
	}
	resp := respAny.(models.ListResponse)
	if resp.Total != 3 {
		t.Errorf("Total (all) = %d, want 3", resp.Total)
	}

	// zh — 2.
	respAny, err = svc.ListEntries("products", ListEntriesParams{Page: 1, PageSize: 50, Locale: "zh"})
	if err != nil {
		t.Fatalf("ListEntries zh: %v", err)
	}
	resp = respAny.(models.ListResponse)
	if resp.Total != 2 {
		t.Errorf("Total (zh) = %d, want 2", resp.Total)
	}
}

func TestContentTypeService_CreateEntryTranslation_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "author1", "author")

	seedContentType(t, svc, "products", "Product")

	source, err := svc.CreateEntry("products", CreateEntryRequest{
		Data:   map[string]interface{}{"name": "Widget", "price": float64(10)},
		Locale: "en",
	}, user.ID)
	if err != nil {
		t.Fatalf("CreateEntry source: %v", err)
	}

	translated, err := svc.CreateEntryTranslation("products", source.DocumentID, "zh", CreateEntryRequest{
		Data: map[string]interface{}{"name": "小工具"}, // override name, inherit price
	}, user.ID)
	if err != nil {
		t.Fatalf("CreateEntryTranslation: %v", err)
	}
	if translated.Locale != "zh" {
		t.Errorf("Locale = %q, want zh", translated.Locale)
	}
	if translated.TranslationGroupID == nil {
		t.Fatal("TranslationGroupID should be set")
	}
	if *translated.TranslationGroupID != source.ID {
		t.Errorf("TranslationGroupID = %d, want %d", *translated.TranslationGroupID, source.ID)
	}
	// Inherited field.
	if p, ok := translated.Data["price"].(float64); !ok || p != 10 {
		t.Errorf("inherited price = %v, want 10", translated.Data["price"])
	}
	// Overridden field.
	if translated.Data["name"] != "小工具" {
		t.Errorf("overridden name = %v, want 小工具", translated.Data["name"])
	}
}

func TestContentTypeService_CreateEntryTranslation_DuplicateLocale(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "author1", "author")

	seedContentType(t, svc, "products", "Product")

	source, err := svc.CreateEntry("products", CreateEntryRequest{
		Data: map[string]interface{}{"name": "X"},
	}, user.ID)
	if err != nil {
		t.Fatalf("CreateEntry source: %v", err)
	}

	if _, err := svc.CreateEntryTranslation("products", source.DocumentID, "zh", CreateEntryRequest{
		Data: map[string]interface{}{"name": "甲"},
	}, user.ID); err != nil {
		t.Fatalf("first zh translation: %v", err)
	}

	_, err = svc.CreateEntryTranslation("products", source.DocumentID, "zh", CreateEntryRequest{
		Data: map[string]interface{}{"name": "乙"},
	}, user.ID)
	if err == nil {
		t.Fatal("expected error for duplicate locale")
	}
}

func TestContentTypeService_ListEntryTranslations(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "author1", "author")

	seedContentType(t, svc, "products", "Product")

	source, err := svc.CreateEntry("products", CreateEntryRequest{
		Data: map[string]interface{}{"name": "X"},
	}, user.ID)
	if err != nil {
		t.Fatalf("CreateEntry source: %v", err)
	}
	svc.CreateEntryTranslation("products", source.DocumentID, "zh", CreateEntryRequest{
		Data: map[string]interface{}{"name": "甲"},
	}, user.ID)
	svc.CreateEntryTranslation("products", source.DocumentID, "ja", CreateEntryRequest{
		Data: map[string]interface{}{"name": "X-ja"},
	}, user.ID)

	translations, err := svc.ListEntryTranslations("products", source.DocumentID)
	if err != nil {
		t.Fatalf("ListEntryTranslations: %v", err)
	}
	if len(translations) != 2 {
		t.Errorf("got %d translations, want 2", len(translations))
	}
}

func TestContentTypeService_ListEntryTranslations_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	seedContentType(t, svc, "products", "Product")

	_, err := svc.ListEntryTranslations("products", "nonexistent-doc-id")
	if err == nil {
		t.Fatal("expected error for missing entry")
	}
}

// seedContentType creates a minimal content type with a single required text
// field "name" for use in i18n tests.
func seedContentType(t *testing.T, svc *ContentTypeService, uid, name string) {
	t.Helper()
	if _, err := svc.CreateContentType(CreateContentTypeRequest{
		UID:  uid,
		Name: name,
		Fields: []CreateFieldRequest{
			{Name: "name", Label: "Name", FieldType: "text", Required: true},
			{Name: "price", Label: "Price", FieldType: "float"},
		},
	}); err != nil {
		t.Fatalf("seedContentType %s: %v", uid, err)
	}
}
