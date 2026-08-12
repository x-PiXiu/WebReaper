// Package indexnow 提供 IndexNow 协议实现（内容发布后自动通知 Bing/Yandex 收录）。
//
// IndexNow 协议（https://www.indexnow.org）：
//   - POST https://api.indexnow.org/indexnow
//   - JSON: {host, key, keyLocation, urlList}
//   - key 是站点专属密钥，需在 keyLocation 指向的 URL 托管一个包含 key 的纯文本文件
//   - Bing/Yandex/Seznam 免费支持；提交后新页面在数小时~数天内被收录
//
// 整洁架构定位：本包是 adapter 层的"框架与驱动"——HTTP 细节全部封在这里，
// 用例层只依赖 port.URLSubmitter 接口。
package indexnow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"webreaper/internal/usecase/port"
)

// indexNowEndpoint IndexNow 提交端点。
const indexNowEndpoint = "https://api.indexnow.org/indexnow"

// indexNowKeyPattern IndexNow key 格式（文档要求）：
// 8-128 个十六进制字符（a-z/A-Z/0-9 和 `-`）。提交前校验，格式错直接报错不浪费请求。
var indexNowKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9-]{8,128}$`)

// Submitter 是 port.URLSubmitter 的 IndexNow 实现。
type Submitter struct {
	host        string // 站点域名（不含协议和路径，如 content.example.com）
	key         string // IndexNow 密钥
	keyLocation string // 密钥文件 URL（IndexNow 会访问它校验）
	httpClient  *http.Client
}

// NewSubmitter 创建 IndexNow 提交器。
// host 从 PublicBaseURL 解析（去掉协议/路径）；keyLocation 由调用方提供
// （指向本服务的 /public/indexnow-key.txt 端点）。
// key 必须符合 IndexNow 格式（8-128 个 a-zA-Z0-9-），否则直接报错。
func NewSubmitter(baseURL, key, keyLocation string) (*Submitter, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("无效的 PublicBaseURL: %s", baseURL)
	}
	if !indexNowKeyPattern.MatchString(key) {
		return nil, fmt.Errorf("IndexNow key 格式无效（需 8-128 个字母/数字/连字符）: %q", key)
	}
	return &Submitter{
		host:        u.Host,
		key:         key,
		keyLocation: keyLocation,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// indexNowRequest IndexNow 协议请求体。
type indexNowRequest struct {
	Host        string   `json:"host"`
	Key         string   `json:"key"`
	KeyLocation string   `json:"keyLocation"`
	URLList     []string `json:"urlList"`
}

// jsonMarshal 序列化（测试用独立函数，避免测试 import encoding/json 细节）。
func jsonMarshal(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

// SubmitURLs 提交 URL 列表（尽力而为：网络错误/非 2xx 返回错误但不 panic）。
func (s *Submitter) SubmitURLs(ctx context.Context, urls []string) error {
	if len(urls) == 0 {
		return nil
	}
	body, err := json.Marshal(indexNowRequest{
		Host:        s.host,
		Key:         s.key,
		KeyLocation: s.keyLocation,
		URLList:     urls,
	})
	if err != nil {
		return fmt.Errorf("marshal indexnow request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, indexNowEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create indexnow request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("indexnow submit: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		// 429 = IndexNow 服务限流（常见：同一 key 提交过频，或 keyLocation 公网不可验证）。
		// 商户端提示要可读——裸 "HTTP 429" 用户无法理解。
		if resp.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("收录服务繁忙（限流），请稍后再试或检查收录配置")
		}
		return fmt.Errorf("indexnow submit HTTP %d", resp.StatusCode)
	}
	return nil
}

var _ port.URLSubmitter = (*Submitter)(nil)
