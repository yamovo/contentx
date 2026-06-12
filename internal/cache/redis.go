package cache

import (
	"context"
	"fmt"
	"time"
)

// RedisDriver implements cache using Redis via a minimal TCP client.
// For production, use github.com/redis/go-redis/v9.
// This is a placeholder that can be swapped with a real Redis client.
type RedisDriver struct {
	addr   string
	prefix string
}

// NewRedisDriver creates a new Redis cache driver.
func NewRedisDriver(addr, prefix string) *RedisDriver {
	if prefix == "" {
		prefix = "vortex:"
	}
	return &RedisDriver{addr: addr, prefix: prefix}
}

func (d *RedisDriver) Get(_ context.Context, key string) ([]byte, error) {
	// TODO: implement with go-redis or raw TCP
	return nil, ErrCacheMiss
}

func (d *RedisDriver) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	// TODO: implement with go-redis or raw TCP
	_ = fmt.Sprintf("%s%s", d.prefix, key)
	return nil
}

func (d *RedisDriver) Delete(_ context.Context, key string) error {
	return nil
}

func (d *RedisDriver) Flush(_ context.Context) error {
	return nil
}
