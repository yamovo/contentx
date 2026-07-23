package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalDriver_UploadAndDelete(t *testing.T) {
	dir := t.TempDir()
	d := NewLocalDriver(dir, "/uploads")

	content := []byte("hello world")
	url, err := d.Upload(context.Background(), "2024/01/test.txt", bytes.NewReader(content), "text/plain")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if url != "/uploads/2024/01/test.txt" {
		t.Fatalf("unexpected URL: %s", url)
	}

	// Verify file exists on disk.
	fullPath := filepath.Join(dir, "2024", "01", "test.txt")
	got, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("content mismatch: got %q, want %q", got, content)
	}

	// Delete should remove the file.
	if err := d.Delete(context.Background(), "2024/01/test.txt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		t.Fatalf("file should not exist after delete")
	}
}

func TestLocalDriver_DeleteNonExistent_NoError(t *testing.T) {
	dir := t.TempDir()
	d := NewLocalDriver(dir, "/uploads")

	// Deleting a non-existent file should return nil (idempotent).
	if err := d.Delete(context.Background(), "nope.txt"); err != nil {
		t.Fatalf("Delete non-existent should return nil, got: %v", err)
	}
}

func TestLocalDriver_GetURL(t *testing.T) {
	d := NewLocalDriver(t.TempDir(), "/static")
	if got := d.GetURL("img/photo.png"); got != "/static/img/photo.png" {
		t.Fatalf("GetURL: got %q", got)
	}
}

func TestLocalDriver_GetSignedURL_EqualsGetURL(t *testing.T) {
	d := NewLocalDriver(t.TempDir(), "/static")
	// Local driver has no signing; GetSignedURL delegates to GetURL.
	if got := d.GetSignedURL("file.txt", 0); got != d.GetURL("file.txt") {
		t.Fatalf("GetSignedURL should equal GetURL for local driver")
	}
}

func TestLocalDriver_UploadNestedDirs(t *testing.T) {
	dir := t.TempDir()
	d := NewLocalDriver(dir, "/u")

	// Deeply nested key should create all intermediate directories.
	key := "a/b/c/d/file.bin"
	if _, err := d.Upload(context.Background(), key, strings.NewReader("x"), "application/octet-stream"); err != nil {
		t.Fatalf("Upload nested: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a", "b", "c", "d", "file.bin")); err != nil {
		t.Fatalf("nested file should exist: %v", err)
	}
}

// ─── Path traversal security tests (Round 6 / F6) ───

func TestLocalDriver_Upload_PathTraversal_Rejected(t *testing.T) {
	dir := t.TempDir()
	d := NewLocalDriver(dir, "/uploads")

	cases := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32",
		"foo/../../../bar",
		"foo/../../baz",
	}
	for _, key := range cases {
		t.Run(key, func(t *testing.T) {
			_, err := d.Upload(context.Background(), key, strings.NewReader("pwned"), "text/plain")
			if err == nil {
				t.Fatalf("Upload with traversal key %q should fail", key)
			}
			if !strings.Contains(err.Error(), "escapes base path") && !strings.Contains(err.Error(), "invalid key") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLocalDriver_Delete_PathTraversal_Rejected(t *testing.T) {
	dir := t.TempDir()
	// Place a sentinel file outside basePath to prove Delete can't reach it.
	sentinelDir := filepath.Join(dir, "outside")
	_ = os.MkdirAll(sentinelDir, 0755)
	sentinel := filepath.Join(sentinelDir, "secret.txt")
	if err := os.WriteFile(sentinel, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	basePath := filepath.Join(dir, "uploads")
	d := NewLocalDriver(basePath, "/uploads")

	// Attempt to delete the sentinel via traversal.
	if err := d.Delete(context.Background(), "../outside/secret.txt"); err == nil {
		t.Fatal("Delete with traversal key should fail")
	}

	// Sentinel must still exist.
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel file should still exist after rejected delete: %v", err)
	}
}

func TestLocalDriver_SafePath_ValidKeys(t *testing.T) {
	d := NewLocalDriver("/tmp/uploads", "/u")

	valid := []string{
		"file.txt",
		"2024/01/file.txt",
		"a/b/c/d.txt",
		"photo.png",
	}
	for _, key := range valid {
		t.Run(key, func(t *testing.T) {
			_, err := d.safePath(key)
			if err != nil {
				t.Fatalf("safePath(%q) should succeed: %v", key, err)
			}
		})
	}
}
