package cache

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Driver defines the interface for cache backends.
type Driver interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Flush(ctx context.Context) error
}

// Config holds cache configuration.
type Config struct {
	Driver string // "memory" or "redis"
	Redis  RedisConfig
	Memory MemoryConfig
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	Prefix   string
}

// MemoryConfig holds in-memory cache settings.
type MemoryConfig struct {
	MaxEntries int
	DefaultTTL time.Duration
}

// New creates a cache Driver from the given configuration.
// Supported drivers: "memory" (default) and "redis". For redis, the
// connection is verified with a ping and an error is returned on failure
// so the caller can decide whether to fall back to memory.
func New(cfg Config) (Driver, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Driver)) {
	case "redis":
		d := NewRedisDriver(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB, cfg.Redis.Prefix)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := d.Ping(ctx); err != nil {
			return nil, fmt.Errorf("redis ping failed: %w", err)
		}
		return d, nil
	case "memory", "":
		return NewMemoryDriver(cfg.Memory.MaxEntries), nil
	default:
		return nil, fmt.Errorf("unknown cache driver: %q", cfg.Driver)
	}
}
