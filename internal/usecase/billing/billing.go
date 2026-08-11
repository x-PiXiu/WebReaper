// Package billing 实现经济系统用例：套餐管理 / 订阅 / 订单 / 配额查询。
//
// 整洁架构：
//   - 依赖 port 接口（PlanRepo/SubscriptionRepo/OrderRepo/QuotaStore/PaymentGateway），
//     不感知具体存储（GORM/Redis）与支付实现（mock/stripe）。
//   - 计费规则（订阅有效性、配额扣减、订单状态机）在实体层 + 本包，
//     handler 只做 DTO 转换。
//   - 支付网关可选注入：nil=线下模式（admin 手动开通），非 nil=拉起在线支付。
package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// BillingUseCase 经济系统用例。
type BillingUseCase struct {
	planRepo   port.PlanRepository
	subRepo    port.SubscriptionRepository
	orderRepo  port.OrderRepository
	payment    port.PaymentGateway   // 可选：nil=线下模式（admin 手动开通）
	settingRepo port.SystemSettingRepository // 支付网关运行时配置（admin 后台设置）
	usageStats port.UsageStatsQueryer // 用量统计（可选；X-01 成本分析用）
	perMTokenCents int                // 每百万 tokens 参考成本（分；0=不估算金额，只报 token）
}

func NewBillingUseCase(plan port.PlanRepository, sub port.SubscriptionRepository, order port.OrderRepository) *BillingUseCase {
	return &BillingUseCase{planRepo: plan, subRepo: sub, orderRepo: order}
}

// SetPaymentGateway 注入支付网关（可选；未注入=线下模式，CreateOrder 返回待支付订单无 URL）。
func (uc *BillingUseCase) SetPaymentGateway(g port.PaymentGateway) {
	if g != nil {
		uc.payment = g
	}
}

// SetUsageStats 注入用量统计查询器（可选；X-01 成本分析——收入侧 RevenueReport 已存在，
// 补成本侧后"收入 vs 成本"双报表，商业闭环收口）。
func (uc *BillingUseCase) SetUsageStats(q port.UsageStatsQueryer) {
	if q != nil {
		uc.usageStats = q
	}
}

// SetReferencePricePerMToken 设置每百万 tokens 参考成本（分；0=只报 token 不估算金额）。
// 配置依赖：不同 LLM 模型单价不同（如 MiniMax M2.5 约 ¥1/百万 tokens），
// 运营按实际模型从 .env 配置（LLM_COST_PER_MToken）。
func (uc *BillingUseCase) SetReferencePricePerMToken(cents int) {
	uc.perMTokenCents = cents
}

// SetSettingRepo 注入系统设置仓储（可选；用于支付网关运行时配置管理）。
func (uc *BillingUseCase) SetSettingRepo(r port.SystemSettingRepository) {
	if r != nil {
		uc.settingRepo = r
	}
}

// ---- 支付网关配置管理（admin 运行时设置）----

// GetPaymentConfig 读取当前支付网关配置（admin 后台展示用）。
// 敏感字段 key 脱敏返回（只显示前 4 位）。
func (uc *BillingUseCase) GetPaymentConfig(ctx context.Context) (map[string]string, error) {
	if uc.settingRepo == nil {
		return map[string]string{"gateway": "mock"}, nil
	}
	s, err := uc.settingRepo.Get(ctx, entity.SettingKeyPaymentConfig)
	if err != nil {
		return map[string]string{"gateway": "mock"}, nil // 未配置 → mock
	}
	// 解析 JSON 配置
	var cfg map[string]string
	if err := json.Unmarshal([]byte(s.Value), &cfg); err != nil {
		return map[string]string{"gateway": "mock"}, nil
	}
	// 脱敏 key
	if k, ok := cfg["key"]; ok && len(k) > 4 {
		cfg["key"] = k[:4] + "****"
	}
	return cfg, nil
}

