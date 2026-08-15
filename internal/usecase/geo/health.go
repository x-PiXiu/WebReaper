package geo

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ============ GEO 健康分聚合（单一事实源）============

// HealthUseCase 聚合监测/内容数据，产出 GEO 健康报告（总分+五指数+竞品对标+环比+品牌级分值）。
//
// 设计（v3 归位）：健康分是应用级业务规则，此前在前端 geoHealth.ts 被三处以不同
// 口径各自合成——列表徽章（不含内容资产）/工作区头部/工作台卡片数字不同（用户当 bug 报）、
// 每页逐品牌 N+1 扇出、口径随页面漂移。归位后端后：
//   - 一次请求出全量口径，前端只消费（geoHealth.ts 降级为接口不可用时的兜底）
//   - 品牌级健康分与总分同口径（含内容资产）——三处展示位统一
//   - 竞品对标与 GET /geo/monitor-results 同源同口径（每关键词最新一条）
type HealthUseCase struct {
	brandRepo   port.BrandRepository
	resultRepo  port.MonitoringResultRepository
	contentRepo port.OptimizedContentRepository
	// cache 可选读缓存（R2 性能：驾驶舱每次进入的 逐品牌趋势+内容扇出 全量打库——
	// 缓存 60s+抖动，写操作后主动失效；nil=直查）。三防（抖动/空值/singleflight）在适配器。
	cache port.CacheStore
}

func NewHealthUseCase(br port.BrandRepository, rr port.MonitoringResultRepository, cr port.OptimizedContentRepository) *HealthUseCase {
	return &HealthUseCase{brandRepo: br, resultRepo: rr, contentRepo: cr}
}

// SetCache 注入读缓存（可选；Redis 实现）。
func (uc *HealthUseCase) SetCache(c port.CacheStore) {
	if c != nil {
		uc.cache = c
	}
}

// healthCacheTTL 缓存基准 TTL（适配器加抖动防雪崩；前端 staleTime 同为 60s——
// 服务端缓存把 N+1 扇出收敛为每租户每分钟最多一次全量计算）。
const healthCacheTTL = 60 * time.Second

// monitorResultsCacheTTL 监测结果列表缓存 TTL（矩阵/总览/引用 Tab 共用——
// 500 条扫描+Go 层去重是最重读路径；写后主动失效消除陈旧窗口）。
const monitorResultsCacheTTL = 60 * time.Second

// CachedLatestByTenant 带缓存的监测结果列表（矩阵页/总览/引用 Tab 的共享读侧）。
// R2 性能：LatestByTenant 每次全量扫描 500 条 + Go 层 (keyword,engine) 去重——
// 多 Tab 同帧触发时收敛为一次查询。写侧（Monitor 用例）主动 Del 失效。
func (uc *HealthUseCase) CachedLatestByTenant(ctx context.Context, tenantID string) ([]entity.MonitoringResult, error) {
	if uc.cache == nil {
		return uc.resultRepo.LatestByTenant(ctx, tenantID)
	}
	cached, err := uc.cache.GetOrCompute(ctx, MonitorResultsCacheKey(tenantID), monitorResultsCacheTTL, func(ctx context.Context) (string, error) {
		rs, err := uc.resultRepo.LatestByTenant(ctx, tenantID)
		if err != nil {
			return "", err
		}
		b, _ := json.Marshal(rs)
		return string(b), nil
	})
	if err != nil || cached == "" {
		return uc.resultRepo.LatestByTenant(ctx, tenantID)
	}
	var rs []entity.MonitoringResult
	if json.Unmarshal([]byte(cached), &rs) != nil {
		return uc.resultRepo.LatestByTenant(ctx, tenantID)
	}
	return rs, nil
}

// 健康分权重（与行业驾驶舱口径一致：提及覆盖为主、语义维度次之、资产/信源补足）。
const (
	wCoverage  = 0.4
	wSentiment = 0.2
	wFirstPick = 0.2
	wAsset     = 0.1
	wSource    = 0.1
)

// HealthIndicators 五指数（均 0-100）。
type HealthIndicators struct {
	MentionCoverage float64 // 平均提及率（无监测数据的品牌按 0 计入分母——如实反映"还没开始"）
	SentimentScore  float64 // 正面(+1)/负面(-1)/中性(0) 均值映射到 0-100
	FirstPickRate   float64 // 首选率：FirstPickCount/SampleCount；旧数据退回 avg_position==1 近似
	ContentAsset    float64 // 已发布×15 + 未发布×5（封顶 100）
	SourceIntegrity float64 // 最新结果中被自营站引用（SelfSourceCount>0）的占比
}

