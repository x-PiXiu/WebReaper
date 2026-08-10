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
	planRepo port.PlanRepository
	subRepo  port.SubscriptionRepository
	orderRepo port.OrderRepository
}

func NewBillingUseCase(plan port.PlanRepository, sub port.SubscriptionRepository, order port.OrderRepository) *BillingUseCase {
	return &BillingUseCase{planRepo: plan, subRepo: sub, orderRepo: order}
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
