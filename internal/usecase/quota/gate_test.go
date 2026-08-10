package quota

import (
	"context"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// mockUsageQueryer 内存计数器，模拟 usages 表 COUNT。
type mockUsageQueryer struct {
	counts map[string]int // key: tenantID|scene → 已用次数
}

func (m *mockUsageQueryer) CountSince(_ context.Context, tenantID, scene string, _ time.Time) (int, error) {
	return m.counts[tenantID+"|"+scene], nil
}

func newMockGate(limit int, used int) (*Gate, *mockUsageQueryer) {
	usage := &mockUsageQueryer{counts: map[string]int{}}
	gate := &Gate{
		planRepo: &mockPlanRepo{plans: map[string]entity.Plan{
			"plan-free": {ID: "plan-free", Quotas: map[string]int{"monitor": limit}},
		}},
		subRepo:     &mockSubRepo{}, // 无订阅 → 降级 free
		usageQueryer: usage,
		freePlanID:  "plan-free",
	}
	return gate, usage
}

func TestCheckPass(t *testing.T) {
	// free 配额 5，已用 2 → 通过
	gate, usage := newMockGate(5, 0)
	usage.counts["t1|monitor"] = 2
	if err := gate.Check(context.Background(), "t1", "monitor"); err != nil {
		t.Fatalf("配额内应通过，得到: %v", err)
	}
}

func TestCheckExceeded(t *testing.T) {
	// free 配额 5，已用 5 → ErrQuotaExceeded
	gate, usage := newMockGate(5, 0)
	usage.counts["t1|monitor"] = 5
	err := gate.Check(context.Background(), "t1", "monitor")
	if err != pkg.ErrQuotaExceeded {
		t.Fatalf("超限应返回 ErrQuotaExceeded，得到: %v", err)
	}
}

func TestCheckUnlimited(t *testing.T) {
	// team 套餐 -1（无限） → 永远通过
	gate, _ := newMockGate(0, 0)
	gate.planRepo.(*mockPlanRepo).plans["plan-free"] = entity.Plan{
		ID: "plan-free", Quotas: map[string]int{"monitor": -1},
	}
	for i := 0; i < 100; i++ {
		if err := gate.Check(context.Background(), "t1", "monitor"); err != nil {
			t.Fatalf("无限配额不应报错（第 %d 次）: %v", i, err)
		}
	}
}

func TestCheckSceneNotAllowed(t *testing.T) {
	// free 未配置 video 场景（默认 0） → 不允许使用
	gate, _ := newMockGate(5, 0)
	err := gate.Check(context.Background(), "t1", "video")
	if err != pkg.ErrQuotaExceeded {
		t.Fatalf("未开通场景应返回 ErrQuotaExceeded，得到: %v", err)
	}
}

func TestCheckNilGatePass(t *testing.T) {
	// nil Gate 直接通过（向后兼容）
	var gate *Gate
	if err := gate.Check(context.Background(), "t1", "monitor"); err != nil {
		t.Fatalf("nil Gate 应通过，得到: %v", err)
	}
}

func TestCheckEmptyTenantPass(t *testing.T) {
	// admin 旁路（空租户）不计配额
	gate, _ := newMockGate(1, 0)
	if err := gate.Check(context.Background(), "", "monitor"); err != nil {
		t.Fatalf("空租户应通过（admin 旁路），得到: %v", err)
	}
}

func TestQuotaForDisplay(t *testing.T) {
	gate, usage := newMockGate(10, 0)
	usage.counts["t1|monitor"] = 7
	limit, used, err := gate.QuotaFor(context.Background(), "t1", "monitor")
	if err != nil || limit != 10 || used != 7 {
		t.Fatalf("期望 limit=10 used=7，得到 limit=%d used=%d err=%v", limit, used, err)
	}
}

// ---- mock 辅助 ----

type mockPlanRepo struct {
	plans map[string]entity.Plan
}

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
func (m *mockPlanRepo) ListActive(_ context.Context) ([]entity.Plan, error) {
	return nil, nil
}
func (m *mockPlanRepo) ListAll(_ context.Context) ([]entity.Plan, error) {
	return nil, nil
}
func (m *mockPlanRepo) Delete(_ context.Context, id string) error {
	delete(m.plans, id)
	return nil
}

type mockSubRepo struct{}

func (m *mockSubRepo) Save(_ context.Context, s entity.Subscription) error         { return nil }
func (m *mockSubRepo) FindByTenant(_ context.Context, _ string) (entity.Subscription, error) {
	return entity.Subscription{}, pkg.ErrNotFound // 无订阅 → 降级 free
}
func (m *mockSubRepo) ListAll(_ context.Context) ([]entity.Subscription, error) { return nil, nil }

// 编译期断言：确保 mock 实现了 port 接口
var _ port.PlanRepository = (*mockPlanRepo)(nil)
var _ port.SubscriptionRepository = (*mockSubRepo)(nil)
var _ port.UsageQueryer = (*mockUsageQueryer)(nil)
