package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalDriver stores files on the local filesystem.
type LocalDriver struct {
	basePath string
	urlPrefix string
}

// NewLocalDriver creates a new local storage driver.
func NewLocalDriver(basePath, urlPrefix string) *LocalDriver {
	os.MkdirAll(basePath, 0755)
	return &LocalDriver{basePath: basePath, urlPrefix: urlPrefix}
}

func (d *LocalDriver) Upload(_ context.Context, key string, reader io.Reader, _ string) (string, error) {
	fullPath := filepath.Join(d.basePath, key)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return d.GetURL(key), nil
}

func (d *LocalDriver) Delete(_ context.Context, key string) error {
	fullPath := filepath.Join(d.basePath, key)
	err := os.Remove(fullPath)
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
