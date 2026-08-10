// Package stats 实现"平台总览统计聚合"用例。
//
// 职责：聚合各仓储的统计数据，返回管理后台平台总览所需的全量指标。
// 整洁架构：依赖 port 接口（仓储聚合方法），不关心具体 SQL 实现。
// 任一子统计失败不阻断其余（降级返回 0），保证页面可用性。
//
// 重构说明：原依赖 DataItemRepository 的 4 个统计（StatusBreakdown/DailyTrend/
// SourceDist/TopTags）已移除——数据采集域已删除，这些指标无数据源。
package stats

import (
	"context"

	"webreaper/internal/usecase/port"
)

// StatsUseCase 平台总览统计用例。
type StatsUseCase struct {
	userRepo    port.UserRepository
	brandRepo   port.BrandRepository
	keywordRepo port.KeywordRepository
	monitorRepo port.MonitoringResultRepository
	contentRepo port.OptimizedContentRepository
	jobRepo     port.PublishJobRepository
}

func NewStatsUseCase(
	userRepo port.UserRepository,
	brandRepo port.BrandRepository,
	keywordRepo port.KeywordRepository,
	monitorRepo port.MonitoringResultRepository,
	contentRepo port.OptimizedContentRepository,
	jobRepo port.PublishJobRepository,
) *StatsUseCase {
	return &StatsUseCase{
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
	// 平台规模（核心数字卡片）
	Users             int `json:"users"`              // 平台商户（用户）总数
	Brands            int `json:"brands"`             // 品牌资产总数
	Keywords          int `json:"keywords"`           // 关键词总数
	MonitorResults    int `json:"monitor_results"`    // 监测结果总数（累计探测）
	OptimizedContents int `json:"optimized_contents"` // 优化内容总数
	PublishedContents int `json:"published_contents"` // 已发布公开内容数
	PublishJobs       int `json:"publish_jobs"`       // 发布任务总数
}

// Get 聚合全量统计。任一子统计失败不阻断其余（降级返回 0）。
// 仓储可能为 nil（未配置 DB 时 GEO/发布仓储不装配）——nil 视为该域指标恒为 0。
func (uc *StatsUseCase) Get(ctx context.Context) StatsView {
	view := StatsView{}

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

	return view
}
