package geo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ============ 附近同行用例（现实世界双榜）============

// NearbyUseCase 编排"附近同行对比排名"：
//   地图榜（现实世界）：以门店位置为中心，POI 周边搜索同行业门店（距离/评分）
//   AI 榜（虚拟世界）：AI 榜单探查结果——附近同行被 AI 提及的情况（谁在 AI 引擎里更响）
// 双榜对照让老板同时看到"物理距离上的对手"和"AI 声量上的对手"。
type NearbyUseCase struct {
	brandRepo   port.BrandRepository
	storeRepo   port.StoreLocationRepository
	resultRepo  port.MonitoringResultRepository
	probeRepo   port.AIRankProbeRepository // 可选；AI 榜单探查结果（v2：AI 榜真实数据源）
	searcher    port.POISearcher // 可选；nil/未配置时降级只显示 AI 榜
	measurer    port.DistanceMeasurer // 可选；P2 驾车耗时（未配置时只显示直线距离）
	quotaGate   port.QuotaStore  // 配额检查门（可选；X-01：nearby 场景，地图 API 有成本）
	usageRec    port.UsageRecorder // 可选；nearby 场景计量（非 LLM 调用，配额计数用）
}

func NewNearbyUseCase(brandRepo port.BrandRepository, storeRepo port.StoreLocationRepository, resultRepo port.MonitoringResultRepository) *NearbyUseCase {
	return &NearbyUseCase{brandRepo: brandRepo, storeRepo: storeRepo, resultRepo: resultRepo}
}

// SetAIRankProbeRepo 注入 AI 榜单探查结果仓储（可选；v2：AI 榜数据源升级——注入后
// GetRanking 的 AI 榜优先用探查结果（全量补位），未注入/无数据回落监测竞品提及率）。
func (uc *NearbyUseCase) SetAIRankProbeRepo(r port.AIRankProbeRepository) {
	if r != nil {
		uc.probeRepo = r
	}
}

// SetPOISearcher 注入周边搜索器（可选；未注入/未配置时降级只显示 AI 榜）。
func (uc *NearbyUseCase) SetPOISearcher(s port.POISearcher) {
	if s != nil {
		uc.searcher = s
	}
}

// SetDistanceMeasurer 注入距离测量器（可选；P2——地图榜"驾车耗时"，未注入时只显示直线距离）。
func (uc *NearbyUseCase) SetDistanceMeasurer(m port.DistanceMeasurer) {
	if m != nil {
		uc.measurer = m
	}
}

// SetQuotaGate 注入配额检查门（可选；X-01 经济系统收口——nearby 场景按次限额）。
// 地图 POI 搜索是第三方 API 调用（有成本），超限返回 ErrQuotaExceeded → HTTP 402。
func (uc *NearbyUseCase) SetQuotaGate(g port.QuotaStore) {
	if g != nil {
		uc.quotaGate = g
	}
}

// SetUsageRecorder 注入用量记录器（可选；nearby 场景配额计数数据源）。
// 地图搜索不烧 LLM token，记一条 TotalTokens=0 的 usage 作为"业务动作计数"。
func (uc *NearbyUseCase) SetUsageRecorder(r port.UsageRecorder) {
	if r != nil {
		uc.usageRec = r
	}
}

// NearbyRanking 附近同行双榜视图（API 契约由 handler 转换）。
type NearbyRanking struct {
	Store       entity.StoreLocation // 主门店（无门店时 IsValid()==false，前端提示先建门店）
	MapRanking  []MapRankEntry       // 地图榜：按距离升序
	AIRanking   []AIRankEntry        // AI 榜：按提及率降序（含未上榜门店——全量补位）
	OwnRate     float64              // 自己的 AI 提及率（最近监测均值；无数据为 -1）
	MapAvailable bool                // 地图服务是否可用（false=未配置/搜索失败，降级提示）
	SearchKeyword string             // 实际使用的搜索词（调试/展示用）
	// ---- AI 榜来源与覆盖（v2：AI 榜单探查）----
	AIRankFromProbe bool   // true=AI 榜来自"AI 榜单探查"（真实搜索+名单归因）；false=旧逻辑（监测竞品提及率）
	AIRankProbedAt  string // 探查时间（展示"更新于 xx"）
	AIRankTotal     int    // 附近同行总数（地图榜同源）
	AIRankMentioned int    // 被 AI 提及的门店数（上榜率 = mentioned/total）
	AIRankSample    int    // 探查采样次数（问法数）
}

