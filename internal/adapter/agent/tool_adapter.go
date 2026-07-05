// Package agent 提供 trpc-agent-go 的 Agent 执行器（适配器层）。
package agent

import (
	"context"
	"fmt"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// toolAdapter 把 port.CrawlerTool 适配为 trpc-agent-go 的 tool.CallableTool。
// 每次爬取结果自动存入 DataItemRepo（pending_review 状态）。
type toolAdapter struct {
	crawler      port.CrawlerTool
	dataItemRepo port.DataItemRepository // 可为 nil（测试/无DB场景）
	logger       port.Logger             // 可为 nil（降级时用 fmt 兜底）
}

// newCallableToolAdapter 包装为 CallableTool，注入 DataItemRepo 和 Logger。
func newCallableToolAdapter(c port.CrawlerTool, repo port.DataItemRepository, logger port.Logger) tool.CallableTool {
	return &toolAdapter{crawler: c, dataItemRepo: repo, logger: logger}
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

// Call 执行爬取 → 存库 → 返回结果给 LLM。
func (a *toolAdapter) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	result, err := a.crawler.Execute(ctx, string(jsonArgs))
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}

	// 落库：把爬取结果存为 DataItem（pending_review）
	if a.dataItemRepo != nil && result.IsValid() {
		// 合规过滤：入库前对 PII 脱敏（邮箱/手机号/身份证/银行卡）
		// 这是数据合规的第一道关卡——所有爬虫结果都经此入库。
		redactor := entity.DefaultPIIRedactor
		item := entity.DataItem{
			ID:         fmt.Sprintf("item-%d", time.Now().UnixNano()),
			Title:      redactor.Redact(result.Title),
			Content:    redactor.Redact(result.Content),
			SourceURL:  result.SourceURL,
			RawContent: result.RawContent, // 原始内容保留备查（可选：也可脱敏）
			Status:     entity.ItemStatusPendingReview,
			Metadata:   a.enrichMetadata(result), // 加版权来源标记
			Tags:       []string{},
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if saveErr := a.dataItemRepo.Save(ctx, item); saveErr != nil {
			// 存库失败不中断流程，LLM 仍能拿到数据
			if a.logger != nil {
				a.logger.Warn("save data item failed", port.Err(saveErr))
			}
		}
	}

	// 返回给 LLM（截断防超 token）
	return map[string]any{
		"source_url": result.SourceURL,
		"title":      result.Title,
		"content":    truncateForLLM(result.Content, 8000),
		"saved":      a.dataItemRepo != nil,
	}, nil
}

// enrichMetadata 补充版权与合规元数据。
// 标注数据来源、采集时间、版权声明，让下游使用方知晓出处。
func (a *toolAdapter) enrichMetadata(result entity.DataItem) map[string]string {
	md := result.Metadata
	if md == nil {
		md = make(map[string]string)
	}
	// 版权来源标记（合规：注明出处）
	if _, ok := md["source_url"]; !ok {
		md["source_url"] = result.SourceURL
	}
	md["copyright_notice"] = "本数据采集自公开网页，版权归原网站所有，仅供学习研究使用"
	md["collected_via"] = "WebReaper"
	return md
}

// truncateForLLM 截断内容。
func truncateForLLM(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n...[内容过长已截断]"
}

// ConvertTools 批量转换，注入 DataItemRepo 和 Logger（大写导出供 ai 包调用）。
func ConvertTools(crawlers []port.CrawlerTool, repo port.DataItemRepository, logger port.Logger) []tool.Tool {
	tools := make([]tool.Tool, 0, len(crawlers))
	for _, c := range crawlers {
		tools = append(tools, newCallableToolAdapter(c, repo, logger))
	}
	return tools
}
