package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryLock_AcquireAndRelease(t *testing.T) {
	l := NewMemoryLock()
	ctx := context.Background()

	// 第一次获取成功
	release, ok, err := l.Acquire(ctx, "test-key", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected to acquire lock")
	}

	// 第二次获取同一 key 应失败
	_, ok2, err := l.Acquire(ctx, "test-key", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok2 {
		t.Fatal("expected lock to be held, second acquire should fail")
	}

	// 释放后应能再次获取
	if err := release(); err != nil {
		t.Fatalf("release failed: %v", err)
	}

	release3, ok3, err := l.Acquire(ctx, "test-key", 10*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok3 {
		t.Fatal("expected to acquire lock after release")
	}
	release3()
}

func TestMemoryLock_DifferentKeys(t *testing.T) {
	l := NewMemoryLock()
	ctx := context.Background()

	_, ok1, err := l.Acquire(ctx, "key-a", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok1 {
		t.Fatal("expected to acquire key-a")
	}

	_, ok2, err := l.Acquire(ctx, "key-b", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok2 {
		t.Fatal("expected to acquire key-b independently")
	}
}

func TestMemoryLock_TTLExpiry(t *testing.T) {
	l := NewMemoryLock()
	ctx := context.Background()

	// 获取锁，TTL 50ms
	_, ok, err := l.Acquire(ctx, "ttl-key", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected to acquire lock")
	}

	// 立即再次获取应失败
	_, ok2, _ := l.Acquire(ctx, "ttl-key", 50*time.Millisecond)
	if ok2 {
		t.Fatal("expected lock to be held")
	}

	// 等待 TTL 过期后应能再次获取
	time.Sleep(80 * time.Millisecond)
	_, ok3, err := l.Acquire(ctx, "ttl-key", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok3 {
		t.Fatal("expected to acquire lock after TTL expiry")
	}
}

func TestRedisLock_AcquireAndRelease(t *testing.T) {
	// RedisLock 需要 Redis 实例，单元测试跳过。
	// 集成测试在 docker-compose 环境中运行。
	t.Skip("RedisLock requires Redis instance — run in integration environment")
}

func TestRandomToken_Uniqueness(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		token := randomToken()
		if tokens[token] {
			t.Fatalf("duplicate token generated at iteration %d: %s", i, token)
		}
		tokens[token] = true
	}
}

func TestEndsWithColon(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"contentx:", true},
		{"contentx", false},
		{":", true},
		{"", false},
		{"contentx:lock", false},
	}
	for _, tt := range tests {
		got := endsWithColon(tt.input)
		if got != tt.expected {
			t.Errorf("endsWithColon(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
