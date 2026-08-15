// Package bing 提供 Bing Webmaster API 的收录状态检查（port.IndexStatusChecker 实现）。
//
// API：GetUrlInfo
//   GET https://ssl.bing.com/webmaster/api.svc/json/GetUrlInfo
//       ?apikey={key}&siteUrl={site}&url={url}
//   返回 UrlInfo 对象：LastCrawledDate 有效 = 已被 Bing 抓取
//   （公开 API 可拿到的最强收录信号；Bing 不公开"搜索索引位次"数据）。
//
// 迁移记录（2026-08-14）：早期实现使用 GetUrlSubmissionStatus 端点，该端点
// 在公开 API 中不存在（实测返回 HTML "Endpoint not found"），导致所有查询
// 恒为 error、收录验证形同虚设。已迁移至 GetUrlInfo 并基于真实响应样本重写解析。
package bing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

// Checker Bing 站长 API 收录状态检查器。
type Checker struct {
	apiKey string
	site   string // 已验证站点（如 https://content.example.com）
	client *http.Client
}

func NewChecker(apiKey, site string) *Checker {
	return &Checker{
		apiKey: apiKey,
		site:   site,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// CheckURLs 批量查询收录状态（Bing API 单 URL 查询，串行；数量少可接受）。
// 未配置 key/site 时返回空 map（任务空转）。
func (c *Checker) CheckURLs(ctx context.Context, urls []string) (map[string]string, error) {
	if c.apiKey == "" || c.site == "" {
		return nil, nil
	}
	result := make(map[string]string, len(urls))
	for _, u := range urls {
		status, err := c.checkOne(ctx, u)
		if err != nil {
			result[u] = "error" // 单条失败不阻断整体
			continue
		}
		result[u] = status
	}
	return result, nil
}

// checkOne 传输层：调 GetUrlInfo 查询单条 URL，返回收录状态。
// 传输职责（URL 拼接 / 超时 / 状态码）与解析职责（parseBody）分离，便于单测。
func (c *Checker) checkOne(ctx context.Context, pageURL string) (string, error) {
	endpoint := fmt.Sprintf("https://ssl.bing.com/webmaster/api.svc/json/GetUrlInfo?apikey=%s&siteUrl=%s&url=%s",
		url.QueryEscape(c.apiKey), url.QueryEscape(c.site), url.QueryEscape(pageURL))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "error", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "error", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "error", fmt.Errorf("Bing API 返回 %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return c.parseBody(body), nil
}

// urlInfoResponse GetUrlInfo 响应契约。
// 用显式类型描述响应而非 `any` 类型体操——弱类型解析会在 API 变更时悄悄错判，
// 2026-08-14 的端点迁移 bug（解析 HTML 错误页失败→恒 error）即源于此。
type urlInfoResponse struct {
	D         *urlInfo `json:"d"` // null = Bing 从未发现该 URL
	ErrorCode int      `json:"ErrorCode"`
	Message   string   `json:"Message"` // 如 "ERROR!!! InvalidApiKey"
}

// urlInfo Bing UrlInfo 对象（只保留判定所需字段）。
type urlInfo struct {
	DiscoveryDate   string `json:"DiscoveryDate"`   // .NET JSON 日期，如 /Date(1786604400000-0700)/
	LastCrawledDate string `json:"LastCrawledDate"` // .NET JSON 日期；MinValue = 从未抓取
	HttpStatus      int    `json:"HttpStatus"`
	IsPage          bool   `json:"IsPage"`
	URL             string `json:"Url"`
}

// parseBody 解析 GetUrlInfo 响应为收录状态（纯函数，无 IO，可单测）。
// 状态语义：
//   - 解析失败 / ErrorCode ≠ 0         → "error"（密钥无效 / 限流 / 站点未验证）
//   - {"d": null}                      → "pending"（Bing 从未发现该 URL）
//   - LastCrawledDate 有效（正毫秒）   → "indexed"（已被抓取）
//   - LastCrawledDate 为 MinValue      → "pending"（已发现但从未抓取）
func (c *Checker) parseBody(body []byte) string {
	var resp urlInfoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "error"
	}
	if resp.ErrorCode != 0 {
		return "error"
	}
	if resp.D == nil {
		return "pending"
	}
	if dotnetMillis(resp.D.LastCrawledDate) > 0 {
		return "indexed"
	}
	return "pending"
}

// dotnetMillis 解析 .NET JSON 日期 "/Date(毫秒[±时区])/" 的毫秒数。
// 返回 0 表示"无日期"：格式非法，或值为 DateTime.MinValue（-62135568000000，
// 即 0001-01-01，Bing 用它表示"从未抓取"——实测未收录 URL 的 LastCrawledDate）。
var dotnetDateRe = regexp.MustCompile(`^/Date\((-?\d+)([+-]\d+)?\)/$`)

func dotnetMillis(s string) int64 {
	m := dotnetDateRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	ms, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil || ms <= 0 { // 非正毫秒（含 MinValue）视为"无日期"
		return 0
	}
	return ms
}
