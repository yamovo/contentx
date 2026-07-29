package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// S3Driver stores files on S3-compatible storage (AWS S3, MinIO, Cloudflare R2,
// Alibaba OSS) using the official minio-go-v7 client. The client handles AWS
// Signature V4, multipart uploads, retries, and S3 error codes — replacing the
// previous hand-rolled V2 signer that was incompatible with modern services.
type S3Driver struct {
	config    S3Config
	publicURL string
	client    *minio.Client
}

// NewS3Driver creates a new S3 storage driver backed by minio-go-v7.
// The bucket is auto-created on first use if it does not exist.
func NewS3Driver(cfg S3Config) *S3Driver {
	// minio-go requires a bare host:port. Normalize once and reuse the value
	// for both the client and the derived public URL.
	endpoint := strings.TrimSuffix(
		strings.TrimPrefix(strings.TrimPrefix(cfg.Endpoint, "https://"), "http://"),
		"/",
	)

	publicURL := strings.TrimSuffix(cfg.PublicURL, "/")
	if publicURL == "" {
		scheme := "https"
		if !cfg.UseSSL {
			scheme = "http"
		}
		if cfg.PathStyle {
			publicURL = fmt.Sprintf("%s://%s/%s", scheme, endpoint, cfg.Bucket)
		} else {
			publicURL = fmt.Sprintf("%s://%s.%s", scheme, cfg.Bucket, endpoint)
		}
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       cfg.UseSSL,
		Region:       cfg.Region,
		BucketLookup: bucketLookupType(cfg.PathStyle),
		Transport:    otelhttp.NewTransport(http.DefaultTransport),
	})
	if err != nil {
		// Configuration errors are programmer bugs; fail loud. NewS3Driver is
		// called at startup, not per-request.
		panic(fmt.Sprintf("storage: invalid S3 config: %v", err))
	}

	return &S3Driver{
		config:    cfg,
		publicURL: publicURL,
		client:    client,
	}
}

func bucketLookupType(pathStyle bool) minio.BucketLookupType {
	if pathStyle {
		return minio.BucketLookupPath
	}
	return minio.BucketLookupAuto
}

// Upload stores a file and returns its public URL. Uses multipart upload for
// large files (handled automatically by minio-go when size is unknown).
func (d *S3Driver) Upload(ctx context.Context, key string, reader io.Reader, contentType string) (string, error) {
	// Ensure the bucket exists (idempotent — MinIO returns no error if it
	// already exists; AWS requires us to check first).
	if err := d.ensureBucket(ctx); err != nil {
		return "", fmt.Errorf("ensure bucket: %w", err)
	}

	_, err := d.client.PutObject(ctx, d.config.Bucket, key, reader, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("upload %q: %w", key, err)
	}

	return d.GetURL(key), nil
}

// Delete removes a file. A 404 (NoSuchKey) is treated as success so deletes
// are idempotent, matching the previous driver's behaviour.
func (d *S3Driver) Delete(ctx context.Context, key string) error {
	err := d.client.RemoveObject(ctx, d.config.Bucket, key, minio.RemoveObjectOptions{})
	if err == nil {
		return nil
	}
	// minio-go already returns a typed errorResponse for 404; treat it as nil.
	if isNotFound(err) {
		return nil
	}
	return fmt.Errorf("delete %q: %w", key, err)
}

// GetURL returns the public URL for a key.
func (d *S3Driver) GetURL(key string) string {
	return d.publicURL + "/" + key
}

// GetSignedURL returns a time-limited presigned URL for private access.
func (d *S3Driver) GetSignedURL(key string, expiry time.Duration) string {
	// PresignedGetObject is a network call only insofar as signing; it does
	// not contact the server. We use context.Background() because the Driver
	// interface's GetSignedURL has no ctx parameter.
	u, err := d.client.PresignedGetObject(context.Background(), d.config.Bucket, key, expiry, nil)
	if err != nil {
		// Signing failures are configuration errors; fall back to the plain
		// URL so callers still get a usable link (with no signature).
		return d.GetURL(key)
	}
	return u.String()
}

// ensureBucket creates the bucket if it does not exist. Safe to call on every
// upload — BucketExists + MakeBucket are idempotent. Skipped on AWS when the
// bucket already exists.
func (d *S3Driver) ensureBucket(ctx context.Context) error {
	exists, err := d.client.BucketExists(ctx, d.config.Bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return d.client.MakeBucket(ctx, d.config.Bucket, minio.MakeBucketOptions{
		Region: d.config.Region,
	})
}

// isNotFound reports whether err is an S3 NoSuchKey / 404 response.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	resp := minio.ToErrorResponse(err)
	return resp.Code == "NoSuchKey" || resp.StatusCode == 404
}

// Client returns the underlying minio.Client, exposed for integration tests
// and advanced callers that need direct SDK access (e.g. bucket policies).
func (d *S3Driver) Client() *minio.Client { return d.client }
