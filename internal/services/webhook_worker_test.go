package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/database"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ──────────────────────────────────────────────────────────────────────────────
// Webhook 持久化投递队列 worker 测试
//
// 用真实内存/临时 SQLite + httptest 验证完整链路：Dispatch 入队 → worker 认领 →
// 单次 HTTP 尝试 → 终态/重调度，以及并发上限、崩溃复投、仓库层 claim/complete SQL。
// ──────────────────────────────────────────────────────────────────────────────

// newWebhookTestDB returns an isolated SQLite database with all tables migrated.
func newWebhookTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "webhook_test.db")
	db, err := gorm.Open(sqlite.Open(path+"?_journal_mode=WAL&_busy_timeout=5000"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// newTestWorker builds a worker targeting loopback test servers with short,
// deterministic timings. It does NOT call Start — callers drive processDue or
// Start explicitly.
func newTestWorker(db *gorm.DB, opts ...func(*WebhookWorker)) *WebhookWorker {
	w := NewWebhookWorker(db, config.QueueConfig{
		MaxWorkers: 4,
		MaxRetries: 3,
		RetryDelay: time.Millisecond, // backoff: 1ms, 2ms, 4ms
	})
	w.client = newWebhookHTTPClient(true) // allow loopback (SSRF block disabled for tests)
	w.pollInterval = 10 * time.Millisecond
	w.jitter = func(d time.Duration) time.Duration { return d } // deterministic
	for _, o := range opts {
		o(w)
	}
	return w
}

// seedWebhook inserts an active webhook and returns its ID.
func seedWebhook(t *testing.T, db *gorm.DB, id uint, url, secret string) {
	t.Helper()
	wh := models.Webhook{ID: id, Name: "test", URL: url, Secret: secret,
		Events: models.StringSlice{"e"}, IsActive: true}
	if err := db.Create(&wh).Error; err != nil {
		t.Fatalf("seed webhook: %v", err)
	}
}

// seedDueDelivery inserts a pending delivery that is immediately due.
func seedDueDelivery(t *testing.T, db *gorm.DB, webhookID uint) *models.WebhookDelivery {
	t.Helper()
	d := models.WebhookDelivery{
		WebhookID:   webhookID,
		Event:       "e",
		Payload:     `{"event":"e","data":"x"}`,
		Status:      models.WebhookDeliveryPending,
		NextRetryAt: time.Now().Add(-time.Second), // already due
	}
	if err := db.Create(&d).Error; err != nil {
		t.Fatalf("seed delivery: %v", err)
	}
	return &d
}

// getDelivery reloads a delivery row from the DB.
func getDelivery(t *testing.T, db *gorm.DB, id uint) models.WebhookDelivery {
	t.Helper()
	var d models.WebhookDelivery
	if err := db.First(&d, id).Error; err != nil {
		t.Fatalf("reload delivery %d: %v", id, err)
	}
	return d
}

// runOnce claims and processes all currently-due deliveries, then waits for
// in-flight work to finish (Stop without ever starting the loop).
func runOnce(w *WebhookWorker) {
	w.processDue()
	w.Stop()
}

// ---------- success / payload / HMAC ----------

func TestWebhookWorker_Success(t *testing.T) {
	var (
		gotBody string
		gotHdr  http.Header
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotHdr = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	db := newWebhookTestDB(t)
	seedWebhook(t, db, 1, srv.URL, "")
	d := seedDueDelivery(t, db, 1)

	w := newTestWorker(db)
	runOnce(w)

	got := getDelivery(t, db, d.ID)
	if got.Status != models.WebhookDeliverySuccess {
		t.Errorf("status = %s, want success", got.Status)
	}
	if got.ResponseCode != http.StatusOK {
		t.Errorf("response_code = %d, want 200", got.ResponseCode)
	}
	if got.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", got.Attempts)
	}
	if got.CompletedAt == nil {
		t.Error("expected completed_at set")
	}

	// One terminal log row, success.
	var logs []models.WebhookLog
	db.Find(&logs)
	if len(logs) != 1 || !logs[0].Success || logs[0].Retries != 0 {
		t.Errorf("unexpected logs: %+v", logs)
	}

	if gotHdr.Get("X-ContentX-Event") != "e" {
		t.Errorf("X-ContentX-Event = %q", gotHdr.Get("X-ContentX-Event"))
	}
	if gotHdr.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", gotHdr.Get("Content-Type"))
	}
	if !bytes.Contains([]byte(gotBody), []byte(`"event":"e"`)) {
		t.Errorf("body missing event: %s", gotBody)
	}
}

func TestWebhookWorker_HMACSignature(t *testing.T) {
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get("X-ContentX-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	db := newWebhookTestDB(t)
	seedWebhook(t, db, 1, srv.URL, "my-secret")
	d := seedDueDelivery(t, db, 1)

	w := newTestWorker(db)
	runOnce(w)

	if gotSig == "" || gotSig[:7] != "sha256=" {
		t.Fatalf("unexpected signature: %q", gotSig)
	}
	got := getDelivery(t, db, d.ID)
	h := hmac.New(sha256.New, []byte("my-secret"))
	h.Write([]byte(got.Payload))
	want := "sha256=" + hex.EncodeToString(h.Sum(nil))
	if gotSig != want {
		t.Errorf("signature mismatch: want %s, got %s", want, gotSig)
	}
}

func TestWebhookWorker_CustomHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	db := newWebhookTestDB(t)
	seedWebhook(t, db, 1, srv.URL, "secret")
	if err := db.Model(&models.Webhook{}).Where("id = ?", 1).Update(
		"headers",
		models.StringSlice{
			"X-Tenant: acme",
			"Authorization: Bearer custom",
			"Content-Type: text/plain",
			"X-ContentX-Signature: attacker",
			"malformed",
		},
	).Error; err != nil {
		t.Fatalf("set headers: %v", err)
	}
	d := seedDueDelivery(t, db, 1)

	w := newTestWorker(db)
	runOnce(w)

	if status := getDelivery(t, db, d.ID).Status; status != models.WebhookDeliverySuccess {
		t.Fatalf("status = %s, want success", status)
	}
	if got.Get("X-Tenant") != "acme" || got.Get("Authorization") != "Bearer custom" {
		t.Fatalf("custom headers not delivered: %v", got)
	}
	if got.Get("Content-Type") != "application/json" {
		t.Errorf("reserved Content-Type overridden: %q", got.Get("Content-Type"))
	}
	if got.Get("X-ContentX-Signature") == "attacker" {
		t.Error("reserved signature header was overridden")
	}
}

func TestWebhookWorker_4xxTerminalFailure(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	db := newWebhookTestDB(t)
	seedWebhook(t, db, 1, srv.URL, "")
	d := seedDueDelivery(t, db, 1)

	w := newTestWorker(db)
	runOnce(w)

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 HTTP call (no retry for 4xx), got %d", got)
	}
	got := getDelivery(t, db, d.ID)
	if got.Status != models.WebhookDeliveryFailed {
		t.Errorf("status = %s, want failed", got.Status)
	}
	if got.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", got.Attempts)
	}
	var logs []models.WebhookLog
	db.Find(&logs)
	if len(logs) != 1 || logs[0].Success || logs[0].Response != http.StatusNotFound {
		t.Errorf("unexpected failure log: %+v", logs)
	}
}