// SetPaymentConfig 保存支付网关配置（admin 后台设置）。
// gateway 字段决定用哪个通道（当前支持 mock / zpay）。
func (uc *BillingUseCase) SetPaymentConfig(ctx context.Context, cfg map[string]string) error {
	if uc.settingRepo == nil {
		return fmt.Errorf("系统设置仓储未注入")
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化支付配置失败: %w", err)
	}
	return uc.settingRepo.Save(ctx, entity.SystemSetting{
		Key:   entity.SettingKeyPaymentConfig,
		Value: string(data),
	})
}

// ---- 异步回调处理（支付网关 webhook）----

// HandleCallback 处理支付网关异步回调（统一入口）。
//
// 流程：
//  1. 验签（VerifyCallback）——防伪造回调
//  2. 查订单 ——校验订单存在 + 金额一致
//  3. 调 ConfirmPayment ——标记 paid + 开通订阅
//
// 返回 "success" 给支付平台（ZPAY 要求返回纯 success 字符串，否则重试）。
// 调用方（handler）应把返回值作为 HTTP 响应体。
func (uc *BillingUseCase) HandleCallback(ctx context.Context, params map[string]string) (string, error) {
	if uc.payment == nil {
		return "fail", fmt.Errorf("支付网关未注入")
	}

	// ① 验签 + 提取回调数据
	result, err := uc.payment.VerifyCallback(ctx, params)
	if err != nil {
		return "fail", fmt.Errorf("回调验签失败: %w", err)
	}

	// ② 查订单 + 校验金额
	order, err := uc.orderRepo.FindByID(ctx, result.OutTradeNo)
	if err != nil {
		return "fail", fmt.Errorf("订单不存在: %s", result.OutTradeNo)
	}
	if result.AmountCents != order.AmountCents {
		return "fail", fmt.Errorf("回调金额 %d 与订单金额 %d 不一致", result.AmountCents, order.AmountCents)
	}

	// ③ 确认支付（幂等——已 paid 直接返回）
	if result.Status == "paid" {
		if _, err := uc.ConfirmPayment(ctx, order.ID); err != nil {
			return "fail", fmt.Errorf("确认支付失败: %w", err)
		}
	}

	return "success", nil
}

// ---- 套餐管理（admin）----

// ListPlans 列出全部套餐（admin 含下架）。
func (uc *BillingUseCase) ListPlans(ctx context.Context) ([]entity.Plan, error) {
	return uc.planRepo.ListAll(ctx)
}

// ListActivePlans 列出在售套餐（商户端购买页用）。
func (uc *BillingUseCase) ListActivePlans(ctx context.Context) ([]entity.Plan, error) {
	return uc.planRepo.ListActive(ctx)
}

// SavePlan 创建或更新套餐（admin）。
func (uc *BillingUseCase) SavePlan(ctx context.Context, p entity.Plan) (entity.Plan, error) {
	if p.ID == "" || p.Name == "" || p.Level == "" {
		return entity.Plan{}, fmt.Errorf("%w: id/name/level 必填", pkg.ErrInvalidArgument)
	}
	if p.Status == "" {
		p.Status = entity.PlanStatusActive
	}
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now
	if err := uc.planRepo.Save(ctx, p); err != nil {
		return entity.Plan{}, err
	}
	return p, nil
}

// DeletePlan 下架套餐（物理删除；已有订阅不受影响——订阅快照了 PlanID）。
func (uc *BillingUseCase) DeletePlan(ctx context.Context, id string) error {
	return uc.planRepo.Delete(ctx, id)
}

// ---- 订阅查询 ----

// GetSubscription 取租户当前订阅（无订阅返回 ErrNotFound，调用方可降级到 free）。
func (uc *BillingUseCase) GetSubscription(ctx context.Context, tenantID string) (entity.Subscription, error) {
	return uc.subRepo.FindByTenant(ctx, tenantID)
}

// ListSubscriptions 全部订阅（admin 看全局）。
func (uc *BillingUseCase) ListSubscriptions(ctx context.Context) ([]entity.Subscription, error) {
	return uc.subRepo.ListAll(ctx)
}

// ---- 订单 ----

// ListOrdersByTenant 租户订单流水（商户端"我的订单"）。
func (uc *BillingUseCase) ListOrdersByTenant(ctx context.Context, tenantID string) ([]entity.Order, error) {
	return uc.orderRepo.ListByTenant(ctx, tenantID)
}

