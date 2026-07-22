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

// LoginGuard tracks failed login attempts and locks accounts.
// Safe for concurrent use.
type LoginGuard struct {
	mu             sync.RWMutex
	attempts       map[string]*attemptRecord
	maxAttempts    int
	lockDuration   time.Duration
	windowDuration time.Duration
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
	}
	for _, opt := range opts {
		opt(g)
	}
	// Background cleanup every minute.
	go g.cleanup()
	return g
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

// cleanup periodically removes expired records.
func (g *LoginGuard) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
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

// Compile-time check.
var _ = (*LoginGuard)(nil)
