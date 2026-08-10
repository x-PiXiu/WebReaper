package port

import "context"

// URLSubmitter 是"收录通知"接口（内容发布后通知搜索引擎收录）。
//
// 实现：IndexNow（Bing/Yandex 免费协议）——发布即通知，替代人工提交 sitemap。
// 用例层只依赖此接口；未配置 Key 时不注入（nil 安全，发布流程不受影响）。
type URLSubmitter interface {
	// SubmitURLs 提交一批新页面的 URL 通知收录（尽力而为：失败不阻断业务）。
	SubmitURLs(ctx context.Context, urls []string) error
}
