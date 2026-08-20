// Package ai 提供 Agnes 2.5 Flash 视觉 LLM 实现（截图→分析→决策）。
//
// 用于 browsertools.VisionLLM 接口：AgentRecover 的"眼睛"。
// 看到 截图 → 理解 页面状态 → 决策 下一步浏览器操作。
//
// Agnes 2.5 Flash 特性（Docs/第三方/agnes）：
//   - Endpoint: POST https://apihub.agnes-ai.com/v1/chat/completions
//   - 模型名: agnes-2.5-flash
//   - 图像输入: messages[].content[].image_url（公开 URL 或 data URI）
//   - 兼容 OpenAI Chat Completions 格式
//   - 当前免费（$0/1M tokens）
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"webreaper/internal/adapter/publisher/browsertools"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- 请求/响应结构（OpenAI Chat Completions 兼容）----

type agnesMessage struct {
	Role    string         `json:"role"`
	Content []agnesContent `json:"content,omitempty"`
}

type agnesContent struct {
	Type     string        `json:"type"` // "text" 或 "image_url"
	Text     string        `json:"text,omitempty"`
	ImageURL *agnesImageURL `json:"image_url,omitempty"`
}

type agnesImageURL struct {
	URL string `json:"url"` // 公开 URL 或 data:image/png;base64,...
}

