package services

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/cache"
	"github.com/yamovo/contentx/internal/models"
)

// ─── Content Type CRUD Tests ────────────────────────────────────────────────

func TestContentTypeService_Create_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	ct, err := svc.CreateContentType(CreateContentTypeRequest{
		UID:  "product",
		Name: "Product",
		Fields: []CreateFieldRequest{
			{Name: "title", Label: "Title", FieldType: "text", Required: true},
			{Name: "price", Label: "Price", FieldType: "float"},
		},
	}, 1)
	if err != nil {
		t.Fatalf("create content type: %v", err)
	}
	if ct.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if ct.UID != "product" {
		t.Fatalf("expected uid 'product', got '%s'", ct.UID)
	}
	if len(ct.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(ct.Fields))
	}
}

func TestContentTypeService_Create_DuplicateUID(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	req := CreateContentTypeRequest{
		UID: "event", Name: "Event",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}
	svc.CreateContentType(req, 1)

	_, err := svc.CreateContentType(req, 1)
	if err == nil {
		t.Fatal("expected error for duplicate UID")
	}
}

func TestContentTypeService_Create_InvalidUID(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	_, err := svc.CreateContentType(CreateContentTypeRequest{
		UID:    "Invalid UID!",
		Name:   "Bad",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text"}},
	}, 1)
	if err == nil {
		t.Fatal("expected error for invalid UID")
	}
}

func TestContentTypeService_Create_InvalidFieldType(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	_, err := svc.CreateContentType(CreateContentTypeRequest{
		UID:    "test_invalid",
		Name:   "Test",
		Fields: []CreateFieldRequest{{Name: "f", Label: "F", FieldType: "nonexistent_type"}},
	}, 1)
	if err == nil {
		t.Fatal("expected error for invalid field type")
	}
}

func TestContentTypeService_Create_EnumWithoutOptions(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	_, err := svc.CreateContentType(CreateContentTypeRequest{
		UID:    "test_enum",
		Name:   "Test Enum",
		Fields: []CreateFieldRequest{{Name: "status", Label: "Status", FieldType: "enum"}},
	}, 1)
	if err == nil {
		t.Fatal("expected error for enum without options")
	}
}

func TestContentTypeService_List_Empty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	types, err := svc.ListContentTypes(1)
	if err != nil {
		t.Fatalf("list content types: %v", err)
	}
	if len(types) != 0 {
		t.Fatalf("expected 0 types, got %d", len(types))
	}
}

func TestContentTypeService_List_WithData(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "type_a", Name: "Type A",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text"}},
	}, 1)
	svc.CreateContentType(CreateContentTypeRequest{
		UID: "type_b", Name: "Type B",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text"}},
	}, 1)

	types, err := svc.ListContentTypes(1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(types))
	}
}

func TestContentTypeService_Get_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "faq", Name: "FAQ",
		Fields: []CreateFieldRequest{{Name: "question", Label: "Q", FieldType: "text", Required: true}},
	}, 1)

	ct, err := svc.GetContentType("faq", 1)
	if err != nil {
		t.Fatalf("get content type: %v", err)
	}
	if ct.Name != "FAQ" {
		t.Fatalf("expected name 'FAQ', got '%s'", ct.Name)
	}
	if len(ct.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(ct.Fields))
	}
}

func TestContentTypeService_Get_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	_, err := svc.GetContentType("nonexistent", 1)
	if err == nil {
		t.Fatal("expected error for non-existent content type")
	}
}

func TestContentTypeService_Delete_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "to_delete", Name: "Temp",
		Fields: []CreateFieldRequest{{Name: "x", Label: "X", FieldType: "text"}},
	}, 1)

	if err := svc.DeleteContentType("to_delete", 1); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var count int64
	db.Model(&models.ContentType{}).Where("uid = ?", "to_delete").Count(&count)
	if count != 0 {
		t.Fatal("content type should be deleted")
	}
}

func TestContentTypeService_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	err := svc.DeleteContentType("nonexistent", 1)
	if err == nil {
		t.Fatal("expected error for deleting non-existent type")
	}
}

