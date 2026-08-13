package repository

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

func TestCategoryRepository_TenantIsolation(t *testing.T) {
	db := setupTestDB(t)
	repo := NewCategoryRepository(db)

	cat := &models.Category{TenantID: 1, Name: "T1", Slug: "t1"}
	if err := repo.Create(cat); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Cross-tenant reads return not found / empty.
	if _, err := repo.GetByID(cat.ID, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("GetByID cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	if _, err := repo.FindByID(cat.ID, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("FindByID cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	cats, err := repo.List(true, 2)
	if err != nil || len(cats) != 0 {
		t.Fatalf("List cross-tenant = %d/%v, want 0/nil", len(cats), err)
	}

	// Cross-tenant writes are no-ops; row stays intact for tenant 1.
	if err := repo.Delete(cat.ID, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("Delete cross-tenant = %v, want gorm.ErrRecordNotFound (no-op)", err)
	}
	if err := repo.UpdateFields(cat.ID, map[string]interface{}{"name": "Hacked"}, 2); err != nil {
		t.Fatalf("UpdateFields cross-tenant: %v", err)
	}
	got, err := repo.FindByID(cat.ID, 1)
	if err != nil || got.Name != "T1" {
		t.Fatalf("tenant 1 category changed after cross-tenant writes: %v/%+v", err, got)
	}

	// Slug uniqueness is tenant-scoped.
	slug, err := repo.EnsureUniqueSlug("t1", 0, 2)
	if err != nil || slug != "t1" {
		t.Fatalf("EnsureUniqueSlug cross-tenant = %q/%v, want t1/nil", slug, err)
	}
	slug2, err := repo.EnsureUniqueSlug("t1", 0, 1)
	if err != nil || slug2 == "t1" {
		t.Fatalf("EnsureUniqueSlug same-tenant should suffix, got %q/%v", slug2, err)
	}
}
