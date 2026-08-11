package port

import (
	"context"
	"time"

	"webreaper/internal/domain/entity"
)

// ---- LLM 用量计量（经济系统预留接口）----
//
// 设计动机：
//   - 用量是横切关注点——所有 LLM 调用（对话/监测/内容生成/编排）都经过
//     AIGenerator，在适配器层统一计量，业务零感知。
//   - 目前 token 统计只进日志；本接口让用量可落库，
//     后续套餐/额度/账单/报表都基于这份数据，无需返工。
//   - 可选注入（nil 跳过）：未装配 recorder 时行为完全不变（渐进增强）。

// UsageRecorder LLM 用量记录器（适配器实现：GORM 落库 / 日志 / 上报）。
type UsageRecorder interface {
	// RecordUsage 记录一次 LLM 调用用量（适配器负责持久化；失败仅记日志不影响主流程）。
	RecordUsage(ctx context.Context, rec entity.UsageRecord) error
}

// SceneUsage 按场景聚合的用量统计（X-01 成本分析）。
type SceneUsage struct {
	Scene       string
	Calls       int     // LLM 调用次数（或业务动作计数，如 nearby 的 TotalTokens=0 记录）
	TotalTokens int64   // token 总量（非 LLM 场景为 0）
}

// UsageStatsQueryer 用量统计查询（成本分析/运营报表——商业闭环成本侧）。
// 由 UsageRecorder 实现者一并实现（同数据源 usages 表）。
type UsageStatsQueryer interface {
	// SumBySceneSince 统计指定时间点以来的用量（按场景分组，token 降序）。
	SumBySceneSince(ctx context.Context, since time.Time) ([]SceneUsage, error)
}

// ---- 上下文传递（租户/场景）：调用方声明，记录方消费 ----
// 用 context 而非改接口签名——ChatStream 等基础接口零破坏，
// 后台任务（定时监测）ctx 无租户则记空（平台消耗）。

type usageCtxKey struct{}

// WithUsageContext 在 ctx 注入用量上下文（租户 + 场景）。
// 调用方（usecase/handler）在调 AI 前调用，记录方（adapter）从中取值。
func WithUsageContext(ctx context.Context, tenantID, scene string) context.Context {
	return context.WithValue(ctx, usageCtxKey{}, usageCtx{tenantID: tenantID, scene: scene})
}

type usageCtx struct {
	tenantID string
	scene    string
}

// UsageTenantFrom 从 ctx 取租户（未注入返回空）。
func UsageTenantFrom(ctx context.Context) string {
	if v, ok := ctx.Value(usageCtxKey{}).(usageCtx); ok {
		return v.tenantID
	}
	return ""
}

// UsageSceneFrom 从 ctx 取场景（未注入返回空）。
func UsageSceneFrom(ctx context.Context) string {
	if v, ok := ctx.Value(usageCtxKey{}).(usageCtx); ok {
		return v.scene
	}
	return ""
}
