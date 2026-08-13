// 收录验证任务：每日查询已发布内容的 Bing 收录状态，回写内容表。
//
// 效果追踪闭环（提交 ≠ 收录）：IndexNow 提交只保证"已通知"，
// 商户需要知道自己的内容是否真的被搜索引擎收录——这是付费说服力的关键一环。
// 未配置 BING_API_KEY / BING_SITE_URL 时任务空转（不报错）。
//
// 收录成功通知：内容从 pending → indexed 时推送站内通知（主动唤醒——
// "内容已被收录"是 GEO 付费说服力的关键事件，参照 DailyMonitorTask 的
// SetNotifier 模式；未注入通知器则静默回写）。
package scheduledtask

import (
	"context"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/notification"
	"webreaper/internal/usecase/port"
)

// IndexCheckTask 收录状态验证（每日一次）。
type IndexCheckTask struct {
	contentRepo port.OptimizedContentRepository
	checker     port.IndexStatusChecker
	baseURL     string // 公开站根地址（拼每篇文章 URL）
	notify      *notification.NotifyUseCase // 收录成功通知（可选；nil=静默回写）
	logger      port.Logger
}

// indexedMeta 收录回写所需的内容元信息（URL → 租户/内容映射）。
type indexedMeta struct {
	tenantID, id, title string
}

func NewIndexCheckTask(contentRepo port.OptimizedContentRepository, checker port.IndexStatusChecker, baseURL string, logger port.Logger) *IndexCheckTask {
	return &IndexCheckTask{
		contentRepo: contentRepo,
		checker:     checker,
		baseURL:     baseURL,
		logger:      logger,
	}
}

// SetNotifier 注入站内通知器（内容被收录时主动唤醒商户；可选）。
func (t *IndexCheckTask) SetNotifier(n *notification.NotifyUseCase) {
	t.notify = n
}

func (t *IndexCheckTask) Name() string            { return "index-status-check" }
func (t *IndexCheckTask) Interval() time.Duration { return 24 * time.Hour }

func (t *IndexCheckTask) Execute(ctx context.Context) error {
	if t.checker == nil || t.baseURL == "" {
		return nil // 未配置 Bing 凭据：空转
	}
	contents, err := t.contentRepo.ListPublished(ctx)
	if err != nil {
		return fmt.Errorf("枚举已发布内容失败: %w", err)
	}
	if len(contents) == 0 {
		return nil
	}

	// 只验证尚未确认收录的（indexed 的跳过，节省 API 配额）
	urls := make([]string, 0, len(contents))
	urlOf := make(map[string]indexedMeta, len(contents))
	for _, c := range contents {
		if c.IndexStatus == entity.IndexStatusIndexed {
			continue
		}
		u := strings.TrimRight(t.baseURL, "/") + "/public/articles/" + c.ID
		urls = append(urls, u)
		urlOf[u] = indexedMeta{c.TenantID, c.ID, c.Title}
	}
	if len(urls) == 0 {
		return nil // 全部已收录
	}

	statusMap, err := t.checker.CheckURLs(ctx, urls)
	if err != nil {
		return fmt.Errorf("收录状态查询失败: %w", err)
	}
	indexed, pending, failed := 0, 0, 0
	for u, status := range statusMap {
		m, ok := urlOf[u]
		if !ok {
			continue
		}
		switch status {
		case entity.IndexStatusIndexed:
			indexed++
			_ = t.contentRepo.UpdateIndexStatus(ctx, m.tenantID, m.id, entity.IndexStatusIndexed, time.Now())
			t.notifyIndexed(ctx, m)
		case entity.IndexStatusError:
			failed++
		default:
			pending++
			_ = t.contentRepo.UpdateIndexStatus(ctx, m.tenantID, m.id, entity.IndexStatusPending, time.Time{})
		}
	}
	t.logger.Info("收录状态验证完成",
		port.Int("checked", len(urls)),
		port.Int("indexed", indexed),
		port.Int("pending", pending),
		port.Int("failed", failed),
	)
	return nil
}

// notifyIndexed 内容确认被收录时推送站内通知（付费说服力事件；尽力而为）。
func (t *IndexCheckTask) notifyIndexed(ctx context.Context, m indexedMeta) {
	if t.notify == nil {
		return
	}
	title := m.title
	if title == "" {
		title = m.id
	}
	_ = t.notify.Push(ctx, m.tenantID, entity.NotificationTypeContentIndexed,
		"内容已被搜索引擎收录 🎉",
		fmt.Sprintf("你的内容《%s》已被 Bing 收录——AI 搜索引擎可直接引用，曝光机会提升。", title),
		"/m/content",
	)
}
