package provider

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"webreaper/internal/usecase/port"
)

// SwitchingProvider 按当前生效 Key 动态委派的生成服务商（mock↔真实热切换）。
//
// 修复的 bug：此前 main 启动时按"那一刻"的 Key 一次性选定 mock 或 ViduProvider，
// 之后在管理后台保存 Key 也无法把运行中的 mock 切成真实——因为热更新接口
// （port.ConfigurableProvider）只有 ViduProvider 实现，对 mock 的类型断言静默失败，
// 唯一出路是重启。本类型把"用哪个"从装配期推迟到每次调用期：
//   - resolve 返回非空 Key 且厂商启用 → 委派真实 provider
//   - 否则 → 委派 mock（演示模式，不发真实请求）
//
// 切换粒度 = resolve 缓存 TTL（默认 10s）——管理后台保存后 ≤10s 全链路生效，
// 无需重启。已在途的 mock 任务（task_id 以 "mock-" 开头）无论当前 Key 状态，
// Poll/Cancel 始终路由回 mock——防止 Key 切换后 mock 任务去真实 API 查 404 空转。
type SwitchingProvider struct {
	real  port.GenerationProvider
	mock  port.GenerationProvider
	// resolve 当前生效 Key 与启用状态（DB 厂商配置优先，环境变量兜底——由 main 装配）
	resolve func() (key string, enabled bool)
	mu       sync.Mutex
	keyCache string
	enabled  bool
	cachedAt time.Time
	ttl      time.Duration
}

// NewSwitchingProvider 创建切换器。resolve 每次调用应是廉价的（main 侧可再包缓存）。
func NewSwitchingProvider(real, mock port.GenerationProvider, resolve func() (string, bool)) *SwitchingProvider {
	sp := &SwitchingProvider{real: real, mock: mock, resolve: resolve, ttl: 10 * time.Second}
	// 初始化缓存（避免首次调用与并发下重复 resolve）
	sp.refresh()
	return sp
}

func (p *SwitchingProvider) refresh() {
	key, enabled := p.resolve()
	p.keyCache, p.enabled, p.cachedAt = key, enabled, time.Now()
}

// current 当前应委派的 provider（TTL 缓存的 Key 判定；mock 任务前缀路由见 withTask）。
func (p *SwitchingProvider) current() port.GenerationProvider {
	p.mu.Lock()
	if time.Since(p.cachedAt) > p.ttl {
		p.refresh()
	}
	key, enabled := p.keyCache, p.enabled
	p.mu.Unlock()
	if key != "" && enabled {
		return p.real
	}
	return p.mock
}

// withTask 按任务 ID 路由：mock- 前缀（mock 提交的在途任务）固定回 mock。
func (p *SwitchingProvider) withTask(taskID string) port.GenerationProvider {
	if strings.HasPrefix(taskID, "mock-") {
		return p.mock
	}
	return p.current()
}

func (p *SwitchingProvider) Name() string { return p.current().Name() }

// UpdateAPIKey 管理后台保存 Key 后即时生效（port.ConfigurableProvider）：
// 刷新本切换器缓存 + 同步推给真实 provider（其内部也有 TTL keySource 兜底）。
func (p *SwitchingProvider) UpdateAPIKey(key string) {
	p.mu.Lock()
	p.keyCache, p.enabled, p.cachedAt = key, true, time.Now()
	p.mu.Unlock()
	if cp, ok := p.real.(port.ConfigurableProvider); ok {
		cp.UpdateAPIKey(key)
	}
}

var _ port.ConfigurableProvider = (*SwitchingProvider)(nil)

func (p *SwitchingProvider) Submit(ctx context.Context, endpoint string, body map[string]any) (port.SubmitResult, error) {
	return p.current().Submit(ctx, endpoint, body)
}

func (p *SwitchingProvider) Poll(ctx context.Context, taskID string) (port.GenerationStatus, error) {
	return p.withTask(taskID).Poll(ctx, taskID)
}

func (p *SwitchingProvider) Cancel(ctx context.Context, taskID string) error {
	return p.withTask(taskID).Cancel(ctx, taskID)
}

func (p *SwitchingProvider) VerifyCallback(ctx context.Context, header http.Header, body []byte, requestURI string) error {
	return p.current().VerifyCallback(ctx, header, body, requestURI)
}

func (p *SwitchingProvider) QueryCredits(ctx context.Context) (int, error) {
	return p.current().QueryCredits(ctx)
}

func (p *SwitchingProvider) TranslateError(code string) string {
	return p.current().TranslateError(code)
}
