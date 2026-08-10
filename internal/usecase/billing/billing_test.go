package billing

import (
	"context"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
)

// 完整覆盖：下单 → 确认支付 → 订阅开通；重复确认幂等；手动开通。
func TestCreateOrderConfirmAssignFlow(t *testing.T) {
	planRepo := newMockPlanRepo()
	subRepo := newMockSubRepo()
	orderRepo := newMockOrderRepo()
	uc := NewBillingUseCase(planRepo, subRepo, orderRepo)

	// seed 一个 pro 套餐
	proPlan := entity.Plan{ID: "plan-pro", Name: "专业版", Level: "pro", PriceCents: 29900, Status: entity.PlanStatusActive, Quotas: map[string]int{"monitor": 500}}
	_ = planRepo.Save(context.Background(), proPlan)

	// ① 下单
	result, err := uc.CreateOrder(context.Background(), CreateOrderInput{TenantID: "t1", PlanID: "plan-pro"})
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	if result.Order.Status != entity.OrderStatusPending || result.Order.AmountCents != 29900 {
		t.Fatalf("订单状态/金额异常: status=%s amount=%d", result.Order.Status, result.Order.AmountCents)
	}

	// ② 确认支付 → 订阅开通
	sub, err := uc.ConfirmPayment(context.Background(), result.Order.ID)
	if err != nil {
		t.Fatalf("确认支付失败: %v", err)
	}
	if sub.Status != entity.SubscriptionStatusActive || sub.PlanID != "plan-pro" {
		t.Fatalf("订阅异常: status=%s plan=%s", sub.Status, sub.PlanID)
	}
	if !sub.PeriodEnd.After(sub.PeriodStart) {
		t.Fatalf("计费周期异常：end 应晚于 start")
	}

	// ③ 订单状态应已变 paid
	order, _ := orderRepo.FindByID(context.Background(), result.Order.ID)
	if order.Status != entity.OrderStatusPaid {
		t.Fatalf("订单状态应为 paid，得到 %s", order.Status)
	}

	// ④ 幂等：再次确认不报错，订阅不变
	sub2, err := uc.ConfirmPayment(context.Background(), result.Order.ID)
	if err != nil {
		t.Fatalf("重复确认应幂等，得到: %v", err)
	}
	if sub2.ID != sub.ID {
		t.Fatalf("幂等确认应返回同一订阅")
	}
}

func TestCreateOrderPlanNotFound(t *testing.T) {
	uc := NewBillingUseCase(newMockPlanRepo(), newMockSubRepo(), newMockOrderRepo())
	_, err := uc.CreateOrder(context.Background(), CreateOrderInput{TenantID: "t1", PlanID: "not-exist"})
	if err == nil {
		t.Fatal("套餐不存在应报错")
	}
}

func TestCreateOrderArchivedPlan(t *testing.T) {
	planRepo := newMockPlanRepo()
	_ = planRepo.Save(context.Background(), entity.Plan{ID: "plan-old", Name: "旧版", Level: "pro", Status: entity.PlanStatusArchived, Quotas: map[string]int{}})
	uc := NewBillingUseCase(planRepo, newMockSubRepo(), newMockOrderRepo())
	_, err := uc.CreateOrder(context.Background(), CreateOrderInput{TenantID: "t1", PlanID: "plan-old"})
	if err == nil {
		t.Fatal("下架套餐应不可购买")
	}
}

func TestAssignPlanManual(t *testing.T) {
	planRepo := newMockPlanRepo()
	_ = planRepo.Save(context.Background(), entity.Plan{ID: "plan-team", Name: "团队版", Level: "team", Status: entity.PlanStatusActive, Quotas: map[string]int{"monitor": -1}})
	subRepo := newMockSubRepo()
	uc := NewBillingUseCase(planRepo, subRepo, newMockOrderRepo())

	sub, err := uc.AssignPlan(context.Background(), "t1", "plan-team")
	if err != nil {
		t.Fatalf("手动开通失败: %v", err)
	}
	if sub.PlanID != "plan-team" || sub.Status != entity.SubscriptionStatusActive {
		t.Fatalf("手动开通订阅异常: %+v", sub)
	}

	// 续期：再次 AssignPlan 同租户应复用订阅 ID
	sub2, _ := uc.AssignPlan(context.Background(), "t1", "plan-team")
	if sub2.ID != sub.ID {
		t.Fatalf("续期应复用订阅 ID")
	}
}

// ---- 内存 mock ----

type mockPlanRepo struct{ plans map[string]entity.Plan }

func newMockPlanRepo() *mockPlanRepo { return &mockPlanRepo{plans: map[string]entity.Plan{}} }
func (m *mockPlanRepo) Save(_ context.Context, p entity.Plan) error {
	m.plans[p.ID] = p
	return nil
}
func (m *mockPlanRepo) FindByID(_ context.Context, id string) (entity.Plan, error) {
	if p, ok := m.plans[id]; ok {
		return p, nil
	}
	return entity.Plan{}, pkg.ErrNotFound
}
func (m *mockPlanRepo) ListActive(_ context.Context) ([]entity.Plan, error) { return nil, nil }
func (m *mockPlanRepo) ListAll(_ context.Context) ([]entity.Plan, error)    { return nil, nil }
func (m *mockPlanRepo) Delete(_ context.Context, id string) error          { delete(m.plans, id); return nil }

type mockSubRepo struct{ subs map[string]entity.Subscription }

func newMockSubRepo() *mockSubRepo { return &mockSubRepo{subs: map[string]entity.Subscription{}} }
func (m *mockSubRepo) Save(_ context.Context, s entity.Subscription) error {
	m.subs[s.TenantID] = s
	return nil
}
func (m *mockSubRepo) FindByTenant(_ context.Context, tenantID string) (entity.Subscription, error) {
	if s, ok := m.subs[tenantID]; ok {
		return s, nil
	}
	return entity.Subscription{}, pkg.ErrNotFound
}
func (m *mockSubRepo) ListAll(_ context.Context) ([]entity.Subscription, error) { return nil, nil }

type mockOrderRepo struct{ orders map[string]entity.Order }

func newMockOrderRepo() *mockOrderRepo { return &mockOrderRepo{orders: map[string]entity.Order{}} }
func (m *mockOrderRepo) Save(_ context.Context, o entity.Order) error {
	m.orders[o.ID] = o
	return nil
}
func (m *mockOrderRepo) FindByID(_ context.Context, id string) (entity.Order, error) {
	if o, ok := m.orders[id]; ok {
		return o, nil
	}
	return entity.Order{}, pkg.ErrNotFound
}
func (m *mockOrderRepo) ListByTenant(_ context.Context, _ string) ([]entity.Order, error) { return nil, nil }
func (m *mockOrderRepo) ListAll(_ context.Context) ([]entity.Order, error)                { return nil, nil }
func (m *mockOrderRepo) UpdateStatus(_ context.Context, id, status, paymentID string, paidAt time.Time) error {
	o := m.orders[id]
	o.Status = status
	if paymentID != "" {
		o.PaymentID = paymentID
	}
	if !paidAt.IsZero() {
		o.PaidAt = paidAt
	}
	m.orders[id] = o
	return nil
}
