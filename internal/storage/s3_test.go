package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

// newTestS3Driver creates an S3Driver with the given config. Used by config-
// level tests that don't make network calls.
func newTestS3Driver(t *testing.T, cfg S3Config) *S3Driver {
	t.Helper()
	// Avoid panicking on bad config in tests by recovering; tests that
	// intentionally pass bad config should call NewS3Driver directly.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewS3Driver panicked: %v", r)
		}
	}()
	return NewS3Driver(cfg)
}

func TestS3Driver_GetURL(t *testing.T) {
	d := newTestS3Driver(t, S3Config{
		Endpoint:  "s3.example.com",
		Bucket:    "mybucket",
		PublicURL: "https://cdn.example.com",
	})
	if got := d.GetURL("img.png"); got != "https://cdn.example.com/img.png" {
		t.Fatalf("GetURL: got %q", got)
	}
}

func TestS3Driver_NewS3Driver_PublicURLDerivation(t *testing.T) {
	cases := []struct {
		name    string
		cfg     S3Config
		wantURL string
	}{
		{
			name: "path style no SSL",
			cfg: S3Config{
				Endpoint:  "minio.local:9000",
				Bucket:    "test",
				UseSSL:    false,
				PathStyle: true,
			},
			wantURL: "http://minio.local:9000/test",
		},
		{
			name: "path style with SSL",
			cfg: S3Config{
				Endpoint:  "minio.local:9000",
				Bucket:    "test",
				UseSSL:    true,
				PathStyle: true,
			},
			wantURL: "https://minio.local:9000/test",
		},
		{
			name: "virtual-hosted no SSL",
			cfg: S3Config{
				Endpoint:  "s3.amazonaws.com",
				Bucket:    "mybucket",
				UseSSL:    false,
				PathStyle: false,
			},
			wantURL: "http://mybucket.s3.amazonaws.com",
		},
		{
			name: "virtual-hosted with SSL",
			cfg: S3Config{
				Endpoint:  "s3.amazonaws.com",
				Bucket:    "mybucket",
				UseSSL:    true,
				PathStyle: false,
			},
			wantURL: "https://mybucket.s3.amazonaws.com",
		},
		{
			name: "public URL override",
			cfg: S3Config{
				Endpoint:  "s3.amazonaws.com",
				Bucket:    "mybucket",
				PublicURL: "https://cdn.example.com",
			},
			wantURL: "https://cdn.example.com",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := newTestS3Driver(t, c.cfg)
			if d.publicURL != c.wantURL {
				t.Fatalf("publicURL: got %q, want %q", d.publicURL, c.wantURL)
			}
		})
	}
}

func TestS3Driver_NewS3Driver_StripsSchemeFromEndpoint(t *testing.T) {
	// Regression: if the user configures Endpoint as "https://minio.local:9000"
	// (with scheme), minio-go would fail. NewS3Driver must strip the scheme.
	cases := []string{"minio.local:9000", "http://minio.local:9000", "https://minio.local:9000"}
	for _, ep := range cases {
		d := newTestS3Driver(t, S3Config{Endpoint: ep, Bucket: "b", UseSSL: false, PathStyle: true})
		if d.client == nil {
			t.Fatalf("client should not be nil for endpoint %q", ep)
		}
		if d.GetURL("file.txt") != "http://minio.local:9000/b/file.txt" {
			t.Fatalf("derived URL retained endpoint scheme for %q: %s", ep, d.GetURL("file.txt"))
		}
	}
}

func TestS3Driver_Client_returnsUnderlyingClient(t *testing.T) {
	d := newTestS3Driver(t, S3Config{Endpoint: "s3.example.com", Bucket: "b"})
	if d.Client() == nil {
		t.Fatal("Client() should return non-nil minio.Client")
	}
}

func TestS3Driver_isNotFound(t *testing.T) {
	if isNotFound(nil) {
		t.Error("nil should not be not-found")
	}
}

// ─── Integration tests ─────────────────────────────────────────────────────
//
// These tests require a real S3-compatible service (MinIO, R2, AWS S3).
// They are skipped unless the S3_TEST_ENDPOINT environment variable is set.
//
// Quick local setup with Docker:
//   docker run -p 9000:9000 -p 9001:9001 \
//     -e MINIO_ROOT_USER=minioadmin -e MINIO_ROOT_PASSWORD=minioadmin \
//     minio/minio server /data --console-address ":9001"
//
// Then run:
//   S3_TEST_ENDPOINT=localhost:9000 \
//   S3_TEST_ACCESS_KEY=minioadmin \
//   S3_TEST_SECRET_KEY=minioadmin \
//   go test ./internal/storage/ -run TestS3Integration -v -count=1

