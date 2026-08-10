// Package urlsubmit 提供"收录提交"的渠道实现（百度 API 推送 + 多通道组合）。
//
// 整洁架构定位：本包是 adapter 层的"框架与驱动"——各搜索引擎的推送协议
// （HTTP 细节、分片、响应解析）全部封在这里，用例层只依赖 port.URLSubmitter。
// 新增收录渠道 = 在此包新增一个实现 + main 装配，用例层零改动（开闭原则）。
package urlsubmit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"webreaper/internal/usecase/port"
)

// baiduPushEndpoint 百度主动推送 API（数据开放平台，token 绑定已验证域名）。
// 参考 https://ziyuan.baidu.com/ 文档：POST text/plain，每行一个 URL。
const baiduPushEndpoint = "http://data.zz.baidu.com/urls"

// baiduChunkSize 百度单次提交上限（文档规定 2000 条/次）。
const baiduChunkSize = 2000

// BaiduSubmitter 是 port.URLSubmitter 的百度主动推送实现。
//
// 特点（对比 IndexNow）：
//   - 国内主流量渠道（百度收录主要来源）
//   - token 与站点绑定（ziyuan.baidu.com 获取准入 token）
//   - 单次 ≤2000 条，每日配额约 10 万（与手动提交共享）
//   - 响应 JSON 含 success/remain/not_same_site/not_valid 统计
type BaiduSubmitter struct {
	site       string // 已验证域名（如 content.example.com）
	token      string // 准入 token
	httpClient *http.Client
}

// NewBaiduSubmitter 创建百度提交器（校验 site/token 非空）。
func NewBaiduSubmitter(site, token string) (*BaiduSubmitter, error) {
	if site == "" {
		return nil, fmt.Errorf("baidu site 不能为空")
	}
	if token == "" {
		return nil, fmt.Errorf("baidu token 不能为空")
	}
	return &BaiduSubmitter{
		site:       site,
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// baiduPushResponse 百度推送响应（success=成功数，remain=当日剩余配额）。
type baiduPushResponse struct {
	Remain      int      `json:"remain"`
	Success     int      `json:"success"`
	NotSameSite []string `json:"not_same_site"`
	NotValid    []string `json:"not_valid"`
}

// SubmitURLs 提交 URL 列表（自动按 2000 条分片；部分失败返回聚合错误）。
func (s *BaiduSubmitter) SubmitURLs(ctx context.Context, urls []string) error {
	if len(urls) == 0 {
		return nil
	}
	var errs []error
	for _, chunk := range chunkURLs(urls, baiduChunkSize) {
		if err := s.submitChunk(ctx, chunk); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrors(errs)
}

// submitChunk 提交一个分片（text/plain 每行一 URL）。
func (s *BaiduSubmitter) submitChunk(ctx context.Context, chunk []string) error {
	var body bytes.Buffer
	for i, u := range chunk {
		if i > 0 {
			body.WriteByte('\n')
		}
		body.WriteString(u)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baiduPushEndpoint+"?site="+s.site+"&token="+s.token, &body)
	if err != nil {
		return fmt.Errorf("create baidu request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("baidu push: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("baidu push HTTP %d: %s", resp.StatusCode, truncateBytes(data, 200))
	}
	var pr baiduPushResponse
	if err := json.Unmarshal(data, &pr); err != nil {
		return fmt.Errorf("baidu push 响应解析失败: %s", truncateBytes(data, 200))
	}
	if pr.Success != len(chunk) {
		return fmt.Errorf("baidu push 部分失败: success=%d/%d remain=%d not_valid=%v not_same_site=%v",
			pr.Success, len(chunk), pr.Remain, pr.NotValid, pr.NotSameSite)
	}
	return nil
}

// chunkURLs 按 chunkSize 分片（纯函数，可单测）。
func chunkURLs(urls []string, chunkSize int) [][]string {
	if chunkSize <= 0 {
		chunkSize = 1
	}
	var chunks [][]string
	for i := 0; i < len(urls); i += chunkSize {
		end := i + chunkSize
		if end > len(urls) {
			end = len(urls)
		}
		chunks = append(chunks, urls[i:end])
	}
	return chunks
}

// truncateBytes 截断响应体（错误信息展示用，避免刷屏）。
func truncateBytes(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	return string(b[:maxLen]) + "..."
}

var _ port.URLSubmitter = (*BaiduSubmitter)(nil)
