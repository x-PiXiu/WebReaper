package geo

import (
	"context"
	"math"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// 健康分纯函数表驱动测试（v3 归位：口径从前端 geoHealth.ts 迁入后端，必须可离线验证）。

func TestComputeHealthIndicators(t *testing.T) {
	latest := []*entity.MonitoringResult{
		{Sentiment: "positive", SampleCount: 10, FirstPickCount: 4, AvgPosition: 1, SelfSourceCount: 1},
		{Sentiment: "negative", SampleCount: 10, FirstPickCount: 1, AvgPosition: 3, SelfSourceCount: 0},
		{Sentiment: "neutral", SampleCount: 10, FirstPickCount: 1, AvgPosition: 2, SelfSourceCount: 0},
	}
	got := computeHealthIndicators(0.5, latest, 10, 4)

	if got.MentionCoverage != 50 {
		t.Errorf("MentionCoverage = %v, want 50", got.MentionCoverage)
	}
	// 情感：(1 - 1 + 0)/3 = 0 → (0+1)*50 = 50
	if got.SentimentScore != 50 {
		t.Errorf("SentimentScore = %v, want 50", got.SentimentScore)
	}
	// 首选率（新数据）：6/30 = 20%
	if got.FirstPickRate != 20 {
		t.Errorf("FirstPickRate = %v, want 20（FirstPickCount 6/30 采样）", got.FirstPickRate)
	}
	// 内容资产：4*15 + 6*5 = 90
	if got.ContentAsset != 90 {
		t.Errorf("ContentAsset = %v, want 90", got.ContentAsset)
	}
	// 信源完整：1/3 ≈ 33%
	if got.SourceIntegrity != 33 {
		t.Errorf("SourceIntegrity = %v, want 33", got.SourceIntegrity)
	}
}

// 旧数据（FirstPickCount=0）回退 avg_position==1 近似——F1-2 后需 ≥3 条有位次结果才展示，
// 不足返回 -1（前端"积累中"），修复 1 条命中即 100% 的矛盾展示。
func TestComputeHealthIndicatorsFirstPickFallback(t *testing.T) {
	// 4 条有位次结果（2 条第 1）→ 近似 50%
	latest := []*entity.MonitoringResult{
		{Sentiment: "positive", SampleCount: 5, AvgPosition: 1},
		{Sentiment: "positive", SampleCount: 5, AvgPosition: 1},
		{Sentiment: "positive", SampleCount: 5, AvgPosition: 2},
		{Sentiment: "positive", SampleCount: 5, AvgPosition: 2},
	}
	got := computeHealthIndicators(1.0, latest, 0, 0)
	if got.FirstPickRate != 50 {
		t.Errorf("FirstPickRate = %v, want 50（avg_position==1 近似 2/4）", got.FirstPickRate)
	}

	// 采样不足（仅 1 条命中）→ -1 积累中（曾显示误导性 100%）
	insufficient := []*entity.MonitoringResult{{Sentiment: "positive", SampleCount: 1, AvgPosition: 1}}
	got2 := computeHealthIndicators(1.0, insufficient, 0, 0)
	if got2.FirstPickRate != -1 {
		t.Errorf("FirstPickRate = %v, want -1（积累中）", got2.FirstPickRate)
	}
}

func TestComputeHealthTotalWeights(t *testing.T) {
	i := HealthIndicators{MentionCoverage: 100, SentimentScore: 100, FirstPickRate: 100, ContentAsset: 100, SourceIntegrity: 100}
	if got := computeHealthTotal(i); got != 100 {
		t.Errorf("全满指数总分 = %v, want 100", got)
	}
	// 仅提及覆盖 50 分：50*0.4 = 20
	i = HealthIndicators{MentionCoverage: 50}
	if got := computeHealthTotal(i); got != 20 {
		t.Errorf("单指数总分 = %v, want 20", got)
	}
}

func TestCompetitorHealthFrom(t *testing.T) {
	base := time.Now()
	latest := []entity.MonitoringResult{
		{
			KeywordID: "k1", ProbedAt: base, MentionRate: 0.4,
			CompetitorRates:      map[string]float64{"竞品A": 0.8, "竞品B": 0.2},
			CompetitorSentiments: map[string]string{"竞品A": "positive"},
		},
		{
			KeywordID: "k2", ProbedAt: base.Add(time.Minute), MentionRate: 0.6,
			CompetitorRates:      map[string]float64{"竞品A": 0.6},
			CompetitorSentiments: map[string]string{"竞品A": "positive"},
		},
	}
	ch := competitorHealthFrom(latest)
	if ch.Size != 2 {
		t.Fatalf("Size = %d, want 2", ch.Size)
	}
	if math.Abs(ch.SelfAvg-0.5) > 1e-9 {
		t.Errorf("SelfAvg = %v, want 0.5", ch.SelfAvg)
	}
	// 竞品A 均值 70%，竞品B 20%——威胁榜降序
	if ch.Threats[0].Name != "竞品A" || ch.Threats[0].AvgRate != 70 {
		t.Errorf("Threats[0] = %+v, want 竞品A/70", ch.Threats[0])
	}
	if ch.Threats[0].Sentiment != "positive" {
		t.Errorf("竞品A 多数投票情感 = %q, want positive", ch.Threats[0].Sentiment)
	}
	// gap = 0.5 - 0.45 = +5.0 个百分点
	if ch.GapPct != 5 {
		t.Errorf("GapPct = %v, want 5", ch.GapPct)
	}
}

func TestLatestByKeywordKeepsNewest(t *testing.T) {
	base := time.Now()
	rs := []entity.MonitoringResult{
		{KeywordID: "k1", ProbedAt: base, MentionRate: 0.1},
		{KeywordID: "k1", ProbedAt: base.Add(time.Hour), MentionRate: 0.9}, // 最新
		{KeywordID: "k2", ProbedAt: base, MentionRate: 0.5},
	}
	got := latestByKeyword(rs)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for _, r := range got {
		if r.KeywordID == "k1" && r.MentionRate != 0.9 {
			t.Errorf("k1 应保留最新一条 (0.9), got %v", r.MentionRate)
		}
	}
}

func TestPrevHealthTotal(t *testing.T) {
	now := time.Now()
	bts := []brandTrend{
		{
			BrandID: "b1",
			Trend: []entity.MonitoringResult{
				{ProbedAt: now.Add(-10 * 24 * time.Hour), MentionRate: 0.2, Sentiment: "neutral"}, // 上一期窗口内最新
				{ProbedAt: now.Add(-1 * 24 * time.Hour), MentionRate: 0.8, Sentiment: "positive"}, // 本期
			},
		},
	}
	prev := prevHealthTotal(bts, 0, 0, now.Add(-7*24*time.Hour))
	if prev == nil {
		t.Fatal("有历史时 PrevTotal 不应为 nil")
	}
	// 上一期：coverage=20, sentiment=50, firstPick=0, asset=0, source=0 → 20*0.4+50*0.2=18
	if *prev != 18 {
		t.Errorf("PrevTotal = %v, want 18", *prev)
	}

	// 无历史（全部监测都在 7 天内）→ nil
	bts[0].Trend = bts[0].Trend[1:]
	if p := prevHealthTotal(bts, 0, 0, now.Add(-7*24*time.Hour)); p != nil {
		t.Errorf("无历史应返回 nil, got %v", *p)
	}
}

// Report 集成测试：fakes 装配 → 报告含品牌级分值与竞品对标（三处展示位统一口径）。
type healthFakeContentRepo struct {
	port.OptimizedContentRepository
	contents []entity.OptimizedContent
}

func (f *healthFakeContentRepo) ListByBrand(_ context.Context, _, _ string) ([]entity.OptimizedContent, error) {
	return f.contents, nil
}

type healthFakeResultRepo struct {
	port.MonitoringResultRepository
	trends map[string][]entity.MonitoringResult
	latest []entity.MonitoringResult
}

func (f *healthFakeResultRepo) Trend(_ context.Context, _, brandID string, _ int) ([]entity.MonitoringResult, error) {
	return f.trends[brandID], nil
}

func (f *healthFakeResultRepo) LatestByTenant(_ context.Context, _ string) ([]entity.MonitoringResult, error) {
	return f.latest, nil
}

func TestHealthReportBrandTotals(t *testing.T) {
	now := time.Now()
	brandRepo := &monFakeBrandRepo{brands: map[string]entity.Brand{
		"b1": {ID: "b1", TenantID: "t1", Name: "品牌一"},
	}}
	resultRepo := &healthFakeResultRepo{
		trends: map[string][]entity.MonitoringResult{
			"b1": {
				{ProbedAt: now.Add(-10 * 24 * time.Hour), MentionRate: 0.2, SampleCount: 5},
				{ProbedAt: now, MentionRate: 0.8, SampleCount: 5, FirstPickCount: 2, AvgPosition: 1, Sentiment: "positive", SelfSourceCount: 1},
			},
		},
		latest: []entity.MonitoringResult{
			{KeywordID: "k1", ProbedAt: now, MentionRate: 0.8, CompetitorRates: map[string]float64{"竞品A": 0.9}},
		},
	}
	contentRepo := &healthFakeContentRepo{contents: []entity.OptimizedContent{
		{Status: "published"}, {Status: "draft"},
	}}
	uc := NewHealthUseCase(brandRepo, resultRepo, contentRepo)
	report, err := uc.Report(context.Background(), "t1")
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if len(report.Brands) != 1 || report.Brands[0].BrandID != "b1" {
		t.Fatalf("品牌级分值缺失: %+v", report.Brands)
	}
	if report.Brands[0].Total != report.Total {
		t.Errorf("单品牌租户的品牌分应与总分一致: %v vs %v", report.Brands[0].Total, report.Total)
	}
	if report.Competitor.Size != 1 || report.Competitor.Threats[0].Name != "竞品A" {
		t.Errorf("竞品对标缺失: %+v", report.Competitor)
	}
	if report.PrevTotal == nil {
		t.Error("有 10 天前历史时 PrevTotal 不应为 nil")
	}
	if report.Indicators.FirstPickRate != 40 {
		t.Errorf("FirstPickRate = %v, want 40（2/5 采样）", report.Indicators.FirstPickRate)
	}
}
