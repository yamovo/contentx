package repository

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

func TestContentTypeRepository_CreateAndFindByUID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewContentTypeRepository(db)

	ct := &models.ContentType{
		Name:        "Product",
		UID:         "product",
		Description: "Product pages",
	}
	if err := repo.Create(ct); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ct.ID == 0 {
		t.Fatal("ID should be set")
	}

	got, err := repo.FindByUID("product", 1)
	if err != nil {
		t.Fatalf("FindByUID: %v", err)
	}
	if got.Name != "Product" {
		t.Fatalf("unexpected name: %q", got.Name)
	}
}

func TestContentTypeRepository_FindByUID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewContentTypeRepository(db)
	_, err := repo.FindByUID("nonexistent", 1)
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestContentTypeRepository_CountByUID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewContentTypeRepository(db)
	repo.Create(&models.ContentType{Name: "Blog", UID: "blog"})

	count, err := repo.CountByUID("blog", 1)
	if err != nil {
		t.Fatalf("CountByUID: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count=1, got %d", count)
	}

	count, _ = repo.CountByUID("nonexistent", 1)
	if count != 0 {
		t.Fatalf("expected count=0 for nonexistent UID, got %d", count)
	}
}

func TestContentTypeRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewContentTypeRepository(db)
	repo.Create(&models.ContentType{Name: "A", UID: "a"})
	repo.Create(&models.ContentType{Name: "B", UID: "b"})

	types, err := repo.List(1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(types) < 2 {
		t.Fatalf("expected at least 2 types, got %d", len(types))
	}
}

func TestContentTypeRepository_List_PreloadsFields(t *testing.T) {
	db := setupTestDB(t)
	repo := NewContentTypeRepository(db)

	ct := &models.ContentType{Name: "With Fields", UID: "with-fields"}
	if err := repo.Create(ct); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Add fields directly.
	db.Create(&models.ContentField{
		ContentTypeID: ct.ID,
		Name:          "price",
		Label:         "Price",
		FieldType:     "number",
		SortOrder:     2,
	})
	db.Create(&models.ContentField{
		ContentTypeID: ct.ID,
		Name:          "sku",
		Label:         "SKU",
		FieldType:     "text",
		SortOrder:     1,
	})

	types, _ := repo.List(1)
	var got *models.ContentType
	for i := range types {
		if types[i].UID == "with-fields" {
			got = &types[i]
			break
		}
	}
	if got == nil {
		t.Fatal("content type not found")
	}
	if len(got.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(got.Fields))
	}
	// Fields should be ordered by sort_order ASC.
	if got.Fields[0].Name != "sku" {
		t.Fatalf("expected first field to be 'sku' (sort_order=1), got %q", got.Fields[0].Name)
	}
}

func TestContentTypeRepository_Delete_Cascades(t *testing.T) {
	db := setupTestDB(t)
	repo := NewContentTypeRepository(db)

	ct := &models.ContentType{Name: "ToDelete", UID: "to-delete"}
	repo.Create(ct)

	// Add a field and an entry.
	db.Create(&models.ContentField{
		ContentTypeID: ct.ID,
		Name:          "f",
		Label:         "F",
		FieldType:     "text",
	})
	db.Create(&models.ContentEntry{
		ContentTypeID: ct.ID,
		DocumentID:    "test-doc-001",
	})

	if err := repo.Delete(ct.ID, 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify type is gone.
	_, err := repo.FindByID(ct.ID, 1)
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}

	// Verify fields were cascade-deleted.
	var fieldCount int64
	db.Model(&models.ContentField{}).Where("content_type_id = ?", ct.ID).Count(&fieldCount)
	if fieldCount != 0 {
		t.Fatalf("expected 0 fields after delete, got %d", fieldCount)
	}

	// Verify entries were cascade-deleted.
	var entryCount int64
	db.Model(&models.ContentEntry{}).Where("content_type_id = ?", ct.ID).Count(&entryCount)
	if entryCount != 0 {
		t.Fatalf("expected 0 entries after delete, got %d", entryCount)
	}
}

