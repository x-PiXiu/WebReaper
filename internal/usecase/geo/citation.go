package geo

import (
	"context"
	"regexp"

	"webreaper/internal/usecase/port"
)

// ============ 内容引用统计（P5-02 评分校准基础设施）============

// CitationUseCase 统计每篇内容被 AI 回答引用的次数（归因细化到篇）。
//
// 数据流（P5-01 的 Sources 字段 → 本篇）：
//   monitoring results 的 sources 里出现 /public/articles/{id} → 该内容被引用 1 次。
//   汇总后：
//   - 内容工作台展示"这篇被引用 N 次"（老板能看到哪篇内容真正起作用）
//   - 沉淀"内容 GEO 评分 vs 实际被引用次数"对照——P5-02 评分校准的数据源
//     （数据量足够后可回归标定评分维度权重，见计划文档 P5-02）。
type CitationUseCase struct {
	resultRepo port.MonitoringResultRepository
}

func NewCitationUseCase(rr port.MonitoringResultRepository) *CitationUseCase {
	return &CitationUseCase{resultRepo: rr}
}

// articleURLRe 匹配公开站文章 URL（来源中的链接形态：
// https://content.example.com/public/articles/oc-xxx）。
var articleURLRe = regexp.MustCompile(`/public/articles/([A-Za-z0-9\-_]+)`)

// GetByBrand 统计品牌下每篇内容的引用次数（按 contentID 聚合）。
// 无监测数据/无引用时返回空 map（不报错）。
func (uc *CitationUseCase) GetByBrand(ctx context.Context, tenantID, brandID string) (map[string]int, error) {
	// 用 Trend 而非 LatestByBrand——引用统计要看"历史所有回答"（近 200 条足够）
	results, err := uc.resultRepo.Trend(ctx, tenantID, brandID, 200)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for _, r := range results {
		for _, s := range r.Sources {
			if m := articleURLRe.FindStringSubmatch(s); len(m) == 2 && m[1] != "" {
				counts[m[1]]++
			}
		}
	}
	return counts, nil
}
