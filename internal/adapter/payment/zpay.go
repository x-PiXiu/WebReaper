package payment

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// ZPayGateway 基于 ZPAY 易支付协议的支付适配器。
//
// API 依据 Docs/第三方/支付/ZPAY支付/ZPAY-API.md：
//   - 创建支付（API 接口）：POST https://zpayz.cn/mapi.php（返回二维码/支付链接）
//   - 查询订单：GET https://zpayz.cn/api.php?act=order
//   - 异步回调通知：notify_url 接收 GET 参数，验签后返回 "success"
//   - 签名算法：MD5（参数按 ASCII 排序拼接 + 密钥 → md5 小写）
//
// 配置通过 ZPayConfig 注入（admin 后台可管理，支持运行时切换）。
type ZPayGateway struct {
	pid       string // 商户 ID
	key       string // 商户密钥（签名用）
	notifyURL string // 异步回调地址（本服务公开端点）
	returnURL string // 支付完成后浏览器跳转地址
	client    *http.Client
}

// ZPayConfig ZPAY 运行时配置（admin 后台设置）。
type ZPayConfig struct {
	PID       string `json:"pid"`        // 商户 ID
	Key       string `json:"key"`        // 商户密钥
	NotifyURL string `json:"notify_url"` // 异步回调地址（公网可达）
	ReturnURL string `json:"return_url"` // 支付完成跳转地址
}

// NewZPayGateway 创建 ZPAY 适配器。
func NewZPayGateway(cfg ZPayConfig) *ZPayGateway {
	return &ZPayGateway{
		pid:       cfg.PID,
		key:       cfg.Key,
		notifyURL: cfg.NotifyURL,
		returnURL: cfg.ReturnURL,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

// IsConfigured 判断 ZPAY 配置是否就绪（pid + key 非空）。
func (g *ZPayGateway) IsConfigured() bool {
	return g.pid != "" && g.key != ""
}

func (g *ZPayGateway) Name() string { return "zpay" }

// CreatePayment 调 ZPAY API 接口创建支付（mapi.php），返回支付页 URL。
// 默认走支付宝（type=alipay）；订单号用 order.ID；金额从分转元。
func (g *ZPayGateway) CreatePayment(ctx context.Context, order entity.Order) (string, string, error) {
	// 先构建不含签名的参数（签名时过滤 sign/sign_type）
	params := map[string]string{
		"pid":          g.pid,
		"type":         "alipay",
		"out_trade_no": order.ID,
		"notify_url":   g.notifyURL,
		"name":         fmt.Sprintf("WebReaper %s 套餐订阅", order.PlanID),
		"money":        fmt.Sprintf("%.2f", float64(order.AmountCents)/100),
		"clientip":     "127.0.0.1",
	}
	// 计算签名后加入参数（sign 不参与自身签名计算）
	params["sign"] = g.sign(params)
	params["sign_type"] = "MD5"

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://zpayz.cn/mapi.php", strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("ZPAY 创建支付请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var parsed struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		TradeNo string `json:"trade_no"`
		PayURL  string `json:"payurl"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", fmt.Errorf("解析 ZPAY 响应失败: %w (body: %s)", err, string(body))
	}
	if parsed.Code != 1 {
		return "", "", fmt.Errorf("ZPAY 创建支付失败: %s", parsed.Msg)
	}

	// 优先返回支付跳转 URL（直接跳支付页）；没有则用二维码 URL
	payURL := parsed.PayURL
	if payURL == "" {
		return "", "", fmt.Errorf("ZPAY 响应缺少 payurl")
	}
	return payURL, parsed.TradeNo, nil
}

// QueryPayment 查询 ZPAY 订单状态（api.php?act=order）。
// 返回统一状态：paid / pending / failed。
func (g *ZPayGateway) QueryPayment(ctx context.Context, outTradeNo string) (string, error) {
	endpoint := fmt.Sprintf("https://zpayz.cn/api.php?act=order&pid=%s&key=%s&out_trade_no=%s",
		url.QueryEscape(g.pid), url.QueryEscape(g.key), url.QueryEscape(outTradeNo))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ZPAY 查询订单失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var parsed struct {
		Code   int    `json:"code"`
		Msg    string `json:"msg"`
		Status int    `json:"status"` // 1=成功 0=未支付
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("解析 ZPAY 查询响应失败: %w", err)
	}
	if parsed.Code != 1 {
		return "failed", fmt.Errorf("ZPAY 查询失败: %s", parsed.Msg)
	}
	if parsed.Status == 1 {
		return "paid", nil
	}
	return "pending", nil
}

// VerifyCallback 验证 ZPAY 异步回调签名。
//
// ZPAY 回调是 GET 参数，签名规则（文档 §6）：
//   1. 过滤 sign/sign_type 和空值
//   2. 参数按 ASCII 排序拼接 a=b&c=d
//   3. 追加 key 后 MD5 → 小写
//
// 验签通过后提取统一 CallbackResult。金额校验由调用方做（比对订单原金额）。
func (g *ZPayGateway) VerifyCallback(_ context.Context, params map[string]string) (port.CallbackResult, error) {
	// 验签
	expectedSign := g.sign(params)
	actualSign := params["sign"]
	if actualSign == "" || expectedSign != actualSign {
		return port.CallbackResult{}, fmt.Errorf("ZPAY 回调签名验证失败")
	}

	// 提取统一结果
	outTradeNo := params["out_trade_no"]
	if outTradeNo == "" {
		return port.CallbackResult{}, fmt.Errorf("回调缺少 out_trade_no")
	}
	amountCents := 0
	if money := params["money"]; money != "" {
		var yuan float64
		fmt.Sscanf(money, "%f", &yuan)
		amountCents = int(yuan * 100)
	}
	status := "pending"
	if params["trade_status"] == "TRADE_SUCCESS" {
		status = "paid"
	}
	return port.CallbackResult{
		OutTradeNo:  outTradeNo,
		TradeNo:     params["trade_no"],
		AmountCents: amountCents,
		Status:      status,
	}, nil
}

// sign ZPAY MD5 签名算法（文档 §6）。
//   1. 过滤 sign/sign_type 和空值
//   2. 参数按 ASCII 排序拼接 a=b&c=d
//   3. 追加 key 后 MD5 → 小写
func (g *ZPayGateway) sign(params map[string]string) string {
	// 过滤 + 排序
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if v == "" || k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接 a=b&c=d
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(params[k])
	}
	// 追加 key + MD5
	sb.WriteString(g.key)
	sum := md5.Sum([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}
