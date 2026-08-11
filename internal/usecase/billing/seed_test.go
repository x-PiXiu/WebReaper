package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

var errNotFound = errors.New("not found")

// ---- X-01 测试：套餐配额语义 + 兼容升级 + 成本分析 ----

func TestDefaultPlans_QuotaSemantics(t *testing.T) {
	plans := DefaultPlans()
	byID := map[string]entity.Plan{}
	for _, p := range plans {
		byID[p.ID] = p
	}

	free := byID["plan-free"]
	// X-01 关键断言：免费版 monitor 配额必须支持"至少一次完整闭环"——
	// 1 关键词×1 引擎×5 采样 = 10 次 LLM 调用；500 次 ≈ 50 次单关键词监测
	if free.Quotas["monitor"] < 100 {
		t.Errorf("free.monitor = %d, 应 ≥100（否则免费用户连一次像样的监测都做不了）", free.Quotas["monitor"])
	}
	// 新场景必须存在（nearby/diagnose）
	if _, ok := free.Quotas["nearby"]; !ok {
		t.Error("free 套餐缺少 nearby 场景配额")
	}
	if _, ok := free.Quotas["diagnose"]; !ok {
		t.Error("free 套餐缺少 diagnose 场景配额")
	}
	// team 全无限
	team := byID["plan-team"]
	for scene, v := range team.Quotas {
		if v != -1 {
			t.Errorf("team.%s = %d, 应为 -1（无限）", scene, v)
		}
	}
	// 价格阶梯：free < pro < team
	if byID["plan-free"].PriceCents >= byID["plan-pro"].PriceCents {
		t.Error("价格阶梯错误")
	}
}

// fakePlanRepo 内存套餐仓储（SeedPlans 兼容升级测试用）。
type fakePlanRepo struct {
	plans map[string]entity.Plan
}

func (f *fakePlanRepo) Save(ctx context.Context, p entity.Plan) error {
	if f.plans == nil {
		f.plans = map[string]entity.Plan{}
	}
	f.plans[p.ID] = p
	return nil
}
func (f *fakePlanRepo) FindByID(ctx context.Context, id string) (entity.Plan, error) {
	if p, ok := f.plans[id]; ok {
		return p, nil
	}
	return entity.Plan{}, errNotFound
}
func (f *fakePlanRepo) List(ctx context.Context) ([]entity.Plan, error) {
	var out []entity.Plan
	for _, p := range f.plans {
		out = append(out, p)
	}
	return out, nil
}
func (f *fakePlanRepo) ListActive(ctx context.Context) ([]entity.Plan, error) { return f.List(ctx) }
func (f *fakePlanRepo) ListAll(ctx context.Context) ([]entity.Plan, error)   { return f.List(ctx) }
func (f *fakePlanRepo) Delete(ctx context.Context, id string) error          { return nil }

func TestSeedPlans_CompatibleUpgrade(t *testing.T) {
	ctx := context.Background()
	repo := &fakePlanRepo{}

	// 模拟旧库：plan-free 只有旧场景（monitor=30，运营"修改过"的值）
	now := time.Now()
	oldPlan := entity.Plan{
		ID: "plan-free", Name: "免费版", Level: "free", PriceCents: 0, SortOrder: 1,
		Quotas: map[string]int{"monitor": 30, "content-gen": 5},
		Status: entity.PlanStatusActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Save(ctx, oldPlan); err != nil {
		t.Fatalf("save old plan: %v", err)
	}

	if err := SeedPlans(ctx, repo); err != nil {
		t.Fatalf("SeedPlans: %v", err)
	}

	got, _ := repo.FindByID(ctx, "plan-free")
	// 关键断言 1：运营修改过的 monitor=30 不被覆盖（保留运营优先）
	if got.Quotas["monitor"] != 30 {
		t.Errorf("monitor 被覆盖 = %d, 应保留运营值 30", got.Quotas["monitor"])
	}
	// 关键断言 2：缺失的新场景被补齐（nearby/diagnose 自动升级）
	if got.Quotas["nearby"] == 0 {
		t.Error("nearby 场景配额未被补齐")
	}
	if got.Quotas["diagnose"] == 0 {
		t.Error("diagnose 场景配额未被补齐")
	}
	// 关键断言 3：新套餐（plan-pro/plan-team）被完整写入
	if _, err := repo.FindByID(ctx, "plan-pro"); err != nil {
		t.Error("plan-pro 未写入")
	}
}

// ---- CostAnalysis 成本分析测试（X-01 商业闭环成本侧）----

type fakeUsageStats struct {
	scenes []port.SceneUsage
	configs []port.SceneConfigUsage
}

func (f *fakeUsageStats) SumBySceneSince(ctx context.Context, since time.Time) ([]port.SceneUsage, error) {
	return f.scenes, nil
}

func (f *fakeUsageStats) SumBySceneAndConfigSince(ctx context.Context, since time.Time) ([]port.SceneConfigUsage, error) {
	return f.configs, nil
}

func TestBillingUseCase_CostAnalysis(t *testing.T) {
	uc := NewBillingUseCase(&fakePlanRepo{}, nil, nil)
	uc.SetUsageStats(&fakeUsageStats{scenes: []port.SceneUsage{
		{Scene: "monitor", Calls: 1000, TotalTokens: 800_000},
		{Scene: "content-gen", Calls: 50, TotalTokens: 200_000},
		{Scene: "nearby", Calls: 10, TotalTokens: 0}, // 非 LLM 计数
	}})
	uc.SetReferencePricePerMToken(100) // ¥1/百万 tokens

	report, err := uc.CostAnalysis(context.Background(), 30)
	if err != nil {
		t.Fatalf("CostAnalysis: %v", err)
	}
	if report.TotalCalls != 1060 {
		t.Errorf("TotalCalls = %d, want 1060", report.TotalCalls)
	}
	if report.TotalTokens != 1_000_000 {
		t.Errorf("TotalTokens = %d, want 1000000", report.TotalTokens)
	}
	// 100 万 tokens × ¥1/百万 = 100 分
	if report.TotalCostCents != 100 {
		t.Errorf("TotalCostCents = %d, want 100", report.TotalCostCents)
	}
	// nearby 无 token 不应产生成本
	for _, s := range report.Scenes {
		if s.Scene == "nearby" && s.EstCostCents != 0 {
			t.Errorf("nearby 不应有成本: %d", s.EstCostCents)
		}
	}

	t.Run("单价为 0 → 只报 token 不估算金额", func(t *testing.T) {
		uc2 := NewBillingUseCase(&fakePlanRepo{}, nil, nil)
		uc2.SetUsageStats(&fakeUsageStats{scenes: []port.SceneUsage{{Scene: "monitor", Calls: 1, TotalTokens: 100}}})
		r, _ := uc2.CostAnalysis(context.Background(), 30)
		if r.TotalCostCents != 0 {
			t.Errorf("单价 0 时不应估算金额: %d", r.TotalCostCents)
		}
	})

	t.Run("未注入 usageStats → 空报告不报错", func(t *testing.T) {
		uc3 := NewBillingUseCase(&fakePlanRepo{}, nil, nil)
		r, err := uc3.CostAnalysis(context.Background(), 30)
		if err != nil {
			t.Fatalf("不应报错: %v", err)
		}
		if len(r.Scenes) != 0 {
			t.Errorf("应返回空场景: %+v", r.Scenes)
		}
	})
}