func TestWebhookWorker_WebhookDeleted(t *testing.T) {
	db := newWebhookTestDB(t)
	// No webhook seeded → GetByID returns not found.
	d := seedDueDelivery(t, db, 999)

	w := newTestWorker(db)
	runOnce(w)

	got := getDelivery(t, db, d.ID)
	if got.Status != models.WebhookDeliveryFailed {
		t.Errorf("status = %s, want failed", got.Status)
	}
	if got.LastError == "" {
		t.Error("expected non-empty last_error")
	}
	// No log row for a deleted webhook.
	var logs []models.WebhookLog
	if db.Find(&logs).RowsAffected != 0 || len(logs) != 0 {
		t.Errorf("expected 0 logs for deleted webhook, got %d", len(logs))
	}
}

// ---------- concurrency limit ----------

func TestWebhookWorker_ConcurrencyLimit(t *testing.T) {
	var (
		active atomic.Int32
		maxC   atomic.Int32
		calls  atomic.Int32
	)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := active.Add(1)
		for {
			m := maxC.Load()
			if cur <= m || maxC.CompareAndSwap(m, cur) {
				break
			}
		}
		calls.Add(1)
		<-release
		active.Add(-1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	db := newWebhookTestDB(t)
	seedWebhook(t, db, 1, srv.URL, "")
	for i := 0; i < 4; i++ {
		seedDueDelivery(t, db, 1)
	}

	w := newTestWorker(db, func(w *WebhookWorker) {
		w.concurrency = 2
		w.maxAttempts = 1 // failures (if any) must not reschedule
	})
	w.Start()

	// Wait until both slots are filled.
	deadline := time.Now().Add(2 * time.Second)
	for active.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if active.Load() != 2 {
		t.Fatalf("expected 2 in-flight deliveries, got %d", active.Load())
	}
	if maxC.Load() != 2 {
		t.Errorf("expected max concurrency 2, got %d", maxC.Load())
	}

	close(release) // let the first batch finish

	// Wait for all 4 deliveries to be attempted.
	deadline = time.Now().Add(2 * time.Second)
	for calls.Load() < 4 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	w.Stop()

	if got := calls.Load(); got != 4 {
		t.Errorf("expected 4 total attempts, got %d", got)
	}
	if maxC.Load() != 2 {
		t.Errorf("max concurrency exceeded cap: got %d, want 2", maxC.Load())
	}
}

// ---------- crash recovery (stale requeue on Start) ----------

func TestWebhookWorker_RequeuesStaleOnStart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	db := newWebhookTestDB(t)
	seedWebhook(t, db, 1, srv.URL, "")

	// Simulate a crash mid-attempt: a row left in 'delivering' that is due.
	stale := models.WebhookDelivery{
		WebhookID:   1,
		Event:       "e",
		Payload:     `{"event":"e"}`,
		Status:      models.WebhookDeliveryDelivering,
		NextRetryAt: time.Now().Add(-time.Second),
	}
	if err := db.Create(&stale).Error; err != nil {
		t.Fatalf("seed stale: %v", err)
	}

	w := newTestWorker(db)
	w.Start()

	// Start requeues stale → pending → claimed → delivered.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got := getDelivery(t, db, stale.ID)
		if got.Status == models.WebhookDeliverySuccess {
			break
		}
		time.Sleep(time.Millisecond)
	}
	w.Stop()

	got := getDelivery(t, db, stale.ID)
	if got.Status != models.WebhookDeliverySuccess {
		t.Errorf("stale delivery not redelivered after Start: status = %s", got.Status)
	}
}

