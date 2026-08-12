package scheduledtask

import (
	"context"
	"strconv"
	"time"

	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/port"
)

// GenerationCleanupTask 生成任务清理（P3——避免 generation_tasks 无限增长）。
//
// 策略（Docs/Plans/03 §P3）：每日清理 30 天前的终态任务（success/failed/cancelled）
// + 同阈值过期素材文件。活跃任务不动。
type GenerationCleanupTask struct {
	uc         *generation.GenerationUseCase
	logger     port.Logger
	interval   time.Duration
	retainDays int
}

// NewGenerationCleanupTask 创建清理任务（24h 间隔，保留 30 天）。
func NewGenerationCleanupTask(uc *generation.GenerationUseCase, logger port.Logger) *GenerationCleanupTask {
	return &GenerationCleanupTask{uc: uc, logger: logger, interval: 24 * time.Hour, retainDays: 30}
}

func (t *GenerationCleanupTask) Name() string            { return "generation-cleanup" }
func (t *GenerationCleanupTask) Interval() time.Duration { return t.interval }

func (t *GenerationCleanupTask) Execute(ctx context.Context) error {
	if t.uc == nil {
		return nil
	}
	tasks, files, err := t.uc.CleanupOldTasks(ctx, t.retainDays)
	if err != nil {
		if t.logger != nil {
			t.logger.Warn("生成任务清理失败", port.String("err", err.Error()))
		}
		return nil // 清理失败不致命（下轮重试）
	}
	if (tasks > 0 || files > 0) && t.logger != nil {
		t.logger.Info("生成任务清理完成",
			port.String("tasks", strconv.FormatInt(tasks, 10)),
			port.String("files", strconv.Itoa(files)),
			port.String("retain_days", strconv.Itoa(t.retainDays)))
	}
	return nil
}
