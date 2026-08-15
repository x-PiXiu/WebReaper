// Package metrics 提供 port.MetricsCollector 实现。
//
// 两个实现：
//   - NoopMetrics：不采集（默认——未配 Redis 时零开销）
//   - RedisMetrics：Redis INCR 计数器（多实例共享，/debug/metrics 端点输出）
package metrics

import (
	"context"

	"github.com/redis/go-redis/v9"

	"webreaper/internal/usecase/port"
)

// NoopMetrics 不采集（nil 安全兜底——业务不依赖可观测性）。
type NoopMetrics struct{}

func NewNoopMetrics() *NoopMetrics { return &NoopMetrics{} }

func (m *NoopMetrics) Incr(_ context.Context, _ string) error              { return nil }
func (m *NoopMetrics) Get(_ context.Context, _ string) (int64, error)     { return 0, nil }
func (m *NoopMetrics) All(_ context.Context, _ string) (map[string]int64, error) {
	return map[string]int64{}, nil
}

var _ port.MetricsCollector = (*NoopMetrics)(nil)

// RedisMetrics 基于 Redis INCR 的计数器实现（key 前缀 "metric:" 隔离业务 key）。
type RedisMetrics struct {
	client *redis.Client
}

func NewRedisMetrics(client *redis.Client) *RedisMetrics {
	return &RedisMetrics{client: client}
}

func (m *RedisMetrics) Incr(ctx context.Context, key string) error {
	return m.client.Incr(ctx, "metric:"+key).Err()
}

func (m *RedisMetrics) Get(ctx context.Context, key string) (int64, error) {
	v, err := m.client.Get(ctx, "metric:"+key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return v, err
}

func (m *RedisMetrics) All(ctx context.Context, prefix string) (map[string]int64, error) {
	keys, err := m.client.Keys(ctx, "metric:"+prefix+"*").Result()
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(keys))
	for _, k := range keys {
		v, err := m.client.Get(ctx, k).Int64()
		if err != nil {
			continue
		}
		out[k[len("metric:"):]] = v // 去前缀输出
	}
	return out, nil
}

var _ port.MetricsCollector = (*RedisMetrics)(nil)