// MapRankEntry 地图榜条目。
type MapRankEntry struct {
	Name       string
	Address    string
	DistanceM  int     // 距门店距离（米）
	Rating     float64 // 评分（0=无数据）
	Category   string
	OpenStatus string
	Lat, Lng   float64
	// ---- 门店卡扩展（v5 show_fields=business,navi；无数据留空）----
	CityName      string // 所属城市
	AdName        string // 所属区县
	Cost          string // 人均消费
	BusinessArea  string // 所属商圈
	OpenTimeToday string // 今日营业时间
	Tag           string // 特色菜（美食 POI）
	Tel           string // 联系电话
	EntrLocation  string // 入口经纬度（导航到达点）
	PhotoURL      string // 首张照片
	// ---- 驾车耗时（P2 距离测量补全；0=未测得）----
	DriveDistanceM  int // 驾车距离（米）
	DriveDurationSec int // 驾车耗时（秒）
}

// AIRankEntry AI 榜条目。
type AIRankEntry struct {
	Name       string  // 门店名
	Rate       float64 // 提及率（0-1）
	SampleCnt  int     // 统计的采样次数（数据量）
	Mentioned  bool    // 是否被 AI 提及（false=未上榜——全量补位展示）
	MentionCnt int     // 提及次数（探查口径）
}

