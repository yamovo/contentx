package auth

import "time"

// ResilientTokenStore 组合 Redis 与内存黑名单，消除 RedisTokenStore 的
// fail-open 风险（SEC-2）：
//
//   - Revoke 双写：同时写入 Redis（多实例共享、重启保留）与本地内存
//     （Redis 故障期间的兜底）。
//   - IsRevoked 先查本地内存再查 Redis：即使 Redis 完全不可达，
//     本实例吊销过的 token 也会被立即拒绝，不再放行。
//
// 边界：Redis 故障窗口内，*其他实例*吊销的 token 在本实例仍不可见——
// 这是 fallback 方案的固有限制；故障恢复后 Redis 数据仍在，自动收敛。
type ResilientTokenStore struct {
	redis *RedisTokenStore
	local *Blacklist
}

// NewResilientTokenStore 创建组合 token store。redis 不可为 nil；
// local 为 nil 时自动创建一个新的内存黑名单。
func NewResilientTokenStore(redis *RedisTokenStore, local *Blacklist) *ResilientTokenStore {
	if local == nil {
		local = NewBlacklist()
	}
	return &ResilientTokenStore{redis: redis, local: local}
}

// Revoke 双写 Redis 与本地内存黑名单。
func (s *ResilientTokenStore) Revoke(tokenStr string, expiresAt time.Time) {
	if s == nil {
		return
	}
	s.local.Revoke(tokenStr, expiresAt)
	s.redis.Revoke(tokenStr, expiresAt)
}

// IsRevoked 先查本地内存（O(1)、覆盖 Redis 故障窗口），再查 Redis
// （覆盖多实例与重启场景）。
func (s *ResilientTokenStore) IsRevoked(tokenStr string) bool {
	if s == nil {
		return false
	}
	if s.local.IsRevoked(tokenStr) {
		return true
	}
	return s.redis.IsRevoked(tokenStr)
}

// Consume first claims the token locally, then atomically claims it in Redis.
// A Redis failure is returned (fail closed) because otherwise two instances
// could both accept the same refresh token during an outage.
func (s *ResilientTokenStore) Consume(tokenStr string, expiresAt time.Time) (bool, error) {
	if s == nil || s.local == nil || s.redis == nil {
		return false, ErrTokenStoreUnavailable
	}
	localOK, err := s.local.Consume(tokenStr, expiresAt)
	if err != nil || !localOK {
		return localOK, err
	}
	sharedOK, err := s.redis.Consume(tokenStr, expiresAt)
	if err != nil {
		return false, err
	}
	return sharedOK, nil
}

// Cleanup 清理本地内存黑名单中的过期条目（Redis 侧由 TTL 自动回收）。
func (s *ResilientTokenStore) Cleanup() {
	if s == nil {
		return
	}
	s.local.Cleanup()
}

// Compile-time check: *ResilientTokenStore 实现 TokenStore。
var _ TokenStore = (*ResilientTokenStore)(nil)
