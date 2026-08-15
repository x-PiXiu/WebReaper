package urlsubmit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"webreaper/internal/usecase/port"
)

// bingSubmitEndpoint Bing URL Submission API 批量提交端点（var 以便测试注入）。
// 参考 https://www.bing.com/webmasters/url-submission-api
var bingSubmitEndpoint = "https://ssl.bing.com/webmaster/api.svc/json/SubmitUrlBatch"

// bingChunkSize Bing 单次批量提交上限（文档规定单请求最多 100 条）。
const bingChunkSize = 100

// BingSubmitter 是 port.URLSubmitter 的 Bing URL Submission API 实现。
//
// 定位（IndexNow 的兜底渠道）：
//   - 仅 Bing；与 Bing Webmaster Tools 后台"URL 提交"共享每日 100 条配额
//     （GetUrlSubmissionQuota 实测：DailyQuota=100 / MonthlyQuota=1800）
//   - 同一账号 API key 即可（与收录验证 GetUrlInfo 共用 BING_API_KEY / BING_SITE_URL）
//   - HTTP 200 = 已受理进爬取队列，**不保证收录**——与 IndexNow 同为"通知"语义，
//     是否进入索引由 Bing 按内容质量决定
type BingSubmitter struct {
	apiKey     string
	site       string // 已验证站点（如 https://content.example.com）
	httpClient *http.Client
}

// NewBingSubmitter 创建 Bing 提交器（校验 apiKey/site 非空）。
func NewBingSubmitter(apiKey, site string) (*BingSubmitter, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("bing apiKey 不能为空")
	}
	if site == "" {
		return nil, fmt.Errorf("bing site 不能为空")
	}
	return &BingSubmitter{
		apiKey:     apiKey,
		site:       site,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// bingSubmitRequest SubmitUrlBatch 请求体（JSON）。
type bingSubmitRequest struct {
	SiteURL string   `json:"siteUrl"`
	URLList []string `json:"urlList"`
}

// SubmitURLs 提交 URL 列表（自动按 100 条分片；部分失败返回聚合错误）。
func (s *BingSubmitter) SubmitURLs(ctx context.Context, urls []string) error {
	if len(urls) == 0 {
		return nil
	}
	var errs []error
	for _, chunk := range chunkURLs(urls, bingChunkSize) {
		if err := s.submitChunk(ctx, chunk); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrors(errs)
}

// submitChunk 提交一个分片（JSON：siteUrl + urlList；apikey 走查询参数）。
func (s *BingSubmitter) submitChunk(ctx context.Context, chunk []string) error {
	payload := bingSubmitRequest{SiteURL: s.site, URLList: chunk}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal bing request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		bingSubmitEndpoint+"?apikey="+url.QueryEscape(s.apiKey), bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create bing request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("bing submit: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	// 200 = 已受理；429 = 配额用尽；其他 = 失败
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bing submit HTTP %d: %s", resp.StatusCode, truncateBytes(body, 200))
	}
	return nil
}

var _ port.URLSubmitter = (*BingSubmitter)(nil)
