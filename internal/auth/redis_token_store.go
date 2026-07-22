package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisTokenStore 实现 TokenStore 接口，将吊销的 JWT 存入 Redis。
//
// 优势：
//   - 多实例共享：任意节点吊销的 token 对所有节点立即生效
//   - 重启不丢失：服务重启后吊销状态保留
//   - 自动过期：每个 key 设置与 JWT 相同的 TTL，过期后由 Redis 自动回收
//
// 兼容性：保留 nil 行为（store == nil 时调用方应跳过检查）。
type RedisTokenStore struct {
	client *redis.Client
	prefix string
}

// NewRedisTokenStore 创建一个基于 Redis 的 TokenStore。
// prefix 为 Redis key 前缀，默认 "contentx:jwt:blacklist:"。
func NewRedisTokenStore(client *redis.Client, prefix string) *RedisTokenStore {
	if prefix == "" {
		prefix = "contentx:jwt:blacklist:"
	}
	return &RedisTokenStore{client: client, prefix: prefix}
}

// Revoke 将 token 加入黑名单，TTL 设为 JWT 过期时刻到当前时间的差值。
// 若 expiresAt 已过去，则直接跳过（token 自然失效）。
func (s *RedisTokenStore) Revoke(tokenStr string, expiresAt time.Time) {
	if s == nil || s.client == nil {
		return
	}
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return
	}
	// 用 token 的 SHA-256 摘要作为 key，避免长 token 字符串占用过多内存。
	key := s.key(tokenStr)
	_ = s.client.Set(context.Background(), key, "1", ttl).Err()
}

// IsRevoked 检查 token 是否在黑名单中。
func (s *RedisTokenStore) IsRevoked(tokenStr string) bool {
	if s == nil || s.client == nil {
		return false
	}
	key := s.key(tokenStr)
	n, err := s.client.Exists(context.Background(), key).Result()
	if err != nil {
		// Redis 故障时采用 fail-open：不阻断已验证有效的 token。
		// 这与现有 Blacklist（内存版）的 IsRevoked 返回 false 语义一致。
		return false
	}
	return n > 0
}

// key 生成带前缀的 Redis key，使用 token 的 SHA-256 摘要。
func (s *RedisTokenStore) key(tokenStr string) string {
	sum := sha256.Sum256([]byte(tokenStr))
	return s.prefix + hex.EncodeToString(sum[:])
}

// Ping 验证 Redis 连通性，启动时调用。
func (s *RedisTokenStore) Ping(ctx context.Context) error {
	if s == nil || s.client == nil {
		return errors.New("redis token store: nil client")
	}
	return s.client.Ping(ctx).Err()
}

// Compile-time check: *RedisTokenStore 实现 TokenStore。
var _ TokenStore = (*RedisTokenStore)(nil)
