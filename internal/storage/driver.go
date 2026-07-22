package storage

import (
	"context"
	"io"
	"time"
)

// Driver defines the interface for file storage backends.
type Driver interface {
	// Upload stores a file and returns its public URL.
	Upload(ctx context.Context, key string, reader io.Reader, contentType string) (string, error)

	// Delete removes a file.
	Delete(ctx context.Context, key string) error

	// GetURL returns the public URL for a key.
	GetURL(key string) string

	// GetSignedURL returns a time-limited signed URL (for private files).
	GetSignedURL(key string, expiry time.Duration) string
}

// Config holds storage configuration.
type Config struct {
	Driver    string // "local" or "s3"
	LocalPath string
	URLPrefix string
	S3        S3Config
}

// S3Config holds S3-compatible storage settings.
type S3Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	PublicURL string // CDN URL override
	UseSSL    bool
	PathStyle bool // for MinIO
}
