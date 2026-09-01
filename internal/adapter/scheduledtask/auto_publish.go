// auto_publish.go 生成→发布→监测自动闭环（27号优化）。
//
// 设计：
//   - 每5分钟扫描已完成的生成任务（state=success，sub_type 为可发布类型）
//   - 检查该租户/品牌是否启用自动发布（system_settings.auto_publish_enabled）
//   - 如果启用，自动创建发布任务（半自动模式，生成预填URL）
//   - 发布成功后自动触发监测（通过 MonitorTrigger）
package scheduledtask

import (
	"context"
	"encoding/json"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/account"
	"webreaper/internal/usecase/port"
)

// AutoPublishTask 生成→发布自动闭环任务。
type AutoPublishTask struct {
	genRepo     port.GenerationTaskRepository
	jobRepo     port.PublishJobRepository
	accountRepo port.AccountRepository
	publishUC   *account.PublishUseCase
	settingRepo port.SystemSettingRepository
	logger      port.Logger
}

// NewAutoPublishTask 创建自动发布任务。
func NewAutoPublishTask(
	genRepo port.GenerationTaskRepository,
	jobRepo port.PublishJobRepository,
	accountRepo port.AccountRepository,
	publishUC *account.PublishUseCase,
	settingRepo port.SystemSettingRepository,
	logger port.Logger,
) *AutoPublishTask {
	return &AutoPublishTask{
		genRepo:     genRepo,
		jobRepo:     jobRepo,
		accountRepo: accountRepo,
		publishUC:   publishUC,
		settingRepo: settingRepo,
		logger:      logger,
	}
}

func (t *AutoPublishTask) Name() string            { return "auto-publish" }
func (t *AutoPublishTask) Interval() time.Duration { return 5 * time.Minute }

// 可自动发布的生成类型（视频/图片类，音频/主体类不自动发布）。
var autoPublishableTypes = map[string]bool{
	entity.GenerationTypeVideo: true,
	entity.GenerationTypeImage: true,
}

func (t *AutoPublishTask) Execute(ctx context.Context) error {
	if t.publishUC == nil || t.settingRepo == nil {
		return nil
	}

	// 检查全局自动发布开关
	setting, err := t.settingRepo.Get(ctx, "auto_publish_enabled")
	if err != nil || setting.Value != "true" {
		return nil // 未启用
	}

	// 获取全部租户的已完成生成任务（最近1小时内）
	since := time.Now().Add(-1 * time.Hour)
	tasks, err := t.genRepo.ListActive(ctx, 100)
	if err != nil {
		return err
	}

	published := 0
	for _, task := range tasks {
		if task.State != entity.TaskStateSuccess {
			continue
		}
		if !autoPublishableTypes[task.Type] {
			continue
		}
		if task.FinishedAt == nil || task.FinishedAt.Before(since) {
			continue // 只处理最近1小时完成的任务
		}

		// 检查是否已创建过发布任务（防重复）
		existingJobs, _ := t.jobRepo.ListByTenant(ctx, task.TenantID, 100)
		alreadyPublished := false
		for _, job := range existingJobs {
			if job.ContentID == task.ID {
				alreadyPublished = true
				break
			}
		}
		if alreadyPublished {
			continue
		}

		// 检查租户是否有可用账号
		accounts, _ := t.accountRepo.ListByPlatform(ctx, task.TenantID, "")
		if len(accounts) == 0 {
			continue // 无可用账号，跳过
		}

		// 创建发布任务（半自动模式）
		creations := parseCreations(task.CreationsJSON)
		if len(creations) == 0 {
			continue
		}

		job := entity.PublishJob{
			ID:        "pj-auto-" + task.ID,
			TenantID:  task.TenantID,
			AccountID: accounts[0].ID,
			Platform:  "", // 自动选择
			ContentID: task.ID,
			BrandID:   task.BrandID,
			Title:     "AI生成内容-" + task.SubType,
			Content:   "",
			Mode:      entity.PublishModeSemiAuto,
			Status:    entity.PublishStatusPending,
			CreatedAt: time.Now(),
		}

		if err := t.jobRepo.Save(ctx, job); err != nil {
			t.logger.Warn("自动发布任务创建失败",
				port.String("task_id", task.ID),
				port.Err(err))
			continue
		}
		published++
	}

	if published > 0 {
		t.logger.Info("自动发布任务已创建",
			port.Int("count", published))
	}
	return nil
}

// parseCreations 解析 creations JSON。
func parseCreations(jsonStr string) []map[string]any {
	if jsonStr == "" {
		return nil
	}
	var creations []map[string]any
	json.Unmarshal([]byte(jsonStr), &creations)
	return creations
}
