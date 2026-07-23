package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalDriver stores files on the local filesystem.
type LocalDriver struct {
	basePath  string
	urlPrefix string
}

// NewLocalDriver creates a new local storage driver.
func NewLocalDriver(basePath, urlPrefix string) *LocalDriver {
	_ = os.MkdirAll(basePath, 0755)
	return &LocalDriver{basePath: basePath, urlPrefix: urlPrefix}
}

// safePath joins the key with basePath and verifies the resolved path stays
// within basePath. This prevents path traversal attacks via keys containing
// ".." segments or absolute paths (Round 6 / F6 security fix).
//
// Backslashes are normalized to forward slashes so the check works
// consistently on both Windows and Linux — a Linux server must still reject
// Windows-style "..\.." traversal attempts.
func (d *LocalDriver) safePath(key string) (string, error) {
	normalized := strings.ReplaceAll(key, "\\", "/")
	fullPath := filepath.Join(d.basePath, normalized)
	rel, err := filepath.Rel(d.basePath, fullPath)
	if err != nil {
		return "", fmt.Errorf("invalid key %q: %w", key, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid key %q: escapes base path", key)
	}
	return fullPath, nil
}

func (d *LocalDriver) Upload(_ context.Context, key string, reader io.Reader, _ string) (string, error) {
	fullPath, err := d.safePath(key)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(f, reader); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return d.GetURL(key), nil
}

func (d *LocalDriver) Delete(_ context.Context, key string) error {
	fullPath, err := d.safePath(key)
	if err != nil {
		return err
	}
	err = os.Remove(fullPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (d *LocalDriver) GetURL(key string) string {
	return d.urlPrefix + "/" + key
}

func (d *LocalDriver) GetSignedURL(key string, _ time.Duration) string {
	return d.GetURL(key)
}