func TestContentTypeRepository_CountEntriesByTypeID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewContentTypeRepository(db)

	ct := &models.ContentType{Name: "CountTest", UID: "count-test"}
	repo.Create(ct)

	db.Create(&models.ContentEntry{ContentTypeID: ct.ID, DocumentID: "doc-1"})
	db.Create(&models.ContentEntry{ContentTypeID: ct.ID, DocumentID: "doc-2"})

	count, err := repo.CountEntriesByTypeID(ct.ID, 1)
	if err != nil {
		t.Fatalf("CountEntriesByTypeID: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 entries, got %d", count)
	}
}

// TestContentTypeRepository_TenantIsolation verifies RFC-001 §5: every
// content-type query carries the tenant scope and cross-tenant access is
// invisible (not found / empty), never an error leaking existence.
func TestContentTypeRepository_TenantIsolation(t *testing.T) {
	db := setupTestDB(t)
	repo := NewContentTypeRepository(db)

	ct := &models.ContentType{TenantID: 1, Name: "T1 Product", UID: "t1-product"}
	if err := repo.Create(ct); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Cross-tenant reads return not found / empty.
	if _, err := repo.FindByUID("t1-product", 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("FindByUID cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	if _, err := repo.FindByID(ct.ID, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("FindByID cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	types, err := repo.List(2)
	if err != nil || len(types) != 0 {
		t.Fatalf("List cross-tenant = %d/%v, want 0/nil", len(types), err)
	}
	count, err := repo.CountByUID("t1-product", 2)
	if err != nil || count != 0 {
		t.Fatalf("CountByUID cross-tenant = %d/%v, want 0/nil", count, err)
	}

	// UID uniqueness check input is tenant-scoped (CountByUID above returns 0
	// for tenant 2); DB-level composite uniqueness is covered by migration
	// tests. A second tenant type with its own UID coexists fine.
	ct2 := &models.ContentType{TenantID: 2, Name: "T2 Product", UID: "t2-product"}
	if err := repo.Create(ct2); err != nil {
		t.Fatalf("Create tenant 2 type: %v", err)
	}

	// Cross-tenant delete is a no-op; both rows stay intact.
	if err := repo.Delete(ct.ID, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("Delete cross-tenant = %v, want gorm.ErrRecordNotFound (no-op)", err)
	}
	if _, err := repo.FindByID(ct.ID, 1); err != nil {
		t.Fatalf("tenant 1 content type missing after cross-tenant delete: %v", err)
	}

	// Same-tenant delete cascades only within the tenant.
	entryRepo := NewContentEntryRepository(db)
	if err := entryRepo.Create(&models.ContentEntry{TenantID: 1, ContentTypeID: ct.ID, DocumentID: "t1-doc"}); err != nil {
		t.Fatalf("Create entry: %v", err)
	}
	if err := repo.Delete(ct.ID, 1); err != nil {
		t.Fatalf("Delete same-tenant: %v", err)
	}
	var entryCount int64
	db.Model(&models.ContentEntry{}).Where("content_type_id = ?", ct.ID).Count(&entryCount)
	if entryCount != 0 {
		t.Fatalf("expected 0 entries after same-tenant delete, got %d", entryCount)
	}
	if _, err := repo.FindByID(ct2.ID, 2); err != nil {
		t.Fatalf("tenant 2 content type must survive tenant 1 delete: %v", err)
	}
}

// TestContentEntryRepository_TenantIsolation verifies entry queries never
// cross tenant boundaries (RFC-001 §5), including identifier guessing via
// document_id.
func TestContentEntryRepository_TenantIsolation(t *testing.T) {
	db := setupTestDB(t)
	typeRepo := NewContentTypeRepository(db)
	entryRepo := NewContentEntryRepository(db)

	ct1 := &models.ContentType{TenantID: 1, Name: "T1", UID: "t1-type"}
	ct2 := &models.ContentType{TenantID: 2, Name: "T2", UID: "t2-type"}
	if err := typeRepo.Create(ct1); err != nil {
		t.Fatalf("Create ct1: %v", err)
	}
	if err := typeRepo.Create(ct2); err != nil {
		t.Fatalf("Create ct2: %v", err)
	}

	entry := &models.ContentEntry{TenantID: 1, ContentTypeID: ct1.ID, DocumentID: "doc-t1", Data: models.JSONMap{"color": "red"}}
	if err := entryRepo.Create(entry); err != nil {
		t.Fatalf("Create entry: %v", err)
	}

	// Cross-tenant reads return not found / empty — even with a valid type id
	// of the other tenant and a guessed document_id.
	if _, err := entryRepo.FindByDocumentID(ct2.ID, "doc-t1", 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("FindByDocumentID cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	entries, total, err := entryRepo.List(ContentEntryListFilter{TypeID: ct2.ID, Page: 1, PageSize: 10}, 2)
	if err != nil || total != 0 || len(entries) != 0 {
		t.Fatalf("List cross-tenant = %d/%d/%v, want 0/0/nil", total, len(entries), err)
	}
	found, err := entryRepo.FindByIDs(ct2.ID, []uint{entry.ID}, 2)
	if err != nil || len(found) != 0 {
		t.Fatalf("FindByIDs cross-tenant = %d/%v, want 0/nil", len(found), err)
	}
	searched, err := entryRepo.Search(ct2.ID, "red", 10, 2)
	if err != nil || len(searched) != 0 {
		t.Fatalf("Search cross-tenant = %d/%v, want 0/nil", len(searched), err)
	}
	exported, err := entryRepo.ExportAll(ct2.ID, 2)
	if err != nil || len(exported) != 0 {
		t.Fatalf("ExportAll cross-tenant = %d/%v, want 0/nil", len(exported), err)
	}

	// Cross-tenant delete is a no-op.
	rows, err := entryRepo.DeleteByDocumentID(ct2.ID, "doc-t1", 2)
	if err != nil || rows != 0 {
		t.Fatalf("DeleteByDocumentID cross-tenant = %d/%v, want 0/nil", rows, err)
	}
	if _, err := entryRepo.FindByDocumentID(ct1.ID, "doc-t1", 1); err != nil {
		t.Fatalf("tenant 1 entry missing after cross-tenant delete: %v", err)
	}

	// i18n translation queries stay inside the tenant.
	trs, err := entryRepo.ListTranslations(ct2.ID, entry.ID, 0, 2)
	if err != nil || len(trs) != 0 {
		t.Fatalf("ListTranslations cross-tenant = %d/%v, want 0/nil", len(trs), err)
	}
	if _, err := entryRepo.FindTranslationInLocale(ct2.ID, entry.ID, "en", 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("FindTranslationInLocale cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}

	// A second tenant entry with its own document_id coexists fine
	// (composite document_id uniqueness is covered by migration tests).
	entry2 := &models.ContentEntry{TenantID: 2, ContentTypeID: ct2.ID, DocumentID: "doc-t2"}
	if err := entryRepo.Create(entry2); err != nil {
		t.Fatalf("Create tenant 2 entry: %v", err)
	}
}

func TestJSONFieldEqual_Dialects(t *testing.T) {
	cases := []struct {
		dialect    string
		wantClause string
		wantArgs   []interface{}
	}{
		{"postgres", "data::jsonb ->> ? = ?", []interface{}{"color", "red"}},
		{"mysql", "JSON_UNQUOTE(JSON_EXTRACT(data, ?)) = ?", []interface{}{"$.color", "red"}},
		{"sqlite", "json_extract(data, ?) = ?", []interface{}{"$.color", "red"}},
		{"unknown", "json_extract(data, ?) = ?", []interface{}{"$.color", "red"}},
	}
	for _, tc := range cases {
		clause, args := jsonFieldEqual(tc.dialect, "color", "red")
		if clause != tc.wantClause {
			t.Errorf("%s: clause = %q, want %q", tc.dialect, clause, tc.wantClause)
		}
		if len(args) != len(tc.wantArgs) {
			t.Fatalf("%s: got %d args, want %d", tc.dialect, len(args), len(tc.wantArgs))
		}
		for i := range args {
			if args[i] != tc.wantArgs[i] {
				t.Errorf("%s: args[%d] = %v, want %v", tc.dialect, i, args[i], tc.wantArgs[i])
			}
		}
	}
}

func TestContentEntryRepository_List_JSONFilters(t *testing.T) {
	db := setupTestDB(t)
	typeRepo := NewContentTypeRepository(db)
	entryRepo := NewContentEntryRepository(db)

	ct := &models.ContentType{Name: "Product", UID: "product-filter"}
	if err := typeRepo.Create(ct); err != nil {
		t.Fatalf("Create type: %v", err)
	}

	entries := []*models.ContentEntry{
		{ContentTypeID: ct.ID, DocumentID: "p-1", Data: models.JSONMap{"color": "red", "size": "large"}},
		{ContentTypeID: ct.ID, DocumentID: "p-2", Data: models.JSONMap{"color": "blue", "size": "large"}},
		{ContentTypeID: ct.ID, DocumentID: "p-3", Data: models.JSONMap{"color": "red", "size": "small"}},
	}
	for _, e := range entries {
		if err := entryRepo.Create(e); err != nil {
			t.Fatalf("Create entry %s: %v", e.DocumentID, err)
		}
	}

	// Single-field filter hits two entries.
	got, total, err := entryRepo.List(ContentEntryListFilter{
		TypeID: ct.ID, Page: 1, PageSize: 10,
		Filters: map[string]string{"color": "red"},
	}, 1)
	if err != nil {
		t.Fatalf("List(color=red): %v", err)
	}
	if total != 2 || len(got) != 2 {
		t.Fatalf("expected 2 matches for color=red, got total=%d len=%d", total, len(got))
	}

	// Combined filters narrow down to one entry.
	got, total, err = entryRepo.List(ContentEntryListFilter{
		TypeID: ct.ID, Page: 1, PageSize: 10,
		Filters: map[string]string{"color": "red", "size": "small"},
	}, 1)
	if err != nil {
		t.Fatalf("List(color=red,size=small): %v", err)
	}
	if total != 1 || len(got) != 1 || got[0].DocumentID != "p-3" {
		t.Fatalf("expected only p-3, got total=%d entries=%v", total, got)
	}

	// Miss: no entry has this value.
	_, total, err = entryRepo.List(ContentEntryListFilter{
		TypeID: ct.ID, Page: 1, PageSize: 10,
		Filters: map[string]string{"color": "green"},
	}, 1)
	if err != nil {
		t.Fatalf("List(color=green): %v", err)
	}
	if total != 0 {
		t.Fatalf("expected 0 matches for color=green, got %d", total)
	}
}

func TestContentEntryRepository_List_SkipsInvalidFilterNames(t *testing.T) {
	db := setupTestDB(t)
	typeRepo := NewContentTypeRepository(db)
	entryRepo := NewContentEntryRepository(db)

	ct := &models.ContentType{Name: "Guarded", UID: "guarded-filter"}
	typeRepo.Create(ct)
	entryRepo.Create(&models.ContentEntry{
		ContentTypeID: ct.ID, DocumentID: "g-1",
		Data: models.JSONMap{"color": "red"},
	})

	// Malformed field names (from arbitrary query params) must be skipped
	// instead of producing a JSON path error; valid ones still apply.
	got, total, err := entryRepo.List(ContentEntryListFilter{
		TypeID: ct.ID, Page: 1, PageSize: 10,
		Filters: map[string]string{`bad"]name`: "x", "drop table": "y", "color": "red"},
	}, 1)
	if err != nil {
		t.Fatalf("List with invalid filter names: %v", err)
	}
	if total != 1 || len(got) != 1 || got[0].DocumentID != "g-1" {
		t.Fatalf("expected g-1 to match, got total=%d entries=%v", total, got)
	}
}