// ─── Content Entry CRUD Tests ───────────────────────────────────────────────

func TestContentTypeService_CreateEntry_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "entryuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "note", Name: "Note",
		Fields: []CreateFieldRequest{
			{Name: "title", Label: "Title", FieldType: "text", Required: true},
			{Name: "body", Label: "Body", FieldType: "rich_text"},
		},
	}, 1)

	entry, err := svc.CreateEntry("note", CreateEntryRequest{
		Data: map[string]interface{}{"title": "My Note", "body": "<p>Content</p>"},
	}, 1, user.ID)
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if entry.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if entry.DocumentID == "" {
		t.Fatal("expected non-empty document ID")
	}
}

func TestContentTypeService_CreateEntry_MissingRequired(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "requser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "req_test", Name: "Required Test",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)

	_, err := svc.CreateEntry("req_test", CreateEntryRequest{
		Data: map[string]interface{}{"body": "no title here"},
	}, 1, user.ID)
	if err == nil {
		t.Fatal("expected error for missing required field")
	}
}

func TestContentTypeService_CreateEntry_EnumValidation(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "enumuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "enum_test", Name: "Enum Test",
		Fields: []CreateFieldRequest{
			{Name: "status", Label: "Status", FieldType: "enum", Options: []string{"active", "inactive"}},
		},
	}, 1)

	// Valid enum value.
	_, err := svc.CreateEntry("enum_test", CreateEntryRequest{
		Data: map[string]interface{}{"status": "active"},
	}, 1, user.ID)
	if err != nil {
		t.Fatalf("expected success for valid enum: %v", err)
	}

	// Invalid enum value.
	_, err = svc.CreateEntry("enum_test", CreateEntryRequest{
		Data: map[string]interface{}{"status": "invalid_value"},
	}, 1, user.ID)
	if err == nil {
		t.Fatal("expected error for invalid enum value")
	}
}

func TestContentTypeService_CreateEntry_BooleanValidation(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "booluser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "bool_test", Name: "Bool Test",
		Fields: []CreateFieldRequest{{Name: "enabled", Label: "Enabled", FieldType: "boolean"}},
	}, 1)

	// Valid boolean.
	_, err := svc.CreateEntry("bool_test", CreateEntryRequest{
		Data: map[string]interface{}{"enabled": true},
	}, 1, user.ID)
	if err != nil {
		t.Fatalf("expected success for valid boolean: %v", err)
	}

	// Invalid boolean (string instead of bool).
	_, err = svc.CreateEntry("bool_test", CreateEntryRequest{
		Data: map[string]interface{}{"enabled": "yes"},
	}, 1, user.ID)
	if err == nil {
		t.Fatal("expected error for non-boolean value")
	}
}

func TestContentTypeService_GetEntry_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "getentryuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "get_test", Name: "Get Test",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)

	created, _ := svc.CreateEntry("get_test", CreateEntryRequest{
		Data: map[string]interface{}{"title": "Found Me"},
	}, 1, user.ID)

	entry, err := svc.GetEntry("get_test", created.DocumentID, 1)
	if err != nil {
		t.Fatalf("get entry: %v", err)
	}
	if entry.DocumentID != created.DocumentID {
		t.Fatal("document ID mismatch")
	}
}

func TestContentTypeService_GetEntry_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "nf_test", Name: "NF Test",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text"}},
	}, 1)

	_, err := svc.GetEntry("nf_test", "nonexistent-uuid", 1)
	if err == nil {
		t.Fatal("expected error for non-existent entry")
	}
}

