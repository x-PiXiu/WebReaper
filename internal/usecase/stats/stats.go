// Package stats 实现"仪表盘统计聚合"用例。
//
// 职责：聚合各仓储的统计数据，返回仪表盘所需的全量指标。
// 整洁架构：依赖 port 接口（仓储聚合方法），不关心具体 SQL 实现。
package stats

import (
	"context"

	"webreaper/internal/usecase/port"
)

// StatsUseCase 仪表盘统计用例。
type StatsUseCase struct {
	itemRepo port.DataItemRepository
}

func NewStatsUseCase(itemRepo port.DataItemRepository) *StatsUseCase {
	return &StatsUseCase{itemRepo: itemRepo}
}

// StatsView 仪表盘全量统计（一次返回，避免前端多次请求）。
type StatsView struct {
	Totals          map[string]int     `json:"totals"`            // 总量计数：data_items / pending_review / approved / rejected
	StatusBreakdown map[string]int     `json:"status_breakdown"`  // 状态分布（环形图）
	DailyTrend      []port.DailyCount  `json:"daily_trend"`       // 近 14 天趋势（折线图）
	SourceDist      []port.GroupCount  `json:"source_distribution"` // 数据源分布（饼图）
	TopTags         []port.GroupCount  `json:"top_tags"`          // 标签 Top 8（条形图）
}

// Get 聚合全量统计。任一子统计失败不阻断其余（降级返回空）。
func (uc *StatsUseCase) Get(ctx context.Context) StatsView {
	view := StatsView{
		Totals:          map[string]int{},
		StatusBreakdown: map[string]int{},
	}

	// 状态分布 + 总量（同一查询）
	if statusCounts, err := uc.itemRepo.CountByStatus(ctx); err == nil {
		view.StatusBreakdown = statusCounts
		total := 0
		for status, cnt := range statusCounts {
			total += cnt
			view.Totals[status] = cnt
		}
		view.Totals["data_items"] = total
	}

	// 近 14 天趋势
	if trend, err := uc.itemRepo.DailyCounts(ctx, 14); err == nil {
		view.DailyTrend = trend
	}

	// 数据源分布（按 metadata.crawler_type）
	if dist, err := uc.itemRepo.GroupByMetaKey(ctx, "crawler_type"); err == nil {
		view.SourceDist = dist
	}

	// 标签 Top 8
	if tags, err := uc.itemRepo.TopTags(ctx, 8); err == nil {
		view.TopTags = tags
	}

	return view
}
