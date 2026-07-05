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
	Name     string
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
}

// Create 创建 LLM 配置：校验 + 持久化。
func (uc *LLMConfigUseCase) Create(ctx context.Context, in CreateInput) (entity.LLMConfig, error) {
	cfg := entity.LLMConfig{
		Name:     in.Name,
		Provider: in.Provider,
		APIKey:   in.APIKey,
		BaseURL:  in.BaseURL,
		Model:    in.Model,
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
