package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestS3Driver creates an S3Driver pointing at the given test server.
func newTestS3Driver(t *testing.T, srv *httptest.Server, pathStyle bool) *S3Driver {
	t.Helper()
	cfg := S3Config{
		Endpoint:  srv.URL[strings.Index(srv.URL, "://")+3:],
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		AccessKey: "test-access",
		SecretKey: "test-secret",
		UseSSL:    false,
		PathStyle: pathStyle,
	}
	return NewS3Driver(cfg)
}

func TestS3Driver_Upload_Success(t *testing.T) {
	var (
		gotMethod string
		gotPath   string
		gotBody   []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := newTestS3Driver(t, srv, true)
	content := []byte("file content")
	url, err := d.Upload(context.Background(), "dir/file.txt", bytes.NewReader(content), "text/plain")
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Fatalf("expected PUT, got %s", gotMethod)
	}
	if !strings.Contains(gotPath, "dir/file.txt") {
		t.Fatalf("path should contain key, got %s", gotPath)
	}
	if !bytes.Equal(gotBody, content) {
		t.Fatalf("body mismatch: got %q, want %q", gotBody, content)
	}
	if !strings.Contains(url, "dir/file.txt") {
		t.Fatalf("URL should contain key: %s", url)
	}
}

func TestS3Driver_Upload_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := newTestS3Driver(t, srv, true)
	_, err := d.Upload(context.Background(), "file.txt", strings.NewReader("x"), "text/plain")
	if err == nil {
		t.Fatal("Upload should fail on 500")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestS3Driver_Delete_Success(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	d := newTestS3Driver(t, srv, true)
	if err := d.Delete(context.Background(), "file.txt"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", gotMethod)
	}
}

func TestS3Driver_Delete_NotFound_NoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	d := newTestS3Driver(t, srv, true)
	// 404 should be treated as success (idempotent delete).
	if err := d.Delete(context.Background(), "missing.txt"); err != nil {
		t.Fatalf("Delete on 404 should return nil, got: %v", err)
	}
}

func TestS3Driver_Delete_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	d := newTestS3Driver(t, srv, true)
	err := d.Delete(context.Background(), "file.txt")
	if err == nil {
		t.Fatal("Delete should fail on 403")
	}
	if !strings.Contains(err.Error(), "status 403") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestS3Driver_GetURL(t *testing.T) {
	d := NewS3Driver(S3Config{
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
			d := NewS3Driver(c.cfg)
			if d.publicURL != c.wantURL {
				t.Fatalf("publicURL: got %q, want %q", d.publicURL, c.wantURL)
			}
		})
	}
}

func TestS3Driver_ObjectURL_PathStyle_Scheme(t *testing.T) {
	// Regression test for Round 6 / F6: PathStyle used to hardcode "http://"
	// regardless of UseSSL. Now both styles derive scheme from UseSSL.
	d := NewS3Driver(S3Config{
		Endpoint:  "minio.local:9000",
		Bucket:    "test",
		UseSSL:    true,
		PathStyle: true,
	})
	url := d.objectURL("file.txt")
	if !strings.HasPrefix(url, "https://") {
		t.Fatalf("PathStyle with UseSSL=true should use https, got %s", url)
	}
	if !strings.Contains(url, "file.txt") {
		t.Fatalf("URL should contain key: %s", url)
	}
}

func TestS3Driver_GetSignedURL(t *testing.T) {
	d := NewS3Driver(S3Config{
		Endpoint:  "s3.example.com",
		Bucket:    "mybucket",
		SecretKey: "secret",
		UseSSL:    true,
	})
	url := d.GetSignedURL("file.txt", 1*time.Hour)
	if !strings.Contains(url, "Expires=") {
		t.Fatalf("signed URL should contain Expires: %s", url)
	}
	if !strings.Contains(url, "Signature=") {
		t.Fatalf("signed URL should contain Signature: %s", url)
	}
}

func TestS3Driver_SignRequest_AddsAuthHeader(t *testing.T) {
	d := NewS3Driver(S3Config{
		Endpoint:  "s3.example.com",
		Bucket:    "mybucket",
		AccessKey: "AKIA123",
		SecretKey: "secret",
	})
	req, _ := http.NewRequest("PUT", "http://example.com/bucket/key", nil)
	req.Header.Set("Date", "Mon, 02 Jan 2026 15:04:05 GMT")
	d.signRequest(req, "PUT", "key")
	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS AKIA123:") {
		t.Fatalf("Authorization header should start with 'AWS AKIA123:', got %q", auth)
	}
}
