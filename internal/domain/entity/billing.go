package entity

import "time"

// ---- 经济系统域（订阅 / 计费 / 配额）----
//
// 设计动机（整洁架构）：
//   - 纯 struct + 领域规则，零框架依赖。计费是 SaaS 的命脉，规则必须可单测。
//   - Plan（套餐）配置化：入库可管理，运营改价/调配额无需发版（对应 architect-handbook
//     "推迟决策"——把会变的定价策略放数据，不放代码）。
//   - Subscription（订阅）绑定租户与套餐，按月计费周期；Order（订单）记录每次支付流水。
//   - 配额按"场景"维度（monitor/content-gen/video/chat…），与 UsageRecord.Scene 对齐——
//     计量扣减与计费配额用同一套键，天然闭环。
//
// 价格用分（PriceCents int）避免浮点误差——财务系统的铁律。

// Plan 套餐定义（可管理、可热更新）。
type Plan struct {
	ID         string         `json:"id"`          // plan-free / plan-pro / plan-team
	Name       string         `json:"name"`        // 显示名（免费版 / 专业版 / 团队版）
	Level      string         `json:"level"`       // 层级标识：free / pro / team（用于功能门禁比较）
	PriceCents int            `json:"price_cents"` // 月费（分）；0=免费
	Quotas     map[string]int `json:"quotas"`      // 场景→当月配额（monitor:500, content-gen:50）；-1=无限；缺省=0（不允许）
	Features   []string       `json:"features"`    // 功能白名单（auto-monitor / video / multi-account…）
	Status     string         `json:"status"`      // active / archived（下架后老订阅保留，新购不可选）
	SortOrder  int            `json:"sort_order"`  // 展示排序（升序）
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// 套餐状态常量。
const (
	PlanStatusActive   = "active"   // 在售
	PlanStatusArchived = "archived" // 下架（保留历史订阅，不可新购）
)

// QuotaFor 取某场景配额；未配置返回 0（不允许使用）。
func (p Plan) QuotaFor(scene string) int {
	if q, ok := p.Quotas[scene]; ok {
		return q
	}
	return 0
}

// HasFeature 判断套餐是否含某功能（功能门禁用）。
func (p Plan) HasFeature(feature string) bool {
	for _, f := range p.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// Subscription 租户订阅（一个租户同一时刻只有一个有效订阅）。
type Subscription struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	PlanID      string    `json:"plan_id"`
	Status      string    `json:"status"`        // active / expired / cancelled
	PeriodStart time.Time `json:"period_start"`  // 当前计费周期开始（按月）
	PeriodEnd   time.Time `json:"period_end"`    // 当前计费周期结束（到期续费或降级）
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// 订阅状态常量。
const (
	SubscriptionStatusActive    = "active"    // 有效（在计费周期内）
	SubscriptionStatusExpired   = "expired"   // 已到期（未续费，降级到 free）
	SubscriptionStatusCancelled = "cancelled" // 已取消（用户主动退订）
)

// IsActive 判断订阅是否有效（状态 active 且在计费周期内）。
func (s Subscription) IsActive(now time.Time) bool {
	return s.Status == SubscriptionStatusActive && now.Before(s.PeriodEnd) && !now.Before(s.PeriodStart)
}

// Order 订单（每次支付一条流水）。
type Order struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	PlanID         string    `json:"plan_id"`
	AmountCents    int       `json:"amount_cents"`    // 实付金额（分）
	Status         string    `json:"status"`          // pending / paid / refunded / failed
	PaymentGateway string    `json:"payment_gateway"` // mock / stripe / alipay
	PaymentID      string    `json:"payment_id"`      // 第三方支付流水号
	CreatedAt      time.Time `json:"created_at"`
	PaidAt         time.Time `json:"paid_at"`
}

// 订单状态常量。
const (
	OrderStatusPending  = "pending"  // 待支付（已创建订单，等用户付款）
	OrderStatusPaid     = "paid"     // 已支付（开通/续费订阅）
	OrderStatusRefunded = "refunded" // 已退款
	OrderStatusFailed   = "failed"   // 支付失败
)
