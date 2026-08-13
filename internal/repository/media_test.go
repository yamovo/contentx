package repository

import (
	"testing"

	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

func TestMediaRepository_TenantIsolation(t *testing.T) {
	db := setupTestDB(t)
	repo := NewMediaRepository(db)

	media := &models.Media{
		TenantID:     1,
		Filename:     "t1.jpg",
		OriginalName: "t1.jpg",
		FilePath:     "/t1.jpg",
		MimeType:     "image/jpeg",
		FileSize:     100,
		Folder:       "/",
		UploaderID:   1,
	}
	if err := repo.Create(media); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Cross-tenant reads.
	if _, err := repo.GetByID(media.ID, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("GetByID cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	if _, err := repo.FindByID(media.ID, 2); err != gorm.ErrRecordNotFound {
		t.Fatalf("FindByID cross-tenant = %v, want gorm.ErrRecordNotFound", err)
	}
	if list, err := repo.FindByIDs([]uint{media.ID}, 2); err != nil || len(list) != 0 {
		t.Fatalf("FindByIDs cross-tenant = %d/%v, want 0/nil", len(list), err)
	}
	_, total, err := repo.List(MediaListFilter{}, 2)
	if err != nil || total != 0 {
		t.Fatalf("List cross-tenant = %d/%v, want 0/nil", total, err)
	}
	folders, err := repo.ListFolders(2)
	if err != nil || len(folders) != 0 {
		t.Fatalf("ListFolders cross-tenant = %v/%v, want empty/nil", folders, err)
	}
	stats, err := repo.Stats(2)
	if err != nil || stats.TotalFiles != 0 {
		t.Fatalf("Stats cross-tenant = %+v/%v, want zero/nil", stats, err)
	}

	// Cross-tenant writes are no-ops.
	if err := repo.UpdateFields(media.ID, map[string]interface{}{"alt": "hacked"}, 2); err != nil {
		t.Fatalf("UpdateFields cross-tenant: %v", err)
	}
	if n, err := repo.DeleteByIDs([]uint{media.ID}, 2); err != nil || n != 0 {
		t.Fatalf("DeleteByIDs cross-tenant = %d/%v, want 0/nil", n, err)
	}
	if err := repo.Delete(media, 2); err != nil {
		t.Fatalf("Delete cross-tenant: %v", err)
	}

	// Tenant 1 row intact.
	got, err := repo.FindByID(media.ID, 1)
	if err != nil || got.Alt != "" || got.Filename != "t1.jpg" {
		t.Fatalf("tenant 1 media changed after cross-tenant writes: %v/%+v", err, got)
	}
}
