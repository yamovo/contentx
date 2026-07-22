package services

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/models"
	"gorm.io/gorm"
)

// newStorageTestConfig returns an UploadConfig suitable for storage-driver
// tests. StoragePath points to a temp dir so any accidental local-disk writes
// (which would indicate the driver path wasn't taken) are isolated.
func newStorageTestConfig(t *testing.T) config.UploadConfig {
	t.Helper()
	return config.UploadConfig{
		MaxSize:      10 << 20,
		AllowedTypes: []string{"image/png", "image/svg+xml", "text/plain"},
		StoragePath:  t.TempDir(),
		URLPrefix:    "/uploads",
	}
}

// pngContent builds a minimal PNG-magic-prefixed payload so
// http.DetectContentType reports image/png.
func pngContent(n int) []byte {
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	return append(pngHeader, bytes.Repeat([]byte{0x42}, n)...)
}

func TestStorage_SetStorageDriver(t *testing.T) {
	repo := &MockMediaRepository{}
	s := NewMediaServiceWithRepo(repo, newStorageTestConfig(t))
	if s.store != nil {
		t.Fatal("expected nil store by default")
	}
	d := &MockStorageDriver{BaseURL: "https://cdn.example.com"}
	s.SetStorageDriver(d)
	if s.store == nil {
		t.Fatal("expected non-nil store after SetStorageDriver")
	}
}

func TestStorage_Upload_DelegatesToDriver(t *testing.T) {
	repo := &MockMediaRepository{}
	store := &MockStorageDriver{BaseURL: "https://cdn.example.com"}
	s := NewMediaServiceWithRepo(repo, newStorageTestConfig(t))
	s.SetStorageDriver(store)

	content := pngContent(128)
	fh := newMultipartHeader(t, "photo.png", content)
	f, header := openFileHeader(t, fh)
	defer f.Close()

	media, err := s.Upload(f, header, "pics", "alt", "title", "cap", "desc", 7)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// Driver must have received exactly one Upload call.
	if len(store.UploadCalls) != 1 {
		t.Fatalf("expected 1 driver Upload call, got %d", len(store.UploadCalls))
	}
	call := store.UploadCalls[0]
	if !strings.HasPrefix(call.Key, "pics/") {
		t.Errorf("Upload key = %q, want prefix 'pics/'", call.Key)
	}
	if call.ContentType != "image/png" {
		t.Errorf("ContentType = %q, want image/png", call.ContentType)
	}
	if !bytes.Equal(call.Content, content) {
		t.Errorf("uploaded bytes do not match input (got %d, want %d)", len(call.Content), len(content))
	}

	// FilePath should store the object key (not a local path), and the URL
	// should come from the driver.
	if media.FilePath != call.Key {
		t.Errorf("FilePath = %q, want key %q", media.FilePath, call.Key)
	}
	if !strings.HasPrefix(media.URL, "https://cdn.example.com/") {
		t.Errorf("URL = %q, want cdn prefix", media.URL)
	}
	if !strings.Contains(media.URL, call.Key) {
		t.Errorf("URL = %q, want it to contain key %q", media.URL, call.Key)
	}

	// No local-disk file should have been created.
	localPath := filepath.Join(s.cfg.StoragePath, call.Key)
	if _, err := os.Stat(localPath); !os.IsNotExist(err) {
		t.Errorf("expected no local file at %q, got err=%v", localPath, err)
	}

	// repo.Create must have been invoked once with the same media record.
	if len(repo.CreatedMedia) != 1 {
		t.Fatalf("expected 1 repo.Create call, got %d", len(repo.CreatedMedia))
	}
	if repo.CreatedMedia[0].FilePath != call.Key {
		t.Errorf("repo.Create FilePath = %q, want %q", repo.CreatedMedia[0].FilePath, call.Key)
	}
}

