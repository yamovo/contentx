package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryDriver_SetGet(t *testing.T) {
	d := NewMemoryDriver(100)
	ctx := context.Background()

	if err := d.Set(ctx, "k1", []byte("v1"), time.Minute); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := d.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "v1" {
		t.Fatalf("expected v1, got %q", got)
	}
}

func TestMemoryDriver_GetMiss(t *testing.T) {
	d := NewMemoryDriver(100)
	if _, err := d.Get(context.Background(), "missing"); err != ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}
}

func TestMemoryDriver_Expiry(t *testing.T) {
	d := NewMemoryDriver(100)
	ctx := context.Background()
	_ = d.Set(ctx, "k", []byte("v"), 10*time.Millisecond)
	time.Sleep(25 * time.Millisecond)
	if _, err := d.Get(ctx, "k"); err != ErrCacheMiss {
		t.Fatalf("expected miss after expiry, got %v", err)
	}
}

func TestMemoryDriver_Delete(t *testing.T) {
	d := NewMemoryDriver(100)
	ctx := context.Background()
	_ = d.Set(ctx, "k", []byte("v"), time.Minute)
	if err := d.Delete(ctx, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := d.Get(ctx, "k"); err != ErrCacheMiss {
		t.Fatalf("expected miss after delete, got %v", err)
	}
}

func TestMemoryDriver_Flush(t *testing.T) {
	d := NewMemoryDriver(100)
	ctx := context.Background()
	_ = d.Set(ctx, "a", []byte("1"), time.Minute)
	_ = d.Set(ctx, "b", []byte("2"), time.Minute)
	if err := d.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if _, err := d.Get(ctx, "a"); err != ErrCacheMiss {
		t.Fatalf("expected miss after flush for a")
	}
	if _, err := d.Get(ctx, "b"); err != ErrCacheMiss {
		t.Fatalf("expected miss after flush for b")
	}
}

// TestMemoryDriver_ValueIsolation ensures stored values are defensively copied
// so callers cannot corrupt the cache by mutating input or output slices.
func TestMemoryDriver_ValueIsolation(t *testing.T) {
	d := NewMemoryDriver(100)
	ctx := context.Background()

	orig := []byte("hello")
	_ = d.Set(ctx, "k", orig, time.Minute)
	orig[0] = 'X' // mutate source after Set

	got, _ := d.Get(ctx, "k")
	if string(got) != "hello" {
		t.Fatalf("stored value affected by source mutation: %q", got)
	}

	got[0] = 'Y' // mutate returned slice
	got2, _ := d.Get(ctx, "k")
	if string(got2) != "hello" {
		t.Fatalf("stored value affected by returned-slice mutation: %q", got2)
	}
}

func TestMemoryDriver_Eviction(t *testing.T) {
	d := NewMemoryDriver(2)
	ctx := context.Background()
	_ = d.Set(ctx, "a", []byte("1"), time.Minute)
	_ = d.Set(ctx, "b", []byte("2"), time.Minute)
	_ = d.Set(ctx, "c", []byte("3"), time.Minute) // triggers eviction

	stats := d.Stats()
	if entries, ok := stats["entries"].(int); !ok || entries > 2 {
		t.Fatalf("expected <= 2 entries after eviction, got %v", stats["entries"])
	}
}

func TestNewMemoryDriver_DefaultCapacity(t *testing.T) {
	d := NewMemoryDriver(0)
	if d.maxSize != 10000 {
		t.Fatalf("expected default capacity 10000, got %d", d.maxSize)
	}
}