// CompetitorThreat 竞品威胁榜条目。
type CompetitorThreat struct {
	Name      string
	AvgRate   float64 // 0-100
	Sentiment string  // positive/negative/""（中性或无数据）
}

// CompetitorHealth 竞品对标（自家 vs 竞品的坐标系）。
type CompetitorHealth struct {
	SelfAvg float64 // 0-1
	CompAvg float64 // 0-1
	GapPct  float64 // 百分点（+领先/-落后）
	Size    int
	Threats []CompetitorThreat // 按 AvgRate 降序
}

// BrandHealth 品牌级健康分（与总分同口径——三处展示位统一的数据源）。
type BrandHealth struct {
	BrandID        string
	BrandName      string
	Total          float64
	AvgMentionRate float64 // 0-1
}

// HealthReport 租户级 GEO 健康报告。
type HealthReport struct {
	Total      float64
	Indicators HealthIndicators
	PrevTotal  *float64 // 上一期（7 天前窗口口径）总分；无历史为 nil
	Competitor CompetitorHealth
	Brands     []BrandHealth
}

// Report 产出租户健康报告（一次聚合，多页消费）。带可选读缓存。
func (uc *HealthUseCase) Report(ctx context.Context, tenantID string) (HealthReport, error) {
	if uc.cache != nil {
		cached, err := uc.cache.GetOrCompute(ctx, "health-report:"+tenantID, healthCacheTTL, func(ctx context.Context) (string, error) {
			r, err := uc.compute(ctx, tenantID)
			if err != nil {
				return "", err
			}
			b, _ := json.Marshal(r)
			return string(b), nil
		})
		if err == nil && cached != "" {
			var r HealthReport
			if json.Unmarshal([]byte(cached), &r) == nil {
				return r, nil
			}
		}
		// 缓存未命中/损坏/计算错误 → 直查（GetOrCompute 出错时已透传 fetch 错误，此处兜底重算）
	}
	return uc.compute(ctx, tenantID)
}

// compute 真实聚合（缓存 miss 时执行）。
func (uc *HealthUseCase) compute(ctx context.Context, tenantID string) (HealthReport, error) {
	brands, err := uc.brandRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return HealthReport{}, err
	}

	// 内容资产（各品牌一次扇出；品牌级计数同时收集——总分与品牌分同源）
	type contentCounts struct{ total, published int }
	tenantCounts := contentCounts{}
	brandCounts := make(map[string]contentCounts, len(brands))
	for _, b := range brands {
		cs, err := uc.contentRepo.ListByBrand(ctx, tenantID, b.ID)
		if err != nil {
			continue // 单品牌内容查询失败不阻断报告
		}
		cc := contentCounts{total: len(cs)}
		for _, c := range cs {
			if c.Status == "published" {
				cc.published++
			}
		}
		brandCounts[b.ID] = cc
		tenantCounts.total += cc.total
		tenantCounts.published += cc.published
	}

	// 各品牌趋势 + 最新一条（情感/首选/信源的聚合基——与原前端 geoHealth 口径一致）
	bts := make([]brandTrend, 0, len(brands))
	var allLatest []*entity.MonitoringResult
	for _, b := range brands {
		trend, tErr := uc.resultRepo.Trend(ctx, tenantID, b.ID, 30)
		if tErr != nil {
			trend = nil
		}
		bt := summarizeBrandTrend(b.ID, b.Name, trend)
		bts = append(bts, bt)
		if bt.Latest != nil {
			allLatest = append(allLatest, bt.Latest)
		}
	}

	// 总分（租户口径：品牌平均提及率 → 五指数 → 加权）
	var avgRate float64
	if len(bts) > 0 {
		var s float64
		for _, bt := range bts {
			s += bt.AvgRate
		}
		avgRate = s / float64(len(bts))
	}
	indicators := computeHealthIndicators(avgRate, allLatest, tenantCounts.total, tenantCounts.published)
	report := HealthReport{
		Total:      computeHealthTotal(indicators),
		Indicators: indicators,
		Competitor: CompetitorHealth{Threats: []CompetitorThreat{}},
		Brands:     make([]BrandHealth, 0, len(bts)),
	}

	// 上一期（7 天前窗口）：每品牌取窗口内最新一条；内容资产沿用当前（"变化可见"叙事）
	report.PrevTotal = prevHealthTotal(bts, tenantCounts.total, tenantCounts.published, time.Now().Add(-7*24*time.Hour))

	// 竞品对标（与 GET /geo/monitor-results 同源：每关键词最新一条）
	if latestResults, lErr := uc.resultRepo.LatestByTenant(ctx, tenantID); lErr == nil {
		report.Competitor = competitorHealthFrom(latestByKeyword(latestResults))
	}

	// 品牌级健康分（含内容资产——口径与总分统一）
	for _, bt := range bts {
		cc := brandCounts[bt.BrandID]
		var brandLatest []*entity.MonitoringResult
		if bt.Latest != nil {
			brandLatest = append(brandLatest, bt.Latest)
		}
		bi := computeHealthIndicators(bt.AvgRate, brandLatest, cc.total, cc.published)
		report.Brands = append(report.Brands, BrandHealth{
			BrandID:        bt.BrandID,
			BrandName:      bt.Name,
			Total:          computeHealthTotal(bi),
			AvgMentionRate: bt.AvgRate,
		})
	}
	return report, nil
}