// ---------- repository-level claim/complete SQL ----------

func TestWebhookRepo_ClaimAndComplete(t *testing.T) {
	db := newWebhookTestDB(t)
	repo := repository.NewWebhookRepository(db)

	// Two due rows + one not-yet-due row.
	for i := 0; i < 2; i++ {
		db.Create(&models.WebhookDelivery{
			WebhookID: 1, Event: "e", Payload: "{}",
			Status: models.WebhookDeliveryPending, NextRetryAt: time.Now().Add(-time.Second),
		})
	}
	db.Create(&models.WebhookDelivery{
		WebhookID: 1, Event: "e", Payload: "{}",
		Status: models.WebhookDeliveryPending, NextRetryAt: time.Now().Add(time.Minute),
	})

	claimed, err := repo.ClaimDueDeliveries(time.Now(), 10)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 2 {
		t.Fatalf("expected 2 claimed, got %d", len(claimed))
	}
	for _, c := range claimed {
		if c.Status != models.WebhookDeliveryDelivering {
			t.Errorf("claimed row not in delivering state: %s", c.Status)
		}
	}

	// Second claim finds nothing left that is due.
	again, _ := repo.ClaimDueDeliveries(time.Now(), 10)
	if len(again) != 0 {
		t.Errorf("expected 0 on second claim, got %d", len(again))
	}

	// Complete one as success, one as reschedule.
	now := time.Now()
	comp := now
	if err := repo.CompleteDelivery(claimed[0].ID, repository.DeliveryOutcome{
		Status: models.WebhookDeliverySuccess, Attempts: 1, NextRetryAt: claimed[0].NextRetryAt,
		CompletedAt: &comp,
	}); err != nil {
		t.Fatalf("complete success: %v", err)
	}
	if err := repo.CompleteDelivery(claimed[1].ID, repository.DeliveryOutcome{
		Status: models.WebhookDeliveryPending, Attempts: 1, NextRetryAt: now.Add(time.Second),
	}); err != nil {
		t.Fatalf("complete reschedule: %v", err)
	}

	var d0 models.WebhookDelivery
	db.First(&d0, claimed[0].ID)
	if d0.Status != models.WebhookDeliverySuccess {
		t.Errorf("row 0 status = %s, want success", d0.Status)
	}
	var d1 models.WebhookDelivery
	db.First(&d1, claimed[1].ID)
	if d1.Status != models.WebhookDeliveryPending {
		t.Errorf("row 1 status = %s, want pending", d1.Status)
	}
	if !d1.NextRetryAt.After(now) {
		t.Error("rescheduled NextRetryAt should be in the future")
	}

	// RequeueStaleDeliveries with no delivering rows is a no-op.
	n, err := repo.RequeueStaleDeliveries()
	if err != nil || n != 0 {
		t.Errorf("requeue expected 0, got %d (err %v)", n, err)
	}

	// CountPendingDeliveries = the 1 rescheduled + the 1 not-yet-due = 2.
	pending, err := repo.CountPendingDeliveries(1)
	if err != nil || pending != 2 {
		t.Errorf("pending count = %d, want 2 (err %v)", pending, err)
	}
}
