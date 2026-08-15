package geo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// AIRankProbeUseCase "AI 榜单探查"用例——附近同行 AI 榜的数据生产方。
//
// 设计动机（解决 AI 榜稀疏：监测回答里本地小店极少出现，用户看不到对比压力）：
//   - 探查问法：本地化中性问法（商圈+品类+卖点，不点名——真实性红线）
//   - AI 真实搜索回答（复用 port.AIEngineProbe，模拟引擎走搜索工具）
//   - 附近 POI 名单归因匹配：名单不喂给 LLM（无诱导），只在解析阶段做文本匹配——
//     AI 回答是真实搜索所得，我们只是把"它提到的门店"归到"附近同行"上；
//     回答里提到的非附近门店不计入（榜单与地图榜同口径，不被远处名店占领）
//   - 结果缓存 24h（探查消耗 LLM/地图配额；手动刷新 force 重跑）
type AIRankProbeUseCase struct {
	probe       port.AIEngineProbe
	brandRepo   port.BrandRepository
	storeRepo   port.StoreLocationRepository
	keywordRepo port.KeywordRepository
	probeRepo   port.AIRankProbeRepository
	searcher    port.POISearcher
	quotaGate   port.QuotaStore
	usageRec    port.UsageRecorder
	selfDomain  string // 归因（与监测一致；探查暂不统计自营引用，保留字段）
}

// NewAIRankProbeUseCase 创建 AI 榜单探查用例。
func NewAIRankProbeUseCase(probe port.AIEngineProbe, brandRepo port.BrandRepository,
	storeRepo port.StoreLocationRepository, keywordRepo port.KeywordRepository,
	probeRepo port.AIRankProbeRepository, searcher port.POISearcher) *AIRankProbeUseCase {
	return &AIRankProbeUseCase{
		probe: probe, brandRepo: brandRepo, storeRepo: storeRepo,
		keywordRepo: keywordRepo, probeRepo: probeRepo, searcher: searcher,
	}
}

// SetQuotaGate 注入配额门（可选；nearby 场景——与附近同行同额度）。
func (uc *AIRankProbeUseCase) SetQuotaGate(g port.QuotaStore) {
	if g != nil {
		uc.quotaGate = g
	}
}

// SetUsageRecorder 注入业务动作计数（可选）。
func (uc *AIRankProbeUseCase) SetUsageRecorder(r port.UsageRecorder) {
	if r != nil {
		uc.usageRec = r
	}
}

