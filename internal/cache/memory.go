package cache

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrCacheMiss = errors.New("cache miss")

type cacheEntry struct {
	value     []byte
	expiresAt time.Time
}

// MemoryDriver is a simple in-memory cache with LRU-like eviction.
type MemoryDriver struct {
	mu       sync.RWMutex
	entries  map[string]*cacheEntry
	maxSize  int
}

// NewMemoryDriver creates a new in-memory cache.
func NewMemoryDriver(maxEntries int) *MemoryDriver {
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return &MemoryDriver{
		entries: make(map[string]*cacheEntry),
		maxSize: maxEntries,
	}
}

func (d *MemoryDriver) Get(_ context.Context, key string) ([]byte, error) {
	d.mu.RLock()
	entry, exists := d.entries[key]
	d.mu.RUnlock()

	if !exists {
		return nil, ErrCacheMiss
	}
	if time.Now().After(entry.expiresAt) {
		d.mu.Lock()
		delete(d.entries, key)
		d.mu.Unlock()
		return nil, ErrCacheMiss
	}

	result := make([]byte, len(entry.value))
	copy(result, entry.value)
	return result, nil
}

func (d *MemoryDriver) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Evict oldest if at capacity.
	if len(d.entries) >= d.maxSize {
		d.evictOldest()
	}

	valCopy := make([]byte, len(value))
	copy(valCopy, value)

	d.entries[key] = &cacheEntry{
		value:     valCopy,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

func (d *MemoryDriver) Delete(_ context.Context, key string) error {
	d.mu.Lock()
	delete(d.entries, key)
	d.mu.Unlock()
	return nil
}

func (d *MemoryDriver) Flush(_ context.Context) error {
	d.mu.Lock()
	d.entries = make(map[string]*cacheEntry)
	d.mu.Unlock()
	return nil
}

func (d *MemoryDriver) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	for k, v := range d.entries {
		if oldestKey == "" || v.expiresAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.expiresAt
		}
	}
	if oldestKey != "" {
		delete(d.entries, oldestKey)
	}
}

// Stats returns cache statistics.
func (d *MemoryDriver) Stats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return map[string]interface{}{
		"entries":  len(d.entries),
		"capacity": d.maxSize,
	}
}
