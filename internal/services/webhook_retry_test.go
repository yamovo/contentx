package services

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/models"
)

// ──────────────────────────────────────────────────────────────────────────────
// Webhook 队列重试调度语义测试（接续旧版 webhook_retry_test 的意图，改为
// 持久化队列模型：单次尝试 + 重调度，而非进程内 sleep 重试）。
// ──────────────────────────────────────────────────────────────────────────────

func TestWebhookRetry_5xxReschedules(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	db := newWebhookTestDB(t)
	seedWebhook(t, db, 1, srv.URL, "")
	d := seedDueDelivery(t, db, 1)

	w := newTestWorker(db, func(w *WebhookWorker) {
		// 1s base backoff keeps NextRetryAt robustly in the future across
		// the assertion timing.
		w.backoff = func(attempt int) time.Duration { return time.Second }
	}) // MaxRetries=3 → maxAttempts=4
	runOnce(w)

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 HTTP call (single attempt per claim), got %d", got)
	}
	got := getDelivery(t, db, d.ID)
	if got.Status != models.WebhookDeliveryPending {
		t.Errorf("status = %s, want pending (rescheduled)", got.Status)
	}
	if got.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", got.Attempts)
	}
	if !got.NextRetryAt.After(time.Now()) {
		t.Error("expected NextRetryAt in the future after reschedule")
	}
	// No terminal log until the delivery reaches a terminal state.
	var logs []models.WebhookLog
	db.Find(&logs)
	if len(logs) != 0 {
		t.Errorf("expected 0 logs after reschedule, got %d", len(logs))
	}
}

func TestWebhookRetry_5xxThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	db := newWebhookTestDB(t)
	seedWebhook(t, db, 1, srv.URL, "")
	d := seedDueDelivery(t, db, 1)

	w := newTestWorker(db)

	// First attempt: 500 → rescheduled.
	runOnce(w)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("after first cycle: expected 1 call, got %d", got)
	}

	// Advance the retry clock and re-run.
	if err := db.Model(&models.WebhookDelivery{}).Where("id = ?", d.ID).
		Update("next_retry_at", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatalf("reset due: %v", err)
	}
	runOnce(w)

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 total HTTP calls, got %d", got)
	}
	got := getDelivery(t, db, d.ID)
	if got.Status != models.WebhookDeliverySuccess {
		t.Errorf("status = %s, want success", got.Status)
	}
	if got.Attempts != 2 {
		t.Errorf("attempts = %d, want 2", got.Attempts)
	}
	var logs []models.WebhookLog
	db.Find(&logs)
	if len(logs) != 1 {
		t.Fatalf("expected 1 terminal log, got %d", len(logs))
	}
	if !logs[0].Success || logs[0].Retries != 1 {
		t.Errorf("unexpected log: success=%v retries=%d", logs[0].Success, logs[0].Retries)
	}
}

func TestWebhookRetry_Exhausted(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	db := newWebhookTestDB(t)
	seedWebhook(t, db, 1, srv.URL, "")
	d := seedDueDelivery(t, db, 1)

	// MaxRetries=0 → maxAttempts=1: first 5xx exhausts immediately.
	w := newTestWorker(db, func(w *WebhookWorker) {
		w.maxAttempts = 1
	})
	runOnce(w)

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 HTTP call, got %d", got)
	}
	got := getDelivery(t, db, d.ID)
	if got.Status != models.WebhookDeliveryExhausted {
		t.Errorf("status = %s, want exhausted", got.Status)
	}
	var logs []models.WebhookLog
	db.Find(&logs)
	if len(logs) != 1 || logs[0].Success || logs[0].Retries != 0 {
		t.Errorf("unexpected exhausted log: %+v", logs)
	}
}

func TestWebhookRetry_NetworkErrorReschedules(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := srv.URL
	srv.Close() // guaranteed connection refused

	db := newWebhookTestDB(t)
	seedWebhook(t, db, 1, deadURL, "")
	d := seedDueDelivery(t, db, 1)

	w := newTestWorker(db)
	runOnce(w)

	got := getDelivery(t, db, d.ID)
	if got.Status != models.WebhookDeliveryPending {
		t.Errorf("status = %s, want pending (network error should reschedule)", got.Status)
	}
	if got.LastError == "" {
		t.Error("expected non-empty last_error on network failure")
	}
	if got.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", got.Attempts)
	}
}

// TestWebhookRetry_BackoffSequence: exponential base 1ms → 1ms, 2ms, 4ms.
func TestWebhookRetry_BackoffSequence(t *testing.T) {
	db := newWebhookTestDB(t)
	w := newTestWorker(db) // RetryDelay = 1ms
	want := []time.Duration{time.Millisecond, 2 * time.Millisecond, 4 * time.Millisecond}
	for attempt, w2 := range want {
		if got := w.backoff(attempt); got != w2 {
			t.Errorf("backoff(%d) = %v, want %v", attempt, got, w2)
		}
	}
}

// TestWebhookRetry_JitterBounds: fullJitter maps d into [d/2, d).
func TestWebhookRetry_JitterBounds(t *testing.T) {
	d := 100 * time.Millisecond
	for i := 0; i < 1000; i++ {
		got := fullJitter(d)
		if got < d/2 || got >= d {
			t.Fatalf("jitter %v out of [%v, %v)", got, d/2, d)
		}
	}
}

// Ensure config.QueueConfig zero values still produce a sane worker (defaults).
func TestWebhookWorker_DefaultsFromZeroConfig(t *testing.T) {
	db := newWebhookTestDB(t)
	w := NewWebhookWorker(db, config.QueueConfig{})
	if w.concurrency != 4 {
		t.Errorf("default concurrency = %d, want 4", w.concurrency)
	}
	if w.maxAttempts != 1 {
		t.Errorf("default maxAttempts = %d, want 1", w.maxAttempts)
	}
	if w.backoff(0) != 5*time.Second {
		t.Errorf("default backoff(0) = %v, want 5s", w.backoff(0))
	}
}
