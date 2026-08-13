package repository

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

func TestTagRepository_TenantIsolation(t *testing.T) {
	db := setupTestDB(t)
	repo := NewTagRepository(db)

	tag := &models.Tag{TenantID: 1, Name: "T1", Slug: "t1"}
	if err := repo.Create(tag); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Cross-tenant reads.
	if _, err := repo.GetByID(tag.ID, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("GetByID cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	if _, err := repo.FindByID(tag.ID, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("FindByID cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	_, total, err := repo.List(TagListFilter{}, 2)
	if err != nil || total != 0 {
		t.Fatalf("List cross-tenant = %d/%v, want 0/nil", total, err)
	}

	// Cross-tenant writes are no-ops / rejected.
	if err := repo.UpdateFields(tag.ID, map[string]interface{}{"name": "Hacked"}, 2); err != nil {
		t.Fatalf("UpdateFields cross-tenant: %v", err)
	}
	if err := repo.ClearArticleAssociations(tag.ID, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("ClearArticleAssociations cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	if err := repo.MergeTags(tag.ID, tag.ID+1, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("MergeTags cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	if _, err := repo.CountArticleAssociations(tag.ID, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("CountArticleAssociations cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	if n, err := repo.DeleteByIDs([]uint{tag.ID}, 2); err != nil || n != 0 {
		t.Fatalf("DeleteByIDs cross-tenant = %d/%v, want 0/nil", n, err)
	}
	if err := repo.Delete(tag, 2); err != nil {
		t.Fatalf("Delete cross-tenant: %v", err)
	}

	// Tenant 1 row intact.
	got, err := repo.FindByID(tag.ID, 1)
	if err != nil || got.Name != "T1" {
		t.Fatalf("tenant 1 tag changed after cross-tenant writes: %v/%+v", err, got)
	}
}
