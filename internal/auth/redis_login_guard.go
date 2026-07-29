package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLoginGuard 是 LoginGuard 的 Redis 版实现（SEC-6）：
// 失败计数与锁定状态存入 Redis，多实例共享——任意节点触发的锁定
// 对所有节点立即生效，防分布式撞库；与 RedisTokenStore 对称。
//
// 存储结构：
//   - {prefix}fail:{key}  失败计数（INCR，TTL=windowDuration）
//   - {prefix}lock:{key}  锁定标记（SET，TTL=lockDuration）
//
// 故障策略：Redis 出错时委托内嵌的内存版 LoginGuard 兜底（与
// ResilientTokenStore 的 SEC-2 策略一致），本实例的限流不失效。
type RedisLoginGuard struct {
	client         *redis.Client
	prefix         string
	maxAttempts    int
	lockDuration   time.Duration
	windowDuration time.Duration
	fallback       *LoginGuard
}

// NewRedisLoginGuard 创建 Redis 版登录守卫。
// prefix 为 Redis key 前缀，默认 "contentx:login:"。
func NewRedisLoginGuard(client *redis.Client, prefix string) *RedisLoginGuard {
	if prefix == "" {
		prefix = "contentx:login:"
	}
	return &RedisLoginGuard{
		client:         client,
		prefix:         prefix,
		maxAttempts:    defaultMaxAttempts,
		lockDuration:   defaultLockDuration,
		windowDuration: defaultWindowDuration,
		fallback:       NewLoginGuard(),
	}
}

func (g *RedisLoginGuard) failKey(key string) string { return g.prefix + "fail:" + key }
func (g *RedisLoginGuard) lockKey(key string) string { return g.prefix + "lock:" + key }

// ctx 返回每次 Redis 操作的短超时 context，避免 Redis 卡死拖慢登录路径。
func (g *RedisLoginGuard) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}

// Check returns (locked bool, remainingAttempts int).
func (g *RedisLoginGuard) Check(key string) (bool, int) {
	ctx, cancel := g.ctx()
	defer cancel()

	locked, err := g.client.Exists(ctx, g.lockKey(key)).Result()
	if err != nil {
		return g.fallback.Check(key)
	}
	if locked > 0 {
		return true, 0
	}

	count, err := g.client.Get(ctx, g.failKey(key)).Int()
	if err != nil && err != redis.Nil {
		return g.fallback.Check(key)
	}
	remaining := g.maxAttempts - count
	if remaining < 0 {
		remaining = 0
	}
	return false, remaining
}

// RecordFailed increments the failure counter; locks the key when the
// threshold is reached. Returns (locked, remaining).
func (g *RedisLoginGuard) RecordFailed(key string) (bool, int) {
	ctx, cancel := g.ctx()
	defer cancel()

	fk := g.failKey(key)
	count, err := g.client.Incr(ctx, fk).Result()
	if err != nil {
		return g.fallback.RecordFailed(key)
	}
	// 首次失败时设置窗口 TTL（NX 语义由 count==1 保证）。
	if count == 1 {
		_ = g.client.Expire(ctx, fk, g.windowDuration).Err()
	}

	if int(count) >= g.maxAttempts {
		_ = g.client.Set(ctx, g.lockKey(key), "1", g.lockDuration).Err()
		return true, 0
	}
	return false, g.maxAttempts - int(count)
}

// RecordSuccess clears the failure state for the key.
func (g *RedisLoginGuard) RecordSuccess(key string) {
	ctx, cancel := g.ctx()
	defer cancel()

	_ = g.client.Del(ctx, g.failKey(key), g.lockKey(key)).Err()
	// 同步清理 fallback，避免 Redis 故障窗口内的残留计数误伤。
	g.fallback.RecordSuccess(key)
}

// MaxAttempts returns the configured lockout threshold.
func (g *RedisLoginGuard) MaxAttempts() int { return g.maxAttempts }

// Stop terminates the fallback guard's cleanup goroutine.
func (g *RedisLoginGuard) Stop() { g.fallback.Stop() }

// Ping 验证 Redis 连通性，启动时调用。
func (g *RedisLoginGuard) Ping(ctx context.Context) error {
	return g.client.Ping(ctx).Err()
}

// Compile-time check: *RedisLoginGuard 实现 LoginLimiter。
var _ LoginLimiter = (*RedisLoginGuard)(nil)
