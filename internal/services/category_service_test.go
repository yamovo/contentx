package services

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
)

func TestCategoryService_Create(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCategoryService(db)

	req := CreateCategoryRequest{Name: "Tech"}
	cat, err := svc.Create(req)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if cat.Name != "Tech" {
		t.Errorf("Name = %q, want %q", cat.Name, "Tech")
	}
	if cat.Slug == "" {
		t.Error("Slug should be auto-generated")
	}
}

func TestCategoryService_Create_UniqueSlug(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCategoryService(db)

	// Use an explicit slug to test that duplicate slugs get deduplicated.
	cat1, err := svc.Create(CreateCategoryRequest{Name: "First", Slug: "dup-slug"})
	if err != nil {
		t.Fatalf("Create() first error: %v", err)
	}

	cat2, err := svc.Create(CreateCategoryRequest{Name: "Second", Slug: "dup-slug"})
	if err != nil {
		t.Fatalf("Create() second error: %v", err)
	}

	if cat1.Slug != "dup-slug" {
		t.Errorf("First slug = %q, want %q", cat1.Slug, "dup-slug")
	}
	if cat2.Slug == "dup-slug" {
		t.Errorf("Second slug should be unique, got %q", cat2.Slug)
	}
}

func TestCategoryService_List_Tree(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCategoryService(db)

	parent, _ := svc.Create(CreateCategoryRequest{Name: "Parent"})
	svc.Create(CreateCategoryRequest{Name: "Child", ParentID: &parent.ID})

	cats, err := svc.List(true)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(cats) < 2 {
		t.Errorf("Expected >= 2 categories, got %d", len(cats))
	}
}

func TestCategoryService_Update(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCategoryService(db)

	cat, _ := svc.Create(CreateCategoryRequest{Name: "Original"})

	err := svc.Update(cat.ID, CreateCategoryRequest{Name: "Updated"})
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	got, _ := svc.Get(cat.ID)
	if got.Name != "Updated" {
		t.Errorf("Name = %q, want %q", got.Name, "Updated")
	}
}

func TestCategoryService_Delete(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCategoryService(db)

	cat, _ := svc.Create(CreateCategoryRequest{Name: "ToDelete"})

	err := svc.Delete(cat.ID)
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err = svc.Get(cat.ID)
	if err == nil {
		t.Error("Get() should fail after delete")
	}
}

func TestCategoryService_Reorder(t *testing.T) {
	db := setupTestDB(t)
	svc := NewCategoryService(db)

	c1, _ := svc.Create(CreateCategoryRequest{Name: "First"})
	c2, _ := svc.Create(CreateCategoryRequest{Name: "Second"})

	err := svc.Reorder([]ReorderItem{
		{ID: c1.ID, SortOrder: 10},
		{ID: c2.ID, SortOrder: 5},
	})
	if err != nil {
		t.Fatalf("Reorder() error: %v", err)
	}

	got1, _ := svc.Get(c1.ID)
	got2, _ := svc.Get(c2.ID)
	if got1.SortOrder != 10 || got2.SortOrder != 5 {
		t.Errorf("SortOrder: got %d/%d, want 10/5", got1.SortOrder, got2.SortOrder)
	}
}

func TestBuildCategoryTree(t *testing.T) {
	p1 := models.Category{BaseModel: models.BaseModel{ID: 1}, Name: "P1"}
	p2 := models.Category{BaseModel: models.BaseModel{ID: 2}, Name: "P2"}
	c1 := models.Category{BaseModel: models.BaseModel{ID: 3}, Name: "C1", ParentID: uintPtr(1)}
	c2 := models.Category{BaseModel: models.BaseModel{ID: 4}, Name: "C2", ParentID: uintPtr(1)}
	c3 := models.Category{BaseModel: models.BaseModel{ID: 5}, Name: "C3", ParentID: uintPtr(2)}

	cats := []models.Category{p1, p2, c1, c2, c3}
	tree := BuildCategoryTree(cats, nil)

	if len(tree) != 2 {
		t.Errorf("Root nodes = %d, want 2", len(tree))
	}

	// Find P1's children.
	for _, node := range tree {
		if node.Name == "P1" {
			if len(node.Children) != 2 {
				t.Errorf("P1 children = %d, want 2", len(node.Children))
			}
		}
		if node.Name == "P2" {
			if len(node.Children) != 1 {
				t.Errorf("P2 children = %d, want 1", len(node.Children))
			}
		}
	}
}

func uintPtr(v uint) *uint {
	return &v
}
