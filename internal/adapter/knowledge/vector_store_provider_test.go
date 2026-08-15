package knowledge

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// fakeMySQLStore 假 MySQL 驱动（Provider 测试用）。
type fakeMySQLStore struct{ ok bool }

func (f *fakeMySQLStore) Store(context.Context, string, []float32, map[string]string) error {
	return nil
}
func (f *fakeMySQLStore) Search(context.Context, map[string]string, []float32, int) ([]port.VectorSearchResult, error) {
	return nil, nil
}
func (f *fakeMySQLStore) Delete(context.Context, string) error { return nil }
func (f *fakeMySQLStore) IsAvailable() bool                    { return f.ok }

var _ port.VectorStore = (*fakeMySQLStore)(nil)

// fakeCfgProvider 可控配置源（测试用，带调用计数）。
type fakeCfgProvider struct {
	cfg   entity.EmbeddingRuntimeConfig
	err   error
	loads int
}

func (f *fakeCfgProvider) Load(context.Context) (entity.EmbeddingRuntimeConfig, error) {
	f.loads++
	if f.err != nil {
		return entity.EmbeddingRuntimeConfig{}, f.err
	}
	return f.cfg, nil
}

var _ port.EmbeddingConfigProvider = (*fakeCfgProvider)(nil)

// TestVectorStoreProvider_Routing 按配置路由：mysql 返回默认驱动；milvus 走工厂/未装配报错。
func TestVectorStoreProvider_Routing(t *testing.T) {
	mysql := &fakeMySQLStore{ok: true}
	load := func(cfg entity.EmbeddingRuntimeConfig) port.EmbeddingConfigProvider {
		return &fakeCfgProvider{cfg: cfg}
	}

	// 默认（空 vector_db）→ mysql
	p := NewVectorStoreProvider(load(entity.EmbeddingRuntimeConfig{}), mysql)
	store, err := p.Get(context.Background())
	if err != nil || store != port.VectorStore(mysql) {
		t.Errorf("空 vector_db 应返回 mysql: %v %v", store, err)
	}
	if p.CurrentKind() != entity.VectorDBMySQL {
		t.Errorf("CurrentKind 应为 mysql: %s", p.CurrentKind())
	}

	// 显式 mysql → mysql
	p = NewVectorStoreProvider(load(entity.EmbeddingRuntimeConfig{VectorDB: entity.VectorDBMySQL}), mysql)
	store, err = p.Get(context.Background())
	if err != nil || store != port.VectorStore(mysql) {
		t.Errorf("mysql 应返回默认驱动: %v %v", store, err)
	}

	// milvus + 工厂注入 → 返回工厂构建的 store
	milvusStore := &fakeMySQLStore{ok: true}
	p = NewVectorStoreProvider(load(entity.EmbeddingRuntimeConfig{
		VectorDB: entity.VectorDBMilvus, MilvusHost: "10.0.0.1",
	}), mysql, func(_ context.Context, _ entity.EmbeddingRuntimeConfig) (port.VectorStore, error) {
		return milvusStore, nil
	})
	store, err = p.Get(context.Background())
	if err != nil || store != port.VectorStore(milvusStore) {
		t.Errorf("milvus 应走工厂: %v %v", store, err)
	}
	if p.CurrentKind() != entity.VectorDBMilvus {
		t.Errorf("CurrentKind 应为 milvus: %s", p.CurrentKind())
	}

	// milvus 但未注入工厂 → 明确报错（不静默降级）
	p = NewVectorStoreProvider(load(entity.EmbeddingRuntimeConfig{
		VectorDB: entity.VectorDBMilvus, MilvusHost: "10.0.0.1",
	}), mysql)
	_, err = p.Get(context.Background())
	if err == nil {
		t.Error("milvus 未装配工厂应报错")
	}
	if p.LastError() == nil {
		t.Error("LastError 应有记录")
	}
}

// TestVectorStoreProvider_TTL 30s TTL 内复用缓存（不重复读配置）。
func TestVectorStoreProvider_TTL(t *testing.T) {
	mysql := &fakeMySQLStore{ok: true}
	p := NewVectorStoreProvider(&fakeCfgProvider{cfg: entity.EmbeddingRuntimeConfig{}}, mysql)
	if _, err := p.Get(context.Background()); err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	// TTL 内二次 Get 应命中缓存（不报错）
	if _, err := p.Get(context.Background()); err != nil {
		t.Fatalf("TTL 内 Get 失败: %v", err)
	}
}

// TestVectorStoreProvider_FailureCached milvus 未接入失败也按 TTL 缓存（不重复打配置源）。
func TestVectorStoreProvider_FailureCached(t *testing.T) {
	mysql := &fakeMySQLStore{ok: true}
	provider := &fakeCfgProvider{cfg: entity.EmbeddingRuntimeConfig{
		VectorDB: entity.VectorDBMilvus, MilvusHost: "10.0.0.1",
	}}
	p := NewVectorStoreProvider(provider, mysql)
	if _, err := p.Get(context.Background()); err == nil {
		t.Fatal("milvus 未接入应报错")
	}
	if _, err := p.Get(context.Background()); err == nil {
		t.Fatal("第二次仍应报错")
	}
	if provider.loads != 1 {
		t.Errorf("TTL 内失败应只读一次配置: %d", provider.loads)
	}
}
