package port

import (
	"context"
	"time"

	"webreaper/internal/domain/entity"
)

// ---- 经济系统仓储接口（用例层声明，适配器实现）----
//
// 三个聚合根：Plan（套餐定义）/ Subscription（租户订阅）/ Order（支付订单）。
// 所有方法带 tenantID 维度（admin 传空看全局），与既有 GEO 仓储同口径。

// PlanRepository 套餐仓储。
type PlanRepository interface {
	Save(ctx context.Context, p entity.Plan) error
	FindByID(ctx context.Context, id string) (entity.Plan, error)
	// ListActive 列出在售套餐（status=active），按 SortOrder 升序。
	ListActive(ctx context.Context) ([]entity.Plan, error)
	// ListAll 全部套餐（含下架，admin 管理用）。
	ListAll(ctx context.Context) ([]entity.Plan, error)
	Delete(ctx context.Context, id string) error
}

// SubscriptionRepository 租户订阅仓储。
type SubscriptionRepository interface {
	Save(ctx context.Context, s entity.Subscription) error
	// FindByTenant 取租户当前订阅（无订阅返回 ErrNotFound）。
	FindByTenant(ctx context.Context, tenantID string) (entity.Subscription, error)
	// ListAll 全部订阅（admin 看全局）。
	ListAll(ctx context.Context) ([]entity.Subscription, error)
}

// OrderRepository 订单仓储。
type OrderRepository interface {
	Save(ctx context.Context, o entity.Order) error
	FindByID(ctx context.Context, id string) (entity.Order, error)
	// ListByTenant 租户的订单流水。
	ListByTenant(ctx context.Context, tenantID string) ([]entity.Order, error)
	// ListAll 全部订单（admin 收入报表用）。
	ListAll(ctx context.Context) ([]entity.Order, error)
	// UpdateStatus 更新订单状态与支付信息（回调确认用）。
	UpdateStatus(ctx context.Context, id, status, paymentID string, paidAt time.Time) error
}
