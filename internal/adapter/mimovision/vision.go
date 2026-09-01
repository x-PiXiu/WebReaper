// Package mimovision 实现基于小米 MiMo 多模态对话的图片审核通道
// （32号 P2 第二批：port.VisionChat 适配器）。
//
// 2026-09-01 实测：mimo-v2.5 接受 OpenAI vision 格式（image_url 内容，
// data URL 形态），准确识别测试图——零新厂商依赖复用 MiMo 免费额度。
package mimovision

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

const defaultBaseURL = "https://token-plan-cn.xiaomimimo.com/v1/chat/completions"

// Client 是 port.VisionChat 的 MiMo 实现。
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func New(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{apiKey: apiKey, baseURL: baseURL, http: &http.Client{Timeout: 60 * time.Second}}
}

// ChatWithImage 单轮图片问答。公网 URL 直传；本站托管/私网地址由本适配器
// 下载转 data URL（外部厂商拉不到 localhost——与生成域内联同一问题域）。
func (c *Client) ChatWithImage(ctx context.Context, prompt, imageURL string) (string, error) {
	imgURL := imageURL
	if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
		return "", fmt.Errorf("图片地址无效: %s", truncate(imageURL, 80))
	}
	if isPrivateHost(imageURL) || strings.Contains(imageURL, "://localhost") || strings.Contains(imageURL, "://127.0.0.1") {
		dataURL, err := c.downloadAsDataURL(ctx, imageURL)
		if err != nil {
			return "", fmt.Errorf("私网图片转 data URL 失败: %w", err)
		}
		imgURL = dataURL
	}
	body := map[string]any{
		"model": "mimo-v2.5",
		"messages": []map[string]any{{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": prompt},
				{"type": "image_url", "image_url": map[string]any{"url": imgURL}},
			},
		}},
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return "", fmt.Errorf("MiMo vision HTTP %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("MiMo vision 响应无 choices")
	}
	return out.Choices[0].Message.Content, nil
}

// isPrivateHost 私网/本站判定（与生成域 SSRF 判定同语义——这些地址外部拉不到，
// 但本服务自身可下载）。仅用于"是否转 data URL"，不做安全拦截。
func isPrivateHost(u string) bool {
	l := strings.ToLower(u)
	return strings.Contains(l, "://localhost") || strings.Contains(l, "://127.0.0.1") ||
		strings.Contains(l, "://0.0.0.0") || strings.Contains(l, "://10.") ||
		strings.Contains(l, "://192.168.") || strings.Contains(l, "://172.16.")
}

func (c *Client) downloadAsDataURL(ctx context.Context, u string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 15<<20)) // 15MB 上限（与生成域内联同限）
	if err != nil {
		return "", err
	}
	mime := resp.Header.Get("Content-Type")
	if mime == "" || !strings.HasPrefix(mime, "image/") {
		mime = mimeFromExt(u)
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func mimeFromExt(u string) string {
	switch strings.ToLower(path.Ext(u)) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/jpeg"
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
