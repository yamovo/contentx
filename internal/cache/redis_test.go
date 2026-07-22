package cache

import (
	"context"
	"os"
	"testing"
	"time"
)

// redisTestDriver returns a live Redis driver or skips the test when
// REDIS_TEST_ADDR is not set (e.g. in CI without a Redis service).
func redisTestDriver(t *testing.T) *RedisDriver {
	t.Helper()
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR not set; skipping live Redis test")
	}
	d := NewRedisDriver(addr, os.Getenv("REDIS_TEST_PASSWORD"), 0, "contentx_test:")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := d.Ping(ctx); err != nil {
		t.Skipf("redis not reachable at %s: %v", addr, err)
	}
	return d
}

// TestRedisDriver_KeyPrefix runs without a live server: it only checks the
// prefixing logic and default prefix.
func TestRedisDriver_KeyPrefix(t *testing.T) {
	d := NewRedisDriver("localhost:6379", "", 0, "")
	if d.prefix != "contentx:" {
		t.Fatalf("expected default prefix 'contentx:', got %q", d.prefix)
	}
	if got := d.key("abc"); got != "contentx:abc" {
		t.Fatalf("expected prefixed key 'contentx:abc', got %q", got)
	}

	custom := NewRedisDriver("localhost:6379", "", 0, "myapp:")
	if got := custom.key("x"); got != "myapp:x" {
		t.Fatalf("expected 'myapp:x', got %q", got)
	}
}

func TestRedisDriver_RoundTrip(t *testing.T) {
	d := redisTestDriver(t)
	defer d.Close()
	ctx := context.Background()
	_ = d.Flush(ctx)

	if err := d.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := d.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "v" {
		t.Fatalf("expected v, got %q", got)
	}

	if err := d.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := d.Get(ctx, "k"); err != ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss after delete, got %v", err)
	}
}

func TestRedisDriver_Miss(t *testing.T) {
	d := redisTestDriver(t)
	defer d.Close()
	if _, err := d.Get(context.Background(), "does-not-exist"); err != ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}
}

func TestRedisDriver_FlushOnlyPrefix(t *testing.T) {
	d := redisTestDriver(t)
	defer d.Close()
	ctx := context.Background()

	_ = d.Set(ctx, "a", []byte("1"), time.Minute)
	_ = d.Set(ctx, "b", []byte("2"), time.Minute)
	if err := d.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if _, err := d.Get(ctx, "a"); err != ErrCacheMiss {
		t.Fatalf("expected miss after flush, got %v", err)
	}
}