// Run 执行 AI 榜单探查。force=false 且缓存未过期 → 直接返回缓存；否则真实探查并缓存。
// types：POI 分类编码（如餐饮=050000）——与 GetRanking 地图榜同口径。
func (uc *AIRankProbeUseCase) Run(ctx context.Context, tenantID, brandID, types string, force bool) (entity.AIRankProbeResult, error) {
	if uc.probe == nil || uc.probeRepo == nil {
		return entity.AIRankProbeResult{}, errors.New("AI 榜单探查未配置（需 LLM 与收录配置）")
	}
	// 配额检查（X-01：nearby 场景——地图 POI + LLM 探查有成本；超限 402）
	if uc.quotaGate != nil {
		if err := uc.quotaGate.Check(ctx, tenantID, "nearby"); err != nil {
			return entity.AIRankProbeResult{}, err
		}
	}

	// 缓存命中：未过期且非强制 → 直接返回
	if !force {
		if cached, err := uc.probeRepo.Latest(ctx, tenantID, brandID); err == nil {
			if time.Now().Before(cached.ExpireAt) && len(cached.Results) > 0 {
				return cached, nil
			}
		}
	}

	brand, err := uc.brandRepo.FindByID(ctx, tenantID, brandID)
	if err != nil {
		return entity.AIRankProbeResult{}, fmt.Errorf("品牌不存在: %w", err)
	}
	store, err := uc.storeRepo.FindPrimaryByBrand(ctx, brandID)
	if err != nil {
		return entity.AIRankProbeResult{}, errors.New("该品牌还没有门店——先创建门店档案，即可生成 AI 榜")
	}

	// 品类词（问法用料）：关键词提取（去城市/尾词），兜底品牌卖点/名称
	category := uc.categoryWord(ctx, tenantID, brand)
	if category == "" {
		if len(brand.CoreSelling) > 0 {
			category = brand.CoreSelling[0]
		} else {
			category = brand.Name
		}
	}
	localCtx := localContextFromStore(store)
	if localCtx == "" {
		localCtx = store.City
	}

	// 中性问法池（不点名——真实性红线；商圈+品类+卖点驱动）
	questions := buildRankProbeQuestions(category, localCtx, brand.CoreSelling)

	// AI 真实搜索（模拟引擎：default LLM + 搜索工具；问题多样化为真实用户视角）
	probeResult, pErr := uc.probe.Probe(ctx, port.ProbeInput{
		TenantID:       tenantID,
		EngineName:     "", // 模拟引擎（真实引擎接入后走 DirectProbe 直测）
		BrandName:      brand.Name,
		Keyword:        category,
		SampleSize:     len(questions),
		Questions:      questions,
		LocalContext:   localCtx,
		SelfBaseDomain: uc.selfDomain,
	})
	if pErr != nil {
		return entity.AIRankProbeResult{}, fmt.Errorf("AI 榜单探查失败: %w", pErr)
	}

	// 附近 POI 名单（与地图榜同源：品牌名+卖点+类型扫描）
	pois := uc.fetchNearbyPOIs(ctx, brand, store, types)

	// 归因匹配：AI 回答文本 → 名单门店提及
	items := matchPOIInAnswers(probeResult, pois)

	result := entity.AIRankProbeResult{
		TenantID:    tenantID,
		BrandID:     brandID,
		Results:     items,
		SampleCount: probeResult.SampleCount,
		ProbedAt:    time.Now(),
		ExpireAt:    time.Now().Add(entity.AIRankProbeTTL),
	}
	// 业务动作计数（配额数据源）
	if uc.usageRec != nil && tenantID != "" {
		_ = uc.usageRec.RecordUsage(ctx, entity.UsageRecord{TenantID: tenantID, Scene: "nearby"})
	}
	if err := uc.probeRepo.Save(ctx, result); err != nil {
		return entity.AIRankProbeResult{}, fmt.Errorf("AI 榜单探查结果保存失败: %w", err)
	}
	return result, nil
}

// categoryWord 从品牌关键词提取品类词（"成都川菜馆推荐" → "川菜馆"）。
// 规则：去城市名（品牌门店所在城市/商圈）/去尾词（推荐/哪家/哪里/什么/好吃）→ 剩余即品类。
func (uc *AIRankProbeUseCase) categoryWord(ctx context.Context, tenantID string, brand entity.Brand) string {
	if uc.keywordRepo == nil {
		return ""
	}
	ks, err := uc.keywordRepo.ListByBrand(ctx, tenantID, brand.ID)
	if err != nil || len(ks) == 0 {
		return ""
	}
	// 取第一个关键词；位置前缀从门店城市/商圈剥离
	term := ks[0].Term
	if store, sErr := uc.storeRepo.FindPrimaryByBrand(ctx, brand.ID); sErr == nil {
		for _, loc := range []string{store.BusinessArea, store.District, store.City} {
			if loc != "" {
				term = strings.ReplaceAll(term, loc, "")
			}
		}
	}
	// 尾词剥离（推荐/哪家好/哪家/哪里/什么/好吃/怎么样/排名/排行/有哪些）
	for _, suf := range []string{"推荐", "哪家好", "哪家", "哪里", "什么", "好吃", "怎么样", "排名", "排行", "有哪些", "有"} {
		term = strings.TrimSuffix(term, suf)
	}
	term = strings.TrimSpace(term)
	if len([]rune(term)) < 2 {
		return ""
	}
	return term
}

// fetchNearbyPOIs 搜索附近同行名单（与 GetRanking 地图榜同源：品牌名+卖点+类型扫描）。
func (uc *AIRankProbeUseCase) fetchNearbyPOIs(ctx context.Context, brand entity.Brand, store entity.StoreLocation, types string) []port.POIStore {
	if uc.searcher == nil || !store.HasGeo() {
		return nil
	}
	center := port.Location{Lat: store.Lat, Lng: store.Lng}
	var pois []port.POIStore
	keywords := append([]string{brand.Name}, brand.Competitors...)
	if len(brand.CoreSelling) > 0 {
		keywords = append(keywords, brand.CoreSelling[0])
	}
	for _, kw := range uniqueStrings(keywords) {
		if kw == "" {
			continue
		}
		if ps, pErr := uc.searcher.SearchNearby(ctx, center, kw, 0); pErr == nil {
			pois = append(pois, ps...)
		}
	}
	if types != "" {
		if ps, tErr := uc.searcher.SearchNearbyByType(ctx, center, types, 0); tErr == nil {
			pois = append(pois, ps...)
		}
	}
	return dedupePOIs(pois)
}

