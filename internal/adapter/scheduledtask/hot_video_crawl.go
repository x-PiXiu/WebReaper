package scheduledtask

import (
	"context"
	"time"

	"webreaper/internal/usecase/port"
	"webreaper/internal/usecase/hotvideo"
)

// HotVideoCrawlTask 定期热门视频采集（6h 一次，按品牌轮询积累内容）。
//
// 设计（08 计划——热门同款持久化）：原来 ListHotVideos 只在商户进入 tab 时触发
// 搜索，重启/缓存过期就丢。改为定时主动采集：每次对全量品牌调用搜索+LLM 筛选，
// 结果自动落 hot_videos 表（brand_id+url 去重），商户进 tab 时直接从 DB 读
// （带搜索/排序/分页），不再等搜索。
type HotVideoCrawlTask struct {
	uc     *hotvideo.HotVideoUseCase
	repo   port.BrandRepository
	logger port.Logger
}

func NewHotVideoCrawlTask(uc *hotvideo.HotVideoUseCase, repo port.BrandRepository, logger port.Logger) *HotVideoCrawlTask {
	return &HotVideoCrawlTask{uc: uc, repo: repo, logger: logger}
}

func (t *HotVideoCrawlTask) Name() string { return "hot-video-crawl" }

func (t *HotVideoCrawlTask) Interval() time.Duration { return 6 * time.Hour }

func (t *HotVideoCrawlTask) Execute(ctx context.Context) error {
	if t.repo == nil || t.uc == nil {
		return nil
	}
	brands, err := t.repo.ListAll(ctx)
	if err != nil {
		return err
	}
	total := 0
	for _, b := range brands {
		if b.ID == "" {
			continue
		}
		// ListHotVideos 内部已有 24h 缓存 + 10min 站内搜索冷却 + 搜索后自动落库
		videos, vErr := t.uc.ListHotVideos(ctx, b.TenantID, b.ID, false)
		if vErr != nil {
			if t.logger != nil {
				t.logger.Warn("热门视频采集失败", port.String("brand", b.ID), port.Err(vErr))
			}
			continue
		}
		total += len(videos)
	}
	if total > 0 && t.logger != nil {
		t.logger.Info("热门视频定期采集完成", port.Int("brands", len(brands)), port.Int("videos", total))
	}
	return nil
}