func TestContentTypeService_UpdateEntry_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "updentryuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "upd_test", Name: "Update Test",
		Fields: []CreateFieldRequest{
			{Name: "title", Label: "Title", FieldType: "text", Required: true},
			{Name: "body", Label: "Body", FieldType: "text"},
		},
	}, 1)

	created, _ := svc.CreateEntry("upd_test", CreateEntryRequest{
		Data: map[string]interface{}{"title": "Original"},
	}, 1, user.ID)

	newBody := "Updated body"
	updated, err := svc.UpdateEntry("upd_test", created.DocumentID, UpdateEntryRequest{
		Data: map[string]interface{}{"title": "Original", "body": newBody},
	}, 1, user.ID)
	if err != nil {
		t.Fatalf("update entry: %v", err)
	}

	// Title should still be there (merge, not replace).
	if updated.Data["title"] != "Original" {
		t.Fatalf("expected title to remain 'Original', got '%v'", updated.Data["title"])
	}
	if updated.Data["body"] != newBody {
		t.Fatalf("expected body '%s', got '%v'", newBody, updated.Data["body"])
	}
}

func TestContentTypeService_CreateEntry_CannotPublishDirectly(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "directpubuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "direct_pub", Name: "Direct Publish", DraftPublish: true,
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)

	_, err := svc.CreateEntry("direct_pub", CreateEntryRequest{
		Data:   map[string]interface{}{"title": "Must stay draft"},
		Status: models.EntryStatusPublished,
	}, 1, user.ID)
	if err == nil {
		t.Fatal("expected direct published creation to be rejected")
	}

	result, listErr := svc.ListEntries("direct_pub", ListEntriesParams{Page: 1, PageSize: 10}, 1)
	if listErr != nil {
		t.Fatalf("list entries: %v", listErr)
	}
	if result.(models.ListResponse).Total != 0 {
		t.Fatal("rejected direct publish must not persist an entry")
	}
}

func TestContentTypeService_UpdateEntry_StatusRejected(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "statususer", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "status_test", Name: "Status Test", DraftPublish: true,
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)

	created, _ := svc.CreateEntry("status_test", CreateEntryRequest{
		Data: map[string]interface{}{"title": "Test"},
	}, 1, user.ID)

	published := "published"
	_, err := svc.UpdateEntry("status_test", created.DocumentID, UpdateEntryRequest{
		Status: &published,
	}, 1, user.ID)
	if err == nil {
		t.Fatal("expected status transition through update to be rejected")
	}

	unchanged, getErr := svc.GetEntry("status_test", created.DocumentID, 1)
	if getErr != nil {
		t.Fatalf("get unchanged entry: %v", getErr)
	}
	if unchanged.Status != models.EntryStatusDraft || unchanged.PublishedAt != nil {
		t.Fatalf("rejected update changed publication state: status=%q published_at=%v", unchanged.Status, unchanged.PublishedAt)
	}
}

func TestContentTypeService_DeleteEntry_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "delentryuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "del_test", Name: "Delete Test",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)

	created, _ := svc.CreateEntry("del_test", CreateEntryRequest{
		Data: map[string]interface{}{"title": "Bye"},
	}, 1, user.ID)

	if err := svc.DeleteEntry("del_test", created.DocumentID, 1); err != nil {
		t.Fatalf("delete entry: %v", err)
	}

	_, err := svc.GetEntry("del_test", created.DocumentID, 1)
	if err == nil {
		t.Fatal("entry should be deleted")
	}
}

func TestContentTypeService_DeleteEntry_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "dnf_test", Name: "DNF Test",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text"}},
	}, 1)

	err := svc.DeleteEntry("dnf_test", "nonexistent-uuid", 1)
	if err == nil {
		t.Fatal("expected error for deleting non-existent entry")
	}
}

func TestContentTypeService_PublishEntry(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "pubuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "pub_test", Name: "Pub Test", DraftPublish: true,
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)

	created, _ := svc.CreateEntry("pub_test", CreateEntryRequest{
		Data: map[string]interface{}{"title": "Draft"},
	}, 1, user.ID)

	if created.Status != "draft" {
		t.Fatalf("expected initial status 'draft', got '%s'", created.Status)
	}

	published, err := svc.PublishEntry("pub_test", created.DocumentID, 1, user.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Status != "published" {
		t.Fatalf("expected status 'published', got '%s'", published.Status)
	}
	if published.PublishedAt == nil {
		t.Fatal("expected non-nil PublishedAt")
	}
}

