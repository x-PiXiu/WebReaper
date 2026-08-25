package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// ContentAdapter 内容适配器接口（策略模式）。
//
// 每个平台的内容规范不同（标题长度、内容长度、标签数量、Emoji限制等）。
// 用例层通过此接口将统一内容适配为平台特定格式。
type ContentAdapter interface {
	// Adapt 适配内容到指定平台。
	Adapt(ctx context.Context, req AdaptRequest) (*entity.AdaptedContent, error)
	// Platform 返回支持的平台。
	Platform() string
}

// AdaptRequest 适配请求。
type AdaptRequest struct {
	Platform    string   // 目标平台
	Title       string   // 原始标题
	Description string   // 原始描述
	Tags        []string // 原始标签
	PersonaID   string   // 人设ID（可选，用于风格调整）
}

// ContentAdapterRegistry 内容适配器注册表。
type ContentAdapterRegistry interface {
	// Get 获取指定平台的适配器。
	Get(platform string) (ContentAdapter, error)
	// List 列出所有注册的适配器。
	List() []ContentAdapter
}
