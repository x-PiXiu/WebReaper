package ai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"webreaper/internal/usecase/port"
)

// newHTTPEmbedder 构建钩子（测试可替换注入假实现；默认 OpenAI 兼容 HTTP 实现）。
var newHTTPEmbedder = func(apiKey, baseURL, model string, dimensions ...int) port.Embedder {
	return NewHTTPEmbedder(apiKey, baseURL, model, dimensions...)
}

// CachedEmbedder 是 port.Embedder 的"动态配置"实现（30s TTL 重建，管理后台改模型免重启）。
//
// 设计动机（参照 urlsubmit.CachedProvider 先例）：
//   - 向量嵌入模型（model/base_url/api_key）在管理后台可改，改后不应要求重启。
//   - 每次向量化检查配置缓存：TTL 30s 内复用已构建的 HTTPEmbedder，
//     过期则从 EmbeddingConfigProvider 重读并按配置重建（换模型自动生效）。
//   - 未配置（provider 返回零值）时内部为空——Embed 返回明确错误，调用方降级。
type CachedEmbedder struct {
	load     port.EmbeddingConfigProvider
	ttl      time.Duration
	mu       sync.Mutex
	inner    port.Embedder // 当前生效的向量化器（nil=未配置）
	cachedAt time.Time
	lastErr  error // 最近一次构建错误（诊断用）
}

// NewCachedEmbedder 创建动态向量化器。
func NewCachedEmbedder(load port.EmbeddingConfigProvider) *CachedEmbedder {
	return &CachedEmbedder{load: load, ttl: 30 * time.Second}
}

// Embed 向量化（按配置缓存重建后转发）。
func (e *CachedEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	inner, err := e.current(ctx)
	if err != nil {
		return nil, err
	}
	if inner == nil {
		return nil, fmt.Errorf("向量嵌入未配置（管理后台：收录管理 → 知识库配置）")
	}
	return inner.Embed(ctx, text)
}

// EmbedBatch 批量向量化。
func (e *CachedEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	inner, err := e.current(ctx)
	if err != nil {
		return nil, err
	}
	if inner == nil {
		return nil, fmt.Errorf("向量嵌入未配置（管理后台：收录管理 → 知识库配置）")
	}
	return inner.EmbedBatch(ctx, texts)
}

// Dimension 返回当前生效向量化器的维度（未配置返回 0）。
func (e *CachedEmbedder) Dimension() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.inner == nil {
		return 0
	}
	return e.inner.Dimension()
}

// current 获取当前向量化器（TTL 缓存 + 双检锁；配置变化时重建）。
// 未配置/构建失败也按 TTL 缓存——避免每次调用都重读配置（生成链路高频调用）。
func (e *CachedEmbedder) current(ctx context.Context) (port.Embedder, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if time.Since(e.cachedAt) < e.ttl {
		return e.inner, e.lastErr
	}
	cfg, err := e.load.Load(ctx)
	if err != nil {
		e.lastErr = err
		e.cachedAt = time.Now() // 失败也缓存：TTL 内不重复打配置源
		return nil, err
	}
	if !cfg.IsConfigured() {
		// 未配置：清空内部实现（Embed 返回明确错误，调用方降级）
		e.inner = nil
		e.cachedAt = time.Now()
		e.lastErr = nil
		return nil, nil
	}
	e.inner = newHTTPEmbedder(cfg.APIKey, cfg.BaseURL, cfg.Model, cfg.Dimensions)
	e.cachedAt = time.Now()
	e.lastErr = nil
	return e.inner, nil
}

// LastError 最近一次构建错误（管理后台诊断用）。
func (e *CachedEmbedder) LastError() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastErr
}

var _ port.Embedder = (*CachedEmbedder)(nil)