func TestContentTypeService_UnpublishEntry(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "unpubuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "unpub_test", Name: "Unpub Test", DraftPublish: true,
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)

	created, _ := svc.CreateEntry("unpub_test", CreateEntryRequest{
		Data: map[string]interface{}{"title": "Published"},
	}, 1, user.ID)
	published, err := svc.PublishEntry("unpub_test", created.DocumentID, 1, user.ID)
	if err != nil {
		t.Fatalf("publish before unpublish: %v", err)
	}

	if published.Status != "published" {
		t.Fatalf("expected published status, got '%s'", published.Status)
	}

	unpublished, err := svc.UnpublishEntry("unpub_test", created.DocumentID, 1, user.ID)
	if err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if unpublished.Status != "draft" {
		t.Fatalf("expected status 'draft', got '%s'", unpublished.Status)
	}
	if unpublished.PublishedAt != nil {
		t.Fatal("expected PublishedAt to be cleared after unpublish")
	}
}

func TestContentTypeService_ListEntries(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "listentryuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "list_entries", Name: "List Entries",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)

	for i := 0; i < 3; i++ {
		svc.CreateEntry("list_entries", CreateEntryRequest{
			Data: map[string]interface{}{"title": "Entry"},
		}, 1, user.ID)
	}

	result, err := svc.ListEntries("list_entries", ListEntriesParams{Page: 1, PageSize: 10}, 1)
	if err != nil {
		t.Fatalf("list entries: %v", err)
	}

	lr := result.(models.ListResponse)
	if lr.Total != 3 {
		t.Fatalf("expected total 3, got %d", lr.Total)
	}
	items := lr.Items.([]models.ContentEntry)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

func TestContentTypeService_ListEntries_StatusFilter(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "filteruser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "filter_test", Name: "Filter Test", DraftPublish: true,
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)

	// Create 2 draft + 1 published.
	svc.CreateEntry("filter_test", CreateEntryRequest{Data: map[string]interface{}{"title": "D1"}}, 1, user.ID)
	svc.CreateEntry("filter_test", CreateEntryRequest{Data: map[string]interface{}{"title": "D2"}}, 1, user.ID)
	publishedEntry, err := svc.CreateEntry("filter_test", CreateEntryRequest{
		Data: map[string]interface{}{"title": "P1"},
	}, 1, user.ID)
	if err != nil {
		t.Fatalf("create entry to publish: %v", err)
	}
	if _, err := svc.PublishEntry("filter_test", publishedEntry.DocumentID, 1, user.ID); err != nil {
		t.Fatalf("publish entry: %v", err)
	}

	result, _ := svc.ListEntries("filter_test", ListEntriesParams{Page: 1, PageSize: 10, Status: "published"}, 1)
	lr := result.(models.ListResponse)
	if lr.Total != 1 {
		t.Fatalf("expected 1 published entry, got %d", lr.Total)
	}
}

// ─── Export / Import Tests ──────────────────────────────────────────────────

func TestContentTypeService_ExportEntries(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "exportuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "export_test", Name: "Export",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)

	svc.CreateEntry("export_test", CreateEntryRequest{Data: map[string]interface{}{"title": "A"}}, 1, user.ID)
	svc.CreateEntry("export_test", CreateEntryRequest{Data: map[string]interface{}{"title": "B"}}, 1, user.ID)

	jsonStr, err := svc.ExportEntries("export_test", 1)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	var entries []models.ContentEntry
	if err := json.Unmarshal([]byte(jsonStr), &entries); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 exported entries, got %d", len(entries))
	}
}

