package cache

import (
	"context"
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
	Driver     string // "memory" or "redis"
	Redis      RedisConfig
	Memory     MemoryConfig
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