// ListAllOrders 全部订单（admin 收入报表用）。
func (uc *BillingUseCase) ListAllOrders(ctx context.Context) ([]entity.Order, error) {
	return uc.orderRepo.ListAll(ctx)
}

// ---- 订单创建 + 支付 + 订阅开通 ----

// CreateOrderInput 创建订单入参。
type CreateOrderInput struct {
	TenantID string
	PlanID   string
}

// CreateOrderResult 创建订单结果（含支付页 URL——前端跳转拉起支付）。
type CreateOrderResult struct {
	Order      entity.Order
	PaymentURL string // 支付网关返回的支付页 URL（线下模式为空）
}

// CreateOrder 商户端下单：查套餐价格 → 创建 pending 订单 → 拉起支付（可选）。
func (uc *BillingUseCase) CreateOrder(ctx context.Context, in CreateOrderInput) (CreateOrderResult, error) {
	if in.TenantID == "" || in.PlanID == "" {
		return CreateOrderResult{}, fmt.Errorf("%w: tenant_id/plan_id 必填", pkg.ErrInvalidArgument)
	}
	plan, err := uc.planRepo.FindByID(ctx, in.PlanID)
	if err != nil {
		return CreateOrderResult{}, fmt.Errorf("套餐不存在: %w", err)
	}
	if plan.Status != entity.PlanStatusActive {
		return CreateOrderResult{}, fmt.Errorf("%w: 套餐已下架", pkg.ErrInvalidArgument)
	}

	now := time.Now()
	order := entity.Order{
		ID:          fmt.Sprintf("order-%d", now.UnixNano()),
		TenantID:    in.TenantID,
		PlanID:      in.PlanID,
		AmountCents: plan.PriceCents,
		Status:      entity.OrderStatusPending,
		CreatedAt:   now,
	}
	if uc.payment != nil {
		order.PaymentGateway = uc.payment.Name()
	}
	if err := uc.orderRepo.Save(ctx, order); err != nil {
		return CreateOrderResult{}, err
	}

	// 拉起支付（网关可选：nil=线下模式，admin 手动 ConfirmPayment）
	result := CreateOrderResult{Order: order}
	if uc.payment != nil {
		payURL, payID, pErr := uc.payment.CreatePayment(ctx, order)
		if pErr != nil {
			return result, fmt.Errorf("拉起支付失败: %w", pErr)
		}
		_ = uc.orderRepo.UpdateStatus(ctx, order.ID, order.Status, payID, time.Time{})
		result.PaymentURL = payURL
	}
	return result, nil
}

// ConfirmPayment 确认支付完成（支付回调 / admin 手动确认）→ 开通订阅。
//
// 流程：订单 pending → paid → 创建/续期订阅（计费周期=当月起 30 天）。
// 幂等：订单已是 paid 直接返回（防重复回调）。
func (uc *BillingUseCase) ConfirmPayment(ctx context.Context, orderID string) (entity.Subscription, error) {
	order, err := uc.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return entity.Subscription{}, fmt.Errorf("订单不存在: %w", err)
	}
	if order.Status == entity.OrderStatusPaid {
		// 幂等：已支付直接返回当前订阅
		return uc.subRepo.FindByTenant(ctx, order.TenantID)
	}
	if order.Status != entity.OrderStatusPending {
		return entity.Subscription{}, fmt.Errorf("订单状态 %s 不可确认支付", order.Status)
	}

	// 支付网关二次验证：向网关查询订单真实状态（防"未付款确认"攻击）。
	// mock 模式下 QueryPayment 始终返回 paid；真实网关（zpay/stripe）会查实际支付状态。
	// 仅当有 PaymentID（已创建支付）且网关已注入时才验证。
	if uc.payment != nil && order.PaymentID != "" {
		status, qErr := uc.payment.QueryPayment(ctx, order.ID)
		if qErr != nil {
			return entity.Subscription{}, fmt.Errorf("支付网关查询失败: %w", qErr)
		}
		if status != "paid" {
			return entity.Subscription{}, fmt.Errorf("支付网关未确认付款（状态: %s）", status)
		}
	}

	now := time.Now()
	// ① 订单 → paid
	if err := uc.orderRepo.UpdateStatus(ctx, order.ID, entity.OrderStatusPaid, order.PaymentID, now); err != nil {
		return entity.Subscription{}, err
	}
	// ② 开通/续期订阅（覆盖式：同租户已有订阅则续期）
	periodStart := now
	periodEnd := periodStart.AddDate(0, 1, 0) // +1 月
	sub := entity.Subscription{
		ID:          fmt.Sprintf("sub-%d", now.UnixNano()),
		TenantID:    order.TenantID,
		PlanID:      order.PlanID,
		Status:      entity.SubscriptionStatusActive,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	// 已有订阅则复用 ID（续期而非新建——unique 约束）
	if existing, err := uc.subRepo.FindByTenant(ctx, order.TenantID); err == nil {
		sub.ID = existing.ID
		sub.CreatedAt = existing.CreatedAt
	}
	if err := uc.subRepo.Save(ctx, sub); err != nil {
		return entity.Subscription{}, err
	}
	return sub, nil
}

