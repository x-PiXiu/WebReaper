package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"webreaper/internal/usecase/port"
)

// MiMoLLMAdapter 小米MiMo LLM适配器。
//
// 设计动机：
//   - 小米MiMo使用OpenAI兼容的 /v1/chat/completions 端点
//   - 认证头使用 api-key 而非 Authorization: Bearer
//   - 支持LLM文本对话
//   - 作为Vidu的备选LLM
type MiMoLLMAdapter struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

var _ port.AIGenerator = (*MiMoLLMAdapter)(nil)

// NewMiMoLLMAdapter 创建小米MiMo LLM适配器。
func NewMiMoLLMAdapter(apiKey, baseURL string) *MiMoLLMAdapter {
	if baseURL == "" {
		baseURL = "https://token-plan-cn.xiaomimimo.com/v1"
	}
	return &MiMoLLMAdapter{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

// ChatStream 流式对话（无工具）。
func (a *MiMoLLMAdapter) ChatStream(ctx context.Context, conversationID string, llmConfigName string, messages []port.ChatMessage, onDelta func(delta string)) (string, error) {
	// 构建请求
	req := map[string]any{
		"model":    "mimo-v2.5-pro",
		"messages": a.convertMessages(messages),
		"stream":   true,
	}

	// 发送请求
	resp, err := a.sendRequest(ctx, req)
	if err != nil {
		return "", fmt.Errorf("MiMo LLM 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 处理流式响应
	return a.handleStreamResponse(resp, onDelta)
}

// RunWithTools 带工具流式执行（ReAct）。
func (a *MiMoLLMAdapter) RunWithTools(ctx context.Context, conversationID string, llmConfigName string, task string, systemPrompt string, tools []string, onEvent func(port.ToolEvent)) error {
	// 构建消息
	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": task},
	}

	req := map[string]any{
		"model":    "mimo-v2.5-pro",
		"messages": messages,
	}

	// 发送请求
	resp, err := a.sendRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("MiMo LLM 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 处理响应
	return a.handleResponse(resp, onEvent)
}

// convertMessages 转换消息格式。
func (a *MiMoLLMAdapter) convertMessages(messages []port.ChatMessage) []map[string]string {
	result := make([]map[string]string, 0, len(messages))
	for _, msg := range messages {
		result = append(result, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	return result
}

// sendRequest 发送HTTP请求。
func (a *MiMoLLMAdapter) sendRequest(ctx context.Context, req any) (*http.Response, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 小米MiMo使用api-key头
	httpReq.Header.Set("api-key", a.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("MiMo LLM 返回 HTTP %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}

// handleStreamResponse 处理流式响应。
func (a *MiMoLLMAdapter) handleStreamResponse(resp *http.Response, onDelta func(delta string)) (string, error) {
	var fullContent strings.Builder
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			content := chunk.Choices[0].Delta.Content
			fullContent.WriteString(content)
			if onDelta != nil {
				onDelta(content)
			}
		}
	}

	return fullContent.String(), nil
}

// handleResponse 处理非流式响应。
func (a *MiMoLLMAdapter) handleResponse(resp *http.Response, onEvent func(port.ToolEvent)) error {
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if len(result.Choices) == 0 {
		return fmt.Errorf("无响应内容")
	}

	// 发送文本事件
	if onEvent != nil {
		onEvent(port.ToolEvent{
			Type: "text-delta",
			Text: result.Choices[0].Message.Content,
		})
		onEvent(port.ToolEvent{
			Type: "finish",
		})
	}

	return nil
}
