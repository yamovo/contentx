package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/yamovo/contentx/internal/cache"
)

// ScheduledPublisher is the contract PublishScheduler depends on. It is
// satisfied by *ArticleService (via PublishDueScheduled), and split out so the
// scheduler can be unit-tested with a fake.
type ScheduledPublisher interface {
	PublishDueScheduled(now time.Time) (int, error)
}

// PublishScheduler periodically scans for scheduled articles whose ScheduledAt
// has passed and flips them to published via the ScheduledPublisher. It runs on
// a fixed-interval ticker (no cron library dependency) and is safe to run as a
// long-lived goroutine started at server boot.
//
// 多实例部署时，通过 DistributedLock 协调：每次 Tick 前尝试获取锁，
// 获取失败则跳过本次 sweep，避免多实例重复发布。
type PublishScheduler struct {
	pub      ScheduledPublisher
	interval time.Duration
	logger   *slog.Logger
	lock     cache.DistributedLock // 可选，nil 表示不使用分布式锁（单实例）

	mu      sync.Mutex
	stopCh  chan struct{}
	running bool
}

// NewPublishScheduler builds a scheduler that ticks every interval and asks
// pub to publish any due scheduled articles. A zero interval defaults to 1m.
func NewPublishScheduler(pub ScheduledPublisher, interval time.Duration, logger *slog.Logger) *PublishScheduler {
	if interval <= 0 {
		interval = time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &PublishScheduler{
		pub:      pub,
		interval: interval,
		logger:   logger,
	}
}

// SetDistributedLock 注入分布式锁。多实例部署时必须在 Start 前调用。
// 传 nil 清除锁（降级为单实例模式）。
func (s *PublishScheduler) SetDistributedLock(lock cache.DistributedLock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lock = lock
}

// Start launches the scheduler goroutine. It is idempotent: calling Start on
// an already-running scheduler is a no-op.
func (s *PublishScheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	s.stopCh = make(chan struct{})
	s.running = true
	go s.loop()
	s.logger.Info("publish scheduler started", "interval", s.interval)
}

// Stop signals the scheduler goroutine to exit and waits for it to drain. It is
// idempotent and safe to call from a different goroutine (e.g. graceful shutdown).
func (s *PublishScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	close(s.stopCh)
	s.running = false
	s.mu.Unlock()

	// Give the loop a moment to observe the close and exit. The loop only
	// blocks on the ticker (≤ interval) so this is effectively immediate for
	// sub-minute intervals.
	time.Sleep(10 * time.Millisecond)
	s.logger.Info("publish scheduler stopped")
}

// Running reports whether the scheduler loop is currently active.
func (s *PublishScheduler) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Tick performs one publication sweep. Exposed so callers (e.g. tests or a
// manual admin trigger) can force a sweep without waiting for the ticker.
//
// 若设置了分布式锁，Tick 会先尝试获取锁（TTL 略大于 interval），
// 获取失败说明其他实例正在处理，直接返回 (0, nil) 跳过本次。
func (s *PublishScheduler) Tick() (int, error) {
	// 无锁模式（单实例部署）
	if s.lock == nil {
		return s.pub.PublishDueScheduled(time.Now())
	}

	// 分布式锁模式（多实例部署）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lockKey := "publish-scheduler"
	lockTTL := s.interval + 10*time.Second // TTL 略大于 interval，防止 sweep 中途锁过期
	release, ok, err := s.lock.Acquire(ctx, lockKey, lockTTL)
	if err != nil {
		return 0, err
	}
	if !ok {
		// 其他实例正在处理，跳过本次 sweep
		s.logger.Debug("publish scheduler skipped — lock held by another instance")
		return 0, nil
	}
	defer release()

	return s.pub.PublishDueScheduled(time.Now())
}

func (s *PublishScheduler) loop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			n, err := s.Tick()
			if err != nil {
				s.logger.Error("publish scheduler sweep failed", "error", err)
				continue
			}
			if n > 0 {
				s.logger.Info("publish scheduler published articles", "count", n)
			}
		}
	}
}
