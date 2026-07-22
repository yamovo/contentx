package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// S3Driver stores files on S3-compatible storage (AWS S3, MinIO, Alibaba OSS).
// This is a lightweight HTTP-based implementation that avoids heavy SDK dependencies.
type S3Driver struct {
	config    S3Config
	publicURL string
	client    *http.Client
}

// NewS3Driver creates a new S3 storage driver.
func NewS3Driver(cfg S3Config) *S3Driver {
	publicURL := cfg.PublicURL
	if publicURL == "" {
		scheme := "https"
		if !cfg.UseSSL {
			scheme = "http"
		}
		if cfg.PathStyle {
			publicURL = fmt.Sprintf("%s://%s/%s", scheme, cfg.Endpoint, cfg.Bucket)
		} else {
			publicURL = fmt.Sprintf("%s://%s.%s", scheme, cfg.Bucket, cfg.Endpoint)
		}
	}

	return &S3Driver{
		config:    cfg,
		publicURL: publicURL,
		client:    &http.Client{Timeout: 30 * time.Second, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}
}

func (d *S3Driver) Upload(ctx context.Context, key string, reader io.Reader, contentType string) (string, error) {
	url := d.objectURL(key)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, reader)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	d.signRequest(req, "PUT", key)

	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("upload failed: status %d", resp.StatusCode)
	}

	return d.GetURL(key), nil
}

func (d *S3Driver) Delete(ctx context.Context, key string) error {
	url := d.objectURL(key)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	d.signRequest(req, "DELETE", key)

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		return fmt.Errorf("delete failed: status %d", resp.StatusCode)
	}
	return nil
}

func (d *S3Driver) GetURL(key string) string {
	return d.publicURL + "/" + key
}

func (d *S3Driver) GetSignedURL(key string, expiry time.Duration) string {
	expires := time.Now().Add(expiry).Unix()
	url := fmt.Sprintf("%s?Expires=%d", d.objectURL(key), expires)
	// Simplified signing — in production use proper AWS Signature V4.
	sig := hmacSign(d.config.SecretKey, fmt.Sprintf("%s\n%d", key, expires))
	return url + "&Signature=" + sig
}

func (d *S3Driver) objectURL(key string) string {
	if d.config.PathStyle {
		return fmt.Sprintf("http://%s/%s/%s", d.config.Endpoint, d.config.Bucket, key)
	}
	scheme := "https"
	if !d.config.UseSSL {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s.%s/%s", scheme, d.config.Bucket, d.config.Endpoint, key)
}

func (d *S3Driver) signRequest(req *http.Request, method, key string) {
	// Simplified AWS V2 signing placeholder.
	// For production, use github.com/aws/aws-sdk-go or minio/minio-go.
	date := req.Header.Get("Date")
	signature := hmacSign(d.config.AccessKey, fmt.Sprintf("%s\n\n\n%s\n/%s/%s", method, date, d.config.Bucket, key))
	req.Header.Set("Authorization", fmt.Sprintf("AWS %s:%s", d.config.AccessKey, signature))
}

func hmacSign(key, data string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}
