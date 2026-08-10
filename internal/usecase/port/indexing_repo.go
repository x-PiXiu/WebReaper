package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// IndexingLogRepository 收录提交日志仓储（审计排查用）。
type IndexingLogRepository interface {
	Save(ctx context.Context, log entity.IndexingSubmitLog) error
	// ListRecent 取最近 N 条提交记录（按时间倒序）。
	ListRecent(ctx context.Context, limit int) ([]entity.IndexingSubmitLog, error)
}
