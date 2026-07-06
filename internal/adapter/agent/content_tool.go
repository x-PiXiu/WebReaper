package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ContentGenerationTool 把 port.ContentOrchestrator（图编排）包装成一个 Agent 可调用的工具。
//
// 设计动机（让通用 Agent 能调用图编排作为"子能力"）：
//   - 通用 Agent（ExplorerTaskAgent）用 ReAct 循环自主决定调哪个工具。
//   - 当它判断"这个任务需要生成结构化内容（如面试题）"时，调用本工具。
//   - 本工具内部调图编排（scout→generate→validate），完成后结果回到 Agent 继续编排。
//
// 实现 port.CrawlerTool 接口（复用现有 ToolRegistry + ConvertTools 适配机制），
// 这样无需改动工具注册/适配框架，"内容生成"和"爬虫"在 Agent 看来都是工具。
type ContentGenerationTool struct {
	orchestrator port.ContentOrchestrator
}

// 编译期断言：实现 port.CrawlerTool（复用现有工具机制）。
var _ port.CrawlerTool = (*ContentGenerationTool)(nil)

// NewContentGenerationTool 创建内容生成工具。
func NewContentGenerationTool(orchestrator port.ContentOrchestrator) *ContentGenerationTool {
	return &ContentGenerationTool{orchestrator: orchestrator}
}

func (t *ContentGenerationTool) Name() string { return "generate_content" }

func (t *ContentGenerationTool) Description() string {
	return "为指定主题生成结构化内容（如面试题、知识点总结）。" +
		"内部走完整流程：探查主题范围 → 逐项生成 → 校验完整性 → 不完整则补生成。" +
		"适用于需要系统性、完整覆盖某个主题的场景。" +
		"输入参数：topic（主题，如'trpc-agent-go 框架'）、content_type（'interview_questions'或'knowledge_summary'）。"
}

// contentToolArgs 工具参数。
type contentToolArgs struct {
	Topic       string `json:"topic"`        // 必填：主题
	ContentType string `json:"content_type"` // 内容类型，默认 interview_questions
}

// Execute 执行内容生成（内部调图编排），返回 DataItem 汇总。
func (t *ContentGenerationTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	var args contentToolArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return entity.DataItem{}, fmt.Errorf("parse args: %w", err)
	}
	if args.Topic == "" {
		return entity.DataItem{}, fmt.Errorf("topic is required")
	}
	if args.ContentType == "" {
		args.ContentType = "interview_questions"
	}

	items, err := t.orchestrator.Orchestrate(ctx, port.OrchestrateInput{
		Topic:       args.Topic,
		ContentType: args.ContentType,
	}, nil)
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("orchestrate: %w", err)
	}

	// 把多条结果汇总成一个 DataItem 返回给 Agent（Agent 拿到后可继续编排/落库）
	var sb strings.Builder
	for i, it := range items {
		sb.WriteString(fmt.Sprintf("## 第%d条 [模块:%s] %s\n%s\n\n", i+1, it.Module, it.Title, it.Content))
	}
	now := time.Now()
	return entity.DataItem{
		ID:         fmt.Sprintf("gen-%d", now.UnixNano()),
		Title:      fmt.Sprintf("%s 的%s（共%d条）", args.Topic, contentTypeLabel(args.ContentType), len(items)),
		Content:    sb.String(),
		SourceURL:  "",
		RawContent: sb.String(),
		Status:     entity.ItemStatusPendingReview,
		Metadata: map[string]string{
			"crawler_type": "generate_content",
			"topic":        args.Topic,
			"content_type": args.ContentType,
			"item_count":   fmt.Sprintf("%d", len(items)),
		},
		Tags:      []string{args.ContentType},
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (t *ContentGenerationTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        "generate_content",
		Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"topic":        {Type: "string", Description: "主题（必填），如 'trpc-agent-go 框架'、'Go 并发编程'"},
			"content_type": {Type: "string", Description: "内容类型：interview_questions（面试题）/ knowledge_summary（知识点总结）"},
		},
		Required: []string{"topic"},
	}
}