// GetRanking 生成品牌附近同行双榜。
//
// 流程：取品牌 + 主门店 → 无门店则报错提示先建门店 → 周边搜索（品牌名 + 核心卖点 +
// 手动竞品名；types 非空时额外按 POI 类型扫描，如 050000 餐饮大类）→ 驾车耗时补全 →
// 聚合最新监测结果里的竞品提及率 → 双榜返回。
// 任一部分失败都不阻断整体：地图服务不可用 → 只返回 AI 榜（MapAvailable=false）。
// types：可选 POI 分类编码（六位，多个用 | 分隔）——按类目扫描不依赖名称命中；
// 前端"附近同行"页可传（如餐饮=050000）。
func (uc *NearbyUseCase) GetRanking(ctx context.Context, tenantID, brandID, types string) (NearbyRanking, error) {
	// 配额检查（X-01：nearby 场景按次限额——地图 POI API 有成本；超限 402）
	if uc.quotaGate != nil {
		if err := uc.quotaGate.Check(ctx, tenantID, "nearby"); err != nil {
			return NearbyRanking{}, err
		}
	}
	// 业务动作计数（配额数据源：非 LLM 调用，TotalTokens=0；失败不阻断）
	if uc.usageRec != nil && tenantID != "" {
		_ = uc.usageRec.RecordUsage(ctx, entity.UsageRecord{TenantID: tenantID, Scene: "nearby"})
	}

	brand, err := uc.brandRepo.FindByID(ctx, tenantID, brandID)
	if err != nil {
		return NearbyRanking{}, fmt.Errorf("品牌不存在: %w", err)
	}
	store, err := uc.storeRepo.FindPrimaryByBrand(ctx, brandID)
	if err != nil {
		// 无门店 = 本地功能未启用，明确报错引导建门店
		return NearbyRanking{}, errors.New("该品牌还没有门店——先创建门店档案，即可查看附近同行排名")
	}

	view := NearbyRanking{Store: store, OwnRate: -1}

	// ---- 地图榜（现实世界）----
	if uc.searcher != nil && store.HasGeo() {
		center := port.Location{Lat: store.Lat, Lng: store.Lng}
		// 搜索词：品牌名 + 核心卖点（品类词）+ 手动竞品名（合并去重，各搜一次并集）——
		// 品牌名能命中同类门店，竞品名能精确命中对手，卖点词补全品类命中。
		keywords := append([]string{brand.Name}, brand.Competitors...)
		if len(brand.CoreSelling) > 0 {
			keywords = append(keywords, brand.CoreSelling[0])
		}
		keywords = uniqueStrings(keywords)
		for _, kw := range keywords {
			if kw == "" {
				continue
			}
			pois, pErr := uc.searcher.SearchNearby(ctx, center, kw, 0)
			if pErr != nil {
				if errors.Is(pErr, port.ErrGeoNotConfigured) {
					break // 地图服务未配置——降级只显示 AI 榜
				}
				continue // 单个词失败不阻断（网络抖动等）
			}
			view.SearchKeyword = kw
			view.MapRanking = append(view.MapRanking, poisToEntries(pois)...)
		}
		// POI 类型扫描（P1）：types 非空时按分类编码搜索（大类自动含中/小类）——
		// 不依赖品牌/竞品名命中的全量竞品扫描
		if types != "" {
			if pois, tErr := uc.searcher.SearchNearbyByType(ctx, center, types, 0); tErr == nil {
				view.MapRanking = append(view.MapRanking, poisToEntries(pois)...)
				if view.SearchKeyword == "" {
					view.SearchKeyword = "类型:" + types
				}
			}
		}
		// 去重（多搜索词可能命中同一门店）+ 按距离升序
		view.MapRanking = dedupeMapRanking(view.MapRanking)
		sort.SliceStable(view.MapRanking, func(i, j int) bool {
			return view.MapRanking[i].DistanceM < view.MapRanking[j].DistanceM
		})
		view.MapAvailable = len(view.MapRanking) > 0
		// 驾车耗时补全（P2）：有坐标的门店批量测距（驾车=type 1；失败跳过只显示直线距离）
		uc.fillDriveTimes(ctx, center, &view.MapRanking)
	} else if uc.searcher != nil && !store.HasGeo() {
		// 门店存在但坐标缺失（pending）——提示重试地理编码
		view.MapAvailable = false
	}

	// ---- AI 榜（虚拟世界）----
	// v2 数据源升级：优先用"AI 榜单探查"结果（真实搜索 + 附近名单归因匹配，全量补位——
	// 被提及的上榜、未被提及的显示"未上榜"，对比压力可见）；无探查数据回落旧逻辑
	// （监测结果竞品提及率——数据稀疏但兼容旧版本行为）。
	if uc.probeRepo != nil {
		if cached, pErr := uc.probeRepo.Latest(ctx, tenantID, brandID); pErr == nil && len(cached.Results) > 0 {
			view.AIRankFromProbe = true
			view.AIRankProbedAt = cached.ProbedAt.Format("01-02 15:04")
			view.AIRankSample = cached.SampleCount
			view.AIRankTotal = len(cached.Results)
			view.AIRanking = make([]AIRankEntry, 0, len(cached.Results))
			for _, it := range cached.Results {
				if it.Mentioned {
					view.AIRankMentioned++
				}
				view.AIRanking = append(view.AIRanking, AIRankEntry{
					Name: it.Name, Rate: it.Rate, SampleCnt: cached.SampleCount,
					Mentioned: it.Mentioned, MentionCnt: it.MentionCnt,
				})
			}
		}
	}
	if len(view.AIRanking) == 0 {
		// 回落：监测结果竞品提及率（旧逻辑——只有被 AI 提到的已配置竞品）
		latest, rErr := uc.resultRepo.LatestByBrand(ctx, tenantID, brandID)
		if rErr == nil && len(latest) > 0 {
			compMap := make(map[string]struct {
				rateSum float64
				cnt     int
			})
			for _, r := range latest {
				for name, rate := range r.CompetitorRates {
					c := compMap[name]
					c.rateSum += rate
					c.cnt++
					compMap[name] = c
				}
			}
			view.AIRanking = make([]AIRankEntry, 0, len(compMap))
			for name, c := range compMap {
				view.AIRanking = append(view.AIRanking, AIRankEntry{
					Name: name, Rate: c.rateSum / float64(c.cnt),
					SampleCnt: c.cnt, Mentioned: true,
				})
			}
			sort.SliceStable(view.AIRanking, func(i, j int) bool {
				return view.AIRanking[i].Rate > view.AIRanking[j].Rate
			})
			if len(view.AIRanking) > 10 {
				view.AIRanking = view.AIRanking[:10]
			}
		}
	}
	// 自己的 AI 提及率（最近监测均值；独立于 AI 榜数据源——监测口径不变）
	latest, rErr := uc.resultRepo.LatestByBrand(ctx, tenantID, brandID)
	if rErr == nil && len(latest) > 0 {
		rateSum, rateCnt := 0.0, 0
		for _, r := range latest {
			if r.MentionRate > 0 {
				rateSum += r.MentionRate
				rateCnt++
			}
		}
		if rateCnt > 0 {
			view.OwnRate = rateSum / float64(rateCnt)
		}
	}
	return view, nil
}

