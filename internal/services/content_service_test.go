package services

import (
	"encoding/json"
	"testing"

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
	})
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
	svc.CreateContentType(req)

	_, err := svc.CreateContentType(req)
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
	})
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
	})
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
	})
	if err == nil {
		t.Fatal("expected error for enum without options")
	}
}

func TestContentTypeService_List_Empty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)

	types, err := svc.ListContentTypes()
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
	})
	svc.CreateContentType(CreateContentTypeRequest{
		UID: "type_b", Name: "Type B",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text"}},
	})

	types, err := svc.ListContentTypes()
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
	})

	ct, err := svc.GetContentType("faq")
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

	_, err := svc.GetContentType("nonexistent")
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
	})

	if err := svc.DeleteContentType("to_delete"); err != nil {
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

	err := svc.DeleteContentType("nonexistent")
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
	})

	entry, err := svc.CreateEntry("note", CreateEntryRequest{
		Data: map[string]interface{}{"title": "My Note", "body": "<p>Content</p>"},
	}, user.ID)
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
	})

	_, err := svc.CreateEntry("req_test", CreateEntryRequest{
		Data: map[string]interface{}{"body": "no title here"},
	}, user.ID)
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
	})

	// Valid enum value.
	_, err := svc.CreateEntry("enum_test", CreateEntryRequest{
		Data: map[string]interface{}{"status": "active"},
	}, user.ID)
	if err != nil {
		t.Fatalf("expected success for valid enum: %v", err)
	}

	// Invalid enum value.
	_, err = svc.CreateEntry("enum_test", CreateEntryRequest{
		Data: map[string]interface{}{"status": "invalid_value"},
	}, user.ID)
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
	})

	// Valid boolean.
	_, err := svc.CreateEntry("bool_test", CreateEntryRequest{
		Data: map[string]interface{}{"enabled": true},
	}, user.ID)
	if err != nil {
		t.Fatalf("expected success for valid boolean: %v", err)
	}

	// Invalid boolean (string instead of bool).
	_, err = svc.CreateEntry("bool_test", CreateEntryRequest{
		Data: map[string]interface{}{"enabled": "yes"},
	}, user.ID)
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
	})

	created, _ := svc.CreateEntry("get_test", CreateEntryRequest{
		Data: map[string]interface{}{"title": "Found Me"},
	}, user.ID)

	entry, err := svc.GetEntry("get_test", created.DocumentID)
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
	})

	_, err := svc.GetEntry("nf_test", "nonexistent-uuid")
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
	})

	created, _ := svc.CreateEntry("upd_test", CreateEntryRequest{
		Data: map[string]interface{}{"title": "Original"},
	}, user.ID)

	newBody := "Updated body"
	updated, err := svc.UpdateEntry("upd_test", created.DocumentID, UpdateEntryRequest{
		Data: map[string]interface{}{"title": "Original", "body": newBody},
	}, user.ID)
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

func TestContentTypeService_UpdateEntry_Status(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "statususer", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "status_test", Name: "Status Test",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	})

	created, _ := svc.CreateEntry("status_test", CreateEntryRequest{
		Data: map[string]interface{}{"title": "Test"},
	}, user.ID)

	published := "published"
	updated, err := svc.UpdateEntry("status_test", created.DocumentID, UpdateEntryRequest{
		Status: &published,
	}, user.ID)
	if err != nil {
		t.Fatalf("update status: %v", err)
	}
	if updated.Status != "published" {
		t.Fatalf("expected status 'published', got '%s'", updated.Status)
	}
	if updated.PublishedAt == nil {
		t.Fatal("expected non-nil PublishedAt after publishing")
	}
}

