package backup

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/cache"
)

// fakeRunner is a test BackupRunner that records calls and optionally returns
// an error.
type fakeRunner struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (f *fakeRunner) BackupAll() (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return "", "", f.err
	}
	return "/tmp/db-backup.sql", "/tmp/media-backup.tar.gz", nil
}

func (f *fakeRunner) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestBackupScheduler_InvalidSchedule(t *testing.T) {
	runner := &fakeRunner{}
	s := NewBackupScheduler(runner, "not a valid cron", nil)
	if err := s.Start(); err == nil {
		t.Fatal("expected error for invalid cron schedule, got nil")
	}
	if s.Running() {
		t.Fatal("scheduler should not be running after invalid Start")
	}
}

func TestBackupScheduler_EmptySchedule(t *testing.T) {
	runner := &fakeRunner{}
	s := NewBackupScheduler(runner, "", nil)
	if err := s.Start(); err != nil {
		t.Fatalf("Start with empty schedule should be a no-op, got: %v", err)
	}
	if s.Running() {
		t.Fatal("scheduler should not be running for empty schedule")
	}
	// Tick should still work even when the cron is disabled.
	if err := s.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if runner.CallCount() != 1 {
		t.Fatalf("expected 1 backup call, got %d", runner.CallCount())
	}
}

func TestBackupScheduler_Tick_RunsBackup(t *testing.T) {
	runner := &fakeRunner{}
	s := NewBackupScheduler(runner, "", nil) // empty schedule = no cron, but Tick works

	if err := s.Tick(); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if runner.CallCount() != 1 {
		t.Fatalf("expected 1 backup call, got %d", runner.CallCount())
	}
}

func TestBackupScheduler_Tick_BackupError(t *testing.T) {
	runner := &fakeRunner{err: errors.New("disk full")}
	s := NewBackupScheduler(runner, "", nil)

	err := s.Tick()
	if err == nil {
		t.Fatal("expected error from Tick when backup fails")
	}
	if !errors.Is(err, errors.New("disk full")) {
		// The error is wrapped, so we check the message contains the original.
		if err.Error() == "" || !contains(err.Error(), "disk full") {
			t.Fatalf("expected error to contain 'disk full', got: %v", err)
		}
	}
	if runner.CallCount() != 1 {
		t.Fatalf("expected 1 backup call even on error, got %d", runner.CallCount())
	}
}

func TestBackupScheduler_Tick_WithLock(t *testing.T) {
	runner := &fakeRunner{}
	s := NewBackupScheduler(runner, "", nil)
	s.SetDistributedLock(cache.NewMemoryLock())

	if err := s.Tick(); err != nil {
		t.Fatalf("Tick with lock: %v", err)
	}
	if runner.CallCount() != 1 {
		t.Fatalf("expected 1 backup call, got %d", runner.CallCount())
	}
}

func TestBackupScheduler_Tick_LockHeld_Skips(t *testing.T) {
	runner := &fakeRunner{}
	lock := cache.NewMemoryLock()
	s := NewBackupScheduler(runner, "", nil)
	s.SetDistributedLock(lock)

	// Hold the lock manually so the scheduler's Tick can't acquire it.
	ctx := context.Background()
	release, ok, err := lock.Acquire(ctx, "backup-scheduler", time.Minute)
	if err != nil || !ok {
		t.Fatalf("failed to acquire lock manually: ok=%v err=%v", ok, err)
	}
	defer release()

	// Tick should skip silently (no error, no backup call).
	if err := s.Tick(); err != nil {
		t.Fatalf("Tick should skip silently when lock is held, got: %v", err)
	}
	if runner.CallCount() != 0 {
		t.Fatalf("expected 0 backup calls when lock is held, got %d", runner.CallCount())
	}
}

func TestBackupScheduler_StartStop(t *testing.T) {
	runner := &fakeRunner{}
	s := NewBackupScheduler(runner, "0 3 * * *", nil)

	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !s.Running() {
		t.Fatal("expected scheduler to be running")
	}
	// Start again — should be idempotent.
	if err := s.Start(); err != nil {
		t.Fatalf("second Start should be idempotent: %v", err)
	}
	s.Stop()
	if s.Running() {
		t.Fatal("expected scheduler to be stopped")
	}
	// Stop again — should be idempotent.
	s.Stop()
}

// contains is a minimal strings.Contains to avoid importing strings here.
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