func integrationConfig(t *testing.T) S3Config {
	t.Helper()
	ep := os.Getenv("S3_TEST_ENDPOINT")
	if ep == "" {
		t.Skip("S3_TEST_ENDPOINT not set; skipping S3 integration test")
	}
	return S3Config{
		Endpoint:  ep,
		Bucket:    "contentx-test-" + strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(t.Name(), "/", "-"), "_", "-")),
		Region:    "us-east-1",
		AccessKey: os.Getenv("S3_TEST_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_TEST_SECRET_KEY"),
		UseSSL:    os.Getenv("S3_TEST_USE_SSL") == "true",
		PathStyle: true,
	}
}

func TestS3Integration_UploadDeleteRoundTrip(t *testing.T) {
	cfg := integrationConfig(t)
	d := NewS3Driver(cfg)
	ctx := context.Background()

	content := []byte("hello minio integration test")
	key := "test/upload-delete-roundtrip.txt"

	// Upload
	publicURL, err := d.Upload(ctx, key, bytes.NewReader(content), "text/plain")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if !strings.Contains(publicURL, key) {
		t.Errorf("publicURL %q should contain key %q", publicURL, key)
	}

	// Verify the object exists via the SDK directly (independent of our driver).
	obj, err := d.Client().GetObject(ctx, cfg.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer obj.Close()
	body, err := io.ReadAll(obj)
	if err != nil {
		t.Fatalf("read uploaded object: %v", err)
	}
	if !bytes.Equal(body, content) {
		t.Errorf("body mismatch: got %q, want %q", body, content)
	}

	// Delete (idempotent)
	if err := d.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := d.Client().StatObject(ctx, cfg.Bucket, key, minio.StatObjectOptions{}); !isNotFound(err) {
		t.Fatalf("StatObject after Delete error = %v, want not found", err)
	}
	// Delete again — should not error (idempotent).
	if err := d.Delete(ctx, key); err != nil {
		t.Errorf("Delete of missing key should be idempotent, got: %v", err)
	}
}

func TestS3Integration_GetSignedURL(t *testing.T) {
	cfg := integrationConfig(t)
	d := NewS3Driver(cfg)
	ctx := context.Background()

	key := "test/signed-url.txt"
	content := []byte("signed url content")
	_, err := d.Upload(ctx, key, bytes.NewReader(content), "text/plain")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	defer d.Delete(ctx, key)

	signed := d.GetSignedURL(key, 5*time.Minute)
	if !strings.Contains(signed, "X-Amz-Signature=") {
		t.Fatalf("signed URL should contain AWS V4 signature param: %s", signed)
	}
	resp, err := http.Get(signed) // #nosec G107 -- test URL points to configured local MinIO.
	if err != nil {
		t.Fatalf("GET signed URL: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET signed URL status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read signed URL response: %v", err)
	}
	if !bytes.Equal(body, content) {
		t.Fatalf("signed URL body mismatch: got %q, want %q", body, content)
	}
}

func TestS3Integration_BucketAutoCreate(t *testing.T) {
	cfg := integrationConfig(t)
	d := NewS3Driver(cfg)
	ctx := context.Background()

	// Upload to a fresh bucket — ensureBucket should auto-create it.
	_, err := d.Upload(ctx, "test/bucket-auto-create.txt", strings.NewReader("x"), "text/plain")
	if err != nil {
		t.Fatalf("Upload with auto bucket create: %v", err)
	}
	defer d.Delete(ctx, "test/bucket-auto-create.txt")

	exists, err := d.Client().BucketExists(ctx, cfg.Bucket)
	if err != nil {
		t.Fatalf("BucketExists: %v", err)
	}
	if !exists {
		t.Error("bucket should exist after upload")
	}
}

func TestS3Integration_MultipartUpload(t *testing.T) {
	cfg := integrationConfig(t)
	d := NewS3Driver(cfg)
	ctx := context.Background()

	// minio-go uses multipart upload for unknown-size streams larger than its
	// 16 MiB part size. Crossing that boundary exercises multipart signing and
	// completion against the real compatible service.
	content := bytes.Repeat([]byte("m"), 17<<20)
	key := "test/multipart.bin"
	_, err := d.Upload(ctx, key, bytes.NewReader(content), "application/octet-stream")
	if err != nil {
		t.Fatalf("multipart Upload: %v", err)
	}
	defer d.Delete(ctx, key)

	info, err := d.Client().StatObject(ctx, cfg.Bucket, key, minio.StatObjectOptions{})
	if err != nil {
		t.Fatalf("StatObject multipart upload: %v", err)
	}
	if info.Size != int64(len(content)) {
		t.Fatalf("multipart object size = %d, want %d", info.Size, len(content))
	}
}

func TestS3Integration_InvalidCredentialsRejected(t *testing.T) {
	cfg := integrationConfig(t)
	cfg.SecretKey = "definitely-wrong-secret"
	d := NewS3Driver(cfg)

	_, err := d.Upload(
		context.Background(),
		"test/should-not-upload.txt",
		strings.NewReader("denied"),
		"text/plain",
	)
	if err == nil {
		t.Fatal("Upload with invalid credentials should fail")
	}
}
