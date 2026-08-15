package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ---- Plan 仓储 ----

type GormPlanRepository struct {
	db *gorm.DB
}

func NewGormPlanRepository(db *gorm.DB) *GormPlanRepository {
	return &GormPlanRepository{db: db}
}

func (r *GormPlanRepository) Save(ctx context.Context, p entity.Plan) error {
	return r.db.WithContext(ctx).Save(planToPO(p)).Error
}

func (r *GormPlanRepository) FindByID(ctx context.Context, id string) (entity.Plan, error) {
	var po PlanPO
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.Plan{}, pkg.ErrNotFound
		}
		return entity.Plan{}, err
	}
	return planFromPO(po), nil
}

func (r *GormPlanRepository) ListActive(ctx context.Context) ([]entity.Plan, error) {
	var pos []PlanPO
	if err := r.db.WithContext(ctx).Where("status = ?", entity.PlanStatusActive).
		Order("sort_order ASC, price_cents ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	return planListFromPO(pos), nil
}

func (r *GormPlanRepository) ListAll(ctx context.Context) ([]entity.Plan, error) {
	var pos []PlanPO
	if err := r.db.WithContext(ctx).Order("sort_order ASC, price_cents ASC").Find(&pos).Error; err != nil {
		return nil, err
	}
	return planListFromPO(pos), nil
}

func (r *GormPlanRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&PlanPO{}).Error
}

// ---- Subscription 仓储 ----

type GormSubscriptionRepository struct {
	db *gorm.DB
}

func NewGormSubscriptionRepository(db *gorm.DB) *GormSubscriptionRepository {
	return &GormSubscriptionRepository{db: db}
}

func (r *GormSubscriptionRepository) Save(ctx context.Context, s entity.Subscription) error {
	return r.db.WithContext(ctx).Save(subscriptionToPO(s)).Error
}

func (r *GormSubscriptionRepository) FindByTenant(ctx context.Context, tenantID string) (entity.Subscription, error) {
	var po SubscriptionPO
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.Subscription{}, pkg.ErrNotFound
		}
		return entity.Subscription{}, err
	}
	return subscriptionFromPO(po), nil
}

func (r *GormSubscriptionRepository) ListAll(ctx context.Context) ([]entity.Subscription, error) {
	var pos []SubscriptionPO
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	out := make([]entity.Subscription, 0, len(pos))
	for _, p := range pos {
		out = append(out, subscriptionFromPO(p))
	}
	return out, nil
}

// ---- Order 仓储 ----

type GormOrderRepository struct {
	db *gorm.DB
}

func NewGormOrderRepository(db *gorm.DB) *GormOrderRepository {
	return &GormOrderRepository{db: db}
}

func (r *GormOrderRepository) Save(ctx context.Context, o entity.Order) error {
	return r.db.WithContext(ctx).Save(orderToPO(o)).Error
}

func (r *GormOrderRepository) FindByID(ctx context.Context, id string) (entity.Order, error) {
	var po OrderPO
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.Order{}, pkg.ErrNotFound
		}
		return entity.Order{}, err
	}
	return orderFromPO(po), nil
}

func (r *GormOrderRepository) ListByTenant(ctx context.Context, tenantID string) ([]entity.Order, error) {
	var pos []OrderPO
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).
		Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	return orderListFromPO(pos), nil
}

func (r *GormOrderRepository) ListAll(ctx context.Context) ([]entity.Order, error) {
	var pos []OrderPO
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&pos).Error; err != nil {
		return nil, err
	}
	return orderListFromPO(pos), nil
}

func (r *GormOrderRepository) UpdateStatus(ctx context.Context, id, status, paymentID string, paidAt time.Time) error {
	updates := map[string]any{"status": status}
	if paymentID != "" {
		updates["payment_id"] = paymentID
	}
	if !paidAt.IsZero() {
		updates["paid_at"] = paidAt
	}
	return r.db.WithContext(ctx).Model(&OrderPO{}).Where("id = ?", id).Updates(updates).Error
}