type agnesRequest struct {
	Model       string         `json:"model"`
	Messages    []agnesMessage `json:"messages"`
	Temperature float64        `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
}

type agnesResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// AgnesVisionLLM 是 browsertools.VisionLLM 的视觉 LLM 实现。
// 截图 → 视觉模型分析 → AgentDecision JSON。
//
// 支持两种构造方式：
//   - NewAgnesVisionLLM：直接传参（兼容测试/硬编码）
//   - NewDynamicVisionLLM：从 llm_configs 按 usage="vision" 动态读取（管理后台热切换，零重启）
//
// 转型说明：视觉模型与聊天模型独立配置——聊天模型坏了浏览器 Agent 不受影响，反之亦然。
type AgnesVisionLLM struct {
	apiKey  string
	baseURL string // 默认 https://apihub.agnes-ai.com/v1
	model   string // 默认 agnes-2.5-flash
	logger  port.Logger
	client  *http.Client
	repo    port.LLMConfigRepository // 可选：非 nil 时从 llm_configs 动态读取（30s TTL 热切换）
}

// 确保实现 browsertools.VisionLLM 接口。
var _ browsertools.VisionLLM = (*AgnesVisionLLM)(nil)

// NewAgnesVisionLLM 创建视觉 LLM（直接传参——兼容测试/硬编码）。
func NewAgnesVisionLLM(apiKey, baseURL, model string, logger port.Logger) *AgnesVisionLLM {
	if baseURL == "" {
		baseURL = "https://apihub.agnes-ai.com/v1"
	}
	if model == "" {
		model = "agnes-2.5-flash"
	}
	return &AgnesVisionLLM{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		logger:  logger,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// NewDynamicVisionLLM 从 llm_configs 按 usage="vision" 动态读取（管理后台热切换，零重启）。
// 管理后台配置一条 usage=vision 的 LLM → 30s 缓存过期后自动生效。
// 未配置时 IsConfigured()=false → AgentRecover 降级为 TryQuickRecover（零成本）。
func NewDynamicVisionLLM(repo port.LLMConfigRepository, logger port.Logger) *AgnesVisionLLM {
	return &AgnesVisionLLM{
		repo:   repo,
		logger: logger,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// IsConfigured 是否已配置（直接传参非空 或 从 llm_configs 能读到 vision 配置）。
func (v *AgnesVisionLLM) IsConfigured() bool {
	if v.repo != nil {
		return v.resolveConfig() != nil
	}
	return v.apiKey != ""
}

// resolveConfig 从 llm_configs 读取视觉模型配置（usage="vision"）。
// 管理后台改配置后 30s 缓存过期自动生效——零重启。
func (v *AgnesVisionLLM) resolveConfig() *entity.LLMConfig {
	if v.repo == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cfg, err := v.repo.FindByUsage(ctx, "vision")
	if err != nil || cfg.APIKey == "" {
		return nil
	}
	return &cfg
}

// effectiveConfig 返回生效的配置（动态优先，硬编码兜底）。
func (v *AgnesVisionLLM) effectiveConfig() (apiKey, baseURL, model string) {
	if cfg := v.resolveConfig(); cfg != nil {
		apiKey = cfg.APIKey
		baseURL = cfg.BaseURL
		model = cfg.Model
	} else {
		apiKey = v.apiKey
		baseURL = v.baseURL
		model = v.model
	}
	if apiKey == "" {
		return "", "", ""
	}
	if baseURL == "" {
		baseURL = "https://apihub.agnes-ai.com/v1"
	}
	if model == "" {
		model = "agnes-2.5-flash"
	}
	return apiKey, baseURL, model
}

// AnalyzeScreenshot 实现 VisionLLM 接口——截图 → 视觉模型 → AgentDecision。
// 使用 effectiveConfig 动态读取——管理后台改配置后 30s 缓存过期自动生效。
func (v *AgnesVisionLLM) AnalyzeScreenshot(ctx context.Context, systemPrompt, screenshotBase64 string) (*browsertools.AgentDecision, error) {
	apiKey, baseURL, model := v.effectiveConfig()
	if apiKey == "" {
		return nil, fmt.Errorf("视觉 LLM 未配置——请在管理后台「Agent 配置 → LLM 配置」添加一条用途为「视觉模型」的配置")
	}

	// 构建请求：system prompt + 截图（data URI——OpenAI 兼容格式普遍支持）
	imageURL := fmt.Sprintf("data:image/png;base64,%s", screenshotBase64)

	reqBody := agnesRequest{
		Model: model,
		Messages: []agnesMessage{
			{
				Role: "system",
				Content: []agnesContent{
					{Type: "text", Text: systemPrompt},
				},
			},
			{
				Role: "user",
				Content: []agnesContent{
					{Type: "text", Text: "请分析这张浏览器截图，判断页面出了什么问题，然后返回你的决策 JSON。"},
					{Type: "image_url", ImageURL: &agnesImageURL{URL: imageURL}},
				},
			},
		},
		Temperature: 0.1, // 低随机性——浏览器操作需要确定性
		MaxTokens:   1024,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 发送 HTTP 请求
	url := baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Agnes API 调用失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Agnes API 返回 %d: %s", resp.StatusCode, string(respBody[:min(200, len(respBody))]))
	}

	// 解析响应
	var agnesResp agnesResponse
	if err := json.Unmarshal(respBody, &agnesResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w（body: %s）", err, string(respBody[:min(200, len(respBody))]))
	}

	if agnesResp.Error != nil {
		return nil, fmt.Errorf("Agnes API 错误: %s", agnesResp.Error.Message)
	}

	if len(agnesResp.Choices) == 0 {
		return nil, fmt.Errorf("Agnes 返回空 choices")
	}

	content := agnesResp.Choices[0].Message.Content

	// 从 LLM 回答中提取决策 JSON（可能被 markdown 代码块包裹）
	jsonStr := extractJSONFromContent(content)
	if jsonStr == "" {
		return nil, fmt.Errorf("Agnes 回答中未找到决策 JSON: %s", content[:min(200, len(content))])
	}

	// 解析 AgentDecision
	var decision browsertools.AgentDecision
	if err := json.Unmarshal([]byte(jsonStr), &decision); err != nil {
		return nil, fmt.Errorf("决策 JSON 解析失败: %w（json: %s）", err, jsonStr)
	}

	// 记录审计日志
	if v.logger != nil {
		v.logger.Info("Agnes 视觉分析完成",
			port.String("tool", decision.Tool),
			port.String("reasoning", decision.Reasoning),
			port.Bool("done", decision.Done),
		)
	}

	return &decision, nil
}

// extractJSONFromContent 从 LLM 回答中提取 JSON（兼容 markdown 代码块）。
func extractJSONFromContent(content string) string {
	// 尝试直接解析
	if strings.HasPrefix(strings.TrimSpace(content), "{") {
		return strings.TrimSpace(content)
	}

	// 尝试从 ```json ... ``` 代码块中提取
	start := strings.Index(content, "```json")
	if start >= 0 {
		rest := content[start+7:]
		end := strings.Index(rest, "```")
		if end > 0 {
			return strings.TrimSpace(rest[:end])
		}
	}

	// 尝试从 ``` ... ``` 代码块中提取
	start = strings.Index(content, "```")
	if start >= 0 {
		rest := content[start+3:]
		end := strings.Index(rest, "```")
		if end > 0 {
			return strings.TrimSpace(rest[:end])
		}
	}

	// 尝试找第一个 { 到最后一个 }
	start = strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		return content[start : end+1]
	}

	return ""
}

// min 返回较小值（Go 1.21+ 有内置 min，这里兼容）。
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
