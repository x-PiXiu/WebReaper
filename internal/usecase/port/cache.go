package port

import (
	"context"
	"time"
)

// CacheStore 读穿透缓存端口（R2/R3：热路径聚合不再每次打满数据库）。
//
// 用例层只声明"给我 key 对应的 JSON、没有则回填"的抽象；三防（雪崩/穿透/击穿）
// 全部住在适配器实现里（见 adapter/cache/redis_cache.go）——用例不感知 Redis。
//
// 语义约定：
//   - Get 命中空值标记（防穿透）返回 ("", true, nil)——调用方按"有缓存但值为空"处理
//   - Set 的 ttl 为基准 TTL，适配器负责加随机抖动（防雪崩：同批 key 不同时过期）
//   - 实现失败不应阻断主流程（调用方按 miss 处理并直查数据源）——由适配器保证
type CacheStore interface {
	// Get 取缓存值。found=false 表示未命中（含实现故障降级）。
	Get(ctx context.Context, key string) (value string, found bool, err error)
	// Set 写缓存（value 为空字符串时写入空值标记——防穿透短缓存）。
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	// Del 失效（写操作后主动清缓存）。
	Del(ctx context.Context, keys ...string) error
	// GetOrCompute 取或算（用例层的主要消费面）：命中直接返回；
	// 未命中执行 fetch 回源并回填。适配器保证：同 key 并发未命中只放行一个 fetch
	//（防击穿 singleflight）、fetch 返回空串回填空值标记（防穿透）、TTL 抖动（防雪崩）。
	GetOrCompute(ctx context.Context, key string, ttl time.Duration, fetch func(ctx context.Context) (string, error)) (string, error)
}
