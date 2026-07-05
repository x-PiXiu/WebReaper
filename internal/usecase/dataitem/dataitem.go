// Package dataitem 实现"数据项管理"用例（应用级业务流程编排）。
//
// 职责：
//   - 数据项 / 采集集合的查询（List）
//   - 数据项审核编排：approve 时改状态 + 异步触发结构化与向量化
//
// 设计动机（整洁架构）：
//   - 原先 handler 直接操作仓储做 CRUD，并在 handler 里用 go func 编排
//     "审核→结构化→向量化"业务流，违反了"handler 只做 DTO 转换"的分层原则。
//   - 现把这条业务流下沉到用例层，handler 只调 usecase.Approve()，
//     业务编排可脱离 HTTP 框架单独测试。
package dataitem

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// DataItemUseCase 数据项管理用例。
type DataItemUseCase struct {
	dataItemRepo   port.DataItemRepository
	collectionRepo port.CollectionRepository
	processor      port.ItemProcessor // 审核通过后的结构化+向量化处理器（可为 nil，降级时跳过）
	logger         port.Logger
}

// NewDataItemUseCase 创建数据项用例。
// processor 为 nil 时审核后跳过结构化（如未配置 LLM/向量库）。
func NewDataItemUseCase(
	dataItemRepo port.DataItemRepository,
	collectionRepo port.CollectionRepository,
	processor port.ItemProcessor,
	logger port.Logger,
) *DataItemUseCase {
	return &DataItemUseCase{
		dataItemRepo:   dataItemRepo,
		collectionRepo: collectionRepo,
		processor:      processor,
		logger:         logger,
	}
}

// ListDataItems 列出最近的数据项。
func (uc *DataItemUseCase) ListDataItems(ctx context.Context, limit int) ([]entity.DataItem, error) {
	if limit <= 0 {
		limit = 50
	}
	return uc.dataItemRepo.List(ctx, limit)
}

// ListCollections 列出采集集合。
func (uc *DataItemUseCase) ListCollections(ctx context.Context, limit int) ([]entity.Collection, error) {
	if limit <= 0 {
		limit = 50
	}
	return uc.collectionRepo.List(ctx, limit)
}

// ApproveOutput 审核通过的输出。
type ApproveOutput struct {
	ItemID  string
	Message string
}

// Approve 审核通过：更新状态为 approved，并异步触发结构化+向量化。
//
// 这是"审核→结构化→向量化"业务流的编排点（原在 handler 内）。
// 结构化处理异步执行，不阻塞审核 API 响应；处理失败只记日志，不影响审核结果。
func (uc *DataItemUseCase) Approve(ctx context.Context, itemID string) (ApproveOutput, error) {
	if err := uc.dataItemRepo.UpdateStatus(ctx, itemID, entity.ItemStatusApproved); err != nil {
		return ApproveOutput{}, fmt.Errorf("update status: %w", err)
	}

	// 异步触发结构化处理（不阻塞 API 响应）
	if uc.processor != nil {
		go func(id string) {
			pctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			if err := uc.processor.ProcessItem(pctx, id); err != nil {
				uc.logger.Warn("process item failed after approval",
					port.String("item_id", id), port.Err(err))
			}
		}(itemID)
	}

	return ApproveOutput{
		ItemID:  itemID,
		Message: "已通过，正在后台结构化和向量化",
	}, nil
}

// Reject 审核驳回：更新状态为 rejected。
func (uc *DataItemUseCase) Reject(ctx context.Context, itemID string) error {
	return uc.dataItemRepo.UpdateStatus(ctx, itemID, entity.ItemStatusRejected)
}

// CreateFromContentInput 从内容创建 DataItem 的输入。
type CreateFromContentInput struct {
	Content     string // LLM 返回的原始内容（通常是结构化 JSON）
	FieldMapping string // 字段映射 JSON：{"llm_field":"dataitem_field"}
	SourceURL   string // 来源标记（可选）
}

