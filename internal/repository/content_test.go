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

	got, err := repo.FindByUID("product")
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
	_, err := repo.FindByUID("nonexistent")
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestContentTypeRepository_CountByUID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewContentTypeRepository(db)
	repo.Create(&models.ContentType{Name: "Blog", UID: "blog"})

	count, err := repo.CountByUID("blog")
	if err != nil {
		t.Fatalf("CountByUID: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count=1, got %d", count)
	}

	count, _ = repo.CountByUID("nonexistent")
	if count != 0 {
		t.Fatalf("expected count=0 for nonexistent UID, got %d", count)
	}
}

func TestContentTypeRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewContentTypeRepository(db)
	repo.Create(&models.ContentType{Name: "A", UID: "a"})
	repo.Create(&models.ContentType{Name: "B", UID: "b"})

	types, err := repo.List()
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

	types, _ := repo.List()
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

	if err := repo.Delete(ct.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify type is gone.
	_, err := repo.FindByID(ct.ID)
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

	count, err := repo.CountEntriesByTypeID(ct.ID)
	if err != nil {
		t.Fatalf("CountEntriesByTypeID: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 entries, got %d", count)
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
	})
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
	})
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
	})
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
	})
	if err != nil {
		t.Fatalf("List with invalid filter names: %v", err)
	}
	if total != 1 || len(got) != 1 || got[0].DocumentID != "g-1" {
		t.Fatalf("expected g-1 to match, got total=%d entries=%v", total, got)
	}
}
