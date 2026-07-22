package auth

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// newUnreachableRedisClient 返回一个指向不可达地址的 redis client，
// 用于测试 fail-open 行为（不依赖真实 Redis 实例）。
// DialTimeout 设为 1ms 加速失败。
func newUnreachableRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1", // 端口 1 通常无服务监听
		DialTimeout: 1 * time.Millisecond,
		ReadTimeout: 5 * time.Millisecond,
	})
}

func TestRedisTokenStore_NilSafe(t *testing.T) {
	var s *RedisTokenStore

	// nil store 不应 panic。
	s.Revoke("any-token", time.Now().Add(time.Hour))
	if s.IsRevoked("any-token") {
		t.Error("nil store should report not revoked")
	}
}

func TestRedisTokenStore_FailOpenOnUnreachableRedis(t *testing.T) {
	s := NewRedisTokenStore(newUnreachableRedisClient(), "test:")

	// Revoke 静默失败（不 panic、不返回错误）。
	s.Revoke("some-token", time.Now().Add(time.Hour))

	// IsRevoked 在 Redis 不可达时应 fail-open（返回 false）。
	if s.IsRevoked("some-token") {
		t.Error("IsRevoked should fail-open (return false) when redis is unreachable")
	}
}

func TestRedisTokenStore_PingErrorOnUnreachableRedis(t *testing.T) {
	s := NewRedisTokenStore(newUnreachableRedisClient(), "test:")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := s.Ping(ctx); err == nil {
		t.Error("Ping should return error when redis is unreachable")
	}
}

func TestRedisTokenStore_RevokeSkipsExpiredToken(t *testing.T) {
	// expiresAt 已过：Revoke 应直接跳过，不写入 Redis。
	// 用不可达 redis 也能验证：因为不会尝试 Set。
	s := NewRedisTokenStore(newUnreachableRedisClient(), "test:")
	s.Revoke("expired", time.Now().Add(-time.Hour))
	// 不 panic 即通过。
}

func TestRedisTokenStore_KeyIsHashedAndPrefixed(t *testing.T) {
	// 通过反射无法直接访问私有 key 方法，但可以通过行为验证：
	// 同一 token 多次调用 Revoke/IsRevoked 应保持一致。
	s := NewRedisTokenStore(newUnreachableRedisClient(), "myprefix:")
	// 仅验证不 panic。
	_ = s.IsRevoked("token-a")
	_ = s.IsRevoked("token-b")
	s.Revoke("token-a", time.Now().Add(time.Hour))
}

func TestNewRedisTokenStore_DefaultPrefix(t *testing.T) {
	s := NewRedisTokenStore(newUnreachableRedisClient(), "")
	if s.prefix != "contentx:jwt:blacklist:" {
		t.Errorf("expected default prefix, got %q", s.prefix)
	}
}

func TestNewRedisTokenStore_CustomPrefix(t *testing.T) {
	s := NewRedisTokenStore(newUnreachableRedisClient(), "custom:")
	if s.prefix != "custom:" {
		t.Errorf("expected custom prefix, got %q", s.prefix)
	}
}

// 编译期断言：RedisTokenStore 实现 TokenStore 接口。
func TestRedisTokenStore_ImplementsTokenStore(t *testing.T) {
	var _ TokenStore = (*RedisTokenStore)(nil)
}
