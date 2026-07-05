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

// Delete 删除 Agent 配置。
func (uc *AgentConfigUseCase) Delete(ctx context.Context, name string) error {
	return uc.repo.Delete(ctx, name)
}