// ---- 支付闭环原子写入 ----

// GormPaymentClosureWriter 订单置 paid + 订阅保存在同一 DB 事务内完成，
// 消除"已支付订单但无订阅"的中间态（billing 用例 ConfirmPayment 依赖此保证）。
type GormPaymentClosureWriter struct {
	db *gorm.DB
}

func NewGormPaymentClosureWriter(db *gorm.DB) *GormPaymentClosureWriter {
	return &GormPaymentClosureWriter{db: db}
}

var _ port.PaymentClosureWriter = (*GormPaymentClosureWriter)(nil)

func (r *GormPaymentClosureWriter) MarkPaidAndActivate(ctx context.Context, orderID, paymentID string, paidAt time.Time, sub entity.Subscription) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"status": entity.OrderStatusPaid}
		if paymentID != "" {
			updates["payment_id"] = paymentID
		}
		if !paidAt.IsZero() {
			updates["paid_at"] = paidAt
		}
		if err := tx.Model(&OrderPO{}).Where("id = ?", orderID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Save(subscriptionToPO(sub)).Error
	})
}

// ---- 转换 ----

func planToPO(p entity.Plan) PlanPO {
	quotas, _ := json.Marshal(p.Quotas)
	features, _ := json.Marshal(p.Features)
	return PlanPO{
		ID: p.ID, Name: p.Name, Level: p.Level, PriceCents: p.PriceCents,
		Quotas: string(quotas), Features: string(features),
		Status: p.Status, SortOrder: p.SortOrder,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func planFromPO(p PlanPO) entity.Plan {
	quotas := map[string]int{}
	if p.Quotas != "" {
		_ = json.Unmarshal([]byte(p.Quotas), &quotas)
	}
	var features []string
	if p.Features != "" {
		_ = json.Unmarshal([]byte(p.Features), &features)
	}
	if features == nil {
		features = []string{}
	}
	return entity.Plan{
		ID: p.ID, Name: p.Name, Level: p.Level, PriceCents: p.PriceCents,
		Quotas: quotas, Features: features,
		Status: p.Status, SortOrder: p.SortOrder,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func planListFromPO(pos []PlanPO) []entity.Plan {
	out := make([]entity.Plan, 0, len(pos))
	for _, p := range pos {
		out = append(out, planFromPO(p))
	}
	return out
}

func subscriptionToPO(s entity.Subscription) SubscriptionPO {
	return SubscriptionPO{
		ID: s.ID, TenantID: s.TenantID, PlanID: s.PlanID, Status: s.Status,
		PeriodStart: s.PeriodStart, PeriodEnd: s.PeriodEnd,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func subscriptionFromPO(p SubscriptionPO) entity.Subscription {
	return entity.Subscription{
		ID: p.ID, TenantID: p.TenantID, PlanID: p.PlanID, Status: p.Status,
		PeriodStart: p.PeriodStart, PeriodEnd: p.PeriodEnd,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func orderToPO(o entity.Order) OrderPO {
	return OrderPO{
		ID: o.ID, TenantID: o.TenantID, PlanID: o.PlanID, AmountCents: o.AmountCents,
		Status: o.Status, PaymentGateway: o.PaymentGateway, PaymentID: o.PaymentID,
		CreatedAt: o.CreatedAt, PaidAt: timeToPtr(o.PaidAt),
	}
}

func orderFromPO(p OrderPO) entity.Order {
	return entity.Order{
		ID: p.ID, TenantID: p.TenantID, PlanID: p.PlanID, AmountCents: p.AmountCents,
		Status: p.Status, PaymentGateway: p.PaymentGateway, PaymentID: p.PaymentID,
		CreatedAt: p.CreatedAt, PaidAt: ptrToTime(p.PaidAt),
	}
}

func orderListFromPO(pos []OrderPO) []entity.Order {
	out := make([]entity.Order, 0, len(pos))
	for _, p := range pos {
		out = append(out, orderFromPO(p))
	}
	return out
}
