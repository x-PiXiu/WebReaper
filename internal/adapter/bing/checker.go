// Package bing 提供 Bing Webmaster API 的收录状态检查（port.IndexStatusChecker 实现）。
//
// API：GetUrlSubmissionStatus
//   GET https://ssl.bing.com/webmaster/api.svc/json/GetUrlSubmissionStatus
//       ?apikey={key}&siteUrl={site}&url={url}
//   返回：null=未收录；"Error"=查询失败；"UrlSubmissionStatus" 对象=已收录
package bing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func (c *Checker) checkOne(ctx context.Context, pageURL string) (string, error) {
	endpoint := fmt.Sprintf("https://ssl.bing.com/webmaster/api.svc/json/GetUrlSubmissionStatus?apikey=%s&siteUrl=%s&url=%s",
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
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return c.parseBody(body), nil
}

// parseBody 解析 Bing 收录状态响应：
//   - null：已提交未收录（pending）
//   - "Error"：查询失败（error）
//   - 其他对象：已收录（indexed）
func (c *Checker) parseBody(body []byte) string {
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "error"
	}
	switch v := parsed.(type) {
	case nil:
		return "pending"
	case string:
		if v == "Error" {
			return "error"
		}
		return "pending"
	default:
		return "indexed"
	}
}
