package port

import (
	"context"
	"sync"
)

// 发布通道轴（发布域重构：平台×素材×通道三轴分离的"通道轴"）。
//
// 设计（整洁架构诊断 P1/P2/P4/P5/P6 的解）：
//   - PublishTransport 按"执行机制"分派（link 预填链接 / rpa 浏览器自动化 / api 官方接口），
//     不再与平台身份纠缠——同一平台可注册多条通道共存
//   - TransportRegistry 双键索引（platform → kind → transport），管理后台可按平台
//     强制指定通道（override，自动降级链的头号优先级）
//   - CredentialResolver 把凭证解析（rpa→cookie / api→token）从用例层收口到适配器——
//     用例只表达业务意图，不碰解密细节
//
// 通道选择策略（用例层）：自动降级链 [override > api(OAuth账号) > rpa(cookie账号) > link]，
// 手动 override 是优先级而非死命令——失败仍沿链降级。

// TransportKind 执行机制标识。
const (
	TransportLink = "link" // 半自动：生成预填发布页 URL，用户手动确认
	TransportRPA  = "rpa"  // 浏览器自动化（cookie 会话）
	TransportAPI  = "api"  // 官方开放平台接口（OAuth token）
)

// TransportRequest 通道执行请求（凭证已由 resolver 解析完毕——通道拿到即用）。
type TransportRequest struct {
	Job      PublishJobRequest // 作品内容（标题/正文/素材/形态）
	Account  AccountView       // 账号信息（平台/显示名/凭证类型）
	Cookie   string            // RPA 凭证（cookie 会话，link/api 通道忽略）
	APIToken string            // API 凭证（OAuth access_token，link/rpa 通道忽略）
}

// PublishJobRequest 通道无关的发布内容描述。
type PublishJobRequest struct {
	ID          string // 任务 ID（截图命名/日志追踪——通道层需要）
	TenantID    string
	Title       string
	Content     string
	ContentType string   // article/image/video/audio
	MediaURLs   []string // 媒体 URL（图片[]/视频[mp4]）
	CoverURL    string
	Tags        []string // 标签（B站独立标签框等）
	Category    string   // 平台分区（B站投稿必选）
	Privacy     string   // 可见性（youtube: public/unlisted/private）
}

// AccountView 账号的通道视角（不携带密文——凭证走 resolver 单独解析）。
type AccountView struct {
	ID          string
	Platform    string
	DisplayName string
	AuthType    string // cookie / oauth
}

// TransportResult 通道执行结果。
type TransportResult struct {
	ExternalURL string // 发布产物链接（预填页/已发布作品）
}

// PublishTransport 发布通道策略接口（策略模式——三条执行机制可互换、可共存）。
type PublishTransport interface {
	// Kind 执行机制标识（link/rpa/api）。
	Kind() string
	// Platforms 本通道覆盖的平台列表（注册用）。
	Platforms() []string
	// Publish 执行发布。凭证已在 req 中解析完毕；通道内部不再接触仓储/加密。
	Publish(ctx context.Context, req TransportRequest) (TransportResult, error)
}

// CredentialResolver 凭证解析（rpa→cookie 解密；api→access_token 解密）。
// 把"哪种通道需要哪种凭证、怎么解密"从用例层收口到适配器。
type CredentialResolver interface {
	Resolve(ctx context.Context, tenantID, accountID, kind string) (cookie, apiToken string, err error)
}

// TransportRegistry 双键通道注册表（platform → kind → transport）+ 平台级 override。
type TransportRegistry struct {
	mu        sync.RWMutex
	byPlatKind map[string]map[string]PublishTransport
	overrides  map[string]string // platform → 强制 kind（管理后台手动切换；空值=恢复自动）
}

func NewTransportRegistry() *TransportRegistry {
	return &TransportRegistry{
		byPlatKind: make(map[string]map[string]PublishTransport),
		overrides:  make(map[string]string),
	}
}

// Register 注册通道（覆盖其声明的所有平台；同平台多通道共存——双键）。
func (r *TransportRegistry) Register(t PublishTransport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range t.Platforms() {
		if r.byPlatKind[p] == nil {
			r.byPlatKind[p] = make(map[string]PublishTransport)
		}
		r.byPlatKind[p][t.Kind()] = t
	}
}

// SetOverride 管理后台手动切换：platform 强制走 kind（"" 清除=恢复自动）。
func (r *TransportRegistry) SetOverride(platform, kind string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if kind == "" {
		delete(r.overrides, platform)
	} else {
		r.overrides[platform] = kind
	}
}

// Overrides 返回当前 override 快照（admin 端点/启动恢复用）。
func (r *TransportRegistry) Overrides() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string, len(r.overrides))
	for k, v := range r.overrides {
		out[k] = v
	}
	return out
}

// RestoreOverrides 批量恢复（启动时从 system_settings 加载）。
func (r *TransportRegistry) RestoreOverrides(m map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.overrides = make(map[string]string, len(m))
	for k, v := range m {
		if v != "" {
			r.overrides[k] = v
		}
	}
}

// Select 按优先序选择首个已注册通道（候选链由用例层按意图+凭证构造）。
// override 通道若在候选内则必选它（管理员优先级）；返回 nil = 无可用通道。
func (r *TransportRegistry) Select(platform string, candidates []string) PublishTransport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	kinds := r.byPlatKind[platform]
	if len(kinds) == 0 {
		return nil
	}
	// override 优先（管理员的明确意志）
	if ov, ok := r.overrides[platform]; ok {
		if t, ok2 := kinds[ov]; ok2 {
			return t
		}
	}
	for _, c := range candidates {
		if t, ok := kinds[c]; ok {
			return t
		}
	}
	return nil
}

// Chain 返回降级链上该平台所有可用通道（按候选序 + override 头插；去重）。
// 用例层沿链依次尝试，一条路短路自动切换到下一条。
func (r *TransportRegistry) Chain(platform string, candidates []string) []PublishTransport {
	r.mu.RLock()
	defer r.mu.RUnlock()
	kinds := r.byPlatKind[platform]
	ordered := make([]string, 0, len(candidates)+1)
	if ov, ok := r.overrides[platform]; ok && ov != "" {
		ordered = append(ordered, ov)
	}
	ordered = append(ordered, candidates...)
	seen := make(map[string]bool)
	out := make([]PublishTransport, 0, len(ordered))
	for _, k := range ordered {
		if seen[k] {
			continue
		}
		seen[k] = true
		if t, ok := kinds[k]; ok {
			out = append(out, t)
		}
	}
	return out
}

// RegisteredPlatforms 已注册通道的平台列表（admin 通道矩阵展示用）。
func (r *TransportRegistry) RegisteredPlatforms() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byPlatKind))
	for p := range r.byPlatKind {
		out = append(out, p)
	}
	return out
}

// Kinds 平台已注册的通道列表（能力展示/admin UI 用）。
func (r *TransportRegistry) Kinds(platform string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byPlatKind[platform]))
	for k := range r.byPlatKind[platform] {
		out = append(out, k)
	}
	return out
}
