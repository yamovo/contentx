package auth

import (
	"testing"
	"time"
)

func TestLoginGuard_RecordFailed_Lockout(t *testing.T) {
	g := NewLoginGuard(WithMaxAttempts(3))
	defer g.Stop()

	key := "user@example.com"
	for i := 0; i < 3; i++ {
		locked, remaining := g.RecordFailed(key)
		if locked && i < 2 {
			t.Fatalf("attempt %d should not lock yet", i+1)
		}
		if i == 2 && !locked {
			t.Fatal("third attempt should lock")
		}
		if i < 2 && remaining != 3-(i+1) {
			t.Fatalf("expected %d remaining, got %d", 3-(i+1), remaining)
		}
	}

	locked, remaining := g.Check(key)
	if !locked {
		t.Fatal("should be locked after max attempts")
	}
	if remaining != 0 {
		t.Fatalf("expected 0 remaining when locked, got %d", remaining)
	}
}

func TestLoginGuard_RecordSuccess_Resets(t *testing.T) {
	g := NewLoginGuard(WithMaxAttempts(3))
	defer g.Stop()

	key := "user@example.com"
	g.RecordFailed(key)
	g.RecordFailed(key)
	g.RecordSuccess(key)

	locked, remaining := g.Check(key)
	if locked {
		t.Fatal("should not be locked after success")
	}
	if remaining != 3 {
		t.Fatalf("expected 3 remaining after reset, got %d", remaining)
	}
}

func TestLoginGuard_Check_UnknownKey(t *testing.T) {
	g := NewLoginGuard()
	defer g.Stop()

	locked, remaining := g.Check("nobody@example.com")
	if locked {
		t.Fatal("unknown key should not be locked")
	}
	if remaining != defaultMaxAttempts {
		t.Fatalf("expected %d remaining, got %d", defaultMaxAttempts, remaining)
	}
}

func TestLoginGuard_Stop_Idempotent(t *testing.T) {
	g := NewLoginGuard()
	// Multiple Stop calls must not panic.
	g.Stop()
	g.Stop()
}

func TestLoginGuard_WithCustomDurations(t *testing.T) {
	g := NewLoginGuard(
		WithMaxAttempts(2),
		WithLockDuration(1*time.Minute),
	)
	defer g.Stop()

	g.RecordFailed("k")
	locked, _ := g.RecordFailed("k")
	if !locked {
		t.Fatal("expected lock after 2 attempts")
	}
}
