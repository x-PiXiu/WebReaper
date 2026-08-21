// Package transport 提供发布通道策略的适配器实现（通道轴独立成轴的重构落地）。
//
// 三条通道共存（同一平台可注册多条）：
//   - LinkTransport：包装各平台 Channel.PublishSemiAuto（预填 URL 生成）——现有逻辑平移
//   - RPATransport：包装各平台 AutoPublishChannel.PublishAuto（浏览器自动化）——现有逻辑平移
//   - API 通道：权限批下来后新增实现（结构已就位——真·推迟决策，不预写空壳）
//
// 凭证解耦：通道经 CredentialResolver 拿解密后的凭证，用例层不再碰 vault。
package transport

import (
	"context"
	"fmt"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// LinkTransport 半自动通道：生成预填发布页 URL（包装现有 PublishChannel.PublishSemiAuto）。
type LinkTransport struct {
	channels port.PublishChannelRegistry // 旧注册表复用（平台 → Channel 的 PublishSemiAuto）
}

func NewLinkTransport(cr port.PublishChannelRegistry) *LinkTransport { return &LinkTransport{channels: cr} }

func (t *LinkTransport) Kind() string      { return port.TransportLink }
func (t *LinkTransport) Platforms() []string {
	out := make([]string, 0)
	for _, ch := range t.channels.List() {
		out = append(out, ch.Platform())
	}
	return out
}

func (t *LinkTransport) Publish(ctx context.Context, req port.TransportRequest) (port.TransportResult, error) {
	ch, err := t.channels.Get(req.Account.Platform)
	if err != nil {
		return port.TransportResult{}, err
	}
	acc := entity.Account{ID: req.Account.ID, Platform: req.Account.Platform}
	url, err := ch.PublishSemiAuto(ctx, toLegacyJob(req), acc)
	if err != nil {
		return port.TransportResult{}, fmt.Errorf("生成发布链接失败: %w", err)
	}
	return port.TransportResult{ExternalURL: url}, nil
}

// RPATransport 浏览器自动化通道（包装现有 AutoPublishChannel.PublishAuto）。
// RPA 执行是分钟级流程：内部沿用"立即返回 running + 后台 goroutine 收尾"的既有约定
// ——本方法同步返回时只负责启动，最终状态由包装的既有异步机制落库。
type RPATransport struct {
	channels port.PublishChannelRegistry
}

func NewRPATransport(cr port.PublishChannelRegistry) *RPATransport { return &RPATransport{channels: cr} }

func (t *RPATransport) Kind() string      { return port.TransportRPA }
func (t *RPATransport) Platforms() []string {
	out := make([]string, 0)
	for _, ch := range t.channels.List() {
		if _, ok := ch.(port.AutoPublishChannel); ok {
			out = append(out, ch.Platform())
		}
	}
	return out
}

// Publish RPA 执行。凭证（cookie）已由用例层经 resolver 解析放入 req——通道内部零仓储依赖。
// 注意：返回的 error 是"启动失败"；发布进行中/最终结果由调用方（用例层）的既有异步
// 状态机管理（running → published/failed）。风控类失败（cookie 失效/滑块）返回
// ErrTransportDegradable，用例层据此切换下一通道。
func (t *RPATransport) Publish(ctx context.Context, req port.TransportRequest) (port.TransportResult, error) {
	if req.Cookie == "" {
		return port.TransportResult{}, fmt.Errorf("%w: cookie 凭证缺失（需浏览器通道账号）", ErrTransportDegradable)
	}
	ch, err := t.channels.Get(req.Account.Platform)
	if err != nil {
		return port.TransportResult{}, err
	}
	autoCh, ok := ch.(port.AutoPublishChannel)
	if !ok {
		return port.TransportResult{}, fmt.Errorf("%w: %s 无 RPA 实现", ErrTransportDegradable, req.Account.Platform)
	}
	url, err := autoCh.PublishAuto(ctx, toLegacyJob(req), req.Cookie)
	if err != nil {
		return port.TransportResult{}, fmt.Errorf("RPA 发布失败: %w", err)
	}
	return port.TransportResult{ExternalURL: url}, nil
}

// ErrTransportDegradable 可降级错误——用例层看到它就切换下一通道（自动短路切换）。
// 风控/凭证类失败是暂时的（换通道可解），业务校验失败不该降级（换通道也没用）。
var ErrTransportDegradable = fmt.Errorf("transport degradable")

// VaultCredentialResolver CredentialResolver 实现：按通道类型解密对应凭证。
type VaultCredentialResolver struct {
	vault    port.CookieVault
	accounts port.AccountRepository
}

func NewVaultCredentialResolver(v port.CookieVault, ar port.AccountRepository) *VaultCredentialResolver {
	return &VaultCredentialResolver{vault: v, accounts: ar}
}

// Resolve rpa → cookie 解密；api → access_token 解密。账号不存在/密文缺失报错（不降级标记——由用例层判断）。
func (r *VaultCredentialResolver) Resolve(ctx context.Context, tenantID, accountID, kind string) (string, string, error) {
	acc, err := r.accounts.FindByID(ctx, tenantID, accountID)
	if err != nil {
		return "", "", fmt.Errorf("账号不存在: %w", err)
	}
	switch kind {
	case port.TransportRPA:
		if acc.CookieEncrypted == "" {
			return "", "", fmt.Errorf("该账号无浏览器通道凭证（cookie 未绑定）")
		}
		cookie, dErr := r.vault.Decrypt(acc.CookieEncrypted)
		if dErr != nil {
			return "", "", fmt.Errorf("cookie 解密失败: %w", dErr)
		}
		return cookie, "", nil
	case port.TransportAPI:
		if acc.AccessTokenEnc == "" {
			return "", "", fmt.Errorf("该账号无官方通道凭证（OAuth 未授权）")
		}
		token, dErr := r.vault.Decrypt(acc.AccessTokenEnc)
		if dErr != nil {
			return "", "", fmt.Errorf("token 解密失败: %w", dErr)
		}
		return "", token, nil
	default: // link 无需凭证
		return "", "", nil
	}
}

// toLegacyJob TransportRequest → 旧 PublishJob（包装层转换——旧通道签名不变，零改动平移）。
func toLegacyJob(req port.TransportRequest) entity.PublishJob {
	return entity.PublishJob{
		TenantID:    req.Job.TenantID,
		AccountID:   req.Account.ID,
		Platform:    req.Account.Platform,
		Title:       req.Job.Title,
		Content:     req.Job.Content,
		ContentType: req.Job.ContentType,
		MediaURLs:   req.Job.MediaURLs,
		CoverURL:    req.Job.CoverURL,
	}
}

// 编译期断言。
var (
	_ port.PublishTransport      = (*LinkTransport)(nil)
	_ port.PublishTransport      = (*RPATransport)(nil)
	_ port.CredentialResolver    = (*VaultCredentialResolver)(nil)
)
