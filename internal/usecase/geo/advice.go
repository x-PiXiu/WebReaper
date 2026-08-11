package geo

import (
	"context"
	"fmt"
	"sort"

	"webreaper/internal/usecase/port"
)

// ============ 行动建议用例（P5-05：给老板"下一步做什么"）============

// AdviceUseCase 基于监测/门店/内容数据生成可执行建议（规则引擎，零 LLM 成本）。
//
// 设计动机（老板要的是"做什么"，不是"数字"）：
//   数据看板回答"现在怎么样"，行动建议回答"接下来做什么、做了会怎样"。
//   纯规则驱动（参照 RuleScorer 先例）——确定性、可单测、不烧 token。
type AdviceUseCase struct {
	brandRepo   port.BrandRepository
	storeRepo   port.StoreLocationRepository
	resultRepo  port.MonitoringResultRepository
	contentRepo port.OptimizedContentRepository
}

func NewAdviceUseCase(br port.BrandRepository, sr port.StoreLocationRepository, rr port.MonitoringResultRepository, cr port.OptimizedContentRepository) *AdviceUseCase {
	return &AdviceUseCase{brandRepo: br, storeRepo: sr, resultRepo: rr, contentRepo: cr}
}

// Advice 一条行动建议。
type Advice struct {
	Level   string // high/medium/low
	Message string // 建议内容（老板语言）
	Page    string // 对应前端页面路由（可选；跳转用）
}

// GetAdvice 生成品牌行动建议（最多 5 条，按优先级排序）。
// 任一数据源失败降级为空（建议是增强项，不阻断）。
func (uc *AdviceUseCase) GetAdvice(ctx context.Context, tenantID, brandID string) ([]Advice, error) {
	brand, err := uc.brandRepo.FindByID(ctx, tenantID, brandID)
	if err != nil {
		return nil, fmt.Errorf("品牌不存在: %w", err)
	}

	var advices []Advice
	add := func(level, msg, page string) {
		advices = append(advices, Advice{Level: level, Message: msg, Page: page})
	}

	// ---- 门店维度（本地生活地基——仅 local 品牌）----
	// BizType 分流（P0-2）：online 品牌无门店也无需门店，跳过门店相关建议（消除误导）
	if brand.IsLocal() {
		stores, _ := uc.storeRepo.ListByBrand(ctx, tenantID, brandID)
		if len(stores) == 0 {
			add("high", "还没有门店档案——创建门店（地址/电话/营业时间）后，AI 回答本地问题时才找得到你", "/m/nearby")
		} else {
			pending := 0
			for _, s := range stores {
				if !s.HasGeo() {
					pending++
				}
			}
			if pending == len(stores) {
				add("high", "门店地址尚未定位成功（未配置地图服务或地址无法解析）——重试定位后即可使用附近同行排名", "/m/nearby")
		}
	}
	} // close if brand.IsLocal()

	// ---- 监测维度（AI 声量 + 归因）----
	results, _ := uc.resultRepo.LatestByBrand(ctx, tenantID, brandID)
	if len(results) == 0 {
		add("high", "还没有 AI 监测数据——发起一次监测，看看 AI 怎么评价你的品牌", "/m/keywords")
	} else {
		rateSum, rateCnt := 0.0, 0
		selfSum := 0
		ownRate := 0.0
		for _, r := range results {
			rateSum += r.MentionRate
			rateCnt++
			selfSum += r.SelfSourceCount
		}
		if rateCnt > 0 {
			ownRate = rateSum / float64(rateCnt)
		}
		if ownRate < 0.2 {
			add("high", fmt.Sprintf("AI 提到你的比例偏低（%.0f%%）——生成并发布本地化内容，提升被引用概率", ownRate*100), "/m/content")
		}
		// 归因（P5-01）：被提到但没被引用 = 内容没有被 AI 读到
		if selfSum == 0 {
			add("high", "AI 提到你，但引用的来源里没有你的内容——发布高质量文章（评分≥50）并确认收录成功", "/m/content")
		}
		// 竞品压制
		var comps []struct{ name string; rate float64 }
		for _, r := range results {
			for name, rate := range r.CompetitorRates {
				comps = append(comps, struct{ name string; rate float64 }{name, rate})
			}
		}
		if len(comps) > 0 && ownRate > 0 {
			sort.SliceStable(comps, func(i, j int) bool { return comps[i].rate > comps[j].rate })
			top := comps[0]
			if top.rate > ownRate {
				add("medium", fmt.Sprintf("竞品「%s」的 AI 提及率（%.0f%%）高于你（%.0f%%）——研究它的内容策略，针对性补强", top.name, top.rate*100, ownRate*100), "/m/visibility")
			}
		}
	}

	// ---- 内容维度（发布/质量）----
	contents, _ := uc.contentRepo.ListByBrand(ctx, tenantID, brandID)
	published := 0
	lowScore := 0
	for _, c := range contents {
		if c.Status == "published" {
			published++
		}
		if c.Score.Total > 0 && c.Score.Total < 50 {
			lowScore++
		}
	}
	if len(contents) > 0 && published == 0 {
		add("medium", "已生成 "+fmt.Sprint(len(contents))+" 篇内容但都未发布——发布后 AI 才有机会引用", "/m/content")
	}
	if lowScore > 0 {
		add("medium", fmt.Sprintf("有 %d 篇内容评分低于 50——优化质量后再发布，低分内容会拖累公开站整体权重", lowScore), "/m/content")
	}

	if len(advices) == 0 {
		add("low", "各项指标表现良好——持续监测、保持内容更新节奏即可", "/m/visibility")
	}
	if len(advices) > 5 {
		advices = advices[:5]
	}
	return advices, nil
}
