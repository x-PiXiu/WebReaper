package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// SaveDataItemTool 把 LLM 生成的结构化内容保存为系统数据项。
//
// 设计动机（工具化闭环）：
//   - 让 LLM 自主调用此工具保存结构化数据，而非把 JSON 原文直接显示在聊天界面。
//   - LLM 生成结构化 JSON → 调用本工具保存 → 工具返回"已保存"→ LLM 据此回复友好总结。
//   - 聊天界面只看到总结，不看到一大坨 JSON，体验更好。
//
// 典型场景：面试题生成 Agent → LLM 生成面试题 JSON → 调 save_data_item 存库 → 回复"已生成面试题"。
type SaveDataItemTool struct {
	saver DataItemSaver
}

// NewSaveDataItemTool 创建保存数据工具。
func NewSaveDataItemTool(saver DataItemSaver) *SaveDataItemTool {
	return &SaveDataItemTool{saver: saver}
}

func (t *SaveDataItemTool) Name() string { return "save_data_item" }

func (t *SaveDataItemTool) Description() string {
	return "把生成的结构化内容保存为系统数据项（落库）。" +
		"当你生成了结构化数据（如面试题、文章摘要、知识条目等），应调用此工具保存，而不是直接把 JSON 返回给用户。" +
		"保存后你会收到确认信息，然后你应该给用户一个简洁的文字总结。" +
		"输入参数：content（要保存的内容，通常是 JSON）、field_mapping（字段映射JSON，可空，格式 {\"源字段\":\"目标字段\"}，目标字段可选 title/content/summary/tags）。"
}

type saveDataItemArgs struct {
	Content     string `json:"content"`
	FieldMapping string `json:"field_mapping"`
}

func (t *SaveDataItemTool) Execute(ctx context.Context, argsJSON string) (entity.DataItem, error) {
	var args saveDataItemArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return entity.DataItem{}, fmt.Errorf("parse args: %w", err)
	}
	if args.Content == "" {
		return entity.DataItem{}, fmt.Errorf("content is required")
	}

	id, title, err := t.saver.SaveFromContent(ctx, args.Content, args.FieldMapping, "agent://save_data_item")
	if err != nil {
		return entity.DataItem{}, fmt.Errorf("保存失败: %w", err)
	}

	// 返回给 LLM 的"工具结果"——告诉它保存成功，让它据此生成总结
	resultContent := fmt.Sprintf("✅ 数据已成功保存！\n数据ID: %s\n标题: %s\n\n现在请用简洁的自然语言向用户总结你生成了什么内容，不要重复输出原始 JSON。", id, title)

	return entity.DataItem{
		ID:        id,
		Title:     title,
		Content:   resultContent,
		SourceURL: "agent://save_data_item",
		Status:    entity.ItemStatusApproved,
		Metadata: map[string]string{
			"tool_type": "save_data_item",
			"saved":     "true",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (t *SaveDataItemTool) ToolDeclaration() port.ToolDecl {
	return port.ToolDecl{
		Name:        "save_data_item",
		Description: t.Description(),
		Properties: map[string]port.PropSpec{
			"content": {
				Type:        "string",
				Description: "要保存的结构化内容（通常是 JSON 格式）",
			},
			"field_mapping": {
				Type:        "string",
				Description: "字段映射JSON（可选）。格式 {\"源字段\":\"目标字段\"}，目标字段可选：title/content/summary/tags。例：{\"title\":\"title\",\"stem\":\"content\"}",
			},
		},
		Required: []string{"content"},
	}
}

// 编译期断言。
var _ port.CrawlerTool = (*SaveDataItemTool)(nil)