// buildRankProbeQuestions 本地化中性问法池（不点名——真实性红线；商圈/品类/卖点驱动）。
func buildRankProbeQuestions(category, localCtx string, sellings []string) []string {
	var qs []string
	if localCtx != "" {
		qs = append(qs,
			localCtx+"附近有什么"+category,
			localCtx+"有什么值得推荐的"+category,
			localCtx+"口碑好的"+category+"哪家好",
		)
	}
	qs = append(qs,
		"附近有什么推荐的"+category,
		"口碑好的"+category+"有哪些",
		category+"哪家做得好",
	)
	for _, s := range sellings {
		if s != "" && !strings.Contains(strings.Join(qs, "|"), s) {
			qs = append(qs, s+"做得好的"+category+"推荐")
		}
	}
	return qs
}

// matchPOIInAnswers 归因匹配：AI 回答文本 → 名单门店提及统计（纯函数，可单测）。
// 匹配规则：回答包含门店名（忽略括号变体——"卢记正街饭店(蓉城总店)"按主名"卢记正街饭店"匹配）；
// 统计：被提到的采样数 → 提及率；平均出现位次（回答中首次出现顺序，1=最先）。
func matchPOIInAnswers(pr port.ProbeResult, pois []port.POIStore) []entity.AIRankProbeItem {
	if len(pois) == 0 {
		return nil
	}
	total := pr.SampleCount
	if total <= 0 {
		total = 1
	}
	// 按采样切分回答（【问：...】分段；ProbeResult 只给合并文本——用全文本匹配即可，
	// 提及率口径 = 该门店名在回答中出现的采样段数，用出现次数近似）
	items := make([]entity.AIRankProbeItem, 0, len(pois))
	for _, poi := range pois {
		mainName := poiMainName(poi.Name)
		lower := strings.ToLower(pr.RawSample)
		hit := strings.Contains(lower, strings.ToLower(mainName))
		cnt := 0
		if hit {
			cnt = strings.Count(lower, strings.ToLower(mainName))
		}
		rate := 0.0
		if hit {
			rate = float64(cnt) / float64(total)
			if rate > 1 {
				rate = 1
			}
		}
		items = append(items, entity.AIRankProbeItem{
			Name:       poi.Name,
			Mentioned:  hit,
			MentionCnt: cnt,
			Rate:       rate,
		})
	}
	// 被提及的排前面（提及率降序），未提及的保持原名单顺序（距离序）
	mentioned := make([]entity.AIRankProbeItem, 0, len(items))
	unmentioned := make([]entity.AIRankProbeItem, 0, len(items))
	for _, it := range items {
		if it.Mentioned {
			mentioned = append(mentioned, it)
		} else {
			unmentioned = append(unmentioned, it)
		}
	}
	// 稳定排序（提及率降序；同率保持出现序）
	for i := 1; i < len(mentioned); i++ {
		for j := i; j > 0 && mentioned[j].Rate > mentioned[j-1].Rate; j-- {
			mentioned[j], mentioned[j-1] = mentioned[j-1], mentioned[j]
		}
	}
	return append(mentioned, unmentioned...)
}

// poiMainName 门店名去括号变体（"卢记正街饭店(蓉城总店)" → "卢记正街饭店"）。
func poiMainName(name string) string {
	if idx := strings.Index(name, "("); idx > 0 {
		return strings.TrimSpace(name[:idx])
	}
	if idx := strings.Index(name, "（"); idx > 0 {
		return strings.TrimSpace(name[:idx])
	}
	return name
}

// dedupePOIs 按门店名去重（多搜索词命中同一门店）。
func dedupePOIs(pois []port.POIStore) []port.POIStore {
	seen := make(map[string]bool)
	out := make([]port.POIStore, 0, len(pois))
	for _, p := range pois {
		k := poiMainName(p.Name)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
	}
	return out
}

// Latest 取该品牌最近一次 AI 榜探查缓存（F4：品牌卡徽章等轻量展示——不重跑、不烧配额）。
func (uc *AIRankProbeUseCase) Latest(ctx context.Context, tenantID, brandID string) (entity.AIRankProbeResult, error) {
	return uc.probeRepo.Latest(ctx, tenantID, brandID)
}
