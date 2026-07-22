package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisDriver implements the Driver interface backed by Redis (go-redis/v9).
type RedisDriver struct {
	client *redis.Client
	prefix string
}

// NewRedisDriver creates a new Redis cache driver. The connection is lazy;
// call Ping to verify connectivity before relying on it.
func NewRedisDriver(addr, password string, db int, prefix string) *RedisDriver {
	if prefix == "" {
		prefix = "contentx:"
	}
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &RedisDriver{client: client, prefix: prefix}
}

// key applies the configured prefix to a cache key.
func (d *RedisDriver) key(k string) string {
	return d.prefix + k
}

// Ping verifies the Redis connection is alive.
func (d *RedisDriver) Ping(ctx context.Context) error {
	return d.client.Ping(ctx).Err()
}

func (d *RedisDriver) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := d.client.Get(ctx, d.key(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (d *RedisDriver) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return d.client.Set(ctx, d.key(key), value, ttl).Err()
}

func (d *RedisDriver) Delete(ctx context.Context, key string) error {
	return d.client.Del(ctx, d.key(key)).Err()
}

// Flush removes only keys owned by this driver's prefix (never FLUSHDB the
// whole database, which may be shared with other applications).
func (d *RedisDriver) Flush(ctx context.Context) error {
	iter := d.client.Scan(ctx, 0, d.prefix+"*", 100).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return err
	}
	if len(keys) > 0 {
		return d.client.Del(ctx, keys...).Err()
	}
	return nil
}

// Close releases the underlying Redis client resources.
func (d *RedisDriver) Close() error {
	return d.client.Close()
}

// Client returns the underlying Redis client. Allows other modules (e.g. JWT
// token blacklist) to share the same connection pool instead of opening a
// second one.
func (d *RedisDriver) Client() *redis.Client {
	return d.client
}
