// Package payment 提供支付网关适配器实现（策略替换点）。
//
// 当前策略：MockPaymentGateway（默认，开发/演示）。
// 预留：StripeAdapter（配置 STRIPE_API_KEY 时启用）。
// 与 Vidu provider 同款降级——真实对接需商户资质，当前阶段只做骨架。
package payment

import (
	"context"
	"fmt"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// MockPaymentGateway 模拟支付网关。
//
// 行为：
//   - CreatePayment 返回模拟支付页 URL + 流水号（不调真实 API）
//   - QueryPayment 始终返回 paid（简化演示：创建即视为已支付）
//
// 用途：开发阶段完整演示"选套餐→下单→支付→开通"全流程，不接入真实支付。
type MockPaymentGateway struct {
	baseURL string // 模拟支付页根地址
}

func NewMockPaymentGateway(baseURL string) *MockPaymentGateway {
	if baseURL == "" {
		baseURL = "http://localhost:5173" // 前端地址（mock 支付页）
	}
	return &MockPaymentGateway{baseURL: baseURL}
}

func (g *MockPaymentGateway) Name() string { return "mock" }

// CreatePayment 创建模拟支付：返回支付页 URL（前端展示扫码/确认按钮）+ 流水号。
func (g *MockPaymentGateway) CreatePayment(_ context.Context, order entity.Order) (string, string, error) {
	if order.ID == "" {
		return "", "", fmt.Errorf("订单 ID 不能为空")
	}
	paymentID := fmt.Sprintf("mock-pay-%d", time.Now().UnixNano())
	payURL := fmt.Sprintf("%s/billing/pay?order=%s&pid=%s", g.baseURL, order.ID, paymentID)
	return payURL, paymentID, nil
}

// QueryPayment 查询支付状态。
// mock 行为：创建超过 0 秒即视为已支付（演示用——真实网关需轮询/回调）。
func (g *MockPaymentGateway) QueryPayment(_ context.Context, paymentID string) (string, error) {
	if paymentID == "" {
		return "", fmt.Errorf("payment_id 不能为空")
	}
	return "paid", nil // mock：始终已支付
}

// VerifyCallback mock 验签：直接信任参数（无签名）。
// 真实通道（zpay/stripe）需要做 MD5/HMAC 签名验证。
func (g *MockPaymentGateway) VerifyCallback(_ context.Context, params map[string]string) (port.CallbackResult, error) {
	outTradeNo := params["out_trade_no"]
	if outTradeNo == "" {
		return port.CallbackResult{}, fmt.Errorf("回调缺少 out_trade_no")
	}
	amountStr := params["money"]
	amountCents := 0
	if amountStr != "" {
		var yuan float64
		fmt.Sscanf(amountStr, "%f", &yuan)
		amountCents = int(yuan * 100)
	}
	status := "pending"
	if params["trade_status"] == "TRADE_SUCCESS" || params["status"] == "1" {
		status = "paid"
	}
	return port.CallbackResult{
		OutTradeNo:  outTradeNo,
		TradeNo:     params["trade_no"],
		AmountCents: amountCents,
		Status:      status,
	}, nil
}
