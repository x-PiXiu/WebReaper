// Package douyinoauth 实现抖音开放平台网站应用 OAuth 2.0 授权（port.OAuthProvider）。
//
// 获客智能体架构演进：官方 OAuth 授权绑定（API 通道）替代浏览器扫码绑定（RPA 通道）。
// 流程（授权码模式）：
//  1. ConnectURL → 用户打开授权页（PC 端默认展示二维码，手机抖音扫码确认）
//  2. 抖音回调 redirect_uri?code=xxx&state=xxx（code 一次性有效）
//  3. ExchangeCode：code 换 access_token / refresh_token / open_id
//  4. UserInfo：拉昵称头像做账号显示名；access_token 过期前 RefreshToken 续期
//
// 实现决策（2026-08-20，统一官方 SDK）：
//   抖音 OpenAPI 调用统一走官方 SDK（bytedance/douyin-openapi-sdk-go），
//   包括 OAuth 与后续视频发布（VideoUpload/VideoCreate）——避免"登录手写、
//   其他用 SDK"的双轨维护。本适配器只保留 SDK 不覆盖的三件事：
//   1. ConnectURL：授权页地址拼接（前端跳转，非 API 调用）
//   2. StateCodec：state 签名防 CSRF（我们的安全层）
//   3. token 存储/续期调度：多租户账号体系（DB + AES-GCM），SDK 的 credential
//      助手是单实例内存缓存，多实例部署会 token 互刷——token 一律从账号库解密注入。
//
// SDK 已知限制：方法无 ctx 参数（老版生成码），无法按请求取消——调用方超时
// 由 usecase 层的 context 控制（当前回调/巡检路径均有外层超时兜底）。
package douyinoauth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	credential "github.com/bytedance/douyin-openapi-credential-go/client"
	openapi "github.com/bytedance/douyin-openapi-sdk-go/client"

	"webreaper/internal/domain/entity"
)

const (
	// defaultScope 默认只申请 user_info——抖音授权页规则：请求的 scope 中任何一个
	// 未开通（如 video.create.bind 需定向准入），整个授权请求直接报「scope权限非法」。
	// 拿到视频发布等权限后，通过 DOUYIN_OAUTH_SCOPE 环境变量扩展（逗号分隔）。
	defaultScope = "user_info"
	stateTTL     = 10 * time.Minute // state 有效期（授权流程应在此内完成）
	defaultCallback = "http://localhost:8082/api/v1/merchant/accounts/douyin/oauth/callback"
)

// Client 抖音开放平台 OAuth 客户端（内部为官方 SDK）。
type Client struct {
	sdk          *openapi.Client
	clientKey    string
	clientSecret string
	callbackURL  string // 必须与开放平台控制台「授权回调地址」完全一致
	scope        string
}

// NewClient 创建客户端（初始化官方 SDK）。callbackURL 空时用默认本地开发回调；
// scope 空时用 defaultScope（只能填应用已开通的 scope，逗号分隔）。
func NewClient(clientKey, clientSecret, callbackURL, scope string) (*Client, error) {
	if callbackURL == "" {
		callbackURL = defaultCallback
	}
	if scope == "" {
		scope = defaultScope
	}
	sdk, err := openapi.NewClient(new(credential.Config).
		SetClientKey(clientKey).
		SetClientSecret(clientSecret))
	if err != nil {
		return nil, fmt.Errorf("初始化抖音 OpenAPI SDK 失败: %w", err)
	}
	return &Client{
		sdk:          sdk,
		clientKey:    clientKey,
		clientSecret: clientSecret,
		callbackURL:  callbackURL,
		scope:        scope,
	}, nil
}

// ConnectURL 生成授权页地址（PC 端展示扫码二维码；state 为签名后的绑定上下文）。
func (c *Client) ConnectURL(state string) string {
	q := url.Values{}
	q.Set("client_key", c.clientKey)
	q.Set("response_type", "code")
	q.Set("scope", c.scope)
	q.Set("redirect_uri", c.callbackURL)
	q.Set("state", state)
	return "https://open.douyin.com/platform/oauth/connect/?" + q.Encode()
}

