package auth

import (
	"testing"
	"time"
)

// ─── SEC-2：ResilientTokenStore（Redis + 内存双写）─────────────────────────
//
// 核心验收：Redis 完全不可达时，本实例吊销过的 token 仍必须被拒绝，
// 不得像裸 RedisTokenStore 那样 fail-open。

func newResilientStoreWithDeadRedis() *ResilientTokenStore {
	redis := NewRedisTokenStore(newUnreachableRedisClient(), "test:")
	return NewResilientTokenStore(redis, nil)
}

func TestResilientTokenStore_RevokedTokenRejectedWhenRedisDown(t *testing.T) {
	s := newResilientStoreWithDeadRedis()

	// 吊销时 Redis 写入静默失败，但内存层必须记住。
	s.Revoke("revoked-token", time.Now().Add(time.Hour))

	if !s.IsRevoked("revoked-token") {
		t.Error("SEC-2: revoked token must be rejected even when redis is unreachable (no fail-open)")
	}
}

func TestResilientTokenStore_UnrevokedTokenStillAccepted(t *testing.T) {
	s := newResilientStoreWithDeadRedis()

	// 未吊销的 token 不应被误杀（内存层未命中 → 查 Redis 失败 → 放行）。
	if s.IsRevoked("never-revoked") {
		t.Error("token that was never revoked should not be reported as revoked")
	}
}

func TestResilientTokenStore_ExpiredRevocationReleased(t *testing.T) {
	s := newResilientStoreWithDeadRedis()

	// 吊销条目过期后应自然失效（与 JWT 自身过期一致，不永久占用内存）。
	// 窗口需足够宽：Revoke 内部对不可达 Redis 的写入失败会耗时数百毫秒。
	s.Revoke("short-lived", time.Now().Add(800*time.Millisecond))
	if !s.IsRevoked("short-lived") {
		t.Fatal("token should be revoked before expiry")
	}
	time.Sleep(time.Second)
	if s.IsRevoked("short-lived") {
		t.Error("expired revocation entry should no longer block the token")
	}
}

func TestResilientTokenStore_NilSafe(t *testing.T) {
	var s *ResilientTokenStore

	// nil store 不应 panic。
	s.Revoke("any-token", time.Now().Add(time.Hour))
	if s.IsRevoked("any-token") {
		t.Error("nil store should report not revoked")
	}
	s.Cleanup()
}

func TestNewResilientTokenStore_NilLocalCreatesBlacklist(t *testing.T) {
	redis := NewRedisTokenStore(newUnreachableRedisClient(), "test:")
	s := NewResilientTokenStore(redis, nil)
	if s.local == nil {
		t.Fatal("nil local should be replaced with a fresh Blacklist")
	}
}

func TestResilientTokenStore_CleanupRemovesExpiredLocalEntries(t *testing.T) {
	local := NewBlacklist()
	redis := NewRedisTokenStore(newUnreachableRedisClient(), "test:")
	s := NewResilientTokenStore(redis, local)

	s.Revoke("stale", time.Now().Add(-time.Minute)) // 已过期条目
	s.Revoke("fresh", time.Now().Add(time.Hour))
	s.Cleanup()

	local.mu.Lock()
	_, staleExists := local.tokens["stale"]
	_, freshExists := local.tokens["fresh"]
	local.mu.Unlock()

	if staleExists {
		t.Error("Cleanup should drop expired entries from local blacklist")
	}
	if !freshExists {
		t.Error("Cleanup must keep unexpired entries")
	}
}

// 编译期断言：ResilientTokenStore 实现 TokenStore 接口。
func TestResilientTokenStore_ImplementsTokenStore(t *testing.T) {
	var _ TokenStore = (*ResilientTokenStore)(nil)
}
