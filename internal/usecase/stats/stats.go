// Package stats 实现"平台总览统计聚合"用例。
//
// 职责：聚合各仓储的统计数据，返回管理后台平台总览所需的全量指标。
// 整洁架构：依赖 port 接口（仓储聚合方法），不关心具体 SQL 实现。
// 任一子统计失败不阻断其余（降级返回 0 / 空），保证页面可用性。
package stats

import (
	"context"

	"webreaper/internal/usecase/port"
)

// StatsUseCase 平台总览统计用例。
type StatsUseCase struct {
	itemRepo    port.DataItemRepository
	userRepo    port.UserRepository
	brandRepo   port.BrandRepository
	keywordRepo port.KeywordRepository
	monitorRepo port.MonitoringResultRepository
	contentRepo port.OptimizedContentRepository
	jobRepo     port.PublishJobRepository
}

func NewStatsUseCase(
	itemRepo port.DataItemRepository,
	userRepo port.UserRepository,
	brandRepo port.BrandRepository,
	keywordRepo port.KeywordRepository,
	monitorRepo port.MonitoringResultRepository,
	contentRepo port.OptimizedContentRepository,
	jobRepo port.PublishJobRepository,
) *StatsUseCase {
	return &StatsUseCase{
		itemRepo:    itemRepo,
		userRepo:    userRepo,
		brandRepo:   brandRepo,
		keywordRepo: keywordRepo,
		monitorRepo: monitorRepo,
		contentRepo: contentRepo,
		jobRepo:     jobRepo,
	}
}

// StatsView 平台总览全量统计（一次返回，避免前端多次请求）。
type StatsView struct {
	// ---- 平台规模（核心数字卡片）----
	Users             int `json:"users"`              // 平台商户（用户）总数
	Brands            int `json:"brands"`             // 品牌资产总数
	Keywords          int `json:"keywords"`           // 关键词总数
	MonitorResults    int `json:"monitor_results"`    // 监测结果总数（累计探测）
	OptimizedContents int `json:"optimized_contents"` // 优化内容总数
	PublishedContents int `json:"published_contents"` // 已发布公开内容数
	PublishJobs       int `json:"publish_jobs"`       // 发布任务总数
	DataItems         int `json:"data_items"`         // 采集数据项总数

	// ---- 数据资产明细（数据管理页/趋势图用）----
	StatusBreakdown map[string]int    `json:"status_breakdown"`  // 数据项状态分布（环形图）
	DailyTrend      []port.DailyCount `json:"daily_trend"`       // 近 14 天数据项趋势（折线图）
	SourceDist      []port.GroupCount `json:"source_distribution"` // 数据源分布（饼图）
	TopTags         []port.GroupCount `json:"top_tags"`          // 标签 Top 8（条形图）
}

// Get 聚合全量统计。任一子统计失败不阻断其余（降级返回 0 / 空）。
// 仓储可能为 nil（未配置 DB 时 GEO/发布仓储不装配）——nil 视为该域指标恒为 0。
func (uc *StatsUseCase) Get(ctx context.Context) StatsView {
	view := StatsView{StatusBreakdown: map[string]int{}}

	// ---- 平台规模 ----
	if uc.userRepo != nil {
		if n, err := uc.userRepo.Count(ctx); err == nil {
			view.Users = n
		}
	}
	if uc.brandRepo != nil {
		if n, err := uc.brandRepo.Count(ctx); err == nil {
			view.Brands = n
		}
	}
	if uc.keywordRepo != nil {
		if n, err := uc.keywordRepo.Count(ctx); err == nil {
			view.Keywords = n
		}
	}
	if uc.monitorRepo != nil {
		if n, err := uc.monitorRepo.Count(ctx); err == nil {
			view.MonitorResults = n
		}
	}
	if uc.contentRepo != nil {
		if n, err := uc.contentRepo.Count(ctx); err == nil {
			view.OptimizedContents = n
		}
		if n, err := uc.contentRepo.CountPublished(ctx); err == nil {
			view.PublishedContents = n
		}
	}
	if uc.jobRepo != nil {
		if n, err := uc.jobRepo.Count(ctx); err == nil {
			view.PublishJobs = n
		}
	}

	// ---- 数据资产明细 ----
	// 状态分布 + 数据项总量（同一查询）
	if statusCounts, err := uc.itemRepo.CountByStatus(ctx); err == nil {
		view.StatusBreakdown = statusCounts
		total := 0
		for _, cnt := range statusCounts {
			total += cnt
		}
		view.DataItems = total
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
