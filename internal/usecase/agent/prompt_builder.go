package agent

import (
	"context"
	"fmt"
	"strings"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// PromptBuilder 智能体系统提示词构建器（整洁架构·Usecase层）。
//
// 设计动机：
//   - 智能体系统提示词需要动态配置（管理后台可修改）
//   - 核心规则硬编码（不可修改），补充规则管理后台配置
//   - 复用现有PromptTemplate实体和仓储
//
// 提示词结构：
//   [前缀 - 管理后台配置]
//   [核心规则 - 硬编码]
//   [补充规则 - 管理后台配置]
//   [后缀 - 管理后台配置]
type PromptBuilder struct {
	promptRepo port.PromptTemplateRepository
}

func NewPromptBuilder(promptRepo port.PromptTemplateRepository) *PromptBuilder {
	return &PromptBuilder{promptRepo: promptRepo}
}

// Build 构建智能体系统提示词。
//
// 参数：
//   - tenantID：租户ID（用于查询租户级配置）
//   - permissionLevel：用户权限级别（full_access/semi_auto/manual_confirm）
//
// 返回：
//   - 完整的系统提示词字符串
func (b *PromptBuilder) Build(ctx context.Context, tenantID string, permissionLevel string) (string, error) {
	var parts []string

	// 1. 加载前缀（管理后台配置，可选）
	if b.promptRepo != nil {
		prefix, err := b.promptRepo.Get(ctx, entity.PromptKeyAgentPrefix)
		if err == nil && prefix.Content != "" {
			parts = append(parts, prefix.Content)
		}
	}

	// 2. 硬编码核心规则（不可修改）
	coreRules := b.buildCoreRules(permissionLevel)
	parts = append(parts, coreRules)

	// 3. 加载补充规则（管理后台配置，可选）
	if b.promptRepo != nil {
		// 补充规则通过 Key 前缀区分，支持多条规则
		// 使用现有的PromptTemplate，Key格式：agent-rule-{id}
		// 暂时只支持单条补充规则，后续可扩展
		rules, err := b.promptRepo.Get(ctx, entity.PromptKeyAgentRules)
		if err == nil && rules.Content != "" {
			parts = append(parts, rules.Content)
		}
	}

	// 4. 加载后缀（管理后台配置，可选）
	if b.promptRepo != nil {
		suffix, err := b.promptRepo.Get(ctx, entity.PromptKeyAgentSuffix)
		if err == nil && suffix.Content != "" {
			parts = append(parts, suffix.Content)
		}
	}

	return strings.Join(parts, "\n"), nil
}

// buildCoreRules 构建核心规则（硬编码，不可修改）。
func (b *PromptBuilder) buildCoreRules(permissionLevel string) string {
	return fmt.Sprintf(`你是获客智能体，帮助用户完成品牌营销任务。

用户权限级别：%s
- full_access：所有决策由你决定，无需询问用户
- semi_auto：关键决策（生成内容、选择素材）需要询问用户
- manual_confirm：每一步都需要询问用户

核心执行规则：
1. 先查询厂商能力，了解支持的功能
2. 素材查找优先级：素材库 → 询问用户是否生成 → 用户上传
3. 根据厂商能力和素材情况，决定使用主体模式还是直接数字人
4. 生成执行计划，展示给用户确认
5. 用户确认后执行，可修改
6. 发布前必须得到用户确认
7. 超时未确认自动停止`, permissionLevel)
}
