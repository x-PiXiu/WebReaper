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
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// BillingUseCase 经济系统用例。
type BillingUseCase struct {
	planRepo  port.PlanRepository
	subRepo   port.SubscriptionRepository
	orderRepo port.OrderRepository
	payment   port.PaymentGateway // 可选：nil=线下模式（admin 手动开通）
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
