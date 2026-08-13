package urlsubmit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// LoggingSubmitter 是 port.URLSubmitter 的审计日志装饰器。
//
// 设计（装饰器模式）：
//   - 包装任意 URLSubmitter，每次提交（成功/失败）都记录 IndexingSubmitLog——
//     发布/补提交等"自动触发"的提交因此进入管理后台"提交日志"页，可排查
//     "为什么没被收录"（此前只有手动补提交 ReSubmitAll 记日志，发布时的
//     自动提交无审计记录，失败也无从查起）。
//   - 不改 inner 行为，失败仍向上传播（调用方决定是否阻断）。
type LoggingSubmitter struct {
	inner   port.URLSubmitter
	logRepo port.IndexingLogRepository
}

// NewLoggingSubmitter 包装提交器并注入审计日志仓储。
func NewLoggingSubmitter(inner port.URLSubmitter, logRepo port.IndexingLogRepository) *LoggingSubmitter {
	return &LoggingSubmitter{inner: inner, logRepo: logRepo}
}

// SubmitURLs 转发提交并记录审计日志（成功/失败都记；失败不吞错误）。
func (s *LoggingSubmitter) SubmitURLs(ctx context.Context, urls []string) error {
	err := s.inner.SubmitURLs(ctx, urls)
	status := "success"
	var errMsg string
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}
	_ = s.logRepo.Save(ctx, entity.IndexingSubmitLog{
		ID:          fmt.Sprintf("ix-%d", time.Now().UnixNano()),
		Channel:     entity.IndexingChannelAuto,
		URL:         strings.Join(urls, ", "),
		Status:      status,
		ErrorMsg:    errMsg,
		SubmittedAt: time.Now(),
	})
	return err
}

var _ port.URLSubmitter = (*LoggingSubmitter)(nil)
