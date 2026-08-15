package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"webreaper/internal/usecase/port"
)

// HTTPEmbedder 是 port.Embedder 的 OpenAI 兼容 HTTP 实现（POST {baseURL}/embeddings）。
//
// 兼容 MiniMax/OpenAI/智谱（bigmodel）等 OpenAI 兼容协议厂商——模型/端点/Key 全可配
// （EMBEDDING_* 显式配置，缺省复用 LLM 配置）。
// 维度从首次响应推导并缓存（不硬编码模型维度表，换模型零改动）；
// 可选 dimensions 参数（如智谱 embedding-3 支持 256-2048；0=不传用模型默认）。
type HTTPEmbedder struct {
	apiKey     string
	baseURL    string // 如 https://open.bigmodel.cn/api/paas/v4（自动拼 /embeddings）
	model      string
	dimensions int // 0=不传（模型默认维度）
	dimension  int // 首次响应推导的实际维度（0=未知；之后锁定）
	client     *http.Client
	mu         sync.Mutex
}

// NewHTTPEmbedder 创建 OpenAI 兼容向量化器。
// dimensions 可选：传 >0 时请求体带 dimensions 参数（显式固定维度）。
func NewHTTPEmbedder(apiKey, baseURL, model string, dimensions ...int) *HTTPEmbedder {
	e := &HTTPEmbedder{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
	if len(dimensions) > 0 && dimensions[0] > 0 {
		e.dimensions = dimensions[0]
	}
	return e
}

// Embed 单条文本向量化。
func (e *HTTPEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("embedding 响应为空")
	}
	return vecs[0], nil
}

// EmbedBatch 批量向量化（OpenAI 兼容 input 数组；响应按请求顺序对齐）。
func (e *HTTPEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	payload, _ := json.Marshal(embedRequest{Model: e.model, Input: texts, Dimensions: e.dimensions})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint(), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding HTTP %d: %s", resp.StatusCode, truncateStr(string(body), 200))
	}
	var parsed embedResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("embedding 响应解析失败: %s", truncateStr(string(body), 200))
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embedding 响应数量不符: 期望 %d 实际 %d", len(texts), len(parsed.Data))
	}
	out := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		out[i] = d.Embedding
	}
	e.lockDimension(len(out[0]))
	return out, nil
}

// Dimension 返回向量维度（首次响应后确定；未调用过返回 0）。
func (e *HTTPEmbedder) Dimension() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.dimension
}

// lockDimension 记录首次响应维度（后续不一致也不改——防脏数据）。
func (e *HTTPEmbedder) lockDimension(dim int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.dimension == 0 {
		e.dimension = dim
	}
}

func (e *HTTPEmbedder) endpoint() string {
	return e.baseURL + "/embeddings"
}

// embedRequest OpenAI 兼容 embedding 请求体（dimensions 仅显式配置时发送）。
type embedRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

// embedResponse OpenAI 兼容 embedding 响应体。
type embedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
}

var _ port.Embedder = (*HTTPEmbedder)(nil)
