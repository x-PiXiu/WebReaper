package geo

import (
	"testing"
	"time"

	"webreaper/internal/domain/entity"
)

// 行业全景聚合纯函数测试（v3 P2：admin 看板口径——每品牌"每关键词最新一条"）。

func TestIndustryOverviewFrom(t *testing.T) {
	now := time.Now()
	brands := []entity.Brand{
		{ID: "b1", TenantID: "t1", Name: "品牌甲", Industry: "装修"},
		{ID: "b2", TenantID: "t2", Name: "品牌乙", Industry: "装修"},
		{ID: "b3", TenantID: "t3", Name: "品牌丙", Industry: ""}, // 未填行业 → 未分类
	}
	results := []entity.MonitoringResult{
		// b1：两个关键词各两条（旧+新）——聚合基只取最新
		{BrandID: "b1", KeywordID: "k1", ProbedAt: now.Add(-time.Hour), MentionRate: 0.2, Sentiment: "neutral", Sources: []string{"https://a.example.com/x", "知乎"}},
		{BrandID: "b1", KeywordID: "k1", ProbedAt: now, MentionRate: 0.8, Sentiment: "positive", Sources: []string{"https://a.example.com/y"}},
		{BrandID: "b1", KeywordID: "k2", ProbedAt: now, MentionRate: 0.6, Sentiment: "positive", Sources: []string{"https://a.example.com/y"}},
		// b2：单关键词（sentimented=1 < 2 → 不上美誉度榜）
		{BrandID: "b2", KeywordID: "k3", ProbedAt: now, MentionRate: 0.4, Sentiment: "negative", Sources: []string{"https://b.example.com/z"}},
		// b3：无监测结果（只出现在行业榜，品牌数计入、均值 0）
	}
	ov := industryOverviewFrom(brands, results)

	// 行业榜：装修 = (0.7 + 0.4)/2 = 55；未分类 = 0（b3 无数据）
	if len(ov.Industries) != 2 {
		t.Fatalf("行业数 = %d, want 2: %+v", len(ov.Industries), ov.Industries)
	}
	zx := ov.Industries[0]
	if zx.Industry != "装修" || zx.AvgRate != 55 || zx.BrandCount != 2 {
		t.Errorf("装修行业 = %+v, want AvgRate 55/2 品牌", zx)
	}

	// 美誉度榜：b1 两条最新全正面 = 100%（b2 单采样不上榜）
	if len(ov.Reputation) != 1 {
		t.Fatalf("美誉度榜条数 = %d, want 1: %+v", len(ov.Reputation), ov.Reputation)
	}
	if ov.Reputation[0].BrandName != "品牌甲" || ov.Reputation[0].PositiveRate != 100 || ov.Reputation[0].SampleCount != 2 {
		t.Errorf("美誉度[0] = %+v, want 品牌甲/100 分/2 采样", ov.Reputation[0])
	}

	// 信源域名榜：a.example.com ×3、b.example.com ×1、知乎 ×1（按次数降序）
	if len(ov.TopSources) != 3 {
		t.Fatalf("信源域名数 = %d, want 3: %+v", len(ov.TopSources), ov.TopSources)
	}
	if ov.TopSources[0].Domain != "a.example.com" || ov.TopSources[0].Count != 3 {
		t.Errorf("TopSources[0] = %+v, want a.example.com/3", ov.TopSources[0])
	}
}

func TestSourceDomain(t *testing.T) {
	cases := map[string]string{
		"https://A.example.com/path?q=1": "a.example.com",
		"知乎":                            "知乎",
		"":                              "",
	}
	for in, want := range cases {
		if got := sourceDomain(in); got != want {
			t.Errorf("sourceDomain(%q) = %q, want %q", in, got, want)
		}
	}
}
