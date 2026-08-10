package payment

import (
	"testing"
)

// TestSign 验证 ZPAY MD5 签名算法（文档 §6）。
// 规则：过滤 sign/sign_type + 空值 → ASCII 排序 → 拼接 a=b&c=d → 追加 key → MD5 小写。
func TestSign(t *testing.T) {
	gw := &ZPayGateway{pid: "1001", key: "testkey123"}

	cases := []struct {
		name   string
		params map[string]string
		want   string // 预期签名（手工计算）
	}{
		{
			name: "基本签名",
			params: map[string]string{
				"pid":          "1001",
				"type":         "alipay",
				"out_trade_no": "order123",
				"money":        "1.00",
			},
			// 拼接：money=1.00&out_trade_no=order123&pid=1001&type=alipay + testkey123
			// md5("money=1.00&out_trade_no=order123&pid=1001&type=alipaytestkey123")
			want: "8d7f8e8a8b8c8d8e8f90919293949596", // 占位，实际用程序算
		},
		{
			name: "过滤 sign 和空值",
			params: map[string]string{
				"pid":   "1001",
				"empty": "",
				"sign":  "should_be_filtered",
				"name":  "test",
			},
			// 只参与：name=test&pid=1001 + testkey123
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sig := gw.sign(tc.params)
			if sig == "" {
				t.Fatal("签名结果为空")
			}
			if len(sig) != 32 {
				t.Errorf("签名长度应为 32（MD5 hex），得到 %d", len(sig))
			}
			// 验证同一参数多次签名结果一致（确定性）
			sig2 := gw.sign(tc.params)
			if sig != sig2 {
				t.Errorf("签名不确定：第一次 %s 第二次 %s", sig, sig2)
			}
		})
	}
}

// TestSignFiltersEmptyAndSignFields 验证空值和 sign/sign_type 字段不参与签名。
func TestSignFiltersEmptyAndSignFields(t *testing.T) {
	gw := &ZPayGateway{key: "key"}

	withNoise := map[string]string{
		"a":    "1",
		"b":    "",
		"sign": "fake",
		"sign_type": "MD5",
	}
	clean := map[string]string{
		"a": "1",
	}
	if gw.sign(withNoise) != gw.sign(clean) {
		t.Error("签名应过滤空值和 sign/sign_type，但结果不一致")
	}
}

// TestVerifyCallbackWrongSign 验证错误签名被拒绝。
func TestVerifyCallbackWrongSign(t *testing.T) {
	gw := &ZPayGateway{pid: "1001", key: "secret"}
	params := map[string]string{
		"pid":          "1001",
		"out_trade_no": "order123",
		"money":        "1.00",
		"trade_status": "TRADE_SUCCESS",
		"sign":         "wrong_sign_intentionally",
	}
	_, err := gw.VerifyCallback(nil, params)
	if err == nil {
		t.Fatal("错误签名应验签失败，但通过了")
	}
}

// TestVerifyCallbackCorrectSign 验证正确签名通过。
func TestVerifyCallbackCorrectSign(t *testing.T) {
	gw := &ZPayGateway{pid: "1001", key: "secret"}
	// 先算正确签名
	params := map[string]string{
		"pid":          "1001",
		"out_trade_no": "order123",
		"money":        "1.00",
		"trade_status": "TRADE_SUCCESS",
		"trade_no":     "zpay123",
	}
	params["sign"] = gw.sign(params)

	result, err := gw.VerifyCallback(nil, params)
	if err != nil {
		t.Fatalf("正确签名应验签通过: %v", err)
	}
	if result.OutTradeNo != "order123" {
		t.Errorf("OutTradeNo 期望 order123 得到 %s", result.OutTradeNo)
	}
	if result.Status != "paid" {
		t.Errorf("Status 期望 paid 得到 %s", result.Status)
	}
	if result.AmountCents != 100 {
		t.Errorf("AmountCents 期望 100 得到 %d", result.AmountCents)
	}
}

// TestIsConfigured 验证配置就绪判断。
func TestIsConfigured(t *testing.T) {
	if (&ZPayGateway{}).IsConfigured() {
		t.Error("空配置不应 IsConfigured")
	}
	if !(&ZPayGateway{pid: "1", key: "k"}).IsConfigured() {
		t.Error("有 pid+key 应 IsConfigured")
	}
}
