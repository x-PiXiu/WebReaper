package publisher

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ---- 半自动发布通道（策略模式）----
//
// 半自动 = 系统生成内容 + 预填到平台发布页 URL → 用户手动确认发布。
// 零封号风险（人工确认 = 有审核留痕），合规底线。
//
// 整洁架构要点：
//   - 每个平台实现 port.PublishChannel 接口。
//   - 用例层通过 registry 按平台名获取通道，不直接依赖具体实现。
//   - 后续升级全自动 RPA = 新增 PublishAuto 方法实现，业务零改动（推迟决策）。

// ZhihuChannel 知乎半自动发布通道。
type ZhihuChannel struct{}

var _ port.PublishChannel = (*ZhihuChannel)(nil)

func NewZhihuChannel() *ZhihuChannel { return &ZhihuChannel{} }

func (c *ZhihuChannel) Platform() string           { return "zhihu" }
func (c *ZhihuChannel) SupportedMediaType() []string { return []string{"text"} }
func (c *ZhihuChannel) SupportedContentTypes() []string { return []string{entity.ContentTypeArticle} }

// DisplayName / Constraints 能力声明（port.ChannelInfoProvider）。
func (c *ZhihuChannel) DisplayName() string { return "知乎" }
func (c *ZhihuChannel) Constraints() map[string]entity.ChannelConstraints {
	return map[string]entity.ChannelConstraints{
		entity.ContentTypeArticle: {TitleMaxRunes: 100}, // 知乎专栏标题上限 100 字
	}
}

// PublishSemiAuto 生成知乎写文章页 URL（知乎暂不支持 URL 预填正文，引导用户到发布页）。
func (c *ZhihuChannel) PublishSemiAuto(_ context.Context, job entity.PublishJob, _ entity.Account) (string, error) {
	// 知乎的写文章页：https://zhuanlan.zhihu.com/write
	// 注意：知乎不通过 URL 参数预填正文（有 CSRF 保护），用户需手动粘贴。
	// 返回发布页 URL + title 查询参数（部分场景可用）。
	u := "https://zhuanlan.zhihu.com/write"
	if job.Title != "" {
		u += "?title=" + url.QueryEscape(job.Title)
	}
	return u, nil
}

// XiaohongshuChannel 小红书半自动发布通道。
type XiaohongshuChannel struct{}

var _ port.PublishChannel = (*XiaohongshuChannel)(nil)

func NewXiaohongshuChannel() *XiaohongshuChannel { return &XiaohongshuChannel{} }

func (c *XiaohongshuChannel) Platform() string           { return "xiaohongshu" }
func (c *XiaohongshuChannel) SupportedMediaType() []string { return []string{"text", "image"} }
func (c *XiaohongshuChannel) SupportedContentTypes() []string { return []string{entity.ContentTypeImage, entity.ContentTypeVideo, entity.ContentTypeArticle, entity.ContentTypeAudio} }

// DisplayName / Constraints 能力声明（此前标题 20 字/配图必填散在前端硬编码——规则归位适配器）。
func (c *XiaohongshuChannel) DisplayName() string { return "小红书" }
func (c *XiaohongshuChannel) Constraints() map[string]entity.ChannelConstraints {
	return map[string]entity.ChannelConstraints{
		entity.ContentTypeImage: {TitleMaxRunes: 20, MinImages: 1}, // 图文笔记：标题≤20字、至少1图
		entity.ContentTypeVideo: {TitleMaxRunes: 20},               // 视频笔记：标题≤20字
	}
}

// PublishSemiAuto 生成小红书发布页 URL。
func (c *XiaohongshuChannel) PublishSemiAuto(_ context.Context, _ entity.PublishJob, _ entity.Account) (string, error) {
	// 小红书创作者中心发布页
	return "https://creator.xiaohongshu.com/publish/publish", nil
}

// ---- 发布通道注册表（工厂模式）----

// ChannelRegistry 发布通道注册表实现。
type ChannelRegistry struct {
	channels map[string]port.PublishChannel
}

var _ port.PublishChannelRegistry = (*ChannelRegistry)(nil)

// NewChannelRegistry 创建注册表并注册所有内置通道。
// 注册的是同时支持半自动+全自动的通道（覆盖旧的纯半自动通道）。
func NewChannelRegistry() *ChannelRegistry {
	r := &ChannelRegistry{channels: make(map[string]port.PublishChannel)}
	r.Register(NewZhihuAutoChannel())
	r.Register(NewXiaohongshuAutoChannel())
	r.Register(NewDouyinAutoChannel())     // 抖音（获客智能体转型：视频分发主战场）
	r.Register(NewKuaishouAutoChannel())   // 快手
	return r
}

// Register 注册一个发布通道（新增平台 = 注册新适配器，开闭原则）。
func (r *ChannelRegistry) Register(ch port.PublishChannel) {
	r.channels[ch.Platform()] = ch
}

// Register 按平台名获取通道。
func (r *ChannelRegistry) Get(platform string) (port.PublishChannel, error) {
	// 兼容前端可能传的别名
	platform = normalizePlatform(platform)
	ch, ok := r.channels[platform]
	if !ok {
		return nil, fmt.Errorf("publish channel not registered for platform: %s", platform)
	}
	return ch, nil
}

// AccountStoreUser 支持账号存储注入的通道（cookie 滚动回写用——发布会话后
// 把浏览器最新 cookie 写回账号库，绑定寿命从扫码快照变成滚动续期）。
type AccountStoreUser interface {
	SetAccountStore(ar port.AccountRepository, v port.CookieVault)
}

// SetAccountStore 向所有支持账号存储的通道转发（main 装配时调用一次）。
func (r *ChannelRegistry) SetAccountStore(ar port.AccountRepository, v port.CookieVault) {
	for _, ch := range r.channels {
		if asu, ok := ch.(AccountStoreUser); ok {
			asu.SetAccountStore(ar, v)
		}
	}
}

// List 列出所有已注册通道。
func (r *ChannelRegistry) List() []port.PublishChannel {
	out := make([]port.PublishChannel, 0, len(r.channels))
	for _, ch := range r.channels {
		out = append(out, ch)
	}
	return out
}

// normalizePlatform 平台名归一化（兼容常见别名）。
func normalizePlatform(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	switch p {
	case "知乎":
		return "zhihu"
	case "小红书", "xhs":
		return "xiaohongshu"
	case "抖音", "dy":
		return "douyin"
	case "快手", "ks":
		return "kuaishou"
	}
	return p
}