func TestContentTypeService_DeleteEntry_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "delentryuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "del_test", Name: "Delete Test",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	})

	created, _ := svc.CreateEntry("del_test", CreateEntryRequest{
		Data: map[string]interface{}{"title": "Bye"},
	}, user.ID)

	if err := svc.DeleteEntry("del_test", created.DocumentID); err != nil {
		t.Fatalf("delete entry: %v", err)
	}

	_, err := svc.GetEntry("del_test", created.DocumentID)
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
	})

	err := svc.DeleteEntry("dnf_test", "nonexistent-uuid")
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
	})

	created, _ := svc.CreateEntry("pub_test", CreateEntryRequest{
		Data: map[string]interface{}{"title": "Draft"},
	}, user.ID)

	if created.Status != "draft" {
		t.Fatalf("expected initial status 'draft', got '%s'", created.Status)
	}

	published, err := svc.PublishEntry("pub_test", created.DocumentID, user.ID)
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
		UID: "unpub_test", Name: "Unpub Test", DraftPublish: false,
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	})

	created, _ := svc.CreateEntry("unpub_test", CreateEntryRequest{
		Data:   map[string]interface{}{"title": "Published"},
		Status: "published",
	}, user.ID)

	if created.Status != "published" {
		t.Fatalf("expected initial status 'published', got '%s'", created.Status)
	}

	unpublished, err := svc.UnpublishEntry("unpub_test", created.DocumentID, user.ID)
	if err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if unpublished.Status != "draft" {
		t.Fatalf("expected status 'draft', got '%s'", unpublished.Status)
	}
}

func TestContentTypeService_ListEntries(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "listentryuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "list_entries", Name: "List Entries",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	})

	for i := 0; i < 3; i++ {
		svc.CreateEntry("list_entries", CreateEntryRequest{
			Data: map[string]interface{}{"title": "Entry"},
		}, user.ID)
	}

	result, err := svc.ListEntries("list_entries", ListEntriesParams{Page: 1, PageSize: 10})
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
	})

	// Create 2 draft + 1 published.
	svc.CreateEntry("filter_test", CreateEntryRequest{Data: map[string]interface{}{"title": "D1"}}, user.ID)
	svc.CreateEntry("filter_test", CreateEntryRequest{Data: map[string]interface{}{"title": "D2"}}, user.ID)
	pub := "published"
	svc.CreateEntry("filter_test", CreateEntryRequest{
		Data: map[string]interface{}{"title": "P1"}, Status: pub,
	}, user.ID)

	result, _ := svc.ListEntries("filter_test", ListEntriesParams{Page: 1, PageSize: 10, Status: "published"})
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
	})

	svc.CreateEntry("export_test", CreateEntryRequest{Data: map[string]interface{}{"title": "A"}}, user.ID)
	svc.CreateEntry("export_test", CreateEntryRequest{Data: map[string]interface{}{"title": "B"}}, user.ID)

	jsonStr, err := svc.ExportEntries("export_test")
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
	})

	// Export from another content type, then import.
	svc.CreateContentType(CreateContentTypeRequest{
		UID: "import_src", Name: "Source",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text", Required: true}},
	})
	svc.CreateEntry("import_src", CreateEntryRequest{Data: map[string]interface{}{"title": "Src1"}}, user.ID)
	svc.CreateEntry("import_src", CreateEntryRequest{Data: map[string]interface{}{"title": "Src2"}}, user.ID)

	jsonStr, _ := svc.ExportEntries("import_src")

	count, err := svc.ImportEntries("import_test", jsonStr, user.ID)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 imported entries, got %d", count)
	}

	// Verify entries exist in target.
	result, _ := svc.ListEntries("import_test", ListEntriesParams{Page: 1, PageSize: 10})
	lr := result.(models.ListResponse)
	if lr.Total != 2 {
		t.Fatalf("expected 2 entries in target after import, got %d", lr.Total)
	}
}

func TestContentTypeService_ImportEntries_InvalidJSON(t *testing.T) {
	db := setupTestDB(t)
	svc := NewContentTypeService(db)
	user := createTestUser(t, db, "badjsonuser", "admin")

	svc.CreateContentType(CreateContentTypeRequest{
		UID: "bad_json", Name: "Bad JSON",
		Fields: []CreateFieldRequest{{Name: "title", Label: "Title", FieldType: "text"}},
	})

	_, err := svc.ImportEntries("bad_json", "not valid json", user.ID)
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
	})

	svc.CreateEntry("search_test", CreateEntryRequest{Data: map[string]interface{}{"title": "Go Programming"}}, user.ID)
	svc.CreateEntry("search_test", CreateEntryRequest{Data: map[string]interface{}{"title": "Python Basics"}}, user.ID)
	svc.CreateEntry("search_test", CreateEntryRequest{Data: map[string]interface{}{"title": "Go Advanced"}}, user.ID)

	results, err := svc.SearchEntries("search_test", "Go", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results matching 'Go', got %d", len(results))
	}
}
