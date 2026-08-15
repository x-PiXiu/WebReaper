package geo

import (
	"context"
	"errors"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- 测试替身（谦卑对象模式：probe/仓储只搬运，业务逻辑纯函数化后可离线测试）----

type monFakeProbe struct {
	result port.ProbeResult
	err    error
	calls  int
}

func (f *monFakeProbe) Probe(_ context.Context, _ port.ProbeInput) (port.ProbeResult, error) {
	f.calls++
	return f.result, f.err
}

type monFakeBrandRepo struct {
	port.BrandRepository
	brands map[string]entity.Brand
}

func (f *monFakeBrandRepo) FindByID(_ context.Context, _, id string) (entity.Brand, error) {
	if b, ok := f.brands[id]; ok {
		return b, nil
	}
	return entity.Brand{}, errors.New("品牌不存在")
}

func (f *monFakeBrandRepo) ListByTenant(_ context.Context, _ string) ([]entity.Brand, error) {
	out := make([]entity.Brand, 0, len(f.brands))
	for _, b := range f.brands {
		out = append(out, b)
	}
	return out, nil
}

type monFakeKeywordRepo struct {
	port.KeywordRepository
	kws []entity.Keyword
}

func (f *monFakeKeywordRepo) FindByID(_ context.Context, _, id string) (entity.Keyword, error) {
	for _, k := range f.kws {
		if k.ID == id {
			return k, nil
		}
	}
	return entity.Keyword{}, errors.New("关键词不存在")
}

func (f *monFakeKeywordRepo) ListByBrand(_ context.Context, _, brandID string) ([]entity.Keyword, error) {
	var out []entity.Keyword
	for _, k := range f.kws {
		if k.BrandID == brandID {
			out = append(out, k)
		}
	}
	return out, nil
}

type monFakeResultRepo struct {
	port.MonitoringResultRepository
	saved []entity.MonitoringResult
}

func (f *monFakeResultRepo) Save(_ context.Context, r entity.MonitoringResult) error {
	f.saved = append(f.saved, r)
	return nil
}

func newTestMonitor(brands map[string]entity.Brand, kws []entity.Keyword, probe port.AIEngineProbe) (*MonitorUseCase, *monFakeResultRepo) {
	resultRepo := &monFakeResultRepo{}
	uc := NewMonitorUseCase(&monFakeBrandRepo{brands: brands}, &monFakeKeywordRepo{kws: kws}, resultRepo, probe)
	return uc, resultRepo
}

var monTestBrand = entity.Brand{ID: "b1", TenantID: "t1", Name: "测试品牌", Competitors: []string{"竞品A"}}

// 信源断流回归测试：单关键词路径必须落库 Sources/SelfSourceCount/FirstPickCount/SemanticDegraded，
// 且情感值（LLM 返回任意字符串）在组装点归一为合法枚举。
func TestMonitorKeywordAssemblyIncludesSemanticFields(t *testing.T) {
	probe := &monFakeProbe{result: port.ProbeResult{
		SampleCount: 3, MentionCount: 2, MentionRate: 2.0 / 3.0,
		AvgPosition: 1, Sentiment: "AWESOME", // 非法枚举 → 应归一 neutral
		Competitors:         map[string]int{"竞品A": 3},
		CompetitorSentiments: map[string]string{"竞品A": "EXCELLENT"}, // 非法 → neutral
		OtherBrands:         []string{"新竞品"},
		Sources:             []string{"https://self.example.com/a", "https://media.example.com/b"},
		SelfSourceCount:     1,
		FirstPickCount:      2,
		SemanticDegraded:    true,
	}}
	uc, repo := newTestMonitor(
		map[string]entity.Brand{"b1": monTestBrand},
		[]entity.Keyword{{ID: "k1", TenantID: "t1", BrandID: "b1", Term: "测试词"}},
		probe,
	)
	res, err := uc.MonitorKeyword(context.Background(), MonitorKeywordInput{TenantID: "t1", KeywordID: "k1"})
	if err != nil {
		t.Fatalf("MonitorKeyword: %v", err)
	}
	if res.Sources == nil || len(res.Sources) != 2 {
		t.Errorf("Sources 未落库: %+v", res.Sources)
	}
	if res.SelfSourceCount != 1 {
		t.Errorf("SelfSourceCount = %d, want 1", res.SelfSourceCount)
	}
	if res.FirstPickCount != 2 {
		t.Errorf("FirstPickCount = %d, want 2", res.FirstPickCount)
	}
	if !res.SemanticDegraded {
		t.Error("SemanticDegraded 未落库")
	}
	if res.Sentiment != "neutral" {
		t.Errorf("非法情感应归一 neutral, got %q", res.Sentiment)
	}
	if res.CompetitorSentiments["竞品A"] != "neutral" {
		t.Errorf("竞品非法情感应归一 neutral, got %q", res.CompetitorSentiments["竞品A"])
	}
	if res.CompetitorRates["竞品A"] != 1.0 {
		t.Errorf("竞品提及率应按采样数归一为 1.0, got %v", res.CompetitorRates["竞品A"])
	}
	if len(repo.saved) != 1 || repo.saved[0].SelfSourceCount != 1 {
		t.Errorf("仓储未收到完整结果: %+v", repo.saved)
	}
}

// 信源断流回归测试（v3 H1-①）：全量 Monitor 路径（自动盯盘走的就是它）
// 此前漏存 Sources/SelfSourceCount——收敛到 newMonitoringResult 后必须与单关键词路径同口径。
func TestMonitorFullRunPersistsSources(t *testing.T) {
	probe := &monFakeProbe{result: port.ProbeResult{
		SampleCount: 3, MentionCount: 1, MentionRate: 1.0 / 3.0,
		Sources:         []string{"https://self.example.com/x"},
		SelfSourceCount: 1,
		FirstPickCount:  1,
	}}
	uc, repo := newTestMonitor(
		map[string]entity.Brand{"b1": monTestBrand},
		[]entity.Keyword{{ID: "k1", TenantID: "t1", BrandID: "b1", Term: "词一"}, {ID: "k2", TenantID: "t1", BrandID: "b1", Term: "词二"}},
		probe,
	)
	results, err := uc.Monitor(context.Background(), MonitorInput{TenantID: "t1", BrandID: "b1"})
	if err != nil {
		t.Fatalf("Monitor: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("期望 2 条结果, got %d", len(results))
	}
	for _, r := range results {
		if len(r.Sources) != 1 || r.SelfSourceCount != 1 || r.FirstPickCount != 1 {
			t.Errorf("全量路径信源/首选字段丢失: %+v", r)
		}
	}
	if len(repo.saved) != 2 {
		t.Errorf("仓储收到 %d 条, want 2", len(repo.saved))
	}
}
