package port

import (
	"context"
	"time"
)

// ---- 配额存取（经济系统：计量 → 配额扣减）----
//
// 配额检查的两种实现策略：
//   - 计数派生型（DBUsageQuotaStore）：从 usages 表实时 COUNT 当前周期用量，
//     与 plan.quotas[scene] 对比。零额外状态，与计量天然一致——扣减即"多记一条 usage"。
//   - 计数器型（RedisQuotaStore）：独立计数器，TTL=计费周期。
//     性能更好但需额外组件，且要与计量保持同步。
//
// 当前采用计数派生型（DB 实现）——简单、强一致、无额外依赖。
// 接口抽象保留切换到 Redis 的可能（依赖倒置）。

// UsageQueryer 用量查询（配额计算的数据源；与 UsageRecorder 分离——
// 写入是适配器层职责，查询是经济系统用例职责，分开避免单接口臃肿）。
type UsageQueryer interface {
	// CountSince 统计租户某场景自 since 以来的 LLM 调用次数（计费周期内用量）。
	// tenantID/scene 为空时返回 0（平台消耗不计租户配额）。
	CountSince(ctx context.Context, tenantID, scene string, since time.Time) (int, error)
}

// QuotaStore 配额存取（配额检查装饰器消费；注入到烧 token 的业务用例）。
//
// 一个接口两职责：
//   - Check：业务用例入口检查（超限返回 ErrQuotaExceeded）
//   - QuotaFor：前端用量展示（配额上限 + 已用量）
type QuotaStore interface {
	// Check 检查租户某场景是否还有配额。超限返回 pkg.ErrQuotaExceeded。
	// nil 安全：未注入配额检查时业务用例直接放行（向后兼容）。
	Check(ctx context.Context, tenantID, scene string) error
	// QuotaFor 取租户某场景的配额上限与当前已用量。
	// 返回 limit（-1=无限）、used（当前周期已用）、err。
	QuotaFor(ctx context.Context, tenantID, scene string) (limit, used int, err error)
}
