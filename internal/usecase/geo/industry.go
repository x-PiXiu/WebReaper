package geo

import (
	"context"
	"math"
	"net/url"
	"sort"
	"strings"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ============ 行业全景看板（admin，v3 P2）============

// IndustryUseCase 跨商户聚合行业维度视图：行业能见度榜单、品牌美誉度排行、信源域名排行。
// 数据源：全平台品牌（行业字段）+ 最近监测结果（情感/信源已就绪）——无新采集成本，
// 是平台做行业报告与销售素材的卖点（对齐 Geowise 行业全景看板）。
type IndustryUseCase struct {
	brandRepo  port.BrandRepository
	resultRepo port.MonitoringResultRepository
}

func NewIndustryUseCase(br port.BrandRepository, rr port.MonitoringResultRepository) *IndustryUseCase {
	return &IndustryUseCase{brandRepo: br, resultRepo: rr}
}

// IndustryVisibility 行业能见度榜条目。
type IndustryVisibility struct {
	Industry   string
	AvgRate    float64 // 0-100（该行业品牌的平均提及率）
	BrandCount int
}

// BrandReputation 品牌美誉度榜条目。
type BrandReputation struct {
	BrandName    string
	Industry     string
	PositiveRate float64 // 0-100（最新结果中正面占比）
	SampleCount  int     // 参与聚合的最新结果数（<2 不上榜——单采样不可信）
}

// SourceRank 信源域名榜条目。
type SourceRank struct {
	Domain string
	Count  int
}

// IndustryOverview 行业全景看板输出。
type IndustryOverview struct {
	Industries []IndustryVisibility // 按平均提及率降序
	Reputation []BrandReputation    // 按正面占比降序（top 10）
	TopSources []SourceRank         // 按被引次数降序（top 10）
}

// Overview 产出行业全景（admin 旁路：跨租户聚合）。
func (uc *IndustryUseCase) Overview(ctx context.Context) (IndustryOverview, error) {
	brands, err := uc.brandRepo.ListAll(ctx)
	if err != nil {
		return IndustryOverview{}, err
	}
	results, err := uc.resultRepo.ListRecent(ctx, 500)
	if err != nil {
		return IndustryOverview{}, err
	}
	return industryOverviewFrom(brands, results), nil
}

// industryOverviewFrom 聚合纯函数（表驱动可测）。
// 口径：每品牌取"每关键词最新一条"（与竞品对标/健康报告同口径）。
func industryOverviewFrom(brands []entity.Brand, results []entity.MonitoringResult) IndustryOverview {
	out := IndustryOverview{
		Industries: []IndustryVisibility{},
		Reputation: []BrandReputation{},
		TopSources: []SourceRank{},
	}

	// 按品牌分组，每组内每关键词保留最新一条
	type brandAgg struct {
		rateSum     float64
		rateCount   int
		positive    int
		sentimented int
		sources     []string
	}
	byBrand := make(map[string]*brandAgg)
	for _, r := range results {
		agg := byBrand[r.BrandID]
		if agg == nil {
			agg = &brandAgg{}
			byBrand[r.BrandID] = agg
		}
		agg.sources = append(agg.sources, r.Sources...)
	}
	// 每关键词最新一条（情感/提及率聚合基）
	latest := latestByKeyword(results)
	for _, r := range latest {
		agg, ok := byBrand[r.BrandID]
		if !ok {
			continue
		}
		agg.rateSum += r.MentionRate
		agg.rateCount++
		if s := entity.NormalizeSentiment(r.Sentiment); s == "positive" || s == "negative" {
			agg.sentimented++
			if s == "positive" {
				agg.positive++
			}
		}
	}

	// 行业能见度（全量品牌视角：无监测数据的行业也上榜——均值 0、品牌数可见，
	// 平台运营能看到"哪些行业还没跑出数据"）+ 品牌美誉度（仅 ≥2 条情感采样）
	industryAgg := make(map[string]*IndustryVisibility)
	var reputations []BrandReputation
	for _, b := range brands {
		industry := strings.TrimSpace(b.Industry)
		if industry == "" {
			industry = "未分类"
		}
		iv := industryAgg[industry]
		if iv == nil {
			iv = &IndustryVisibility{Industry: industry}
			industryAgg[industry] = iv
		}
		iv.BrandCount++
		agg := byBrand[b.ID]
		if agg == nil {
			continue
		}
		if agg.rateCount > 0 {
			iv.AvgRate += agg.rateSum / float64(agg.rateCount)
		}
		if agg.sentimented >= 2 {
			reputations = append(reputations, BrandReputation{
				BrandName:    b.Name,
				Industry:     industry,
				PositiveRate: math.Round(float64(agg.positive) / float64(agg.sentimented) * 100),
				SampleCount:  agg.sentimented,
			})
		}
	}
	for _, iv := range industryAgg {
		if iv.BrandCount > 0 {
			iv.AvgRate = math.Round(iv.AvgRate / float64(iv.BrandCount) * 100)
		}
		out.Industries = append(out.Industries, *iv)
	}
	sort.SliceStable(out.Industries, func(i, j int) bool { return out.Industries[i].AvgRate > out.Industries[j].AvgRate })
	sort.SliceStable(reputations, func(i, j int) bool { return reputations[i].PositiveRate > reputations[j].PositiveRate })
	if len(reputations) > 10 {
		reputations = reputations[:10]
	}
	out.Reputation = reputations

	// 信源域名榜（AI 回答引用的来源域名——哪些信源在真正生效）
	domainCount := make(map[string]int)
	for _, agg := range byBrand {
		for _, s := range agg.sources {
			if d := sourceDomain(s); d != "" {
				domainCount[d]++
			}
		}
	}
	for d, n := range domainCount {
		out.TopSources = append(out.TopSources, SourceRank{Domain: d, Count: n})
	}
	sort.SliceStable(out.TopSources, func(i, j int) bool { return out.TopSources[i].Count > out.TopSources[j].Count })
	if len(out.TopSources) > 10 {
		out.TopSources = out.TopSources[:10]
	}
	return out
}

// sourceDomain 提取来源的域名（F2-3 净化：URL 取 host；非 URL 文本匹配平台/媒体
// 白名单返回标准名；未命中归"其他来源"——不再 12 字符截断，杜绝"搜索到的…"类垃圾条目上榜）。
func sourceDomain(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		if u, err := url.Parse(s); err == nil && u.Host != "" {
			return u.Host
		}
	}
	// 常见平台/媒体名（含别名）——命中返回标准展示名
	platforms := []struct{ match, name string }{
		{"大众点评", "大众点评"}, {"点评", "大众点评"}, {"美团", "美团"}, {"饿了么", "饿了么"},
		{"知乎", "知乎"}, {"小红书", "小红书"}, {"抖音", "抖音"}, {"微博", "微博"},
		{"b站", "哔哩哔哩"}, {"bilibili", "哔哩哔哩"}, {"微信", "微信公众号"}, {"公众号", "微信公众号"},
		{"百度", "百度"}, {"央视", "央视网"}, {"cctv", "央视网"}, {"bbc", "BBC"}, {"tripadvisor", "TripAdvisor"},
		{"马蜂窝", "马蜂窝"}, {"携程", "携程"}, {"豆瓣", "豆瓣"},
	}
	for _, p := range platforms {
		if strings.Contains(s, p.match) {
			return p.name
		}
	}
	return "其他来源"
}
