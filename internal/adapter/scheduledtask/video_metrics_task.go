package scheduledtask

import (
	"context"
	"time"

	"webreaper/internal/usecase/account"
	"webreaper/internal/usecase/port"
)

// VideoMetricsTask 每日批量回读已发布作品的互动数据（播放/点赞/评论/分享）。
//
// 数据源：SocialSearcher.GetVideoDetail（站内详情接口——MediaCrawler 协议复刻），
// 需要对应平台的健康 cookie 账号（OAuth 无 video.data 权限的现实约束）。
// 手动刷新（详情 Drawer「立即刷新」）走同一用例方法 RefreshJobMetrics。
type VideoMetricsTask struct {
	uc     *account.PublishUseCase
	logger port.Logger
}

func NewVideoMetricsTask(uc *account.PublishUseCase, logger port.Logger) *VideoMetricsTask {
	return &VideoMetricsTask{uc: uc, logger: logger}
}

func (t *VideoMetricsTask) Name() string            { return "video-metrics-readback" }
func (t *VideoMetricsTask) Interval() time.Duration { return 24 * time.Hour }
func (t *VideoMetricsTask) Execute(ctx context.Context) error {
	t.logger.Info("视频互动数据回读任务启动（全租户已发布作品）", port.String("task", t.Name()))
	t.uc.RunMetricsReadback(ctx)
	return nil
}