// brandTrend 品牌趋势聚合中间态。
type brandTrend struct {
	BrandID string
	Name    string
	Trend   []entity.MonitoringResult
	AvgRate float64              // 品牌平均提及率（trend 均值；无数据=0）
	Latest  *entity.MonitoringResult // 最新一条（probed_at 最大）
}

// summarizeBrandTrend 趋势摘要（纯函数）。
func summarizeBrandTrend(brandID, name string, trend []entity.MonitoringResult) brandTrend {
	bt := brandTrend{BrandID: brandID, Name: name, Trend: trend}
	var sum float64
	for _, r := range trend {
		sum += r.MentionRate
	}
	if len(trend) > 0 {
		bt.AvgRate = sum / float64(len(trend))
	}
	bt.Latest = latestOf(trend)
	return bt
}

// computeHealthIndicators 五指数合成（纯函数——表驱动测试）。
func computeHealthIndicators(avgRate float64, latest []*entity.MonitoringResult, totalContents, publishedContents int) HealthIndicators {
	coverage := math.Round(avgRate * 100)

	sentSum := 0.0
	for _, r := range latest {
		switch entity.NormalizeSentiment(r.Sentiment) {
		case "positive":
			sentSum++
		case "negative":
			sentSum--
		}
	}
	sentiment := 0.0
	if len(latest) > 0 {
		sentiment = math.Round((sentSum/float64(len(latest))+1)*50)
	}

	// 首选率（F1-2 可信度门槛）：真实计数需 ≥3 次采样；旧数据（无计数）需 ≥3 条有位次结果
	// 才回退近似——采样不足返回 -1（前端显示"积累中"）。修复：1 条 avg_position==1 命中
	// 即显示 100%，与提及率并排自相矛盾、伤数字信任。
	var sampleSum, pickSum, ranked, firstPos int
	for _, r := range latest {
		sampleSum += r.SampleCount
		pickSum += r.FirstPickCount
		if r.AvgPosition > 0 {
			ranked++
			if r.AvgPosition == 1 {
				firstPos++
			}
		}
	}
	firstPick := -1.0
	if pickSum > 0 && sampleSum >= 3 {
		firstPick = math.Round(float64(pickSum) / float64(sampleSum) * 100)
	} else if pickSum == 0 && ranked >= 3 {
		firstPick = math.Round(float64(firstPos) / float64(ranked) * 100)
	}

	asset := float64(publishedContents*15+(totalContents-publishedContents)*5)
	if asset > 100 {
		asset = 100
	}

	cited := 0
	for _, r := range latest {
		if r.SelfSourceCount > 0 {
			cited++
		}
	}
	source := 0.0
	if len(latest) > 0 {
		source = math.Round(float64(cited) / float64(len(latest)) * 100)
	}

	return HealthIndicators{
		MentionCoverage: coverage,
		SentimentScore:  sentiment,
		FirstPickRate:   firstPick,
		ContentAsset:    asset,
		SourceIntegrity: source,
	}
}

// computeHealthTotal 加权合成总分（纯函数）。FirstPickRate<0（积累中）按 0 计入——
// 与"无数据=0"语义一致，不为采样不足惩罚总分。
func computeHealthTotal(i HealthIndicators) float64 {
	fp := i.FirstPickRate
	if fp < 0 {
		fp = 0
	}
	return math.Round(
		i.MentionCoverage*wCoverage +
			i.SentimentScore*wSentiment +
			fp*wFirstPick +
			i.ContentAsset*wAsset +
			i.SourceIntegrity*wSource,
	)
}

