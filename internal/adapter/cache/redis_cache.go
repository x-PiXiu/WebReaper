// Package cache 提供 port.CacheStore 的 Redis 实现（三防缓存）。
//
// 三防设计（用户明确要求：不能一直查数据库，但要避免雪崩等问题）：
//   - 防雪崩：TTL 随机抖动（基准 ttl + rand[0, 25%]）——同批写入的 key 不会同一秒集体过期，
//     过期后打向数据库的回源请求在时间上摊开。
//   - 防穿透：空结果写入短 TTL 标记（NULL_VALUE sentinel，30s+抖动）——恶意/高频查询
//     不存在的 key 时，数据库只会被打第一次。
//   - 防击穿：singleflight 合并并发回源——热 key 过期瞬间 N 个并发请求只放一个去查库，
//     其余等结果共享（golang.org/x/sync/singleflight，已在依赖树）。
//
// 故障降级：Redis 不可用时 Get 返回 miss、Set 静默失败（记日志由调用方处理）——
// 缓存层故障不阻断业务（缓存只加速，不是事实源）。
package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"

	"webreaper/internal/usecase/port"
)

// NULL_VALUE 空值标记（防穿透：区分"缓存了空结果"与"没缓存"）。
const NULL_VALUE = "\x00null"

// nullTTL 空值缓存时长（短——空结果通常意味着数据还没产生，不用长缓存）。
const nullTTL = 30 * time.Second

// RedisCache port.CacheStore 的 Redis 实现。
type RedisCache struct {
	client *redis.Client
	group  singleflight.Group
	// R3 可观测：命中率计数（进程内——日志聚合时算总体命中率；同时写入 Redis 供 /debug 端点）
	metrics port.MetricsCollector
}

var _ port.CacheStore = (*RedisCache)(nil)

// NewRedisCache 创建（client 由 main 统一创建共享——锁/缓存/nonce 同一连接池）。
func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{client: client}
}

// SetMetrics 注入指标采集器（可选；R3——缓存命中率写入 Redis INCR）。
func (c *RedisCache) SetMetrics(m port.MetricsCollector) {
	c.metrics = m
}

func (c *RedisCache) trackHit() {
	if c.metrics != nil {
		_ = c.metrics.Incr(context.Background(), port.MetricCacheHits)
	}
}

func (c *RedisCache) trackMiss() {
	if c.metrics != nil {
		_ = c.metrics.Incr(context.Background(), port.MetricCacheMisses)
	}
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, bool, error) {
	v, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		c.trackMiss()
		return "", false, nil
	}
	if err != nil {
		c.trackMiss()
		return "", false, nil
	}
	c.trackHit()
	if v == NULL_VALUE {
		return "", true, nil // 有缓存但值为空（防穿透标记）
	}
	return v, true, nil
}

func (c *RedisCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	v := value
	if value == "" {
		v = NULL_VALUE
		ttl = nullTTL
	}
	return c.client.Set(ctx, key, v, JitterTTL(ttl)).Err()
}

func (c *RedisCache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

// GetOrCompute 取或算（防击穿核心）：同 key 并发未命中经 singleflight 合并，
// 只有一个请求执行 fetch 回源，其余共享其结果——热 key 过期瞬间的并发回源
// 不会全部打到数据库。
func (c *RedisCache) GetOrCompute(ctx context.Context, key string, ttl time.Duration, fetch func(ctx context.Context) (string, error)) (string, error) {
	if v, found, _ := c.Get(ctx, key); found {
		return v, nil // 命中（含空值标记——防穿透）
	}
	v, err, _ := c.group.Do(key, func() (any, error) {
		// 双重检查：排队期间可能已被第一个请求回填
		if v, found, _ := c.Get(ctx, key); found {
			return v, nil
		}
		val, fErr := fetch(ctx)
		if fErr != nil {
			return "", fErr // 回源失败不写缓存（下次重试）——避免把错误缓存住
		}
		_ = c.Set(ctx, key, val, ttl) // 回填失败仅降级（下次再回源）
		return val, nil
	})
	if err != nil {
		return "", err
	}
	s, _ := v.(string)
	return s, nil
}

// JitterTTL 基准 TTL 加随机抖动 [0, 25%)（防雪崩——纯函数可单测）。
func JitterTTL(base time.Duration) time.Duration {
	if base <= 0 {
		return base
	}
	j, err := rand.Int(rand.Reader, big.NewInt(int64(base)/4+1))
	if err != nil {
		return base // 随机源异常退化为基准（不影响正确性）
	}
	return base + time.Duration(j.Int64())
}

// RandomToken 生成随机 token（RedisLock 持有者标识——见 adapter/lock）。
func RandomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
