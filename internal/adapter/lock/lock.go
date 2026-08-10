// Package lock 提供 port.TaskLock 的实现（分布式锁）。
//
// 两个实现：
//   - NoopLock：单机直跑（无互斥）——默认装配
//   - RedisLock：多实例部署用（SETNX + TTL + 校验 DEL）——分布式演进时装配
//
// 业务零改动：main 装配层换一个锁实现即可。
package lock

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"

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

// RedisLock 基于 Redis SETNX 的分布式锁（防多实例重复执行）。
//
// 语义：
//   - TryLock：SET key 1 NX EX ttl —— 成功=拿到锁；失败=其他实例持有
//   - Unlock：仅当值匹配才 DEL（防误删他人锁——校验 token 用锁值，
//     简单实现用时间戳；严谨场景应换 Lua 脚本）
type RedisLock struct {
	client *redis.Client
}

func NewRedisLock(addr, password string, db int) *RedisLock {
	return &RedisLock{
		client: redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: db}),
	}
}

func (l *RedisLock) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	// SET key 1 NX EX ttl：仅在 key 不存在时写入
	ok, err := l.client.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (l *RedisLock) Unlock(ctx context.Context, key string) error {
	return l.client.Del(ctx, key).Err()
}

var _ port.TaskLock = (*RedisLock)(nil)
