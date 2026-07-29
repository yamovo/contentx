package auth

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// ─── SEC-6：RedisLoginGuard（多实例共享登录限流）───────────────────────────
//
// 无 Redis 环境下验证 fallback 行为（不可达 client）；
// 设置 REDIS_TEST_ADDR 时额外验证真实 Redis 的共享锁定语义。

func TestNewRedisLoginGuard_DefaultPrefix(t *testing.T) {
	g := NewRedisLoginGuard(newUnreachableRedisClient(), "")
	defer g.Stop()
	if g.prefix != "contentx:login:" {
		t.Errorf("expected default prefix, got %q", g.prefix)
	}
}

func TestRedisLoginGuard_MaxAttempts(t *testing.T) {
	g := NewRedisLoginGuard(newUnreachableRedisClient(), "test:")
	defer g.Stop()
	if g.MaxAttempts() != defaultMaxAttempts {
		t.Errorf("expected %d, got %d", defaultMaxAttempts, g.MaxAttempts())
	}
}

func TestRedisLoginGuard_FallbackLocksWhenRedisDown(t *testing.T) {
	// Redis 不可达时必须退化到内存版限流，而不是放弃限流（fail-open）。
	g := NewRedisLoginGuard(newUnreachableRedisClient(), "test:")
	defer g.Stop()

	var locked bool
	for i := 0; i < defaultMaxAttempts; i++ {
		locked, _ = g.RecordFailed("attacker")
	}
	if !locked {
		t.Error("SEC-6: fallback guard must lock after max attempts even when redis is down")
	}
	if lockedNow, _ := g.Check("attacker"); !lockedNow {
		t.Error("Check should report locked via fallback when redis is down")
	}
}

func TestRedisLoginGuard_RecordSuccessClearsFallback(t *testing.T) {
	g := NewRedisLoginGuard(newUnreachableRedisClient(), "test:")
	defer g.Stop()

	g.RecordFailed("user1")
	g.RecordSuccess("user1")

	if locked, remaining := g.Check("user1"); locked || remaining != defaultMaxAttempts {
		t.Errorf("expected clean state after success, got locked=%v remaining=%d", locked, remaining)
	}
}

func TestRedisLoginGuard_PingErrorOnUnreachableRedis(t *testing.T) {
	g := NewRedisLoginGuard(newUnreachableRedisClient(), "test:")
	defer g.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := g.Ping(ctx); err == nil {
		t.Error("Ping should return error when redis is unreachable")
	}
}

func TestRedisLoginGuard_StopIsIdempotent(t *testing.T) {
	g := NewRedisLoginGuard(newUnreachableRedisClient(), "test:")
	g.Stop()
	g.Stop() // 二次调用不应 panic
}

// 编译期断言：RedisLoginGuard 实现 LoginLimiter 接口。
func TestRedisLoginGuard_ImplementsLoginLimiter(t *testing.T) {
	var _ LoginLimiter = (*RedisLoginGuard)(nil)
}

// ─── 真实 Redis 测试（REDIS_TEST_ADDR 门控，与 cache/redis_test.go 惯例一致）──

func newLiveRedisLoginGuard(t *testing.T) *RedisLoginGuard {
	t.Helper()
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR not set; skipping live Redis test")
	}
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_TEST_PASSWORD"),
	})
	g := NewRedisLoginGuard(client, "contentx_test:login:")
	t.Cleanup(func() {
		ctx := context.Background()
		keys, _ := client.Keys(ctx, "contentx_test:login:*").Result()
		if len(keys) > 0 {
			_ = client.Del(ctx, keys...).Err()
		}
		g.Stop()
		_ = client.Close()
	})
	return g
}

func TestRedisLoginGuard_Live_LockAfterMaxAttempts(t *testing.T) {
	g := newLiveRedisLoginGuard(t)

	var locked bool
	for i := 0; i < defaultMaxAttempts; i++ {
		locked, _ = g.RecordFailed("live-user")
	}
	if !locked {
		t.Fatal("expected lock after max failed attempts")
	}
	if lockedNow, remaining := g.Check("live-user"); !lockedNow || remaining != 0 {
		t.Errorf("expected locked with 0 remaining, got locked=%v remaining=%d", lockedNow, remaining)
	}
}

func TestRedisLoginGuard_Live_SharedAcrossInstances(t *testing.T) {
	// 模拟两个实例共用同一 Redis：实例 A 触发锁定，实例 B 立即可见。
	g1 := newLiveRedisLoginGuard(t)
	g2 := newLiveRedisLoginGuard(t)

	for i := 0; i < defaultMaxAttempts; i++ {
		g1.RecordFailed("shared-user")
	}
	if locked, _ := g2.Check("shared-user"); !locked {
		t.Error("SEC-6: lockout triggered on instance A must be visible on instance B")
	}
}

func TestRedisLoginGuard_Live_SuccessResets(t *testing.T) {
	g := newLiveRedisLoginGuard(t)

	g.RecordFailed("reset-user")
	g.RecordFailed("reset-user")
	g.RecordSuccess("reset-user")

	if locked, remaining := g.Check("reset-user"); locked || remaining != defaultMaxAttempts {
		t.Errorf("expected clean state after success, got locked=%v remaining=%d", locked, remaining)
	}
}
