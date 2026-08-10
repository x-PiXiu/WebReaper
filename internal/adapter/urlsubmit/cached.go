package urlsubmit

import (
	"context"
	"sync"
	"time"

	"webreaper/internal/adapter/indexnow"
	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ConfigLoader 加载运行时收录配置（由 main 注入：system_settings 优先、env 兜底）。
type ConfigLoader func(ctx context.Context) (entity.IndexingConfig, error)

// CachedProvider 是 port.URLSubmitter 的"动态配置"实现。
//
// 设计动机（运行时配置免重启，参照 LLMConfig 30s TTL 先例）：
//   - 收录配置（IndexNow key / 百度 token）在管理后台可改，改后不应要求重启。
//   - 本 Provider 每次提交时检查配置缓存：TTL 30s 内复用已构建的 MultiSubmitter，
//     过期则从 ConfigLoader 重新读取并按配置重建（新增/移除渠道自动生效）。
//   - 未配置任何渠道时返回 no-op（提交空转成功，不阻断业务）。
type CachedProvider struct {
	load     ConfigLoader
	baseURL  string // 公开站根地址（构建 IndexNow keyLocation 用）
	ttl      time.Duration
	buildFn  func(entity.IndexingConfig) (port.URLSubmitter, error) // 可替换（测试注入 mock 渠道）
	mu       sync.Mutex
	cached   port.URLSubmitter
	cachedAt time.Time
	lastErr  error // 最近一次构建错误（暴露给诊断）
}

// NewCachedProvider 创建动态配置提交器。
func NewCachedProvider(load ConfigLoader, baseURL string) *CachedProvider {
	p := &CachedProvider{load: load, baseURL: baseURL, ttl: 30 * time.Second}
	p.buildFn = p.build // 默认构建（按配置选渠道）
	return p
}

// SubmitURLs 提交（按需重建 submitter 后转发）。
func (p *CachedProvider) SubmitURLs(ctx context.Context, urls []string) error {
	sub, err := p.current(ctx)
	if err != nil {
		return err
	}
	return sub.SubmitURLs(ctx, urls)
}

// current 获取当前 submitter（TTL 缓存 + 双检锁）。
func (p *CachedProvider) current(ctx context.Context) (port.URLSubmitter, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cached != nil && time.Since(p.cachedAt) < p.ttl {
		return p.cached, nil
	}
	cfg, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	sub, err := p.buildFn(cfg)
	if err != nil {
		p.lastErr = err
		return nil, err
	}
	p.cached = sub
	p.cachedAt = time.Now()
	p.lastErr = nil
	return sub, nil
}

// build 按配置构建渠道组合（IndexNow/百度按配置非空启用）。
func (p *CachedProvider) build(cfg entity.IndexingConfig) (port.URLSubmitter, error) {
	var submitters []port.URLSubmitter
	if cfg.IndexNowKey != "" {
		keyLocation := trimRightSlash(p.baseURL) + "/public/indexnow-key.txt"
		s, err := indexnow.NewSubmitter(p.baseURL, cfg.IndexNowKey, keyLocation)
		if err != nil {
			return nil, err
		}
		submitters = append(submitters, s)
	}
	if cfg.BaiduSite != "" && cfg.BaiduToken != "" {
		s, err := NewBaiduSubmitter(cfg.BaiduSite, cfg.BaiduToken)
		if err != nil {
			return nil, err
		}
		submitters = append(submitters, s)
	}
	if len(submitters) == 0 {
		// 未启用任何渠道：no-op 提交器（空转成功，不阻断业务）
		return noopSubmitter{}, nil
	}
	return NewMultiSubmitter(submitters...), nil
}

// LastError 最近一次构建错误（管理后台诊断用）。
func (p *CachedProvider) LastError() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastErr
}

// noopSubmitter 空提交器（无渠道时占位）。
type noopSubmitter struct{}

func (noopSubmitter) SubmitURLs(context.Context, []string) error { return nil }

// trimRightSlash 去尾部斜杠。
func trimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

var _ port.URLSubmitter = (*CachedProvider)(nil)
