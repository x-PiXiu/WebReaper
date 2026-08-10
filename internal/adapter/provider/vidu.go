// Package provider 提供外部生成/发布服务的适配器实现（策略替换点）。
//
// 当前策略：Vidu（视频生成）——用户配置 VIDU_API_KEY 后走真实 API，
// 未配置时使用 MockVideoProvider（模拟进度，开发/演示可用）。
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ViduProvider 基于 Vidu 开放平台（api.vidu.cn）的视频生成适配器。
//
// API 依据 Docs/第三方/Vidu（创建视频任务/任务管理）：
//   - 创建：POST /ent/v2/text2video（文生视频）/ /ent/v2/img2video（图生视频）
//   - 轮询：GET /ent/v2/tasks/{id}/creations（state: created/queueing/processing/success/failed）
type ViduProvider struct {
	apiKey string
	model  string // 默认模型（viduq3-pro / viduq3-turbo ...）
	client *http.Client
}

// NewViduProvider 创建 Vidu 适配器。model 为空时默认 viduq3-pro。
func NewViduProvider(apiKey, model string) *ViduProvider {
	if model == "" {
		model = "viduq3-pro"
	}
	return &ViduProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *ViduProvider) Name() string { return "vidu" }

// Submit 提交生成任务：text 模式走文生视频，material 模式走图生视频。
func (p *ViduProvider) Submit(ctx context.Context, mode, prompt, materialURL string) (string, error) {
	endpoint := "https://api.vidu.cn/ent/v2/text2video"
	body := map[string]any{
		"model":  p.model,
		"prompt": prompt,
	}
	if mode == "material" {
		// 文档要求 images 为 URL 数组（图生视频：1 张；首尾帧/参考生视频：多张）
		endpoint = "https://api.vidu.cn/ent/v2/img2video"
		if materialURL == "" {
			return "", fmt.Errorf("图生视频缺少素材图 URL")
		}
		body["images"] = []string{materialURL}
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Vidu 创建任务请求失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Vidu 创建任务失败 HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var parsed struct {
		TaskID string `json:"task_id"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("解析 Vidu 创建响应失败: %w", err)
	}
	if parsed.TaskID == "" {
		return "", fmt.Errorf("Vidu 响应缺少 task_id: %s", string(raw))
	}
	return parsed.TaskID, nil
}

// Poll 查询生成进度（GET /tasks/{id}/creations）。
func (p *ViduProvider) Poll(ctx context.Context, taskID string) (float64, bool, string, error) {
	endpoint := fmt.Sprintf("https://api.vidu.cn/ent/v2/tasks/%s/creations", url.PathEscape(taskID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, false, "", err
	}
	req.Header.Set("Authorization", "Token "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, false, "", fmt.Errorf("Vidu 查询任务失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode != http.StatusOK {
		return 0, false, "", fmt.Errorf("Vidu 查询任务 HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var parsed struct {
		State     string `json:"state"`
		ErrCode   string `json:"err_code"`
		Creations []struct {
			URL string `json:"url"`
		} `json:"creations"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return 0, false, "", fmt.Errorf("解析 Vidu 查询响应失败: %w", err)
	}
	switch parsed.State {
	case "success":
		if len(parsed.Creations) == 0 || parsed.Creations[0].URL == "" {
			return 0, false, "", fmt.Errorf("Vidu 任务成功但无生成物")
		}
		return 1, true, parsed.Creations[0].URL, nil
	case "failed":
		return 0, false, "", fmt.Errorf("Vidu 任务失败: err_code=%s", parsed.ErrCode)
	default: // created / queueing / processing
		return 0.5, false, "", nil
	}
}
