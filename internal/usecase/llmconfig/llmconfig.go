// Package llmconfig 实现"LLM 配置管理"用例。
//
// 职责：LLMConfig 的增删改查 + 创建时的领域校验。
//
// 设计动机（整洁架构）：
//   - 把原先散落在 handler 里的 IsValid 校验、字段必填检查下沉到用例层。
package llmconfig

import (
	"context"
	"fmt"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// LLMConfigUseCase LLM 配置管理用例。
type LLMConfigUseCase struct {
	repo port.LLMConfigRepository
}

func NewLLMConfigUseCase(repo port.LLMConfigRepository) *LLMConfigUseCase {
	return &LLMConfigUseCase{repo: repo}
}

// CreateInput 创建 LLM 配置的输入。
type CreateInput struct {
	Name        string
	Provider    string
	APIKey      string
	BaseURL     string
	Model       string
	CostPerMTok int
}

// Create 创建 LLM 配置：校验 + 持久化。
func (uc *LLMConfigUseCase) Create(ctx context.Context, in CreateInput) (entity.LLMConfig, error) {
	cfg := entity.LLMConfig{
		Name:        in.Name,
		Provider:    in.Provider,
		APIKey:      in.APIKey,
		BaseURL:     in.BaseURL,
		Model:       in.Model,
		CostPerMTok: in.CostPerMTok,
	}
	if cfg.CostPerMTok <= 0 {
		cfg.CostPerMTok = 100 // 默认 ¥1/百万 tokens（与全局参考价一致）
	}
	if !cfg.IsValid() {
		return entity.LLMConfig{}, fmt.Errorf("llm config 无效：name / api_key / model 不能为空")
	}
	if err := uc.repo.Save(ctx, cfg); err != nil {
		return entity.LLMConfig{}, fmt.Errorf("save llm config: %w", err)
	}
	return cfg, nil
}

// List 列出全部 LLM 配置。
func (uc *LLMConfigUseCase) List(ctx context.Context) ([]entity.LLMConfig, error) {
	return uc.repo.List(ctx)
}

// UpdateInput 修改 LLM 配置的输入（部分更新语义：nil = 保留原值）。
type UpdateInput struct {
	Provider    *string
	APIKey      *string
	BaseURL     *string
	Model       *string
	CostPerMTok *int
}

// Update 修改 LLM 配置：先取原值，应用部分更新，校验后持久化。
// 注意：修改 LLM 配置后，已缓存的 LLM 客户端不会自动失效——
// 调用方（handler 装配层）负责触发缓存清理，或依赖 LLM 缓存的 TTL 过期。
func (uc *LLMConfigUseCase) Update(ctx context.Context, name string, in UpdateInput) (entity.LLMConfig, error) {
	old, err := uc.repo.FindByName(ctx, name)
	if err != nil {
		return entity.LLMConfig{}, fmt.Errorf("llm config %q 不存在: %w", name, err)
	}
	if in.Provider != nil {
		old.Provider = *in.Provider
	}
	if in.APIKey != nil {
		old.APIKey = *in.APIKey
	}
	if in.BaseURL != nil {
		old.BaseURL = *in.BaseURL
	}
	if in.Model != nil {
		old.Model = *in.Model
	}
	if in.CostPerMTok != nil {
		old.CostPerMTok = *in.CostPerMTok
	}
	if !old.IsValid() {
		return entity.LLMConfig{}, fmt.Errorf("llm config 无效：name / api_key / model 不能为空")
	}
	if err := uc.repo.Save(ctx, old); err != nil {
		return entity.LLMConfig{}, fmt.Errorf("update llm config: %w", err)
	}
	return old, nil
}

// Delete 删除 LLM 配置。
func (uc *LLMConfigUseCase) Delete(ctx context.Context, name string) error {
	return uc.repo.Delete(ctx, name)
}

// EnsureDefault 确保存在名为 "default" 的 LLM 配置（首次启动 seed 用）。
// 若已存在则不覆盖；返回最终生效的 default 配置。
func (uc *LLMConfigUseCase) EnsureDefault(ctx context.Context, cfg entity.LLMConfig) error {
	if existing, err := uc.repo.FindByName(ctx, cfg.Name); err == nil && existing.APIKey != "" {
		return nil // 已存在，不覆盖
	}
	return uc.repo.Save(ctx, cfg)
}

// SetDefault 设置默认模型（同 Usage 下互斥——先清除再设置）。
// 管理后台调用：切换默认模型后，未指定 llmConfigName 的用例自动使用新默认。
func (uc *LLMConfigUseCase) SetDefault(ctx context.Context, name string) error {
	return uc.repo.SetDefault(ctx, name)
}
