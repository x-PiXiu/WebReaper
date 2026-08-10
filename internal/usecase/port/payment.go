package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// ---- 支付网关（经济系统：在线收款的抽象边界）----
//
// 整洁架构：支付是"会变的细节"（mock/stripe/alipay/wechat...），
// 用 port 接口隔离——BillingUseCase 只依赖此接口，不感知具体实现。
//
// 与 ViduProvider 同款的降级策略：
//   - MockPaymentGateway（默认）：模拟支付 URL + 自动确认，开发/演示可用
//   - StripeAdapter（预留）：配置 STRIPE_API_KEY 时启用，走真实 API
//
// 真实对接需商户资质/密钥/回调签名验证，属另一阶段工程——当前只做骨架。

// PaymentGateway 支付网关（创建支付 + 查询状态）。
type PaymentGateway interface {
	// CreatePayment 为订单创建支付，返回支付页 URL 与第三方流水号。
	CreatePayment(ctx context.Context, order entity.Order) (paymentURL, paymentID string, err error)
	// QueryPayment 查询支付状态（pending/paid/failed）。
	QueryPayment(ctx context.Context, paymentID string) (status string, err error)
	// Name 网关标识（mock / stripe / alipay）。
	Name() string
}
