// transfer_retry.go 产物转存补偿任务（缺口A修复）。
//
// 背景：applyStatus 首次转存失败仅记 WARN 不阻断终态——任务 success 但产物只有
// 24h 临时 URL。本任务每小时扫描该类任务重试 DownloadAndStore（含 data: URI——缺口B），
// 在 Vidu 24h URL 失效窗口内救回。
package scheduledtask

import (
	"context"
	"time"

	"webreaper/internal/usecase/generation"
	"webreaper/internal/usecase/port"
)

// TransferRetryTask 产物转存补偿（每小时）。
type TransferRetryTask struct {
	uc     *generation.GenerationUseCase
	logger port.Logger
}

func NewTransferRetryTask(uc *generation.GenerationUseCase, logger port.Logger) *TransferRetryTask {
	return &TransferRetryTask{uc: uc, logger: logger}
}

func (t *TransferRetryTask) Name() string            { return "transfer-retry" }
func (t *TransferRetryTask) Interval() time.Duration { return time.Hour }

func (t *TransferRetryTask) Execute(ctx context.Context) error {
	if t.uc == nil {
		return nil
	}
	// 只救 24h 窗口内的（Vidu 产物 URL 24h 失效——窗口外重试必然 403）
	fixed, err := t.uc.RetryPendingTransfers(ctx, time.Now().Add(-23*time.Hour), 100)
	if err != nil {
		t.logger.Warn("转存补偿扫描失败", port.Err(err))
		return err
	}
	if fixed > 0 {
		t.logger.Info("转存补偿完成", port.Int("fixed", fixed))
	}
	return nil
}
