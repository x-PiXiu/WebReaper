// Package quota 实现配额检查装饰器（经济系统的横切关注点）。
//
// 整洁架构定位：
//   - 配额检查是"横切关注点"（cross-cutting concern），不该污染业务用例的编排逻辑。
//   - 本包提供 Gate.Check——业务用例在烧 token 的方法开头调用一行即可。
//   - 与现有 SetRuleScorer/SetRAGRetriever 注入风格一致（可选注入，nil=不检查）。
//
// 计数派生型配额（DBQuotaStore）：
//   - 配额 = plan.quotas[scene]，用量 = usages 表 COUNT（当前计费周期）
//   - 不维护独立计数器状态，与计量天然一致（扣减即"多记一条 usage"）
//   - 强一致、零额外依赖；高频场景可换 RedisQuotaStore（接口已抽象）
package quota

import (
	"context"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// Gate 配额检查门（注入到烧 token 的 usecase）。
//
// 依赖：
//   - planRepo：取套餐配额上限
//   - subRepo：取租户订阅（确定适用哪个套餐 + 计费周期）
//   - usageQueryer：取当前周期已用量
//
// 无订阅的租户：降级到 free 套餐配额（plan-free）。
type Gate struct {
	planRepo    port.PlanRepository
	subRepo     port.SubscriptionRepository
	usageQueryer port.UsageQueryer
	freePlanID  string // 无订阅时降级到的套餐（默认 plan-free）
}

// NewGate 创建配额检查门。
func NewGate(plan port.PlanRepository, sub port.SubscriptionRepository, usage port.UsageQueryer) *Gate {
	return &Gate{
		planRepo: plan, subRepo: sub, usageQueryer: usage,
		freePlanID: "plan-free",
	}
}

// SetFreePlanID 覆盖降级套餐 ID（测试或自定义 free 套餐时用）。
func (g *Gate) SetFreePlanID(id string) {
	if id != "" {
		g.freePlanID = id
	}
}

// Check 检查租户某场景是否还有配额。超限返回 ErrQuotaExceeded。
//
// 行为：
//   - nil Gate：直接通过（未启用计费，向后兼容）
//   - 租户无订阅：用 free 套餐配额，计费周期=当月 1 日起
//   - limit == -1：无限配额（team 套餐），直接通过
//   - limit == 0：该场景不允许使用（套餐未开通此功能）
//   - used >= limit：超限
func (g *Gate) Check(ctx context.Context, tenantID, scene string) error {
	if g == nil {
		return nil // 未启用配额检查
	}
	if tenantID == "" {
		return nil // 平台后台操作（admin 旁路）不计配额
	}

	plan, periodStart, err := g.resolvePlanAndPeriod(ctx, tenantID)
	if err != nil {
		// 取不到套餐信息：放行（计费降级，不阻断主流程）
		return nil
	}

	limit := plan.QuotaFor(scene)
	if limit == -1 {
		return nil // 无限配额
	}
	if limit == 0 {
		return pkg.ErrQuotaExceeded // 套餐未开通此场景
	}

	used, err := g.usageQueryer.CountSince(ctx, tenantID, scene, periodStart)
	if err != nil {
		return nil // 用量查询失败：放行（计费降级）
	}
	if used >= limit {
		return pkg.ErrQuotaExceeded
	}
	return nil
}

// QuotaFor 实现 port.QuotaStore——返回配额上限与当前用量（前端用量进度条用）。
func (g *Gate) QuotaFor(ctx context.Context, tenantID, scene string) (limit, used int, err error) {
	if g == nil || tenantID == "" {
		return -1, 0, nil
	}
	plan, periodStart, err := g.resolvePlanAndPeriod(ctx, tenantID)
	if err != nil {
		return -1, 0, err
	}
	limit = plan.QuotaFor(scene)
	if limit == -1 {
		return -1, 0, nil // 无限
	}
	used, qErr := g.usageQueryer.CountSince(ctx, tenantID, scene, periodStart)
	if qErr != nil {
		return limit, 0, nil
	}
	return limit, used, nil
}

// resolvePlanAndPeriod 取租户适用的套餐与计费周期起点。
func (g *Gate) resolvePlanAndPeriod(ctx context.Context, tenantID string) (entity.Plan, time.Time, error) {
	// 订阅存在且有效 → 用订阅的套餐 + 计费周期
	if sub, err := g.subRepo.FindByTenant(ctx, tenantID); err == nil {
		now := time.Now()
		if sub.IsActive(now) {
			if plan, pErr := g.planRepo.FindByID(ctx, sub.PlanID); pErr == nil {
				return plan, sub.PeriodStart, nil
			}
		}
	}
	// 无订阅或订阅失效 → 降级 free 套餐，周期=当月 1 日起
	plan, err := g.planRepo.FindByID(ctx, g.freePlanID)
	if err != nil {
		return entity.Plan{}, time.Time{}, err
	}
	return plan, startOfCurrentMonth(), nil
}

// startOfCurrentMonth 当月 1 日 0 点（免费版按自然月计费）。
func startOfCurrentMonth() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}
