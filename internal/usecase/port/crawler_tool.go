package port

import (
	"context"
	"sync"

	"webreaper/internal/domain/entity"
)

// toolTenantKey ctx 键（工具执行的租户注入——商户主 Agent 工具的租户隔离）。
type toolTenantKey struct{}

// toolRoleKey ctx 键（工具执行的角色注入——admin 工具的越权防线，32号 F1-6 接线引入）。
type toolRoleKey struct{}

// WithToolRole 把当前会话角色注入 ctx（chat handler 调用；admin 工具执行层校验）。
func WithToolRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, toolRoleKey{}, role)
}

// ToolRoleFrom 从 ctx 取工具执行角色（空=未注入，admin 工具应拒绝执行）。
func ToolRoleFrom(ctx context.Context) string {
	v, _ := ctx.Value(toolRoleKey{}).(string)
	return v
}

// WithToolTenant 把当前商户租户注入 ctx（chat handler 调用；工具执行链全程透传）。
// 工具不得信任 LLM 传来的租户参数——租户一律从 ctx 取（安全边界）。
func WithToolTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, toolTenantKey{}, tenantID)
}

// ToolTenantFrom 从 ctx 取工具执行租户（空=未注入，工具应拒绝执行写操作）。
func ToolTenantFrom(ctx context.Context) string {
	v, _ := ctx.Value(toolTenantKey{}).(string)
	return v
}

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

// ToolRegistry 是工具的注册表（策略模式 + 注册表，与 SpiderRegistry 同构）。
// Agent 按 AgentConfig.Tools 从这里查找允许调用的工具。
//
// 支持运行时动态启用/禁用工具（enabled 状态）：
//   - Register 时默认 enabled=true
//   - SetEnabled(name, false) 禁用后，All()/GetByNames() 自动过滤掉
//     —— Agent 拿不到禁用的工具，等于"该工具不可用"
//   - AllWithStatus 返回含 enabled 状态，供工具面板展示
type ToolRegistry struct {
	mu      sync.RWMutex
	tools   map[string]CrawlerTool
	enabled map[string]bool // 工具启用状态（默认 true）
}

// NewToolRegistry 创建空的工具注册表。
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]CrawlerTool), enabled: make(map[string]bool)}
}

// Register 注册一个工具（默认启用）。
func (r *ToolRegistry) Register(t CrawlerTool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
	r.enabled[t.Name()] = true // 默认启用
}

// SetEnabled 动态启用/禁用工具（工具面板调）。
// 禁用后 Agent 拿不到该工具（All/GetByNames 过滤）。未注册的工具名忽略。
func (r *ToolRegistry) SetEnabled(name string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tools[name]; ok {
		r.enabled[name] = enabled
	}
}

// Lookup 按名称查找工具（不论启用状态）。
func (r *ToolRegistry) Lookup(name string) (CrawlerTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// GetByNames 批量查找（按名称列表），只返回已启用的工具。
// Agent 通过此方法拿工具——禁用的自动被过滤。
func (r *ToolRegistry) GetByNames(names []string) []CrawlerTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []CrawlerTool
	for _, name := range names {
		if t, ok := r.tools[name]; ok && r.enabled[name] {
			result = append(result, t)
		}
	}
	return result
}

// All 返回所有已启用的工具（Agent 用）。
func (r *ToolRegistry) All() []CrawlerTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]CrawlerTool, 0, len(r.tools))
	for name, t := range r.tools {
		if r.enabled[name] {
			result = append(result, t)
		}
	}
	return result
}

// ToolStatus 工具状态（供工具面板展示）。
type ToolStatus struct {
	Name        string
	Description string
	Enabled     bool
}

// AllWithStatus 返回所有工具及其启用状态（工具面板用，含禁用的）。
func (r *ToolRegistry) AllWithStatus() []ToolStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ToolStatus, 0, len(r.tools))
	for name, t := range r.tools {
		result = append(result, ToolStatus{
			Name: name, Description: t.Description(), Enabled: r.enabled[name],
		})
	}
	return result
}