func TestStorage_Upload_DriverError(t *testing.T) {
	repo := &MockMediaRepository{}
	store := &MockStorageDriver{
		BaseURL:   "https://cdn.example.com",
		UploadErr: gorm.ErrInvalidDB,
	}
	s := NewMediaServiceWithRepo(repo, newStorageTestConfig(t))
	s.SetStorageDriver(store)

	content := pngContent(64)
	fh := newMultipartHeader(t, "photo.png", content)
	f, header := openFileHeader(t, fh)
	defer f.Close()

	_, err := s.Upload(f, header, "", "", "", "", "", 1)
	if err == nil || !strings.Contains(err.Error(), "failed to upload to storage") {
		t.Fatalf("expected storage upload error, got %v", err)
	}
	// On driver failure, repo.Create must not be called.
	if len(repo.CreatedMedia) != 0 {
		t.Errorf("expected 0 repo.Create calls, got %d", len(repo.CreatedMedia))
	}
}

func TestStorage_Upload_RepoCreateError_CleansUpViaDriver(t *testing.T) {
	repo := &MockMediaRepository{CreateErr: gorm.ErrInvalidDB}
	store := &MockStorageDriver{BaseURL: "https://cdn.example.com"}
	s := NewMediaServiceWithRepo(repo, newStorageTestConfig(t))
	s.SetStorageDriver(store)

	content := pngContent(64)
	fh := newMultipartHeader(t, "photo.png", content)
	f, header := openFileHeader(t, fh)
	defer f.Close()

	_, err := s.Upload(f, header, "", "", "", "", "", 1)
	if err == nil {
		t.Fatal("expected error from repo.Create")
	}
	// Driver should have uploaded once...
	if len(store.UploadCalls) != 1 {
		t.Fatalf("expected 1 driver Upload call, got %d", len(store.UploadCalls))
	}
	// ...and then rolled back via Delete because repo.Create failed.
	if len(store.DeleteCalls) != 1 {
		t.Fatalf("expected 1 driver Delete (cleanup) call, got %d", len(store.DeleteCalls))
	}
	if store.DeleteCalls[0] != store.UploadCalls[0].Key {
		t.Errorf("cleanup Delete key = %q, want %q", store.DeleteCalls[0], store.UploadCalls[0].Key)
	}
}

func TestStorage_Upload_SVGSanitizedThenUploaded(t *testing.T) {
	repo := &MockMediaRepository{}
	store := &MockStorageDriver{BaseURL: "https://cdn.example.com"}
	s := NewMediaServiceWithRepo(repo, newStorageTestConfig(t))
	s.SetStorageDriver(store)

	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10" fill="red"/></svg>`)
	fh := newMultipartHeader(t, "icon.svg", svg)
	f, header := openFileHeader(t, fh)
	defer f.Close()

	media, err := s.Upload(f, header, "icons", "", "", "", "", 1)
	if err != nil {
		t.Fatalf("Upload SVG failed: %v", err)
	}
	if media.MimeType != "image/svg+xml" {
		t.Errorf("MimeType = %q, want image/svg+xml", media.MimeType)
	}
	if len(store.UploadCalls) != 1 {
		t.Fatalf("expected 1 driver Upload call, got %d", len(store.UploadCalls))
	}
	if store.UploadCalls[0].ContentType != "image/svg+xml" {
		t.Errorf("driver ContentType = %q, want image/svg+xml", store.UploadCalls[0].ContentType)
	}
}

func TestStorage_Delete_DelegatesToDriver(t *testing.T) {
	repo := &MockMediaRepository{
		FindMedia: &models.Media{
			BaseModel: models.BaseModel{ID: 9},
			FilePath:  "2024/01/abc.png",
		},
	}
	store := &MockStorageDriver{BaseURL: "https://cdn.example.com"}
	s := NewMediaServiceWithRepo(repo, newStorageTestConfig(t))
	s.SetStorageDriver(store)

	if err := s.Delete(9); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	// Driver must receive exactly one Delete with the object key.
	if len(store.DeleteCalls) != 1 {
		t.Fatalf("expected 1 driver Delete call, got %d", len(store.DeleteCalls))
	}
	if store.DeleteCalls[0] != "2024/01/abc.png" {
		t.Errorf("Delete key = %q, want 2024/01/abc.png", store.DeleteCalls[0])
	}
	// repo.Delete must have been called once.
	if len(repo.DeletedMedia) != 1 {
		t.Errorf("expected 1 repo.Delete call, got %d", len(repo.DeletedMedia))
	}
}

