package backup

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/yamovo/contentx/internal/cache"
)

// BackupRunner is the contract BackupScheduler depends on. Satisfied by
// *Manager (via BackupAll). Split out so the scheduler can be tested with a
// fake.
type BackupRunner interface {
	BackupAll() (dbPath, mediaPath string, err error)
}

// BackupScheduler runs database+media backups on a cron schedule. It is safe
// to run as a long-lived goroutine started at server boot.
//
// 多实例部署时，通过 DistributedLock 协调：每次触发前尝试获取锁，
// 获取失败则跳过本次备份，避免多实例重复执行。Retention is handled by
// the Manager's existing cleanup (MaxBackups) which runs after each backup.
type BackupScheduler struct {
	runner   BackupRunner
	schedule string
	logger   *slog.Logger
	lock     cache.DistributedLock // optional; nil = single-instance mode

	mu      sync.Mutex
	cron    *cron.Cron
	running bool
}

// NewBackupScheduler creates a scheduler that runs BackupAll on the given cron
// schedule (5-field unix format, e.g. "0 3 * * *" = 3am daily). An empty
// schedule disables the scheduler (Start becomes a no-op).
func NewBackupScheduler(runner BackupRunner, schedule string, logger *slog.Logger) *BackupScheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &BackupScheduler{
		runner:   runner,
		schedule: schedule,
		logger:   logger,
	}
}

// SetDistributedLock injects a distributed lock. Must be called before Start.
// Passing nil clears the lock and degrades to single-instance mode.
func (s *BackupScheduler) SetDistributedLock(lock cache.DistributedLock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lock = lock
}

// Start parses the cron schedule and launches the scheduler. Returns an error
// if the schedule is invalid. Idempotent: calling Start on an already-running
// scheduler is a no-op. An empty schedule is treated as "disabled" (no-op, no
// error).
func (s *BackupScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil
	}
	if s.schedule == "" {
		s.logger.Info("backup scheduler disabled (no schedule configured)")
		return nil
	}

	c := cron.New(cron.WithLogger(cron.PrintfLogger(stdLogger{s.logger})))
	id, err := c.AddFunc(s.schedule, s.runBackup)
	if err != nil {
		return fmt.Errorf("invalid backup schedule %q: %w", s.schedule, err)
	}
	_ = id
	c.Start()
	s.cron = c
	s.running = true
	s.logger.Info("backup scheduler started", "schedule", s.schedule)
	return nil
}

// Stop signals the scheduler to exit. Idempotent and safe to call from a
// different goroutine (e.g. graceful shutdown).
func (s *BackupScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	ctx := s.cron.Stop()
	s.running = false
	s.mu.Unlock()

	// Wait for in-flight jobs to finish (cron.Stop returns a context that is
	// done when all jobs complete).
	select {
	case <-ctx.Done():
	case <-time.After(30 * time.Second):
		s.logger.Warn("backup scheduler stop timed out after 30s")
	}
	s.logger.Info("backup scheduler stopped")
}

// Running returns whether the scheduler is currently active.
func (s *BackupScheduler) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Tick performs one backup immediately, bypassing the cron schedule. This is
// useful for testing and for manual triggers. If a distributed lock is
// configured, Tick acquires it before backing up.
func (s *BackupScheduler) Tick() error {
	return s.runBackupWithLock()
}

// runBackup is the cron entry point.
func (s *BackupScheduler) runBackup() {
	if err := s.runBackupWithLock(); err != nil {
		s.logger.Error("scheduled backup failed", "error", err)
	}
}

// runBackupWithLock acquires the distributed lock (if configured) and then
// runs BackupAll. Returns the backup error, if any.
func (s *BackupScheduler) runBackupWithLock() error {
	s.mu.Lock()
	lock := s.lock
	s.mu.Unlock()

	if lock == nil {
		return s.doBackup()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	lockKey := "backup-scheduler"
	lockTTL := 10 * time.Minute // backups can take a while; TTL is a safety net
	release, ok, err := lock.Acquire(ctx, lockKey, lockTTL)
	if err != nil {
		return fmt.Errorf("acquire backup lock: %w", err)
	}
	if !ok {
		s.logger.Debug("backup scheduler skipped — lock held by another instance")
		return nil
	}
	defer func() { _ = release() }()

	return s.doBackup()
}

// doBackup calls BackupAll and logs the result. The Manager's cleanup
// (MaxBackups retention) runs automatically inside BackupAll.
func (s *BackupScheduler) doBackup() error {
	start := time.Now()
	dbPath, mediaPath, err := s.runner.BackupAll()
	elapsed := time.Since(start)
	if err != nil {
		s.logger.Error("backup failed",
			"error", err,
			"elapsed", elapsed.String(),
		)
		return fmt.Errorf("scheduled backup: %w", err)
	}
	s.logger.Info("scheduled backup completed",
		"db", dbPath,
		"media", mediaPath,
		"elapsed", elapsed.String(),
	)
	return nil
}

// stdLogger adapts *slog.Logger to cron's PrintfLogger interface (io.Writer-ish).
type stdLogger struct{ l *slog.Logger }

func (w stdLogger) Printf(format string, args ...interface{}) {
	w.l.Info(fmt.Sprintf(format, args...))
}
