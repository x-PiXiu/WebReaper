// Package embedding 提供 port.Embedder 的实现（适配器层）。
//
// 通过 OpenAI 兼容协议调用 Embedding API（智谱/OpenAI/硅基流动等）。
// 依赖方向：embedding → net/http + port（向内）。
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"webreaper/internal/config"
	"webreaper/internal/usecase/port"
)

// OpenAIEmbedder 是通过 OpenAI 兼容 API 调用的 Embedding 实现。
// 支持：智谱 embedding-3、OpenAI text-embedding-3-small、硅基流动 BAAI/bge-m3 等。
type OpenAIEmbedder struct {
	apiKey   string
	baseURL  string
	model    string
	client   *http.Client
	dimension int
}

// NewOpenAIEmbedder 创建 Embedding 适配器。
func NewOpenAIEmbedder(cfg config.EmbeddingConfig) *OpenAIEmbedder {
	dim := 1024 // 智谱 embedding-3 默认 1024 维
	return &OpenAIEmbedder{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		client:  &http.Client{Timeout: 30 * time.Second},
		dimension: dim,
	}
}

// embeddingRequest OpenAI 兼容的 Embedding 请求体。
type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embeddingResponse OpenAI 兼容的 Embedding 响应。
type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return vecs[0], nil
}

func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody, _ := json.Marshal(embeddingRequest{Model: e.model, Input: texts})
	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API returned %d: %s", resp.StatusCode, string(body))
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	vectors := make([][]float32, 0, len(result.Data))
	for _, d := range result.Data {
		vectors = append(vectors, d.Embedding)
	}
	// 更新实际维度
	if len(vectors) > 0 {
		e.dimension = len(vectors[0])
	}
	return vectors, nil
}

func (e *OpenAIEmbedder) Dimension() int {
	return e.dimension
}

// 编译期断言
var _ port.Embedder = (*OpenAIEmbedder)(nil)
