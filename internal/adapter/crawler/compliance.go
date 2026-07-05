package crawler

import (
	"strings"
)

// IsLoginRequiredContent 检测内容是否疑似"需要登录才能查看"的页面。
//
// 合规防护（P2）：不绕过认证、不爬登录态/付费墙内容。
// 通过常见登录页特征识别，命中则建议跳过该内容。
//
// 识别特征（保守，宁可漏判不误判）：
//   - 页面标题/正文含 "登录"/"请先登录"/"sign in"/"log in"
//   - 页面主要是登录表单（含 password input）
//   - HTTP 状态为 401/403（但这一层在 HTTP 客户端处理，这里只看内容）
func IsLoginRequiredContent(title, content string) bool {
	// 只检查前 2000 字符（登录提示通常在页首）
	head := strings.ToLower(title + " " + content)
	if len(head) > 2000 {
		head = head[:2000]
	}

	// 强信号：明确的登录要求文案
	strongSignals := []string{
		"请先登录", "请登录后", "登录查看", "登录后查看",
		"sign in to continue", "please log in", "please sign in",
		"login required", "您还没有登录",
	}
	for _, s := range strongSignals {
		if strings.Contains(head, s) {
			return true
		}
	}

	// 弱信号：页面同时有"用户名"+"密码"输入框特征（HTML 残留）
	if strings.Contains(head, `type="password"`) &&
		(strings.Contains(head, "登录") || strings.Contains(head, "log in") || strings.Contains(head, "sign in")) {
		return true
	}

	return false
}

// IsPaywallContent 检测内容是否疑似付费墙页面。
func IsPaywallContent(title, content string) bool {
	head := strings.ToLower(title + " " + content)
	if len(head) > 2000 {
		head = head[:2000]
	}
	signals := []string{
		"订阅后查看", "付费阅读", "开通会员", "购买专栏",
		"subscribe to continue", "subscribe to read", "paywall",
		"premium content", "仅限会员",
	}
	for _, s := range signals {
		if strings.Contains(head, s) {
			return true
		}
	}
	return false
}
