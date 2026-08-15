package scheduledtask

import (
	"context"
	"time"

	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/port"
)

// GenerationPollTask 生成任务轮询驱动（统一调度器注册）。
//
// 设计（Docs/Plans/03 §3.3）：生成任务分钟级完成——用 15-30s 短间隔轮询
// 未终态任务（回调到达后终态幂等跳过；双通道合并）。阶段 1 单机扫描，
// 多实例部署时由分布式锁保证单实例轮询。
type GenerationPollTask struct {
	uc       *generation.GenerationUseCase
	logger   port.Logger
	interval time.Duration
}

// NewGenerationPollTask 创建轮询任务（interval 默认 20s）。
func NewGenerationPollTask(uc *generation.GenerationUseCase, logger port.Logger) *GenerationPollTask {
	return &GenerationPollTask{uc: uc, logger: logger, interval: 20 * time.Second}
}

func (t *GenerationPollTask) Name() string { return "generation-poll" }

func (t *GenerationPollTask) Interval() time.Duration { return t.interval }

func (t *GenerationPollTask) Execute(ctx context.Context) error {
	if t.uc == nil {
		return nil
	}
	n, err := t.uc.PollDue(ctx, 50)
	if err != nil && t.logger != nil {
		t.logger.Warn("生成任务轮询失败", port.String("err", err.Error()))
		return nil // 轮询失败不致命（下轮重试）
	}
	if n > 0 && t.logger != nil {
		t.logger.Info("生成任务轮询完成", port.Int("updated", n))
	}
	// 自动重试执行器（F-fix：ClassifyError/CanAutoRetry 此前无调用方——
	// 限流/内部错误类失败按 1/5/30 分钟退避自动重提，≤3 次）
	if r, rErr := t.uc.RetryDue(ctx, 20); rErr == nil && r > 0 && t.logger != nil {
		t.logger.Info("生成任务自动重试已重提", port.Int("retried", r))
	}
	return nil
}
