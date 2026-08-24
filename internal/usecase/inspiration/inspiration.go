// Package inspiration 实现灵感广场用例（整洁架构·Usecase层）。
//
// 产品语义：商户打开灵感广场，看到各品牌的热门视频数据，无需登录。
// 数据来源：平台方账号统一爬取，存入 DB，用户只读。
package inspiration

import (
	"context"
	"fmt"
	"log"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// UseCase 灵感广场用例。
type UseCase struct {
	videoRepo  port.InspirationVideoRepository
	brandRepo  port.BrandInspirationRepository
	configRepo port.CrawlerConfigRepository
	accountRepo port.CrawlerAccountRepository
	platforms  map[string]port.CrawlerPlatform
}

// NewUseCase 创建灵感广场用例。
func NewUseCase(
	videoRepo port.InspirationVideoRepository,
	brandRepo port.BrandInspirationRepository,
	configRepo port.CrawlerConfigRepository,
	accountRepo port.CrawlerAccountRepository,
) *UseCase {
	return &UseCase{
		videoRepo:   videoRepo,
		brandRepo:   brandRepo,
		configRepo:  configRepo,
		accountRepo: accountRepo,
		platforms:   make(map[string]port.CrawlerPlatform),
	}
}

// RegisterPlatform 注册平台爬虫。
func (uc *UseCase) RegisterPlatform(platform string, crawler port.CrawlerPlatform) {
	uc.platforms[platform] = crawler
}

// List 查询灵感视频列表（用户端，无需登录）。
func (uc *UseCase) List(ctx context.Context, brandID, platform, keyword, sortBy string, page, pageSize int) ([]entity.InspirationVideo, int, error) {
	return uc.videoRepo.List(ctx, brandID, platform, keyword, sortBy, page, pageSize)
}

// GetByID 查询单个灵感视频详情。
func (uc *UseCase) GetByID(ctx context.Context, id string) (entity.InspirationVideo, error) {
	return uc.videoRepo.FindByID(ctx, id)
}

// CrawlBrand 采集指定品牌的热门视频。
//
// 流程：
//  1. 获取品牌的搜索关键词
//  2. 调用平台爬虫搜索
//  3. 保存到 DB + 建立品牌关联
func (uc *UseCase) CrawlBrand(ctx context.Context, platform, brandID string, keywords []string) (*CrawlResult, error) {
	crawler, ok := uc.platforms[platform]
	if !ok {
		return nil, fmt.Errorf("平台 %s 未注册", platform)
	}

	startAt := time.Now()

	// 逐关键词搜索
	allVideos := make([]entity.CrawledVideo, 0)
	for _, keyword := range keywords {
		videos, err := crawler.Search(ctx, entity.SearchOptions{
			Keyword: keyword,
			Limit:   20,
		})
		if err != nil {
			log.Printf("[inspiration] 搜索失败 platform=%s keyword=%s: %v", platform, keyword, err)
			continue
		}
		allVideos = append(allVideos, videos...)
	}

	// 转换为 InspirationVideo
	inspirations := make([]entity.InspirationVideo, 0, len(allVideos))
	for _, v := range allVideos {
		insp := entity.CrawledVideoToInspiration(v)
		insp.ID = fmt.Sprintf("insp-%s-%s", v.Platform, v.VideoID)
		inspirations = append(inspirations, insp)
	}

	// 批量保存（去重）
	newCount, err := uc.videoRepo.SaveBatch(ctx, inspirations)
	if err != nil {
		return nil, fmt.Errorf("保存视频失败: %w", err)
	}

	// 建立品牌关联
	for _, insp := range inspirations {
		if err := uc.brandRepo.Link(ctx, brandID, insp.ID, ""); err != nil {
			log.Printf("[inspiration] 建立品牌关联失败 brand=%s video=%s: %v", brandID, insp.ID, err)
		}
	}

	now := time.Now()
	return &CrawlResult{
		Platform:     platform,
		BrandID:      brandID,
		Keywords:     keywords,
		VideosFound:  len(allVideos),
		VideosNew:    newCount,
		DurationMs:   int(now.Sub(startAt).Milliseconds()),
		FinishedAt:   now,
	}, nil
}

// RefreshMetrics 刷新指定品牌视频的互动指标。
func (uc *UseCase) RefreshMetrics(ctx context.Context, platform string, videoIDs []string) (int, error) {
	crawler, ok := uc.platforms[platform]
	if !ok {
		return 0, fmt.Errorf("平台 %s 未注册", platform)
	}

	updates, err := crawler.RefreshMetrics(ctx, videoIDs)
	if err != nil {
		return 0, err
	}

	updated := 0
	for _, u := range updates {
		if err := uc.videoRepo.UpdateMetrics(ctx, u.VideoID, u); err != nil {
			log.Printf("[inspiration] 更新指标失败 video=%s: %v", u.VideoID, err)
			continue
		}
		updated++
	}
	return updated, nil
}

// IsPlatformAlive 检测平台爬虫是否可用。
func (uc *UseCase) IsPlatformAlive(ctx context.Context, platform string) bool {
	crawler, ok := uc.platforms[platform]
	if !ok {
		return false
	}
	return crawler.IsAlive(ctx)
}

// ListPlatforms 列出所有已注册的平台。
func (uc *UseCase) ListPlatforms() []string {
	platforms := make([]string, 0, len(uc.platforms))
	for p := range uc.platforms {
		platforms = append(platforms, p)
	}
	return platforms
}

// CrawlResult 采集结果。
type CrawlResult struct {
	Platform    string    `json:"platform"`
	BrandID     string    `json:"brand_id"`
	Keywords    []string  `json:"keywords"`
	VideosFound int       `json:"videos_found"`
	VideosNew   int       `json:"videos_new"`
	DurationMs  int       `json:"duration_ms"`
	FinishedAt  time.Time `json:"finished_at"`
}
