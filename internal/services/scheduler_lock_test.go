package services

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yamovo/contentx/internal/cache"
)

// fakePublisher 记录 PublishDueScheduled 调用次数。
type fakePublisher struct {
	calls atomic.Int64
	mu    sync.Mutex
	delay time.Duration
}

func (f *fakePublisher) PublishDueScheduled(now time.Time) (int, error) {
	f.calls.Add(1)
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return 1, nil
}

func (f *fakePublisher) CallCount() int64 {
	return f.calls.Load()
}

// TestPublishScheduler_NoLock_SingleInstance 验证无锁模式正常工作。
func TestPublishScheduler_NoLock_SingleInstance(t *testing.T) {
	pub := &fakePublisher{}
	s := NewPublishScheduler(pub, 50*time.Millisecond, slog.Default())

	_, err := s.Tick()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pub.CallCount() != 1 {
		t.Fatalf("expected 1 call, got %d", pub.CallCount())
	}

	// 再次 Tick 应再次调用
	s.Tick()
	if pub.CallCount() != 2 {
		t.Fatalf("expected 2 calls, got %d", pub.CallCount())
	}
}

// TestPublishScheduler_WithLock_PreventsConcurrentExecution 验证分布式锁阻止并发执行。
func TestPublishScheduler_WithLock_PreventsConcurrentExecution(t *testing.T) {
	pub := &fakePublisher{delay: 100 * time.Millisecond} // 每次 sweep 耗时 100ms
	lock := cache.NewMemoryLock()                         // 进程内锁模拟分布式锁

	s := NewPublishScheduler(pub, 50*time.Millisecond, slog.Default())
	s.SetDistributedLock(lock)

	// 并发 3 个 Tick，因锁存在，只有第 1 个能执行，其余 2 个跳过
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Tick()
		}()
	}
	wg.Wait()

	// 只有 1 次实际调用（其他被锁阻止）
	if pub.CallCount() != 1 {
		t.Fatalf("expected 1 call (locked), got %d", pub.CallCount())
	}

	// 等待 sweep 完成后锁释放，再次 Tick 应成功
	time.Sleep(150 * time.Millisecond)
	s.Tick()
	if pub.CallCount() != 2 {
		t.Fatalf("expected 2 calls after lock release, got %d", pub.CallCount())
	}
}

// TestPublishScheduler_LockReleaseOnError 验证锁在 sweep 出错时也会释放。
func TestPublishScheduler_LockReleaseOnError(t *testing.T) {
	// 用一个总是出错的 publisher
	pub := &errorPublisher{}
	lock := cache.NewMemoryLock()

	s := NewPublishScheduler(pub, 50*time.Millisecond, slog.Default())
	s.SetDistributedLock(lock)

	// 第一次 Tick 出错但应释放锁
	_, err := s.Tick()
	if err == nil {
		t.Fatal("expected error from errorPublisher")
	}

	// 锁应已释放，第二次 Tick 应能再次获取锁并执行
	_, err = s.Tick()
	if err == nil {
		t.Fatal("expected error from errorPublisher")
	}
}

// errorPublisher 总是返回错误。
type errorPublisher struct{}

func (e *errorPublisher) PublishDueScheduled(now time.Time) (int, error) {
	return 0, context.DeadlineExceeded
}

// TestPublishScheduler_SetDistributedLock_Nil 验证传 nil 清除锁。
func TestPublishScheduler_SetDistributedLock_Nil(t *testing.T) {
	pub := &fakePublisher{}
	lock := cache.NewMemoryLock()

	s := NewPublishScheduler(pub, 50*time.Millisecond, slog.Default())
	s.SetDistributedLock(lock)
	s.SetDistributedLock(nil) // 清除锁

	// 无锁模式，每次 Tick 都应直接执行
	s.Tick()
	s.Tick()
	if pub.CallCount() != 2 {
		t.Fatalf("expected 2 calls in no-lock mode, got %d", pub.CallCount())
	}
}

// TestPublishScheduler_StartStop 验证启动停止幂等。
func TestPublishScheduler_StartStop(t *testing.T) {
	pub := &fakePublisher{}
	s := NewPublishScheduler(pub, 50*time.Millisecond, slog.Default())

	// Start 幂等
	s.Start()
	s.Start()
	if !s.Running() {
		t.Fatal("expected scheduler to be running")
	}

	// Stop 幂等
	s.Stop()
	s.Stop()
	if s.Running() {
		t.Fatal("expected scheduler to be stopped")
	}
}
