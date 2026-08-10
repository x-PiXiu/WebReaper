package port

import "context"

// IndexStatusChecker 收录状态检查器（策略：Bing 站长 API / 手动标记）。
//
// 设计动机（效果追踪闭环：提交成功 ≠ 被收录）：
//   - IndexNow 只保证"已通知"，是否真正被索引需要验证
//   - 定时任务每日查询已发布内容的收录状态，结果展示给商户/管理后台
type IndexStatusChecker interface {
	// CheckURLs 批量查询 URL 收录状态。
	// 返回 map[url]status（indexed / pending / error），单条失败不阻断整体。
	CheckURLs(ctx context.Context, urls []string) (map[string]string, error)
}
