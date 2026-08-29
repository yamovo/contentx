package services

// Webhook 持久化投递队列 worker（里程碑「安全与稳定性收口」/ STATUS 进行中：
// Webhook 队列、并发限制和退避策略完善）。
//
// 语义：
//   - Dispatch 只负责把事件写入 webhook_deliveries（pending），本 worker 轮询认领。
//   - 每个认领行执行恰好一次 HTTP 尝试；5xx/网络错误按指数退避 + full jitter
//     重调度，4xx 视为永久失败，重试预算耗尽标记 exhausted。
//   - 并发上限由信号量控制（QUEUE_MAX_WORKERS），不会像旧实现那样无界 fan-out。
//   - 至少一次投递：进程崩溃时停留在 delivering 的行在下次 Start 时被复投。

import (
	"bytes"
	"context"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yamovo/contentx/internal/config"
	"github.com/yamovo/contentx/internal/models"
	"github.com/yamovo/contentx/internal/observability"
	"github.com/yamovo/contentx/internal/repository"
	"gorm.io/gorm"
)

// WebhookWorker drains the persistent webhook delivery queue.
type WebhookWorker struct {
	repo         repository.WebhookRepository
	client       *http.Client
	concurrency  int
	maxAttempts  int // total attempts per delivery, including the initial one
	pollInterval time.Duration
	backoff      func(attempt int) time.Duration // delay before retry #attempt+1 (0-based)
	jitter       func(d time.Duration) time.Duration

	inflight atomic.Int64
	stopCh   chan struct{}
	wg       sync.WaitGroup // run loop + in-flight delivery goroutines
	once     sync.Once
}

// NewWebhookWorker builds a worker backed by a GORM repository. Queue
// behaviour is driven by cfg.Queue (QUEUE_MAX_WORKERS / QUEUE_MAX_RETRIES /
// QUEUE_RETRY_DELAY), reusing the previously reserved QueueConfig.
func NewWebhookWorker(db *gorm.DB, cfg config.QueueConfig) *WebhookWorker {
	return NewWebhookWorkerWithRepo(repository.NewWebhookRepository(db), cfg)
}

// NewWebhookWorkerWithRepo builds a worker with an explicit repository,
// enabling unit tests to inject mocks.
func NewWebhookWorkerWithRepo(repo repository.WebhookRepository, cfg config.QueueConfig) *WebhookWorker {
	concurrency := cfg.MaxWorkers
	if concurrency <= 0 {
		concurrency = 4
	}
	maxRetries := cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	retryDelay := cfg.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 5 * time.Second
	}
	return &WebhookWorker{
		repo:         repo,
		client:       newWebhookHTTPClient(allowPrivateWebhookTargets()),
		concurrency:  concurrency,
		maxAttempts:  1 + maxRetries,
		pollInterval: time.Second,
		backoff: func(attempt int) time.Duration {
			// Exponential: retryDelay, 2x, 4x, ... capped at shift 8 to avoid overflow.
			if attempt > 8 {
				attempt = 8
			}
			return retryDelay << uint(attempt)
		},
		jitter: fullJitter,
		stopCh: make(chan struct{}),
	}
}

// fullJitter randomizes d into [d/2, d), following the AWS "full jitter"
// recommendation so concurrently failing deliveries do not retry in lockstep.
func fullJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	half := int64(d) / 2
	return time.Duration(half + rand.Int63n(half+1))
}

// Start resets deliveries stuck in delivering from a previous run and begins
// the poll loop in the background.
func (w *WebhookWorker) Start() {
	requeued, err := w.repo.RequeueStaleDeliveries()
	if err != nil {
		slog.Error("webhook worker: requeue stale deliveries failed", "error", err)
	} else if requeued > 0 {
		slog.Info("webhook worker: requeued stale deliveries", "count", requeued)
	}
	w.wg.Add(1)
	go w.run()
}

// Stop signals the poll loop to exit and waits for in-flight deliveries to
// finish. Deliveries still mid-attempt after the timeout stay in delivering
// state and are requeued on the next Start (at-least-once).
func (w *WebhookWorker) Stop() {
	w.once.Do(func() { close(w.stopCh) })
	done := make(chan struct{})
	go func() { w.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		slog.Warn("webhook worker: shutdown timed out with deliveries in flight")
	}
}

func (w *WebhookWorker) run() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	// Drain immediately at startup instead of waiting one full interval.
	w.processDue()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.processDue()
		}
	}
}

// processDue claims up to the currently available concurrency and processes
// each claimed delivery in its own goroutine. Called only from the single
// run-loop goroutine, so the inflight bookkeeping needs no locking beyond
// the atomic counter.
func (w *WebhookWorker) processDue() {
	available := int64(w.concurrency) - w.inflight.Load()
	if available <= 0 {
		return
	}
	claimed, err := w.repo.ClaimDueDeliveries(time.Now(), int(available))
	if err != nil {
		slog.Error("webhook worker: claim deliveries failed", "error", err)
		return
	}
	for i := range claimed {
		d := claimed[i]
		w.inflight.Add(1)
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			defer w.inflight.Add(-1)
			w.processDelivery(&d)
		}()
	}
}

