// Package lock 提供 port.TaskLock 的实现（分布式锁）。
//
// 两个实现：
//   - NoopLock：单机直跑（无互斥）——未配置 Redis 时装配
//   - RedisLock：多实例部署用（SETNX + 持有者 token + Lua 校验 DEL）
//
// 业务零改动：main 装配层按 Redis 是否可用切换实现（不可用自动降级 NoopLock 并记日志）。
package lock

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"webreaper/internal/adapter/cache"
	"webreaper/internal/usecase/port"
)

// NoopLock 无操作锁（单机部署：所有任务直接执行）。
type NoopLock struct{}

func NewNoopLock() *NoopLock { return &NoopLock{} }

func (l *NoopLock) TryLock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}

func (l *NoopLock) Unlock(_ context.Context, _ string) error { return nil }

var _ port.TaskLock = (*NoopLock)(nil)

// RedisLock 基于 Redis SETNX 的分布式锁（防多实例重复执行定时任务）。
//
// R1 加固（此前无持有者校验，Unlock 直接 DEL——A 的锁超时被 B 接管后，
// A 收尾时的 DEL 会误删 B 的锁，放第三个实例进来）：
//   - TryLock：SET key {随机token} NX EX ttl——token 是本实例的持有凭证
//   - Unlock：Lua 原子"值==我的 token 才 DEL"——只释放自己持有的锁
type RedisLock struct {
	client *redis.Client
	mu     sync.Mutex
	tokens map[string]string // lockKey → 本实例持有的 token
}

// releaseScript 仅当锁值等于调用者 token 时删除（原子 compare-and-del）。
var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
else
	return 0
end
`)

// NewRedisLock 创建（client 由 main 统一创建——锁/缓存共用连接池）。
func NewRedisLock(client *redis.Client) *RedisLock {
	return &RedisLock{client: client, tokens: map[string]string{}}
}

func (l *RedisLock) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	token := cache.RandomToken()
	ok, err := l.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return false, err
	}
	if ok {
		l.mu.Lock()
		l.tokens[key] = token
		l.mu.Unlock()
	}
	return ok, nil
}

func (l *RedisLock) Unlock(ctx context.Context, key string) error {
	l.mu.Lock()
	token, ok := l.tokens[key]
	delete(l.tokens, key)
	l.mu.Unlock()
	if !ok {
		return nil // 不是本实例持有的锁（已超时被接管）——不动它
	}
	return releaseScript.Run(ctx, l.client, []string{key}, token).Err()
}

var _ port.TaskLock = (*RedisLock)(nil)