// ---- admin 手动开通 ----

// AssignPlan admin 手动给租户开通套餐（跳过支付——线下收款场景）。
func (uc *BillingUseCase) AssignPlan(ctx context.Context, tenantID, planID string) (entity.Subscription, error) {
	if tenantID == "" || planID == "" {
		return entity.Subscription{}, fmt.Errorf("%w: tenant_id/plan_id 必填", pkg.ErrInvalidArgument)
	}
	if _, err := uc.planRepo.FindByID(ctx, planID); err != nil {
		return entity.Subscription{}, fmt.Errorf("套餐不存在: %w", err)
	}
	now := time.Now()
	sub := entity.Subscription{
		ID:          fmt.Sprintf("sub-%d", now.UnixNano()),
		TenantID:    tenantID,
		PlanID:      planID,
		Status:      entity.SubscriptionStatusActive,
		PeriodStart: now,
		PeriodEnd:   now.AddDate(0, 1, 0),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if existing, err := uc.subRepo.FindByTenant(ctx, tenantID); err == nil {
		sub.ID = existing.ID
		sub.CreatedAt = existing.CreatedAt
	}
	if err := uc.subRepo.Save(ctx, sub); err != nil {
		return entity.Subscription{}, err
	}
	return sub, nil
}

// ---- 收入报表（admin）----

// RevenueSummary 收入概览（admin 计费后台仪表盘）。
type RevenueSummary struct {
	TotalRevenueCents  int               `json:"total_revenue_cents"`  // 累计已支付收入（分）
	MonthRevenueCents  int               `json:"month_revenue_cents"`  // 当月收入（分）
	PaidOrders         int               `json:"paid_orders"`          // 已支付订单数
	ActiveSubscriptions int              `json:"active_subscriptions"` // 有效订阅数
	PlanDistribution   map[string]int    `json:"plan_distribution"`    // plan_id → 订户数
}

// RevenueReport 生成收入概览（admin 收入仪表盘用）。
func (uc *BillingUseCase) RevenueReport(ctx context.Context) (RevenueSummary, error) {
	orders, err := uc.orderRepo.ListAll(ctx)
	if err != nil {
		return RevenueSummary{}, err
	}
	subs, err := uc.subRepo.ListAll(ctx)
	if err != nil {
		return RevenueSummary{}, err
	}

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	summary := RevenueSummary{
		PlanDistribution: map[string]int{},
	}
	for _, o := range orders {
		if o.Status == entity.OrderStatusPaid {
			summary.TotalRevenueCents += o.AmountCents
			summary.PaidOrders++
			if !o.PaidAt.Before(monthStart) {
				summary.MonthRevenueCents += o.AmountCents
			}
		}
	}
	for _, s := range subs {
		if s.IsActive(now) {
			summary.ActiveSubscriptions++
			summary.PlanDistribution[s.PlanID]++
		}
	}
	return summary, nil
}

// ---- 商户端用量查询 ----

// MyUsageSummary 我的用量概览（商户端"我的套餐"页用量进度条用）。
type MyUsageSummary struct {
	Subscription *entity.Subscription  `json:"subscription"` // 当前订阅（nil=免费降级）
	Plan         entity.Plan           `json:"plan"`         // 适用套餐
	Usages       map[string]UsageEntry `json:"usages"`       // 场景→用量详情
}

// UsageEntry 单场景用量。
type UsageEntry struct {
	Limit int `json:"limit"` // 配额上限（-1=无限）
	Used  int `json:"used"`  // 当前周期已用
}

// GetMyUsage 取租户用量概览（套餐 + 各场景配额余量）。
// quotaGate 为 nil 时只返回套餐不含用量（降级）。
func (uc *BillingUseCase) GetMyUsage(ctx context.Context, tenantID string, quotaGate port.QuotaStore) (MyUsageSummary, error) {
	// 解析适用套餐（订阅优先，无则 free）
	var plan entity.Plan
	var sub *entity.Subscription
	if s, err := uc.subRepo.FindByTenant(ctx, tenantID); err == nil && s.IsActive(time.Now()) {
		if p, pErr := uc.planRepo.FindByID(ctx, s.PlanID); pErr == nil {
			plan = p
			sc := s
			sub = &sc
		}
	}
	if plan.ID == "" {
		if p, err := uc.planRepo.FindByID(ctx, "plan-free"); err == nil {
			plan = p
		}
	}

	usages := map[string]UsageEntry{}
	for scene := range plan.Quotas {
		limit := plan.QuotaFor(scene)
		if quotaGate != nil {
			if l, used, err := quotaGate.QuotaFor(ctx, tenantID, scene); err == nil {
				usages[scene] = UsageEntry{Limit: l, Used: used}
				continue
			}
		}
		usages[scene] = UsageEntry{Limit: limit, Used: 0}
	}
	return MyUsageSummary{Subscription: sub, Plan: plan, Usages: usages}, nil
}

// ---- X-01 成本分析（商业闭环成本侧）----

// SceneCost 单场景成本条目。
type SceneCost struct {
	Scene        string
	Calls        int     // LLM 调用次数（或业务动作计数）
	TotalTokens  int64   // token 总量（非 LLM 场景为 0）
	EstCostCents int     // 估算成本（分；perMTokenCents=0 时为 0）
}

// CostAnalysis 成本分析报告（admin 运营报表）。
type CostAnalysis struct {
	Days           int   // 统计窗口（天）
	PerMTokenCents int   // 参考单价（分/百万 tokens）
	Scenes         []SceneCost
	TotalCalls     int
	TotalTokens    int64
	TotalCostCents int   // 估算总成本（分）
}

// CostAnalysis 生成近 N 天按场景的成本分析（X-01 商业闭环收口）。
// 数据源：usages 表按场景聚合（SumBySceneSince）；估算成本 = tokens × 参考单价。
// 未注入 usageStats 时返回空报告（降级不报错）。
func (uc *BillingUseCase) CostAnalysis(ctx context.Context, days int) (CostAnalysis, error) {
	if uc.usageStats == nil {
		return CostAnalysis{Days: days, PerMTokenCents: uc.perMTokenCents}, nil
	}
	if days <= 0 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -days)
	scenes, err := uc.usageStats.SumBySceneSince(ctx, since)
	if err != nil {
		return CostAnalysis{}, err
	}
	report := CostAnalysis{Days: days, PerMTokenCents: uc.perMTokenCents}
	for _, s := range scenes {
		cost := 0
		if uc.perMTokenCents > 0 && s.TotalTokens > 0 {
			cost = int(s.TotalTokens * int64(uc.perMTokenCents) / 1_000_000)
		}
		report.Scenes = append(report.Scenes, SceneCost{
			Scene: s.Scene, Calls: s.Calls, TotalTokens: s.TotalTokens, EstCostCents: cost,
		})
		report.TotalCalls += s.Calls
		report.TotalTokens += s.TotalTokens
		report.TotalCostCents += cost
	}
	return report, nil
}
