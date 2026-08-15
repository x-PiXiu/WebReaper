package ai

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// fakeConfigProvider 可控配置源（返回预设配置，带调用计数）。
type fakeConfigProvider struct {
	cfg   entity.EmbeddingRuntimeConfig
	err   error
	loads atomic.Int64
}

func (f *fakeConfigProvider) Load(context.Context) (entity.EmbeddingRuntimeConfig, error) {
	f.loads.Add(1)
	if f.err != nil {
		return entity.EmbeddingRuntimeConfig{}, f.err
	}
	return f.cfg, nil
}

var _ port.EmbeddingConfigProvider = (*fakeConfigProvider)(nil)

// countingEmbedder 记录构建次数的假 embedder（配合可替换的构造钩子）。
type countingEmbedder struct{ n int64 }

func (c *countingEmbedder) Embed(context.Context, string) ([]float32, error) { return []float32{1}, nil }
func (c *countingEmbedder) EmbedBatch(context.Context, []string) ([][]float32, error) {
	return nil, nil
}
func (c *countingEmbedder) Dimension() int { return 1 }

var _ port.Embedder = (*countingEmbedder)(nil)

// TestCachedEmbedder_TTLRebuild 配置变化后 TTL 过期即重建（换模型自动生效）。
func TestCachedEmbedder_TTLRebuild(t *testing.T) {
	var builds atomic.Int64
	origNew := newHTTPEmbedder
	newHTTPEmbedder = func(apiKey, baseURL, model string, _ ...int) port.Embedder {
		builds.Add(1)
		return &countingEmbedder{}
	}
	defer func() { newHTTPEmbedder = origNew }()

	provider := &fakeConfigProvider{cfg: entity.EmbeddingRuntimeConfig{
		Model: "embedding-1", BaseURL: "https://api.test/v1", APIKey: "k1",
	}}
	e := NewCachedEmbedder(provider)
	e.ttl = 0 // TTL=0：每次调用都重读配置（模拟 30s 过期）

	if _, err := e.Embed(context.Background(), "a"); err != nil {
		t.Fatalf("Embed 失败: %v", err)
	}
	if _, err := e.Embed(context.Background(), "a"); err != nil {
		t.Fatalf("Embed 失败: %v", err)
	}
	if builds.Load() != 2 {
		t.Errorf("TTL=0 每次应重建: %d", builds.Load())
	}
}

// TestCachedEmbedder_Unconfigured 未配置 → 明确错误（调用方降级），不 panic。
func TestCachedEmbedder_Unconfigured(t *testing.T) {
	e := NewCachedEmbedder(&fakeConfigProvider{cfg: entity.EmbeddingRuntimeConfig{}})
	if _, err := e.Embed(context.Background(), "a"); err == nil {
		t.Error("未配置应返回错误")
	}
	// provider 出错
	e = NewCachedEmbedder(&fakeConfigProvider{err: errors.New("db down")})
	if _, err := e.Embed(context.Background(), "a"); err == nil {
		t.Error("provider 错误应返回错误")
	}
}

// TestCachedEmbedder_FailureCached 失败也按 TTL 缓存（TTL 内不重复打配置源）。
func TestCachedEmbedder_FailureCached(t *testing.T) {
	p := &fakeConfigProvider{err: errors.New("db down")}
	e := NewCachedEmbedder(p)
	e.ttl = time.Hour // 长 TTL：模拟 30s 窗口内

	if _, err := e.Embed(context.Background(), "a"); err == nil {
		t.Fatal("应返回错误")
	}
	if _, err := e.Embed(context.Background(), "a"); err == nil {
		t.Fatal("第二次仍应返回错误")
	}
	if p.loads.Load() != 1 {
		t.Errorf("TTL 内失败应只读一次配置: %d", p.loads.Load())
	}
}