func uniqueStrings(ss []string) []string {	seen := make(map[string]bool)
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func dedupeMapRanking(entries []MapRankEntry) []MapRankEntry {
	seen := make(map[string]bool)
	out := make([]MapRankEntry, 0, len(entries))
	for _, e := range entries {
		key := e.Name + "|" + e.Address
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	return out
}

// SuggestCompetitorsFromMonitoring 从监测结果蒸馏竞品候选（P0-4 竞品自动沉淀）。
//
// 策略：查品牌最近监测结果 → 聚合 AI 回答中自然出现的其他品牌（CandidateCompetitors，
// 已排除品牌自身与已配置竞品；跨结果去重）→ 按出现次数降序取 top N。
// 对 local/online 都适用——online 品牌的核心竞品来源（AI 回答中提到的对手自动沉淀）。
// 注意：不能从 CompetitorRates 蒸馏——它只统计"已配置竞品"，会被"排除已有竞品"过滤恒空。
func (uc *NearbyUseCase) SuggestCompetitorsFromMonitoring(ctx context.Context, tenantID, brandID string, limit int) ([]CompetitorSuggestion, error) {
	if limit <= 0 {
		limit = 5
	}
	brand, err := uc.brandRepo.FindByID(ctx, tenantID, brandID)
	if err != nil {
		return nil, fmt.Errorf("品牌不存在: %w", err)
	}
	results, err := uc.resultRepo.LatestByBrand(ctx, tenantID, brandID)
	if err != nil || len(results) == 0 {
		return nil, errors.New("还没有监测数据——发起监测后，AI 回答中提到的对手会自动推荐为竞品候选")
	}
	// 聚合候选竞品（跨关键词/引擎/结果统计出现次数；去重保序）
	excluded := map[string]bool{brand.Name: true}
	for _, c := range brand.Competitors {
		excluded[c] = true
	}
	type candidate struct {
		name string
		cnt  int
	}
	poolMap := make(map[string]int)
	for _, r := range results {
		for _, name := range r.CandidateCompetitors {
			name = strings.TrimSpace(name)
			if name == "" || len(name) < 2 || excluded[name] {
				continue
			}
			poolMap[name]++
		}
	}
	pool := make([]candidate, 0, len(poolMap))
	for name, cnt := range poolMap {
		pool = append(pool, candidate{name, cnt})
	}
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].cnt > pool[j].cnt })
	if len(pool) > limit {
		pool = pool[:limit]
	}
	out := make([]CompetitorSuggestion, 0, len(pool))
	for _, c := range pool {
		out = append(out, CompetitorSuggestion{
			Name:     c.name,
			Rating:   float64(c.cnt) / float64(len(results)), // 出现结果占比（0-1）
			Category: "监测识别",
			Address:  fmt.Sprintf("在 %d 次监测结果中出现", c.cnt),
		})
	}
	return out, nil
}

// poisToEntries POI 列表 → 地图榜条目（P2：附带驾车耗时占位，fillDriveTimes 补全）。
func poisToEntries(pois []port.POIStore) []MapRankEntry {
	entries := make([]MapRankEntry, 0, len(pois))
	for _, p := range pois {
		entries = append(entries, MapRankEntry{
			Name: p.Name, Address: p.Address, DistanceM: p.Distance,
			Rating: p.Rating, Category: p.Category, OpenStatus: p.OpenStatus,
			Lat: p.Lat, Lng: p.Lng,
			CityName: p.CityName, AdName: p.AdName, Cost: p.Cost,
			BusinessArea: p.BusinessArea, OpenTimeToday: p.OpenTimeToday,
			Tag: p.Tag, Tel: p.Tel, EntrLocation: p.EntrLocation, PhotoURL: p.PhotoURL,
		})
	}
	return entries
}

// CompetitorSuggestion 竞品推荐候选项（附近同行 POI 按评分/距离排序）。
type CompetitorSuggestion struct {
	Name      string  // 竞品名
	Rating    float64 // 评分（越高越值得对标）
	DistanceM int     // 距门店距离（米，越近越是真对手）
	Address   string  // 地址
	Category  string  // POI 类目
}

