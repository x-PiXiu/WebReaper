// Package agentconfig 实现"Agent 配置管理"用例。
//
// 职责：AgentConfig 的增删改查 + 创建时的领域校验与默认值填充。
//
// 设计动机（整洁架构）：
//   - 原先 handler 直接调仓储 Save，并在 handler 里做 IsValid 校验、设默认 maxIter，
//     违反"handler 只做 DTO 转换、领域规则在用例/实体"的分层。
//   - 现把校验和默认值下沉到用例层，handler 只调 usecase.Create()。
package agentconfig

import (
	"context"
	"fmt"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// AgentConfigUseCase Agent 配置管理用例。
type AgentConfigUseCase struct {
	repo port.AgentConfigRepository
}

func NewAgentConfigUseCase(repo port.AgentConfigRepository) *AgentConfigUseCase {
	return &AgentConfigUseCase{repo: repo}
}

// CreateInput 创建 Agent 的输入（用例层 DTO，不暴露给 HTTP 层的细节）。
type CreateInput struct {
	Name          string
	SystemPrompt  string
	LLMConfigName string // 留空用 default
	MaxIterations int    // 留空（<=0）用默认值
	AutoSave      bool   // 自动落库开关
	FieldMapping  string // 自动落库字段映射 JSON
}

// Create 创建 Agent 配置：校验 + 填默认值 + 持久化。
func (uc *AgentConfigUseCase) Create(ctx context.Context, in CreateInput) (entity.AgentConfig, error) {
	cfg := entity.AgentConfig{
		Name:          in.Name,
		SystemPrompt:  in.SystemPrompt,
		LLMConfigName: in.LLMConfigName,
		MaxIterations: in.MaxIterations,
		AutoSave:      in.AutoSave,
		FieldMapping:  in.FieldMapping,
	}.FillDefaults()
	if !cfg.IsValid() {
		return entity.AgentConfig{}, fmt.Errorf("agent config 无效：name 和 system_prompt 不能为空")
	}
	if err := uc.repo.Save(ctx, cfg); err != nil {
		return entity.AgentConfig{}, fmt.Errorf("save agent config: %w", err)
	}
	return cfg, nil
}

// List 列出全部 Agent 配置。
func (uc *AgentConfigUseCase) List(ctx context.Context) ([]entity.AgentConfig, error) {
	return uc.repo.List(ctx)
}

// UpdateInput 修改 Agent 的输入。
// SystemPrompt 为空表示不改提示词（保持原值）；其余字段同理。
// 设计为"部分更新"语义：调用方可只传需要改的字段，nil/零值字段保留原值。
// 为简化实现，这里采用"零值=保留原值"约定，覆盖大多数场景。
type UpdateInput struct {
	SystemPrompt  *string // 不传（nil）则保留原值
	LLMConfigName *string // 不传（nil）则保留原值；传 "" 表示改为用 default
	MaxIterations *int    // 不传（nil）则保留原值
	AutoSave      *bool   // 不传（nil）则保留原值
	FieldMapping  *string // 不传（nil）则保留原值
}

// Update 修改 Agent 配置：先取原值，应用部分更新，校验后持久化。
// 不存在的 name 返回 "未找到" 错误。修改提示词为空会被 IsValid 拦截。
func (uc *AgentConfigUseCase) Update(ctx context.Context, name string, in UpdateInput) (entity.AgentConfig, error) {
	old, err := uc.repo.FindByName(ctx, name)
	if err != nil {
		return entity.AgentConfig{}, fmt.Errorf("agent config %q 不存在: %w", name, err)
	}
	// 应用部分更新（nil = 保留原值）
	if in.SystemPrompt != nil {
		old.SystemPrompt = *in.SystemPrompt
	}
	if in.LLMConfigName != nil {
		old.LLMConfigName = *in.LLMConfigName
	}
	if in.MaxIterations != nil {
		old.MaxIterations = *in.MaxIterations
	}
	if in.AutoSave != nil {
		old.AutoSave = *in.AutoSave
	}
	if in.FieldMapping != nil {
		old.FieldMapping = *in.FieldMapping
	}
	// 填默认值（MaxIterations<=0 → 10）并校验
	old = old.FillDefaults()
	if !old.IsValid() {
		return entity.AgentConfig{}, fmt.Errorf("agent config 无效：name 和 system_prompt 不能为空")
	}
	if err := uc.repo.Save(ctx, old); err != nil {
		return entity.AgentConfig{}, fmt.Errorf("update agent config: %w", err)
	}
	return old, nil
}

// Delete 删除 Agent 配置。
func (uc *AgentConfigUseCase) Delete(ctx context.Context, name string) error {
	return uc.repo.Delete(ctx, name)
}
