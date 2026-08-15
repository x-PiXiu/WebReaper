package port

import "context"

// MetricsCollector 运营指标采集端口（R3 可观测）。
//
// 用例层只声明"计一次"的抽象；实现在适配器（Redis INCR / 内存 / noop）。
// 语义：计数器只增不减，key 由调用方定义——收在同一包避免散落。
//
// 设计：可选注入——nil 时不采集（零开销），业务不依赖可观测性。
type MetricsCollector interface {
	// Incr 计数器 +1（key 如 "llm:calls" / "llm:errors" / "quota:rejected"）。
	Incr(ctx context.Context, key string) error
	// Get 取计数器当前值（debug 端点用）。
	Get(ctx context.Context, key string) (int64, error)
	// All 取全部以 prefix 开头的计数器（debug 端点聚合输出）。
	All(ctx context.Context, prefix string) (map[string]int64, error)
}

// ---- 指标 key 常量（唯一事实源——埋点与消费共用）----

const (
	MetricLLMCalls       = "llm:calls"        // LLM 调用总次数
	MetricLLMErrors      = "llm:errors"       // LLM 调用失败次数
	MetricLLMSlow        = "llm:slow"         // LLM 慢调用（>30s，可能超时）
	MetricQuotaRejected  = "quota:rejected"   // 配额拒绝（402）次数
	MetricLockContention = "lock:contention"  // 分布式锁竞争（TryLock 失败）次数
	MetricCacheHits      = "cache:hits"       // 缓存命中
	MetricCacheMisses    = "cache:misses"     // 缓存未命中
	MetricMonitorRuns    = "monitor:runs"     // 监测执行次数
	MetricContentGens    = "content:gens"     // 内容生成次数
	MetricGenSubmits     = "generation:submit" // 多媒体生成提交次数
	MetricGenRetries     = "generation:retry"  // 自动重试次数
	MetricGenStuck       = "generation:stuck"  // 卡死超时次数
)