// ExchangeCode 用授权码换 token（code 一次性有效，5 分钟内使用）。
func (c *Client) ExchangeCode(_ context.Context, code string) (*entity.OAuthToken, error) {
	resp, err := c.sdk.OauthAccessToken(new(openapi.OauthAccessTokenRequest).
		SetClientKey(c.clientKey).
		SetClientSecret(c.clientSecret).
		SetCode(code).
		SetGrantType("authorization_code"))
	if err != nil {
		return nil, fmt.Errorf("请求抖音开放平台失败: %w", err)
	}
	if resp == nil || resp.Data == nil {
		return nil, fmt.Errorf("抖音返回空响应")
	}
	d := resp.Data
	if d.ErrorCode != nil && *d.ErrorCode != 0 {
		return nil, fmt.Errorf("抖音 OAuth 失败: error_code=%d %s", *d.ErrorCode, derefStr(d.Description))
	}
	if d.AccessToken == nil || *d.AccessToken == "" {
		return nil, fmt.Errorf("响应缺少 access_token")
	}
	return &entity.OAuthToken{
		AccessToken:      *d.AccessToken,
		RefreshToken:     derefStr(d.RefreshToken),
		ExpiresIn:        int(derefI64(d.ExpiresIn)),
		RefreshExpiresIn: int(derefI64(d.RefreshExpiresIn)),
		OpenID:           derefStr(d.OpenId),
		Scope:            derefStr(d.Scope),
	}, nil
}

// RefreshToken 用 refresh_token 换新的 access_token（旧 refresh_token 同时失效，必须落库新的）。
func (c *Client) RefreshToken(_ context.Context, refreshToken string) (*entity.OAuthToken, error) {
	resp, err := c.sdk.OauthRefreshToken(new(openapi.OauthRefreshTokenRequest).
		SetClientKey(c.clientKey).
		SetRefreshToken(refreshToken).
		SetGrantType("refresh_token"))
	if err != nil {
		return nil, fmt.Errorf("请求抖音开放平台失败: %w", err)
	}
	if resp == nil || resp.Data == nil {
		return nil, fmt.Errorf("抖音返回空响应")
	}
	d := resp.Data
	if d.ErrorCode != nil && *d.ErrorCode != 0 {
		return nil, fmt.Errorf("刷新 token 失败: error_code=%d %s", *d.ErrorCode, derefStr(d.Description))
	}
	if d.AccessToken == nil || *d.AccessToken == "" {
		return nil, fmt.Errorf("刷新响应缺少 access_token")
	}
	return &entity.OAuthToken{
		AccessToken:      *d.AccessToken,
		RefreshToken:     derefStr(d.RefreshToken),
		ExpiresIn:        int(derefI64(d.ExpiresIn)),
		RefreshExpiresIn: int(derefI64(d.RefreshExpiresIn)),
		OpenID:           derefStr(d.OpenId),
		Scope:            derefStr(d.Scope),
	}, nil
}

// UserInfo 拉取授权用户公开信息（昵称/头像）。
func (c *Client) UserInfo(_ context.Context, accessToken, openID string) (*entity.OAuthUserInfo, error) {
	resp, err := c.sdk.OauthUserinfo(new(openapi.OauthUserinfoRequest).
		SetAccessToken(accessToken).
		SetOpenId(openID))
	if err != nil {
		return nil, fmt.Errorf("请求抖音开放平台失败: %w", err)
	}
	if resp == nil || resp.Data == nil {
		return nil, fmt.Errorf("抖音返回空响应")
	}
	return &entity.OAuthUserInfo{
		Nickname: derefStr(resp.Data.Nickname),
		Avatar:   derefStr(resp.Data.Avatar),
	}, nil
}

// derefStr / derefI64 SDK 响应字段全部是指针（生成码风格），安全解引用。
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefI64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// ---- state 签名编解码（port.OAuthStateCodec）----

// StateCodec HMAC-SHA256 签名的 state 编解码器。
//
// state 结构：payload.expUnix.hex(hmac(payload.expUnix))——payload 由调用方自定义
// （AccountUseCase 用 "tenantID|userID"），验签 + 过期校验双重防护。
type StateCodec struct {
	secret []byte
}

func NewStateCodec(secret string) *StateCodec {
	return &StateCodec{secret: []byte(secret)}
}

// SignState payload → "payload.expUnix.signature"。
func (s *StateCodec) SignState(payload string) string {
	exp := time.Now().Add(stateTTL).Unix()
	msg := fmt.Sprintf("%s.%d", payload, exp)
	sig := hex.EncodeToString(s.hmacSum([]byte(msg)))
	return msg + "." + sig
}

// VerifyState 验签 + 过期校验，返回原始 payload。
func (s *StateCodec) VerifyState(state string) (string, error) {
	parts := strings.Split(state, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("state 格式无效")
	}
	payload, expStr, sig := parts[0], parts[1], parts[2]
	msg := []byte(payload + "." + expStr)
	expect := hex.EncodeToString(s.hmacSum(msg))
	if !hmac.Equal([]byte(sig), []byte(expect)) {
		return "", fmt.Errorf("state 签名校验失败")
	}
	var exp int64
	if _, err := fmt.Sscanf(expStr, "%d", &exp); err != nil || time.Now().Unix() > exp {
		return "", fmt.Errorf("state 已过期或无效")
	}
	return payload, nil
}

func (s *StateCodec) hmacSum(data []byte) []byte {
	m := hmac.New(sha256.New, s.secret)
	m.Write(data)
	return m.Sum(nil)
}