func TestStorage_Delete_RepoErrorStillReturnsError(t *testing.T) {
	repo := &MockMediaRepository{
		FindMedia: &models.Media{
			BaseModel: models.BaseModel{ID: 9},
			FilePath:  "key.png",
		},
		DeleteErr: gorm.ErrInvalidDB,
	}
	store := &MockStorageDriver{BaseURL: "https://cdn.example.com"}
	s := NewMediaServiceWithRepo(repo, newStorageTestConfig(t))
	s.SetStorageDriver(store)

	err := s.Delete(9)
	if err == nil {
		t.Fatal("expected error from repo.Delete")
	}
	// Driver Delete should still have been attempted (best-effort cleanup).
	if len(store.DeleteCalls) != 1 {
		t.Errorf("expected 1 driver Delete call, got %d", len(store.DeleteCalls))
	}
}

func TestStorage_BulkDelete_DelegatesToDriver(t *testing.T) {
	repo := &MockMediaRepository{
		FindByIDsRes: []models.Media{
			{BaseModel: models.BaseModel{ID: 1}, FilePath: "a/key1.png"},
			{BaseModel: models.BaseModel{ID: 2}, FilePath: "a/key2.png"},
			{BaseModel: models.BaseModel{ID: 3}, FilePath: "b/key3.png"},
		},
	}
	store := &MockStorageDriver{BaseURL: "https://cdn.example.com"}
	s := NewMediaServiceWithRepo(repo, newStorageTestConfig(t))
	s.SetStorageDriver(store)

	n, err := s.BulkDelete([]uint{1, 2, 3})
	if err != nil {
		t.Fatalf("BulkDelete failed: %v", err)
	}
	if n != 3 {
		t.Errorf("rows = %d, want 3", n)
	}
	// Driver must receive one Delete per file.
	if len(store.DeleteCalls) != 3 {
		t.Fatalf("expected 3 driver Delete calls, got %d", len(store.DeleteCalls))
	}
	want := map[string]bool{"a/key1.png": true, "a/key2.png": true, "b/key3.png": true}
	for _, key := range store.DeleteCalls {
		if !want[key] {
			t.Errorf("unexpected Delete key %q", key)
		}
	}
	if repo.DeleteByIDsCalls != 1 {
		t.Errorf("DeleteByIDsCalls = %d, want 1", repo.DeleteByIDsCalls)
	}
}

func TestStorage_BulkDelete_DriverDeleteErrorIgnored(t *testing.T) {
	// removeStoredFile intentionally ignores driver Delete errors (best-effort),
	// so a failing driver must not abort the bulk operation.
	repo := &MockMediaRepository{
		FindByIDsRes: []models.Media{
			{BaseModel: models.BaseModel{ID: 1}, FilePath: "key1.png"},
			{BaseModel: models.BaseModel{ID: 2}, FilePath: "key2.png"},
		},
	}
	store := &MockStorageDriver{
		BaseURL:   "https://cdn.example.com",
		DeleteErr: gorm.ErrInvalidDB,
	}
	s := NewMediaServiceWithRepo(repo, newStorageTestConfig(t))
	s.SetStorageDriver(store)

	n, err := s.BulkDelete([]uint{1, 2})
	if err != nil {
		t.Fatalf("BulkDelete should not surface driver Delete errors: %v", err)
	}
	if n != 2 {
		t.Errorf("rows = %d, want 2", n)
	}
	if len(store.DeleteCalls) != 2 {
		t.Errorf("expected 2 driver Delete attempts, got %d", len(store.DeleteCalls))
	}
}

func TestStorage_Delete_FallbackToLocalWhenNoDriver(t *testing.T) {
	// When store == nil, Delete must use the legacy local-disk path.
	dir := t.TempDir()
	filePath := filepath.Join(dir, "toDelete.png")
	if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	repo := &MockMediaRepository{
		FindMedia: &models.Media{
			BaseModel: models.BaseModel{ID: 7},
			FilePath:  filePath,
		},
	}
	s := NewMediaServiceWithRepo(repo, config.UploadConfig{StoragePath: dir, URLPrefix: "/uploads"})
	// No SetStorageDriver → legacy path.

	if err := s.Delete(7); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("expected local file removed, got err=%v", err)
	}
}
