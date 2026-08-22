// Package integration 提供能力路由解析（port.CapabilityResolver 实现）。
//
// 设计（能力路由模型）：adapter 注入本接口，按能力 ID（"asr"/"llm-chat"/...）
// 取当前生效配置。实现读 integration_capabilities + integration_vendors（新表），
// 同时兼容旧表（provider_configs/llm_configs）——渐进迁移，旧表最终下线。
// 10s TTL 缓存，切换 is_default ≤10s 全链路生效。
package integration

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// GormIntegrationRepo 是 repository.GormIntegrationRepository 的接口投影（解耦具体实现）。
type GormIntegrationRepo interface {
	ResolveDefault(ctx context.Context, capID string) (entity.ResolvedCap, error)
}

// Resolver port.CapabilityResolver 实现。
type Resolver struct {
	repo     GormIntegrationRepo           // 新表
	provider port.ProviderConfigRepository // 旧表兼容
	llm      port.LLMConfigRepository      // 旧表兼容

	mu       sync.Mutex
	cache    map[string]entity.ResolvedCap
	cachedAt time.Time
	ttl      time.Duration
}

// NewResolver 创建能力路由解析器。
func NewResolver(repo GormIntegrationRepo, provider port.ProviderConfigRepository, llm port.LLMConfigRepository) *Resolver {
	return &Resolver{
		repo: repo, provider: provider, llm: llm,
		cache: map[string]entity.ResolvedCap{}, ttl: 10 * time.Second,
	}
}

// Resolve 按能力 ID 取当前生效配置（新表优先，旧表兜底）。
func (r *Resolver) Resolve(ctx context.Context, capID string) (entity.ResolvedCap, error) {
	r.mu.Lock()
	if time.Since(r.cachedAt) < r.ttl {
		if cap, ok := r.cache[capID]; ok {
			r.mu.Unlock()
			return cap, nil
		}
	}
	r.mu.Unlock()

	cap, err := r.resolve(ctx, capID)

	r.mu.Lock()
	if err == nil {
		r.cache[capID] = cap
		r.cachedAt = time.Now()
	}
	r.mu.Unlock()

	return cap, err
}

func (r *Resolver) resolve(ctx context.Context, capID string) (entity.ResolvedCap, error) {
	// ① 新表优先
	if r.repo != nil {
		if cap, err := r.repo.ResolveDefault(ctx, capID); err == nil && cap.APIKey != "" {
			return cap, nil
		}
	}
	// ② 旧表兜底（渐进迁移期间）
	return r.fallbackOldTables(ctx, capID)
}

// fallbackOldTables 旧表兼容：provider_configs + llm_configs。
// 数据逐步迁移到新表后此路径下线。
func (r *Resolver) fallbackOldTables(ctx context.Context, capID string) (entity.ResolvedCap, error) {
	switch capID {
	case "asr":
		return r.resolveFromProviderConfig(ctx, "asr")
	case "llm-chat", "llm-vision":
		return r.resolveFromLLMConfig(ctx, capID)
	case "tts", "voice-clone":
		return r.resolveFromProviderConfig(ctx, capID)
	default:
		return entity.ResolvedCap{}, nil
	}
}

func (r *Resolver) resolveFromProviderConfig(ctx context.Context, providerName string) (entity.ResolvedCap, error) {
	if r.provider == nil {
		return entity.ResolvedCap{}, nil
	}
	cfgs, err := r.provider.List(ctx)
	if err != nil {
		return entity.ResolvedCap{}, err
	}
	for _, cfg := range cfgs {
		if cfg.Provider == providerName && cfg.APIKey != "" && cfg.Enabled {
			var extra struct {
				Model         string `json:"model"`
				ResponseStyle string `json:"response_style"`
			}
			_ = json.Unmarshal([]byte(cfg.ExtraJSON), &extra)
			return entity.ResolvedCap{
				VendorID:  cfg.Provider,
				BaseURL:   cfg.BaseURL,
				APIKey:    cfg.APIKey,
				Protocol:  "openai",
				Endpoint:  strings.TrimRight(cfg.BaseURL, "/"),
				Model:     extra.Model,
				ExtraJSON: cfg.ExtraJSON,
			}, nil
		}
	}
	return entity.ResolvedCap{}, nil
}

func (r *Resolver) resolveFromLLMConfig(ctx context.Context, capID string) (entity.ResolvedCap, error) {
	if r.llm == nil {
		return entity.ResolvedCap{}, nil
	}
	usage := ""
	if capID == "llm-vision" {
		usage = "vision"
	}
	cfg, err := r.llm.FindByUsage(ctx, usage)
	if err != nil || cfg.APIKey == "" {
		return entity.ResolvedCap{}, err
	}
	return entity.ResolvedCap{
		VendorID: cfg.Provider,
		BaseURL:  cfg.BaseURL,
		APIKey:   cfg.APIKey,
		Protocol: "openai",
		Endpoint: strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions",
		Model:    cfg.Model,
	}, nil
}

// Refresh 强制刷新缓存（管理后台保存后调用）。
func (r *Resolver) Refresh() {
	r.mu.Lock()
	r.cache = map[string]entity.ResolvedCap{}
	r.cachedAt = time.Time{}
	r.mu.Unlock()
}

var _ port.CapabilityResolver = (*Resolver)(nil)
