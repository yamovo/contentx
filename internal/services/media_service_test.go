package services

import (
	"testing"

	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// createTestMedia inserts a media record directly for testing.
func createTestMedia(t *testing.T, db *gorm.DB, filename, mimeType string, size int64, folder string) *models.Media {
	t.Helper()
	media := models.Media{
		Filename:     filename,
		OriginalName: filename,
		FilePath:     "/tmp/" + filename,
		URL:          "/uploads/" + filename,
		MimeType:     mimeType,
		FileSize:     size,
		Folder:       folder,
	}
	if err := db.Create(&media).Error; err != nil {
		t.Fatalf("create test media: %v", err)
	}
	return &media
}

// ─── MediaService Tests ─────────────────────────────────────────────────────

func TestMediaService_List_Empty(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMediaService(db, config.UploadConfig{StoragePath: "/tmp", URLPrefix: "/uploads"})

	media, total, err := svc.List(MediaListParams{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list media: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected 0 media, got %d", total)
	}
	if len(media) != 0 {
		t.Fatalf("expected empty media, got %d", len(media))
	}
}

func TestMediaService_List_WithData(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMediaService(db, config.UploadConfig{StoragePath: "/tmp", URLPrefix: "/uploads"})

	createTestMedia(t, db, "img1.jpg", "image/jpeg", 1024, "/2024/01")
	createTestMedia(t, db, "img2.png", "image/png", 2048, "/2024/01")
	createTestMedia(t, db, "doc.pdf", "application/pdf", 5120, "/docs")

	media, total, err := svc.List(MediaListParams{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list media: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected 3 media, got %d", total)
	}
	if len(media) != 3 {
		t.Fatalf("expected 3 items, got %d", len(media))
	}
}

func TestMediaService_List_FilterByMimeType(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMediaService(db, config.UploadConfig{StoragePath: "/tmp", URLPrefix: "/uploads"})

	createTestMedia(t, db, "img1.jpg", "image/jpeg", 1024, "/img")
	createTestMedia(t, db, "img2.png", "image/png", 2048, "/img")
	createTestMedia(t, db, "doc.pdf", "application/pdf", 5120, "/docs")

	media, total, err := svc.List(MediaListParams{Page: 1, PageSize: 10, MimeType: "image"})
	if err != nil {
		t.Fatalf("list media: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 images, got %d", total)
	}
	if len(media) != 2 {
		t.Fatalf("expected 2 images, got %d", len(media))
	}
}

func TestMediaService_List_FilterByFolder(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMediaService(db, config.UploadConfig{StoragePath: "/tmp", URLPrefix: "/uploads"})

	createTestMedia(t, db, "a.jpg", "image/jpeg", 100, "/folder_a")
	createTestMedia(t, db, "b.jpg", "image/jpeg", 200, "/folder_b")

	media, total, _ := svc.List(MediaListParams{Page: 1, PageSize: 10, Folder: "/folder_a"})
	if total != 1 {
		t.Fatalf("expected 1 in folder_a, got %d", total)
	}
	if len(media) != 1 {
		t.Fatalf("expected 1 media, got %d", len(media))
	}
}

func TestMediaService_List_Search(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMediaService(db, config.UploadConfig{StoragePath: "/tmp", URLPrefix: "/uploads"})

	createTestMedia(t, db, "vacation_photo.jpg", "image/jpeg", 100, "/")
	createTestMedia(t, db, "work_document.pdf", "application/pdf", 200, "/")

	media, total, _ := svc.List(MediaListParams{Page: 1, PageSize: 10, Search: "vacation"})
	if total != 1 {
		t.Fatalf("expected 1 matching 'vacation', got %d", total)
	}
	if len(media) != 1 {
		t.Fatalf("expected 1 media, got %d", len(media))
	}
}

func TestMediaService_Get_Success(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMediaService(db, config.UploadConfig{StoragePath: "/tmp", URLPrefix: "/uploads"})

	created := createTestMedia(t, db, "get.jpg", "image/jpeg", 100, "/")

	media, err := svc.Get(created.ID)
	if err != nil {
		t.Fatalf("get media: %v", err)
	}
	if media.Filename != "get.jpg" {
		t.Fatalf("expected filename 'get.jpg', got '%s'", media.Filename)
	}
}

func TestMediaService_Get_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMediaService(db, config.UploadConfig{StoragePath: "/tmp", URLPrefix: "/uploads"})

	_, err := svc.Get(99999)
	if err == nil {
		t.Fatal("expected error for non-existent media")
	}
}

func TestMediaService_Update(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMediaService(db, config.UploadConfig{StoragePath: "/tmp", URLPrefix: "/uploads"})

	created := createTestMedia(t, db, "update.jpg", "image/jpeg", 100, "/")

	err := svc.Update(created.ID, UpdateMediaRequest{
		Alt:         "Alt text",
		Title:       "Title",
		Description: "Description",
		Folder:      "/new_folder",
	})
	if err != nil {
		t.Fatalf("update media: %v", err)
	}

	var refreshed models.Media
	db.First(&refreshed, created.ID)
	if refreshed.Alt != "Alt text" {
		t.Fatalf("expected alt 'Alt text', got '%s'", refreshed.Alt)
	}
	if refreshed.Folder != "/new_folder" {
		t.Fatalf("expected folder '/new_folder', got '%s'", refreshed.Folder)
	}
}

func TestMediaService_Delete(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMediaService(db, config.UploadConfig{StoragePath: "/tmp", URLPrefix: "/uploads"})

	created := createTestMedia(t, db, "delete.jpg", "image/jpeg", 100, "/")

	if err := svc.Delete(created.ID); err != nil {
		t.Fatalf("delete media: %v", err)
	}

	var count int64
	db.Model(&models.Media{}).Where("id = ?", created.ID).Count(&count)
	if count != 0 {
		t.Fatal("media should be deleted")
	}
}

func TestMediaService_BulkDelete(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMediaService(db, config.UploadConfig{StoragePath: "/tmp", URLPrefix: "/uploads"})

	m1 := createTestMedia(t, db, "bulk1.jpg", "image/jpeg", 100, "/")
	m2 := createTestMedia(t, db, "bulk2.jpg", "image/jpeg", 200, "/")
	m3 := createTestMedia(t, db, "bulk3.jpg", "image/jpeg", 300, "/")

	affected, err := svc.BulkDelete([]uint{m1.ID, m2.ID})
	if err != nil {
		t.Fatalf("bulk delete: %v", err)
	}
	if affected != 2 {
		t.Fatalf("expected 2 affected, got %d", affected)
	}

	var count int64
	db.Model(&models.Media{}).Where("id = ?", m3.ID).Count(&count)
	if count != 1 {
		t.Fatal("m3 should still exist")
	}
}

func TestMediaService_Folders(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMediaService(db, config.UploadConfig{StoragePath: "/tmp", URLPrefix: "/uploads"})

	createTestMedia(t, db, "a.jpg", "image/jpeg", 100, "/folder_a")
	createTestMedia(t, db, "b.jpg", "image/jpeg", 200, "/folder_b")
	createTestMedia(t, db, "c.jpg", "image/jpeg", 300, "/folder_a") // same folder

	folders, err := svc.Folders()
	if err != nil {
		t.Fatalf("folders: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("expected 2 distinct folders, got %d", len(folders))
	}
}

func TestMediaService_Stats(t *testing.T) {
	db := setupTestDB(t)
	svc := NewMediaService(db, config.UploadConfig{StoragePath: "/tmp", URLPrefix: "/uploads"})

	createTestMedia(t, db, "img1.jpg", "image/jpeg", 1024, "/")
	createTestMedia(t, db, "img2.png", "image/png", 2048, "/")
	createTestMedia(t, db, "doc.pdf", "application/pdf", 5120, "/")
	createTestMedia(t, db, "vid.mp4", "video/mp4", 10240, "/")

	stats, err := svc.Stats()
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.TotalFiles != 4 {
		t.Fatalf("expected 4 total files, got %d", stats.TotalFiles)
	}
	if stats.TotalSize != 18432 {
		t.Fatalf("expected total size 18432, got %d", stats.TotalSize)
	}
	if stats.Images != 2 {
		t.Fatalf("expected 2 images, got %d", stats.Images)
	}
	if stats.Videos != 1 {
		t.Fatalf("expected 1 video, got %d", stats.Videos)
	}
	if stats.Documents != 1 {
		t.Fatalf("expected 1 document, got %d", stats.Documents)
	}
}
