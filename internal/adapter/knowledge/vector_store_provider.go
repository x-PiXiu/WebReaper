package knowledge

import (
	"context"
	"fmt"
	"sync"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)
// MilvusFactory 按运行时配置构建 Milvus 向量库（main 装配注入——provider 不直接依赖 SDK，
// 保持开闭原则：向量库接入 = 注入工厂，provider/仓储/检索零改动）。
type MilvusFactory func(ctx context.Context, cfg entity.EmbeddingRuntimeConfig) (port.VectorStore, error)

// VectorStoreProvider 是 port.VectorStore 的"动态配置"工厂（30s TTL，管理后台换向量库免重启）。
//
// 设计动机（管理后台动态修改向量库）：
//   - vector_db 在 kb_embedding_config（system_settings）可改：mysql ↔ milvus。
//   - 每次检索检查配置缓存：TTL 30s 内复用已构建的 store，过期重读配置重建。
//   - mysql 为内置驱动（默认）；milvus 由注入的 MilvusFactory 构建（未注入时明确报错，
//     不静默降级——避免"以为在用 Milvus 实际还在 MySQL"）。
type VectorStoreProvider struct {
	load          port.EmbeddingConfigProvider
	mysql         port.VectorStore      // 默认驱动（常驻）
	milvusFactory MilvusFactory         // 可选：nil = milvus 未接入
	ttl           time.Duration
	mu            sync.Mutex
	cached        port.VectorStore
	kind          string // 当前缓存对应的 vector_db
	cachedAt      time.Time
	lastErr       error
}

// NewVectorStoreProvider 创建向量库工厂。
func NewVectorStoreProvider(load port.EmbeddingConfigProvider, mysql port.VectorStore, milvusFactory ...MilvusFactory) *VectorStoreProvider {
	p := &VectorStoreProvider{load: load, mysql: mysql, ttl: 30 * time.Second}
	if len(milvusFactory) > 0 {
		p.milvusFactory = milvusFactory[0]
	}
	return p
}

// Get 获取当前配置对应的向量库实现。
func (p *VectorStoreProvider) Get(ctx context.Context) (port.VectorStore, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// TTL 内：成功复用缓存；失败也缓存（返回上次错误，不重复打配置源——检索链路高频调用）
	if time.Since(p.cachedAt) < p.ttl {
		if p.lastErr != nil {
			return nil, p.lastErr
		}
		return p.cached, nil
	}
	cfg, err := p.load.Load(ctx)
	if err != nil {
		p.lastErr = err
		p.cachedAt = time.Now() // 失败也缓存：TTL 内不重复查配置
		return nil, err
	}
	store, err := p.build(cfg)
	if err != nil {
		p.lastErr = err
		p.cachedAt = time.Now() // milvus 未接入等：TTL 内直接返回该错误
		return nil, err
	}
	p.cached = store
	p.kind = cfg.EffectiveVectorDB()
	p.cachedAt = time.Now()
	p.lastErr = nil
	return store, nil
}

// build 按配置构建向量库实现。
func (p *VectorStoreProvider) build(cfg entity.EmbeddingRuntimeConfig) (port.VectorStore, error) {
	switch cfg.EffectiveVectorDB() {
	case entity.VectorDBMySQL, "":
		return p.mysql, nil
	case entity.VectorDBMilvus:
		if p.milvusFactory == nil {
			// 配置就位但工厂未注入（平台未装配 Milvus）：明确报错，不静默降级
			return nil, fmt.Errorf("vector_db=milvus 但 Milvus 驱动未装配（保持 mysql 可运行；平台接入 Milvus 后此处自动切换）")
		}
		store, err := p.milvusFactory(context.Background(), cfg)
		if err != nil {
			return nil, err
		}
		return store, nil
	default:
		return nil, fmt.Errorf("未知 vector_db: %s（仅支持 mysql/milvus）", cfg.VectorDB)
	}
}

// CurrentKind 当前生效的向量库类型（诊断用）。
func (p *VectorStoreProvider) CurrentKind() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.kind == "" {
		return entity.VectorDBMySQL
	}
	return p.kind
}

// LastError 最近一次构建错误（管理后台诊断用）。
func (p *VectorStoreProvider) LastError() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastErr
}
