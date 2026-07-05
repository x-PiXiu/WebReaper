package port

import (
	"context"
	"sync"

	"webreaper/internal/domain/entity"
)

// CrawlerTool 是爬虫工具的业务接口（边界）。
//
// 这是 port 层的抽象，不依赖 trpc-agent-go 的 tool 包。
// adapter/crawler 层实现此接口，同时适配为 trpc-agent-go 的 CallableTool。
// 用例层通过此接口管理工具，不关心框架细节。
type CrawlerTool interface {
	// Name 工具名（如 "api_crawler"），用于注册表分派。
	Name() string
	// Description 工具描述（给 Agent/LLM 看的，说明这个工具能干什么）。
	Description() string
	// Execute 执行爬取。argsJSON 是 LLM 传来的参数（JSON）。
	// 返回 DataItem（通用数据项，含原始内容和元数据）。
	Execute(ctx context.Context, argsJSON string) (entity.DataItem, error)
	// ToolDeclaration 返回工具的参数 schema（给 LLM 的工具描述）。
	// 不同爬虫有不同参数（url/method/headers/query 等），各自声明。
	ToolDeclaration() ToolDecl
}

// ToolDecl 是工具声明的业务层抽象（不依赖 trpc-agent-go 的 tool 包）。
type ToolDecl struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Properties  map[string]PropSpec `json:"properties"`
	Required    []string          `json:"required"`
}

// PropSpec 单个参数的规格。
type PropSpec struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ToolRegistry 是爬虫工具的注册表（策略模式 + 注册表，与 SpiderRegistry 同构）。
// Agent 按 AgentConfig.Tools 从这里查找允许调用的工具。
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]CrawlerTool
}

// NewToolRegistry 创建空的工具注册表。
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]CrawlerTool)}
}

// Register 注册一个爬虫工具。
func (r *ToolRegistry) Register(t CrawlerTool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Lookup 按名称查找工具。
func (r *ToolRegistry) Lookup(name string) (CrawlerTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// GetByNames 批量查找（按 AgentConfig.Tools 列表）。
func (r *ToolRegistry) GetByNames(names []string) []CrawlerTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []CrawlerTool
	for _, name := range names {
		if t, ok := r.tools[name]; ok {
			result = append(result, t)
		}
	}
	return result
}

// All 返回所有已注册的工具（供调试/列表展示）。
func (r *ToolRegistry) All() []CrawlerTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]CrawlerTool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}