// SuggestCompetitors 从附近同行 POI 推荐竞品候选（用户建品牌时竞品常不全——自动补）。
//
// 策略：取门店中心点搜附近 POI（品牌名+卖点词）→ 按评分降序、距离升序排序 →
// 排除品牌自身名 + 已有竞品 → 取 top N（默认 5）。
// 业务分流：online 品牌（线上业务）无附近同行概念——返回空 + 错误提示走监测蒸馏。
// 不消耗 nearby 配额（辅助推荐功能，地图 API 成本可接受）。
func (uc *NearbyUseCase) SuggestCompetitors(ctx context.Context, tenantID, brandID string, limit int) ([]CompetitorSuggestion, error) {
	if limit <= 0 {
		limit = 5
	}
	brand, err := uc.brandRepo.FindByID(ctx, tenantID, brandID)
	if err != nil {
		return nil, fmt.Errorf("品牌不存在: %w", err)
	}
	// online 品牌：线上业务无"附近同行"概念（比"附近网络公司"无意义）
	if !brand.IsLocal() {
		return nil, errors.New("线上业务品牌无附近同行——竞品请从监测结果中 AI 提到的对手采纳")
	}
	if uc.searcher == nil {
		return nil, errors.New("地图服务未配置（POI 搜索不可用）")
	}
	store, err := uc.storeRepo.FindPrimaryByBrand(ctx, brandID)
	if err != nil || !store.HasGeo() {
		return nil, errors.New("品牌还没有已定位的门店——先创建门店并完成地理编码")
	}
	// 已有竞品 + 品牌自身名 → 排除集合
	excluded := map[string]bool{brand.Name: true}
	for _, c := range brand.Competitors {
		excluded[c] = true
	}

	center := port.Location{Lat: store.Lat, Lng: store.Lng}
	keywords := append([]string{brand.Name}, brand.Competitors...)
	if len(brand.CoreSelling) > 0 {
		keywords = append(keywords, brand.CoreSelling[0])
	}
	seen := map[string]bool{}
	var pool []MapRankEntry
	for _, kw := range uniqueStrings(keywords) {
		if kw == "" {
			continue
		}
		pois, pErr := uc.searcher.SearchNearby(ctx, center, kw, 0)
		if pErr != nil {
			continue
		}
		for _, e := range poisToEntries(pois) {
			if excluded[e.Name] || seen[e.Name] || e.Name == "" {
				continue
			}
			seen[e.Name] = true
			pool = append(pool, e)
		}
	}
	// 排序：评分降序（同分按距离升序）——高分近邻 = 最值得对标的真竞品
	sort.SliceStable(pool, func(i, j int) bool {
		if pool[i].Rating != pool[j].Rating {
			return pool[i].Rating > pool[j].Rating
		}
		return pool[i].DistanceM < pool[j].DistanceM
	})
	if len(pool) > limit {
		pool = pool[:limit]
	}
	out := make([]CompetitorSuggestion, 0, len(pool))
	for _, e := range pool {
		out = append(out, CompetitorSuggestion{
			Name: e.Name, Rating: e.Rating, DistanceM: e.DistanceM,
			Address: e.Address, Category: e.Category,
		})
	}
	return out, nil
}

// fillDriveTimes 驾车耗时补全（P2）：批量测距（驾车 type=1，目的地=门店）。
// 只对"有坐标且非零"的门店发起；失败（未配置/网络）跳过——只显示直线距离，不阻断。
func (uc *NearbyUseCase) fillDriveTimes(ctx context.Context, dest port.Location, entries *[]MapRankEntry) {
	if uc.measurer == nil || len(*entries) == 0 {
		return
	}
	// 收集有坐标的起点（最多 100 个——高德批量上限）
	type idxLoc struct {
		idx int
		loc port.Location
	}
	var origins []port.Location
	index := make([]idxLoc, 0, len(*entries))
	for i, e := range *entries {
		if e.Lat != 0 && e.Lng != 0 && len(origins) < 100 {
			origins = append(origins, port.Location{Lat: e.Lat, Lng: e.Lng})
			index = append(index, idxLoc{idx: i, loc: port.Location{Lat: e.Lat, Lng: e.Lng}})
		}
	}
	if len(origins) == 0 {
		return
	}
	results, err := uc.measurer.MeasureDistances(ctx, origins, dest, 1) // 1=驾车（考虑路况）
	if err != nil {
		return // 未配置/失败：只显示直线距离
	}
	for _, r := range results {
		if r.OriginIdx >= 0 && r.OriginIdx < len(index) {
			(*entries)[index[r.OriginIdx].idx].DriveDistanceM = r.DistanceM
			(*entries)[index[r.OriginIdx].idx].DriveDurationSec = r.DurationSec
		}
	}
}