func TestContentTypeService_ImportEntries(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "importuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "import_test", Name: "Import",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)

	// Export from another content type, then import.
	svc.CreateContentType(CreateContentTypeRequest{
		UID: "import_src", Name: "Source",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)
	svc.CreateEntry("import_src", CreateEntryRequest{Data: map[string]interface{}{"title": "Src1"}}, 1, user.ID)
	svc.CreateEntry("import_src", CreateEntryRequest{Data: map[string]interface{}{"title": "Src2"}}, 1, user.ID)

	jsonStr, _ := svc.ExportEntries("import_src", 1)

	count, err := svc.ImportEntries("import_test", jsonStr, 1, user.ID)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 imported entries, got %d", count)
	}

	// Verify entries exist in target.
	result, _ := svc.ListEntries("import_test", ListEntriesParams{Page: 1, PageSize: 10}, 1)
	lr := result.(models.ListResponse)
	if lr.Total != 2 {
		t.Fatalf("expected 2 entries in target after import, got %d", lr.Total)
	}
}

func TestContentTypeService_ImportEntries_ForcesDraft(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "importdraftuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "import_draft", Name: "Import Draft", DraftPublish: true,
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)

	input := `[{"status":"published","published_at":"2026-01-01T00:00:00Z","data":{"title":"Imported"}}]`
	count, err := svc.ImportEntries("import_draft", input, 1, user.ID)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if count != 1 {
		t.Fatalf("imported count = %d, want 1", count)
	}

	result, err := svc.ListEntries("import_draft", ListEntriesParams{Page: 1, PageSize: 10}, 1)
	if err != nil {
		t.Fatalf("list imported entries: %v", err)
	}
	items := result.(models.ListResponse).Items.([]models.ContentEntry)
	if len(items) != 1 || items[0].Status != models.EntryStatusDraft || items[0].PublishedAt != nil {
		t.Fatalf("import bypassed draft boundary: %#v", items)
	}
}

func TestContentTypeService_ImportEntries_InvalidJSON(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "badjsonuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "bad_json", Name: "Bad JSON",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text"}},
	}, 1)

	_, err := svc.ImportEntries("bad_json", "not valid json", 1, user.ID)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// ─── Search Tests ───────────────────────────────────────────────────────────

func TestContentTypeService_SearchEntries(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "searchuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "search_test", Name: "Search",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)

	svc.CreateEntry("search_test", CreateEntryRequest{Data: map[string]interface{}{"title": "Go Programming"}}, 1, user.ID)
	svc.CreateEntry("search_test", CreateEntryRequest{Data: map[string]interface{}{"title": "Python Basics"}}, 1, user.ID)
	svc.CreateEntry("search_test", CreateEntryRequest{Data: map[string]interface{}{"title": "Go Advanced"}}, 1, user.ID)

	results, err := svc.SearchEntries("search_test", "Go", 10, 1)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results matching 'Go', got %d", len(results))
	}
}

// ─── Tenant isolation (RFC-001 §5) ─────────────────────────────────────────

