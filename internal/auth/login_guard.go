package auth

import (
	"sync"
	"time"
)

const (
	defaultMaxAttempts    = 5
	defaultLockDuration   = 15 * time.Minute
	defaultWindowDuration = 15 * time.Minute
)

// LoginLimiter is the contract for failed-login throttling backends.
// Implementations must be safe for concurrent use.
//
// 实现：*LoginGuard（内存版，单实例）与 *RedisLoginGuard（多实例共享，SEC-6）。
type LoginLimiter interface {
	// Check returns (locked bool, remainingAttempts int) without mutating state.
	Check(key string) (bool, int)
	// RecordFailed increments the failure counter; returns (locked, remaining).
	RecordFailed(key string) (bool, int)
	// RecordSuccess clears the failure state for the key.
	RecordSuccess(key string)
	// MaxAttempts returns the configured lockout threshold.
	MaxAttempts() int
	// Stop releases background resources. Safe to call multiple times.
	Stop()
}

// LoginGuard tracks failed login attempts and locks accounts.
// Safe for concurrent use.
type LoginGuard struct {
	mu             sync.RWMutex
	attempts       map[string]*attemptRecord
	maxAttempts    int
	lockDuration   time.Duration
	windowDuration time.Duration
	stop           chan struct{}
}

type attemptRecord struct {
	count        int
	lockedUntil  time.Time
	firstAttempt time.Time
}

// LoginGuardOption configures LoginGuard.
type LoginGuardOption func(*LoginGuard)

// WithMaxAttempts sets the maximum failed attempts before lockout.
func WithMaxAttempts(n int) LoginGuardOption {
	return func(g *LoginGuard) { g.maxAttempts = n }
}

// MaxAttempts returns the configured maximum failed attempts before lockout.
func (g *LoginGuard) MaxAttempts() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.maxAttempts
}

// WithLockDuration sets how long an account stays locked.
func WithLockDuration(d time.Duration) LoginGuardOption {
	return func(g *LoginGuard) { g.lockDuration = d }
}

// NewLoginGuard creates a new login guard with optional configuration.
func NewLoginGuard(opts ...LoginGuardOption) *LoginGuard {
	g := &LoginGuard{
		attempts:       make(map[string]*attemptRecord),
		maxAttempts:    defaultMaxAttempts,
		lockDuration:   defaultLockDuration,
		windowDuration: defaultWindowDuration,
		stop:           make(chan struct{}),
	}
	for _, opt := range opts {
		opt(g)
	}
	// Background cleanup every minute.
	go g.cleanup()
	return g
}

// Stop terminates the background cleanup goroutine. It is safe to call
// multiple times; subsequent calls are no-ops.
func (g *LoginGuard) Stop() {
	select {
	case <-g.stop:
		// already closed
	default:
		close(g.stop)
	}
}

// Check returns (locked bool, remainingAttempts int).
// key is typically "username" or "user_id" (case-insensitive).
func (g *LoginGuard) Check(key string) (bool, int) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	rec, exists := g.attempts[key]
	if !exists {
		return false, g.maxAttempts
	}

	// Check if lockout has expired.
	if !rec.lockedUntil.IsZero() && time.Now().Before(rec.lockedUntil) {
		return true, 0
	}

	remaining := g.maxAttempts - rec.count
	if remaining < 0 {
		remaining = 0
	}
	return false, remaining
}

// RecordFailed increments the failed attempt counter for the given key.
// Returns (locked bool, remainingAttempts int).
func (g *LoginGuard) RecordFailed(key string) (bool, int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	rec, exists := g.attempts[key]
	if !exists {
		rec = &attemptRecord{firstAttempt: time.Now()}
		g.attempts[key] = rec
	}

	// Reset if window has expired.
	if time.Since(rec.firstAttempt) > g.windowDuration {
		rec.count = 0
		rec.firstAttempt = time.Now()
		rec.lockedUntil = time.Time{}
	}

	rec.count++

	if rec.count >= g.maxAttempts {
		rec.lockedUntil = time.Now().Add(g.lockDuration)
		return true, 0
	}

	return false, g.maxAttempts - rec.count
}

// RecordSuccess resets the failed attempt counter for the given key.
func (g *LoginGuard) RecordSuccess(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.attempts, key)
}

// cleanup periodically removes expired records. Exits when Stop is called.
func (g *LoginGuard) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-g.stop:
			return
		case <-ticker.C:
			g.mu.Lock()
			now := time.Now()
			for key, rec := range g.attempts {
				if !rec.lockedUntil.IsZero() && now.After(rec.lockedUntil) &&
					now.Sub(rec.firstAttempt) > g.windowDuration {
					delete(g.attempts, key)
				}
			}
			g.mu.Unlock()
		}
	}
}

// Compile-time check: *LoginGuard 实现 LoginLimiter。
var _ LoginLimiter = (*LoginGuard)(nil)
