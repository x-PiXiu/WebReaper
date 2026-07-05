package entity

import "regexp"

// PIIRedactor 个人身份信息（PII）脱敏器（值对象，纯领域逻辑）。
//
// 设计动机（合规 / GDPR / PIPL）：
//   - 采集的内容可能包含邮箱、手机号、身份证号等个人数据。
//   - 法律要求：未经授权不应存储可识别个人的信息。
//   - 脱敏是领域规则（业务不变量），不依赖任何框架，放领域层。
//
// 实现策略：用正则识别 PII，替换为带标记的脱敏形式（保留可识别性但不泄露原值）。
// 这不是密码学级别的匿名化，而是"入库前的第一道合规过滤"。
type PIIRedactor struct{}

// 预编译的正则（init 时编译一次，避免每次调用重编译）。
var (
	// 邮箱：xxx@xxx.xxx → x***@xxx.xxx
	emailRe = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	// 中国大陆手机号：1开头11位，前后需非数字边界（避免匹配身份证号中间片段）
	phoneRe = regexp.MustCompile(`(?:^|\D)(1[3-9]\d{9})(?:\D|$)`)
	// 18位身份证号（简单匹配，含末位X）→ 110101********1234
	idCardRe = regexp.MustCompile(`[1-9]\d{5}(19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]`)
	// 银行卡号（16-19位连续数字，前后需非数字边界，避免误伤身份证）
	// 注：身份证号在 Redact 中先于银行卡处理，已脱敏为含 * 的字符串，不会再被此正则匹配。
	bankCardRe = regexp.MustCompile(`(?:^|\D)([1-9]\d{15,18})(?:\D|$)`)
)

// Redact 对文本进行 PII 脱敏，返回脱敏后的文本。
// 顺序很重要：身份证（18位）必须在银行卡（16-19位）之前处理，
// 否则身份证号会被银行卡正则误匹配。
func (PIIRedactor) Redact(text string) string {
	if text == "" {
		return text
	}
	text = emailRe.ReplaceAllStringFunc(text, maskEmail)
	// 手机号：用捕获组，前后边界字符（空白/标点）保留，只替换中间的 11 位
	text = phoneRe.ReplaceAllStringFunc(text, func(full string) string {
		// 从匹配中提取 11 位手机号（去掉首尾的中英文标点边界）
		return maskPhone(trimPunct(full))
	})
	text = idCardRe.ReplaceAllStringFunc(text, maskIDCard)
	// 银行卡：用捕获组替换（保留前后空白边界）
	text = bankCardRe.ReplaceAllStringFunc(text, func(full string) string {
		// 提取中间的纯数字部分（去掉首尾的非数字边界）
		return maskBankCard(trimPunct(full))
	})
	return text
}

// maskEmail 邮箱脱敏：保留首字符和域名 → a***@example.com
func maskEmail(s string) string {
	at := -1
	for i, c := range s {
		if c == '@' {
			at = i
			break
		}
	}
	if at <= 0 {
		return s
	}
	return string(s[0]) + "***" + s[at:]
}

// maskPhone 手机号脱敏：138****1234
func maskPhone(s string) string {
	if len(s) != 11 {
		return s
	}
	return s[:3] + "****" + s[7:]
}

// trimPunct 去掉字符串首尾的所有非数字字符（用于从正则匹配中提取纯数字）。
func trimPunct(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] < '0' || s[start] > '9') {
		start++
	}
	for end > start && (s[end-1] < '0' || s[end-1] > '9') {
		end--
	}
	return s[start:end]
}

// maskIDCard 身份证脱敏：保留前6后4，中间打码
func maskIDCard(s string) string {
	if len(s) < 10 {
		return s
	}
	return s[:6] + "********" + s[len(s)-4:]
}

// maskBankCard 银行卡脱敏：保留前4后4
func maskBankCard(s string) string {
	if len(s) < 8 {
		return s
	}
	return s[:4] + "****" + s[len(s)-4:]
}

// DefaultPIIRedactor 默认脱敏器实例（无状态，可全局复用）。
var DefaultPIIRedactor = PIIRedactor{}
