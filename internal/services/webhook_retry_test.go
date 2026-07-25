package services

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/models"
)

// zeroBackoff removes retry sleeps so retry tests run instantly.
func zeroBackoff(svc *WebhookService) *WebhookService {
	svc.backoff = func(int) time.Duration { return 0 }
	return svc
}

// TestWebhookRetry_5xxThenSuccess: 前两次返回 500，第三次 200 → 成功，Retries=2。
func TestWebhookRetry_5xxThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	repo := &MockWebhookRepository{}
	svc := zeroBackoff(NewWebhookServiceWithRepo(repo))

	wh := models.Webhook{ID: 1, URL: srv.URL, Events: models.StringSlice{"article.created"}}
	svc.deliver(wh, WebhookPayload{Event: "article.created", Timestamp: time.Now(), Data: "x"})

	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("expected 3 HTTP calls (2 failures + 1 success), got %d", got)
	}
	if len(repo.CreatedLogs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(repo.CreatedLogs))
	}
	log := repo.CreatedLogs[0]
	if !log.Success {
		t.Errorf("expected success=true after retry, got false (error: %s)", log.Error)
	}
	if log.Retries != 2 {
		t.Errorf("expected retries=2, got %d", log.Retries)
	}
	if log.Response != http.StatusOK {
		t.Errorf("expected response 200, got %d", log.Response)
	}
}

// TestWebhookRetry_5xxExhausted: 一直 500 → 初次 + 3 次重试后放弃，Retries=3。
func TestWebhookRetry_5xxExhausted(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	repo := &MockWebhookRepository{}
	svc := zeroBackoff(NewWebhookServiceWithRepo(repo))

	wh := models.Webhook{ID: 1, URL: srv.URL}
	svc.deliver(wh, WebhookPayload{Event: "article.created", Timestamp: time.Now(), Data: "x"})

	if got := atomic.LoadInt32(&calls); got != int32(1+maxWebhookRetries) {
		t.Errorf("expected %d HTTP calls, got %d", 1+maxWebhookRetries, got)
	}
	if len(repo.CreatedLogs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(repo.CreatedLogs))
	}
	log := repo.CreatedLogs[0]
	if log.Success {
		t.Error("expected success=false after exhausting retries")
	}
	if log.Retries != maxWebhookRetries {
		t.Errorf("expected retries=%d, got %d", maxWebhookRetries, log.Retries)
	}
	if log.Response != http.StatusBadGateway {
		t.Errorf("expected response 502, got %d", log.Response)
	}
}

// TestWebhookRetry_4xxNoRetry: 4xx 属永久失败，不应重试。
func TestWebhookRetry_4xxNoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	repo := &MockWebhookRepository{}
	svc := zeroBackoff(NewWebhookServiceWithRepo(repo))

	wh := models.Webhook{ID: 1, URL: srv.URL}
	svc.deliver(wh, WebhookPayload{Event: "article.created", Timestamp: time.Now(), Data: "x"})

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected exactly 1 HTTP call for 4xx (no retry), got %d", got)
	}
	if len(repo.CreatedLogs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(repo.CreatedLogs))
	}
	log := repo.CreatedLogs[0]
	if log.Success {
		t.Error("expected success=false for 404 response")
	}
	if log.Retries != 0 {
		t.Errorf("expected retries=0 for 4xx, got %d", log.Retries)
	}
}

// TestWebhookRetry_NetworkErrorRetries: 网络错误（无法连接）也应触发重试直至耗尽。
func TestWebhookRetry_NetworkErrorRetries(t *testing.T) {
	// 起一个 server 立即关掉，拿到一个必然拒绝连接的地址。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := srv.URL
	srv.Close()

	repo := &MockWebhookRepository{}
	svc := zeroBackoff(NewWebhookServiceWithRepo(repo))

	wh := models.Webhook{ID: 1, URL: deadURL}
	svc.deliver(wh, WebhookPayload{Event: "article.created", Timestamp: time.Now(), Data: "x"})

	if len(repo.CreatedLogs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(repo.CreatedLogs))
	}
	log := repo.CreatedLogs[0]
	if log.Success {
		t.Error("expected success=false on network error")
	}
	if log.Retries != maxWebhookRetries {
		t.Errorf("expected retries=%d on network error, got %d", maxWebhookRetries, log.Retries)
	}
	if log.Error == "" {
		t.Error("expected non-empty error message")
	}
}

// TestWebhookBackoff_Exponential: 指数退避序列 1s/2s/4s。
func TestWebhookBackoff_Exponential(t *testing.T) {
	want := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	for attempt, w := range want {
		if got := webhookBackoff(attempt); got != w {
			t.Errorf("attempt %d: expected %v, got %v", attempt, w, got)
		}
	}
}