// prevHealthTotal 上一期总分：每品牌取 cutoff 前窗口内最新一条重算（无历史返回 nil）。
func prevHealthTotal(bts []brandTrend, totalContents, publishedContents int, cutoff time.Time) *float64 {
	var prevLatest []*entity.MonitoringResult
	var avgSum float64
	hasHistory := false
	for _, bt := range bts {
		var prevLatestOfBrand *entity.MonitoringResult
		for i := range bt.Trend {
			r := bt.Trend[i]
			if !r.ProbedAt.After(cutoff) && (prevLatestOfBrand == nil || r.ProbedAt.After(prevLatestOfBrand.ProbedAt)) {
				prevLatestOfBrand = &bt.Trend[i]
			}
		}
		if prevLatestOfBrand != nil {
			hasHistory = true
			avgSum += prevLatestOfBrand.MentionRate
			prevLatest = append(prevLatest, prevLatestOfBrand)
		} // 无历史品牌按 0 计入平均分母（与当前口径一致）
	}
	if !hasHistory {
		return nil
	}
	avg := 0.0
	if len(bts) > 0 {
		avg = avgSum / float64(len(bts))
	}
	t := computeHealthTotal(computeHealthIndicators(avg, prevLatest, totalContents, publishedContents))
	return &t
}

// latestOf 最新一条（probed_at 最大）。
func latestOf(rs []entity.MonitoringResult) *entity.MonitoringResult {
	var best *entity.MonitoringResult
	for i := range rs {
		if best == nil || rs[i].ProbedAt.After(best.ProbedAt) {
			best = &rs[i]
		}
	}
	return best
}

// latestByKeyword 每关键词取最新一条（竞品对标的聚合基——与前端旧口径一致）。
func latestByKeyword(rs []entity.MonitoringResult) []entity.MonitoringResult {
	best := make(map[string]entity.MonitoringResult)
	var order []string
	for _, r := range rs {
		if cur, ok := best[r.KeywordID]; !ok {
			best[r.KeywordID] = r
			order = append(order, r.KeywordID)
		} else if r.ProbedAt.After(cur.ProbedAt) {
			best[r.KeywordID] = r
		}
	}
	out := make([]entity.MonitoringResult, 0, len(order))
	for _, k := range order {
		out = append(out, best[k])
	}
	return out
}

// competitorHealthFrom 竞品对标聚合（纯函数）：自家均值 vs 各竞品均值 + 威胁榜。
func competitorHealthFrom(latest []entity.MonitoringResult) CompetitorHealth {
	type compAgg struct {
		total float64
		count int
		sents map[string]int
	}
	selfSum := 0.0
	compMap := make(map[string]*compAgg)
	var order []string
	for _, r := range latest {
		selfSum += r.MentionRate
		for name, rate := range r.CompetitorRates {
			agg := compMap[name]
			if agg == nil {
				agg = &compAgg{sents: make(map[string]int)}
				compMap[name] = agg
				order = append(order, name)
			}
			agg.total += rate
			agg.count++
			if s, ok := r.CompetitorSentiments[name]; ok && s != "" {
				agg.sents[entity.NormalizeSentiment(s)]++
			}
		}
	}
	ch := CompetitorHealth{Threats: []CompetitorThreat{}}
	if len(latest) > 0 {
		ch.SelfAvg = selfSum / float64(len(latest))
	}
	for _, name := range order {
		agg := compMap[name]
		t := CompetitorThreat{Name: name, Sentiment: majoritySentimentOf(agg.sents)}
		if agg.count > 0 {
			t.AvgRate = math.Round(agg.total/float64(agg.count)*1000) / 10
		}
		ch.Threats = append(ch.Threats, t)
	}
	sort.SliceStable(ch.Threats, func(i, j int) bool { return ch.Threats[i].AvgRate > ch.Threats[j].AvgRate })
	if len(ch.Threats) > 0 {
		var s float64
		for _, t := range ch.Threats {
			s += t.AvgRate / 100
		}
		ch.CompAvg = s / float64(len(ch.Threats))
	}
	ch.Size = len(ch.Threats)
	ch.GapPct = math.Round((ch.SelfAvg-ch.CompAvg)*1000) / 10
	return ch
}

// majoritySentimentOf 三值多数投票：得票严格多于其他两项者胜出；平票返回 ""（中性）。
func majoritySentimentOf(votes map[string]int) string {
	pos, neg, neu := votes["positive"], votes["negative"], votes["neutral"]
	if pos > neg && pos > neu {
		return "positive"
	}
	if neg > pos && neg > neu {
		return "negative"
	}
	return ""
}
