package crawler

import "fmt"

// crawlErr 统一构造爬虫失败错误。
//
// 背景：爬虫（colly / net/http）的失败有两类——
//   - 网络/超时错误（无 HTTP 响应）
//   - HTTP 状态码错误（4xx/5xx，可能仍返回错误页正文）
//
// 两者都必须显式上报，避免把"错误页正文"当成正常内容存入 DataItem
// （原缺陷：static_crawler 无 OnError 回调，4xx/5xx 异步错误被静默吞掉）。
//
// status 为 0 表示无响应（纯网络错误）；>0 表示收到了 HTTP 响应但状态码非 2xx。
func crawlErr(url string, status int, cause error) error {
	if cause != nil && status == 0 {
		// 纯网络错误（DNS/连接/超时）
		return fmt.Errorf("crawl %s failed: %w", url, cause)
	}
	if status > 0 {
		return fmt.Errorf("crawl %s returned HTTP %d: %v", url, status, cause)
	}
	return fmt.Errorf("crawl %s failed", url)
}
