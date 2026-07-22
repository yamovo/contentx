package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// DistributedLock 分布式锁接口，用于多实例部署时协调后台 worker。
// 获取成功返回 true + release 函数；获取失败返回 false。
type DistributedLock interface {
	// Acquire 尝试获取锁。ttl 为锁的存活时间（防止持有者崩溃后死锁）。
	// 返回 release 函数（调用即释放锁），或 nil 表示未获取到锁。
	Acquire(ctx context.Context, key string, ttl time.Duration) (release func() error, ok bool, err error)
}

// ErrLockNotAcquired 表示锁已被其他实例持有。
var ErrLockNotAcquired = errors.New("lock not acquired")

// --- Redis 实现 ---

// RedisLock 基于 Redis SET NX EX 的分布式锁。
// 释放时用 Lua 脚本校验 owner token，防止误删他人持有的锁。
type RedisLock struct {
	client *redis.Client
	prefix string
}

// NewRedisLock 创建 Redis 分布式锁。prefix 与缓存前缀保持一致。
func NewRedisLock(client *redis.Client, prefix string) *RedisLock {
	if prefix == "" {
		prefix = "contentx:"
	}
	if !endsWithColon(prefix) {
		prefix += ":"
	}
	return &RedisLock{client: client, prefix: prefix + "lock:"}
}

// 释放锁的 Lua 脚本：仅当 key 的 value 匹配 owner token 时才删除。
const unlockScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end
`

func (l *RedisLock) Acquire(ctx context.Context, key string, ttl time.Duration) (func() error, bool, error) {
	owner := randomToken()
	fullKey := l.prefix + key

	ok, err := l.client.SetNX(ctx, fullKey, owner, ttl).Result()
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	release := func() error {
		_, err := l.client.Eval(ctx, unlockScript, []string{fullKey}, owner).Result()
		return err
	}
	return release, true, nil
}

// --- 内存实现（单实例降级）---

// MemoryLock 进程内互斥锁，单实例部署时使用。
// 多实例部署时应该用 RedisLock；若 Redis 不可用，降级到此实现（仅保护同进程并发）。
type MemoryLock struct {
	mu    sync.Mutex
	locks map[string]string // key -> owner token
}

// NewMemoryLock 创建进程内锁。
func NewMemoryLock() *MemoryLock {
	return &MemoryLock{locks: make(map[string]string)}
}

func (l *MemoryLock) Acquire(ctx context.Context, key string, ttl time.Duration) (func() error, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.locks[key]; exists {
		return nil, false, nil
	}

	owner := randomToken()
	l.locks[key] = owner

	// TTL 过期自动清理（防崩溃后死锁）
	go func() {
		time.Sleep(ttl)
		l.mu.Lock()
		if l.locks[key] == owner {
			delete(l.locks, key)
		}
		l.mu.Unlock()
	}()

	release := func() error {
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.locks[key] == owner {
			delete(l.locks, key)
		}
		return nil
	}
	return release, true, nil
}

// --- 辅助函数 ---

// randomToken 生成 16 字节随机 token，用于标识锁持有者。
func randomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func endsWithColon(s string) bool {
	return len(s) > 0 && s[len(s)-1] == ':'
}
