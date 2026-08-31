// auto_monitor_after_publish.go 发布后自动监测（27号优化——发布→监测闭环）。
//
// 设计：
//   - 每10分钟扫描最近发布成功的任务
//   - 检查是否已触发监测（post_mention_rate是否已填充）
//   - 如果未触发，自动调用监测用例更新提及率
package scheduledtask

import (
	"context"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/account"
	"webreaper/internal/usecase/geo"
	"webreaper/internal/usecase/port"
)

// AutoMonitorAfterPublishTask 发布后自动监测任务。
type AutoMonitorAfterPublishTask struct {
	jobRepo     port.PublishJobRepository
	publishUC   *account.PublishUseCase
	monitorUC   *geo.MonitorUseCase
	logger      port.Logger
}

// NewAutoMonitorAfterPublishTask 创建发布后自动监测任务。
func NewAutoMonitorAfterPublishTask(
	jobRepo port.PublishJobRepository,
	publishUC *account.PublishUseCase,
	monitorUC *geo.MonitorUseCase,
	logger port.Logger,
) *AutoMonitorAfterPublishTask {
	return &AutoMonitorAfterPublishTask{
		jobRepo:   jobRepo,
		publishUC: publishUC,
		monitorUC: monitorUC,
		logger:    logger,
	}
}

func (t *AutoMonitorAfterPublishTask) Name() string            { return "auto-monitor-after-publish" }
func (t *AutoMonitorAfterPublishTask) Interval() time.Duration { return 10 * time.Minute }

func (t *AutoMonitorAfterPublishTask) Execute(ctx context.Context) error {
	if t.monitorUC == nil {
		return nil
	}

	// 获取最近24小时内发布的任务
	since := time.Now().Add(-24 * time.Hour)
	recentJobs, err := t.jobRepo.ListPublished(ctx, 100)
	if err != nil {
		return err
	}

	monitored := 0
	for _, job := range recentJobs {
		if job.Status != entity.PublishStatusPublished {
			continue
		}
		if job.PublishedAt.Before(since) {
			continue
		}
		// 检查是否已触发监测（post_mention_rate > 0 表示已触发）
		if job.PostMentionRate > 0 {
			continue
		}
		if job.BrandID == "" {
			continue
		}

		// 触发监测
		rate, err := t.monitorUC.TriggerMonitor(ctx, job.TenantID, job.BrandID)
		if err != nil {
			t.logger.Warn("发布后自动监测失败",
				port.String("job_id", job.ID),
				port.Err(err))
			continue
		}

		// 更新发布任务的发布后提及率
		job.PostMentionRate = rate
		if err := t.jobRepo.Save(ctx, job); err != nil {
			t.logger.Warn("更新发布后提及率失败",
				port.String("job_id", job.ID),
				port.Err(err))
		}
		monitored++
	}

	if monitored > 0 {
		t.logger.Info("发布后自动监测完成",
			port.Int("count", monitored))
	}
	return nil
}