// processDelivery performs exactly one HTTP attempt for a claimed delivery
// and resolves it: terminal success/failed/exhausted, or rescheduled pending
// with backoff. A terminal attempt also writes the WebhookLog row consumed
// by the admin UI, preserving the pre-queue observability surface.
func (w *WebhookWorker) processDelivery(d *models.WebhookDelivery) {
	attempts := d.Attempts + 1

	wh, err := w.repo.GetByID(d.WebhookID, d.TenantID)
	if err != nil || wh == nil {
		// Webhook deleted after enqueue: permanent failure. No log row —
		// logs are cascade-deleted together with the webhook anyway.
		w.complete(d, repository.DeliveryOutcome{
			Status:   models.WebhookDeliveryFailed,
			Attempts: attempts,
			// keep original NextRetryAt; row is terminal
			NextRetryAt: d.NextRetryAt,
			LastError:   "webhook no longer exists",
		})
		return
	}

	start := time.Now()
	status, attemptErr := w.attemptOnce(wh, []byte(d.Payload), d.Event)
	duration := int(time.Since(start).Milliseconds())

	switch {
	case attemptErr == nil && status >= 200 && status < 300:
		w.complete(d, repository.DeliveryOutcome{
			Status:       models.WebhookDeliverySuccess,
			Attempts:     attempts,
			NextRetryAt:  d.NextRetryAt,
			ResponseCode: status,
		})
		w.writeLog(d, true, status, "", attempts-1, duration)
		w.metric(d.Event, "success")

	case attemptErr == nil && status < 500:
		// 3xx/4xx: permanent failure, do not retry.
		w.complete(d, repository.DeliveryOutcome{
			Status:       models.WebhookDeliveryFailed,
			Attempts:     attempts,
			NextRetryAt:  d.NextRetryAt,
			ResponseCode: status,
			LastError:    "endpoint returned non-retryable status",
		})
		w.writeLog(d, false, status, "", attempts-1, duration)
		w.metric(d.Event, "failure")
		slog.Warn("webhook delivery failed permanently", "webhook_id", wh.ID, "status", status, "attempts", attempts)

	default:
		// 5xx or network error: retryable while budget remains.
		errMsg := "endpoint returned " + http.StatusText(status)
		if attemptErr != nil {
			errMsg = attemptErr.Error()
		}
		if attempts >= w.maxAttempts {
			w.complete(d, repository.DeliveryOutcome{
				Status:       models.WebhookDeliveryExhausted,
				Attempts:     attempts,
				NextRetryAt:  d.NextRetryAt,
				ResponseCode: status,
				LastError:    errMsg,
			})
			w.writeLog(d, false, status, errMsg, attempts-1, duration)
			w.metric(d.Event, "exhausted")
			slog.Warn("webhook delivery exhausted retries",
				"webhook_id", wh.ID, "url", wh.URL, "attempts", attempts, "error", errMsg)
			return
		}
		delay := w.jitter(w.backoff(attempts - 1))
		w.complete(d, repository.DeliveryOutcome{
			Status:       models.WebhookDeliveryPending,
			Attempts:     attempts,
			NextRetryAt:  time.Now().Add(delay),
			ResponseCode: status,
			LastError:    errMsg,
		})
		slog.Warn("webhook delivery failed, rescheduled",
			"webhook_id", wh.ID, "attempts", attempts, "retry_in", delay, "error", errMsg)
	}
}

// attemptOnce performs a single HTTP POST attempt and returns the response
// status code. A non-nil error means the request never completed (network
// failure, timeout, SSRF block) and no status was received.
func (w *WebhookWorker) attemptOnce(wh *models.Webhook, body []byte, event string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", wh.URL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	for _, rawHeader := range wh.Headers {
		name, value, ok := strings.Cut(rawHeader, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name == "" || isReservedWebhookHeader(name) {
			continue
		}
		req.Header.Set(name, value)
	}
	req.Header.Set("X-ContentX-Event", event)
	if wh.Secret != "" {
		sig := hmacSign([]byte(wh.Secret), body)
		req.Header.Set("X-ContentX-Signature", "sha256="+sig)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
}

func isReservedWebhookHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Content-Type", "Host", "Content-Length", "X-Contentx-Event", "X-Contentx-Signature":
		return true
	default:
		return false
	}
}

// complete persists the attempt outcome and mirrors it onto the in-memory
// row so callers (and tests) inspecting the delivery see the final state.
// Terminal states stamp CompletedAt.
func (w *WebhookWorker) complete(d *models.WebhookDelivery, outcome repository.DeliveryOutcome) {
	if outcome.Status != models.WebhookDeliveryPending {
		t := time.Now()
		outcome.CompletedAt = &t
	}
	if err := w.repo.CompleteDelivery(d.ID, outcome); err != nil {
		slog.Error("webhook worker: complete delivery failed", "delivery_id", d.ID, "error", err)
	}
	d.Status = outcome.Status
	d.Attempts = outcome.Attempts
	d.NextRetryAt = outcome.NextRetryAt
	d.ResponseCode = outcome.ResponseCode
	d.LastError = outcome.LastError
	d.CompletedAt = outcome.CompletedAt
}

// writeLog records a terminal delivery attempt in webhook_logs (admin UI).
// retries follows the pre-queue convention: attempts made minus the initial one.
func (w *WebhookWorker) writeLog(d *models.WebhookDelivery, success bool, status int, errMsg string, retries, duration int) {
	entry := models.WebhookLog{
		WebhookID: d.WebhookID,
		TenantID:  d.TenantID,
		Event:     d.Event,
		Payload:   d.Payload,
		Response:  status,
		Duration:  duration,
		Success:   success,
		Error:     errMsg,
		Retries:   retries,
	}
	if err := w.repo.CreateLog(&entry); err != nil {
		slog.Error("webhook worker: write delivery log failed", "delivery_id", d.ID, "error", err)
	}
}

// metric counts terminal delivery outcomes, keeping the pre-queue metric
// name and label set so existing dashboards keep working.
func (w *WebhookWorker) metric(event, status string) {
	observability.IncCounterWithLabels(
		"webhook_dispatch_total",
		"Total webhook delivery attempts",
		map[string]string{"event": event, "status": status},
	)
}