// CreateFromContent 从 LLM 生成的结构化内容创建 DataItem（直接 approved）。
//
// 设计动机：打通"对话生成 → 自动落库"闭环。
// 当 Agent 配置了 auto_save=true，对话结束后把 LLM 回复按 field_mapping
// 提取字段，构造 DataItem 直接存为 approved 状态（LLM 已按提示词结构化，无需再审核）。
//
// 字段映射方向：{"llm_field":"dataitem_field"}
// dataitem_field 可选值：title / content / summary / tags / metadata.*
// 例：{"title":"title","stem":"content","answer_good":"summary"}
//   → LLM 返回的 title → DataItem.Title
//   → LLM 返回的 stem → DataItem.Content
//   → LLM 返回的 answer_good → DataItem.Summary
//
// 若内容不是合法 JSON 或无映射命中，整体内容作为 Content 存储（降级保底）。
func (uc *DataItemUseCase) CreateFromContent(ctx context.Context, in CreateFromContentInput) (entity.DataItem, error) {
	item := buildDataItemFromContent(in.Content, in.FieldMapping, in.SourceURL)
	if err := uc.dataItemRepo.Save(ctx, item); err != nil {
		return entity.DataItem{}, fmt.Errorf("save data item: %w", err)
	}
	return item, nil
}

// buildDataItemFromContent 纯领域逻辑：把 LLM 内容 + 字段映射转成 DataItem。
// 提取为包级函数，便于单测，不依赖仓储。
//
// 关键设计：RawContent 始终保存 LLM 返回的完整原始 JSON（用于 raw 模式推送外部系统）；
// Content 保存映射后的内容（用于数据管理页展示）。
func buildDataItemFromContent(content, fieldMapping, sourceURL string) entity.DataItem {
	now := time.Now()
	item := entity.DataItem{
		ID:         fmt.Sprintf("gen-%d", now.UnixNano()),
		Title:      "生成内容",
		Content:    content,
		RawContent: content, // 保留完整原始 JSON，供 raw 模式推送使用
		SourceURL:  sourceURL,
		Status:     entity.ItemStatusApproved, // 直接 approved（LLM 已结构化）
		Tags:       []string{},
		Metadata:   map[string]string{"source": "chat_generated"},
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if sourceURL == "" {
		item.SourceURL = "chat://generated"
	}

	// 尝试按 field_mapping 从 JSON 提取字段
	if content == "" || fieldMapping == "" {
		return item
	}
	applyFieldMapping(&item, content, fieldMapping)
	return item
}

// applyFieldMapping 按 JSON 映射从 LLM 内容提取字段到 DataItem。
// 映射方向：{"llm_field":"dataitem_field"}
func applyFieldMapping(item *entity.DataItem, content, mappingJSON string) {
	// 解析 LLM 内容为 JSON
	var llmData map[string]any
	if err := json.Unmarshal([]byte(extractJSON(content)), &llmData); err != nil {
		return // 不是 JSON，整体作为 content（已在 item.Content）
	}
	// 解析映射
	var mapping map[string]string
	if err := json.Unmarshal([]byte(mappingJSON), &mapping); err != nil {
		return
	}
	for llmField, itemField := range mapping {
		val, ok := llmData[llmField]
		if !ok {
			continue
		}
		strVal := toString(val)
		switch itemField {
		case "title":
			if strVal != "" {
				item.Title = strVal
			}
		case "content":
			if strVal != "" {
				item.Content = strVal
			}
		case "summary":
			if strVal != "" {
				item.Summary = strVal
			}
		case "tags":
			if tags, ok := val.([]any); ok {
				item.Tags = make([]string, 0, len(tags))
				for _, t := range tags {
					item.Tags = append(item.Tags, toString(t))
				}
			} else if strVal != "" {
				item.Tags = []string{strVal}
			}
		default:
			// metadata.xxx
			if len(itemField) > 9 && itemField[:9] == "metadata." {
				if item.Metadata == nil {
					item.Metadata = map[string]string{}
				}
				item.Metadata[itemField[9:]] = strVal
			}
		}
	}
}

// extractJSON 从可能含 markdown 包裹的文本中提取 JSON 对象。
func extractJSON(s string) string {
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == '{' {
			start = i
			break
		}
	}
	if start < 0 {
		return s
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}

// toString 把任意类型转为字符串（JSON 值可能是 string/number/bool）。
func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%v", val)
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}
