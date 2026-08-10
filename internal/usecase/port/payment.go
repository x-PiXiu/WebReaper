package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// ---- 支付网关（经济系统：在线收款的抽象边界）----
//
// 整洁架构：支付是"会变的细节"（mock/zpay/stripe/alipay...），
// 用 port 接口隔离——BillingUseCase 只依赖此接口，不感知具体实现。
//
// 多通道设计：每个支付通道实现此接口，main 装配时按配置选择当前通道。
// 切换通道 = 换适配器实例，业务零改动。
//
// 真实对接的关键安全要求：
//   1. CreatePayment 返回支付页/二维码 URL 给前端
//   2. QueryPayment 供 ConfirmPayment 二次验证（防"未付款确认"攻击）
//   3. VerifyCallback 验证异步回调签名（防伪造回调——支付安全核心）
//   4. CallbackResult 提取订单号+金额（回调验签后统一处理）

// PaymentGateway 支付网关（创建支付 + 查询 + 回调验签）。
type PaymentGateway interface {
	// CreatePayment 为订单创建支付，返回支付页 URL 与第三方流水号。
	CreatePayment(ctx context.Context, order entity.Order) (paymentURL, paymentID string, err error)
	// QueryPayment 查询支付状态（pending/paid/failed）。
	// 用于 ConfirmPayment 二次验证——防止用户未付款直接调 confirm 白嫖。
	QueryPayment(ctx context.Context, paymentID string) (status string, err error)
	// VerifyCallback 验证异步回调签名，提取订单信息。
	// 返回验签结果 + 回调数据（含订单号、金额、支付状态）。
	// 验签失败返回 error——调用方据此拒绝回调。
	// rawParams 是回调原始参数（未解析，各通道格式不同：zpay 是 GET query，stripe 是 JSON body）。
	VerifyCallback(ctx context.Context, rawParams map[string]string) (CallbackResult, error)
	// Name 网关标识（mock / zpay / stripe / alipay）。
	Name() string
}

// CallbackResult 异步回调验签后提取的统一结果。
// 各通道回调格式不同（zpay 是 GET 参数 + MD5 签名，stripe 是 JSON + HMAC 签名），
// 验签后提取为统一结构，BillingUseCase 据此 ConfirmPayment。
type CallbackResult struct {
	OutTradeNo string // 商户订单号（对应 entity.Order.ID）
	TradeNo    string // 第三方流水号（对应 entity.Order.PaymentID）
	AmountCents int   // 实付金额（分）——用于校验金额一致
	Status     string // 支付状态：paid / pending / failed
}
