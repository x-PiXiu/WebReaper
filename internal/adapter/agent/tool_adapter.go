// Package agent 提供 trpc-agent-go 的 Agent 执行器（适配器层）。
package agent

import (
	"context"

	"trpc.group/trpc-go/trpc-agent-go/tool"

	"webreaper/internal/usecase/port"
)

// toolAdapter 把 port.CrawlerTool 适配为 trpc-agent-go 的 tool.CallableTool。
// 执行工具 → 返回结果给 LLM（不落库——保持 Agent 链纯净，与任何仓储零依赖）。
type toolAdapter struct {
	crawler port.CrawlerTool
}

// newCallableToolAdapter 包装为 CallableTool。
func newCallableToolAdapter(c port.CrawlerTool) tool.CallableTool {
	return &toolAdapter{crawler: c}
}

// Declaration 返回工具描述给 LLM（从业务层 ToolDecl 转换为框架层 Declaration）。
func (a *toolAdapter) Declaration() *tool.Declaration {
	decl := a.crawler.ToolDeclaration()
	props := make(map[string]*tool.Schema, len(decl.Properties))
	for k, v := range decl.Properties {
		props[k] = &tool.Schema{Type: v.Type, Description: v.Description}
	}
	return &tool.Declaration{
		Name:        decl.Name,
		Description: decl.Description,
		InputSchema: &tool.Schema{
			Type:       "object",
			Properties: props,
			Required:   decl.Required,
		},
	}
}

// Call 执行工具 → 返回结果给 LLM（不落库——保持 Agent 链纯净）。
func (a *toolAdapter) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	result, err := a.crawler.Execute(ctx, string(jsonArgs))
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}
	return map[string]any{
		"source_url": result.SourceURL,
		"title":      result.Title,
		"content":    truncateForLLM(result.Content, 8000),
	}, nil
}

// truncateForLLM 截断内容。
func truncateForLLM(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n...[内容过长已截断]"
}

// ConvertTools 批量转换（大写导出供 ai 包调用）。
func ConvertTools(crawlers []port.CrawlerTool) []tool.Tool {
	tools := make([]tool.Tool, 0, len(crawlers))
	for _, c := range crawlers {
		tools = append(tools, newCallableToolAdapter(c))
	}
	return tools
}
