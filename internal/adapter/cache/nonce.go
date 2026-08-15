package cache

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// MemoryNonceStore 单机内存 nonce 判重（默认——未配置 Redis 时兜底，多实例失效）。
type MemoryNonceStore struct {
	mu     sync.Mutex
	seen   map[string]time.Time
	ttl    time.Duration
}

func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{seen: map[string]time.Time{}, ttl: 5 * time.Minute}
}

func (s *MemoryNonceStore) Seen(_ context.Context, nonce string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if t, ok := s.seen[nonce]; ok && now.Sub(t) < s.ttl {
		return false
	}
	s.seen[nonce] = now
	// 简单容量防护：超阈值全量清过期
	if len(s.seen) > 1000 {
		for k, v := range s.seen {
			if now.Sub(v) > s.ttl {
				delete(s.seen, k)
			}
		}
	}
	return true
}

// RedisNonceStore Redis 判重（SETNX+EX 原子——多实例安全；Redis 故障时放行
// 并由 HMAC 验签/终态幂等兜底，防重放三层中损失一层不致命）。
type RedisNonceStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisNonceStore(client *redis.Client) *RedisNonceStore {
	return &RedisNonceStore{client: client, ttl: 5 * time.Minute}
}

func (s *RedisNonceStore) Seen(ctx context.Context, nonce string) bool {
	ok, err := s.client.SetNX(ctx, "cb:nonce:"+nonce, 1, s.ttl).Result()
	if err != nil {
		return true // Redis 故障降级放行——验签+幂等仍兜底
	}
	return ok
}
