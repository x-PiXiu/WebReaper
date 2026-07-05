package entity

import "testing"

func TestPIIRedactor_Email(t *testing.T) {
	r := PIIRedactor{}
	got := r.Redact("联系我：john.doe@example.com 或 admin@test.org")
	// 邮箱首字符保留，中间打码
	if contains(got, "john.doe@") || contains(got, "admin@") {
		t.Errorf("邮箱未脱敏: %q", got)
	}
	if !contains(got, "j***@example.com") {
		t.Errorf("邮箱脱敏格式错误: %q", got)
	}
}

func TestPIIRedactor_Phone(t *testing.T) {
	r := PIIRedactor{}
	got := r.Redact("电话：13812345678，备用：15987654321")
	// 手机号保留前3后4
	if contains(got, "13812345678") || contains(got, "15987654321") {
		t.Errorf("手机号未脱敏: %q", got)
	}
	if !contains(got, "138****5678") {
		t.Errorf("手机号脱敏格式错误: %q", got)
	}
}

func TestPIIRedactor_IDCard(t *testing.T) {
	r := PIIRedactor{}
	got := r.Redact("身份证号：110101199001011234")
	if contains(got, "110101199001011234") {
		t.Errorf("身份证号未脱敏: %q", got)
	}
	// 保留前6后4
	if !contains(got, "110101********1234") {
		t.Errorf("身份证号脱敏格式错误: %q", got)
	}
}

func TestPIIRedactor_BankCard(t *testing.T) {
	r := PIIRedactor{}
	got := r.Redact("卡号：6222021234567890123")
	if contains(got, "6222021234567890123") {
		t.Errorf("银行卡号未脱敏: %q", got)
	}
}

func TestPIIRedactor_NoPII(t *testing.T) {
	r := PIIRedactor{}
	// 无 PII 的正常文本应原样返回
	in := "这是一段普通的中文内容，不含任何个人信息。"
	got := r.Redact(in)
	if got != in {
		t.Errorf("无 PII 文本被错误修改: %q", got)
	}
}

func TestPIIRedactor_Empty(t *testing.T) {
	r := PIIRedactor{}
	if got := r.Redact(""); got != "" {
		t.Errorf("空字符串应原样返回, got %q", got)
	}
}

// contains 简化版 strings.Contains（避免引入 fmt 之外的依赖）
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