// TestContentTypeService_TenantIsolation verifies the service layer never
// leaks content types or entries across tenants: cross-tenant lookups return
// ErrNotFound (existence is not leaked) and cross-tenant writes are refused.
func TestContentTypeService_TenantIsolation(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "tenantiso", "admin")

	ct, err := svc.CreateContentType(CreateContentTypeRequest{
		UID: "iso_type", Name: "Isolated",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 1)
	if err != nil {
		t.Fatalf("create type: %v", err)
	}
	if ct.TenantID != 1 {
		t.Fatalf("created type TenantID = %d, want 1", ct.TenantID)
	}
	for _, f := range ct.Fields {
		if f.TenantID != 1 {
			t.Fatalf("field %q TenantID = %d, want 1", f.Name, f.TenantID)
		}
	}

	// Cross-tenant type lookups are invisible.
	if _, err := svc.GetContentType("iso_type", 2); err == nil {
		t.Fatal("GetContentType cross-tenant should fail")
	}
	types, err := svc.ListContentTypes(2)
	if err != nil || len(types) != 0 {
		t.Fatalf("ListContentTypes cross-tenant = %d/%v, want 0/nil", len(types), err)
	}

	// Same UID may be registered by another tenant (service-level check is
	// tenant-scoped), and the two never see each other.
	if _, err := svc.CreateContentType(CreateContentTypeRequest{
		UID: "iso_type", Name: "Isolated T2",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	}, 2); err != nil {
		t.Fatalf("same UID in tenant 2 should succeed: %v", err)
	}

	entry, err := svc.CreateEntry("iso_type", CreateEntryRequest{
		Data: map[string]interface{}{"title": "T1 Entry"},
	}, 1, user.ID)
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if entry.TenantID != 1 {
		t.Fatalf("created entry TenantID = %d, want 1", entry.TenantID)
	}

	// Cross-tenant entry access by guessed document_id is invisible.
	if _, err := svc.GetEntry("iso_type", entry.DocumentID, 2); err == nil {
		t.Fatal("GetEntry cross-tenant should fail")
	}
	result, err := svc.ListEntries("iso_type", ListEntriesParams{Page: 1, PageSize: 10}, 2)
	if err != nil {
		t.Fatalf("ListEntries cross-tenant: %v", err)
	}
	if lr := result.(models.ListResponse); lr.Total != 0 {
		t.Fatalf("ListEntries cross-tenant total = %d, want 0", lr.Total)
	}

	// Cross-tenant writes are refused and the row stays intact.
	if _, err := svc.UpdateEntry("iso_type", entry.DocumentID, UpdateEntryRequest{
		Data: map[string]interface{}{"title": "Hacked"},
	}, 2, user.ID); err == nil {
		t.Fatal("UpdateEntry cross-tenant should fail")
	}
	if _, err := svc.PublishEntry("iso_type", entry.DocumentID, 2, user.ID); err == nil {
		t.Fatal("PublishEntry cross-tenant should fail")
	}
	if err := svc.DeleteEntry("iso_type", entry.DocumentID, 2); err == nil {
		t.Fatal("DeleteEntry cross-tenant should fail")
	}
	got, err := svc.GetEntry("iso_type", entry.DocumentID, 1)
	if err != nil {
		t.Fatalf("tenant 1 entry missing after cross-tenant writes: %v", err)
	}
	if got.Data["title"] != "T1 Entry" {
		t.Fatalf("tenant 1 entry mutated by cross-tenant write: %v", got.Data["title"])
	}

	// Cross-tenant delete of the type is refused; type and entry survive.
	if err := svc.DeleteContentType("iso_type", 3); err == nil {
		t.Fatal("DeleteContentType cross-tenant should fail")
	}
	if _, err := svc.GetContentType("iso_type", 1); err != nil {
		t.Fatalf("tenant 1 type missing after cross-tenant delete: %v", err)
	}

	// Export / search / translations stay inside the tenant.
	if exported, err := svc.ExportEntries("iso_type", 2); err != nil || exported != "[]" {
		t.Fatalf("ExportEntries cross-tenant = %q/%v, want empty", exported, err)
	}
	if found, err := svc.SearchEntries("iso_type", "T1 Entry", 10, 2); err != nil || len(found) != 0 {
		t.Fatalf("SearchEntries cross-tenant = %d/%v, want 0/nil", len(found), err)
	}
	if trs, err := svc.ListEntryTranslations("iso_type", entry.DocumentID, 2); err == nil && len(trs) != 0 {
		t.Fatalf("ListEntryTranslations cross-tenant = %d, want 0", len(trs))
	}
}

// TestContentTypeService_CacheTenantIsolation verifies the contenttype cache
// key carries the tenant: tenant 2 must never hit tenant 1's cached type.
func TestContentTypeService_CacheTenantIsolation(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db).WithCache(cache.NewMemoryDriver(1000), 5*time.Minute)

	if _, err := svc.CreateContentType(CreateContentTypeRequest{
		UID: "cached_type", Name: "Cached",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text"}},
	}, 1); err != nil {
		t.Fatalf("create type: %v", err)
	}

	// Warm tenant 1's cache.
	if _, err := svc.GetContentType("cached_type", 1); err != nil {
		t.Fatalf("GetContentType tenant 1: %v", err)
	}
	// Tenant 2 must miss even though the UID is cached for tenant 1.
	if _, err := svc.GetContentType("cached_type", 2); err == nil {
		t.Fatal("GetContentType cross-tenant should miss the cache and fail")
	}
}
