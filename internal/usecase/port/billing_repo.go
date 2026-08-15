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

// PaymentClosureWriter 支付闭环原子写入器。
//
// 业务约束（用例层声明，适配层实现）：ConfirmPayment 的两步写——
// 订单置为已支付 + 订阅开通/续期——必须原子完成，否则会出现
// "已扣款但未开通"的中间态。接口归用例所有（依赖倒置），
// 适配器决定事务机制（GORM 本地事务；未来跨库时换 Saga 实现，用例零改动）。
// 未注入时用例降级为两段写（无 DB / mock 场景，行为与此前一致）。
type PaymentClosureWriter interface {
	// MarkPaidAndActivate 同一事务内：订单 → paid（记 paymentID/paidAt）+ 保存订阅。
	MarkPaidAndActivate(ctx context.Context, orderID, paymentID string, paidAt time.Time, sub entity.Subscription) error
}
