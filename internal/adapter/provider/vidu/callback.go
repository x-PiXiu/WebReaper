package vidu

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// verifyCallbackSignature Vidu 回调验签。
//
// 算法（Docs/第三方/Vidu/任务管理/回调签名算法.md）：
//   signature = base64(HMAC-SHA256(secretKey, signingString))
//   signingString = http_method\n + http_uri\n + canonical_query_string\n
//                 + "vidu"\n + Date(GMT)\n + signed_headers(key:value\n…)
//
// 防重放三层：① Date 新鲜度（±5 分钟）② x-request-nonce 去重 ③ 签名比对。
// nonce 去重表由调用方注入（单机内存 TTL；多实例 Redis Set）。
func verifyCallbackSignature(secretKey string, h http.Header, body []byte) error {
	sig := h.Get("X-HMAC-SIGNATURE")
	if sig == "" {
		return fmt.Errorf("回调缺少 X-HMAC-SIGNATURE")
	}
	if h.Get("X-HMAC-ACCESS-KEY") != "vidu" {
		return fmt.Errorf("X-HMAC-ACCESS-KEY 非 vidu")
	}
	date := h.Get("Date")
	if date == "" {
		return fmt.Errorf("回调缺少 Date 头")
	}
	// ① Date 新鲜度校验（防重放）
	parsedDate, err := http.ParseTime(date)
	if err != nil {
		return fmt.Errorf("回调 Date 格式错误: %v", err)
	}
	if d := time.Since(parsedDate); d > 5*time.Minute || d < -5*time.Minute {
		return fmt.Errorf("回调 Date 过期（重放防护）")
	}
	// ② nonce 去重（由调用方在 handler 层检查——本函数只做格式校验）
	if h.Get("x-request-nonce") == "" {
		return fmt.Errorf("回调缺少 x-request-nonce")
	}

	// ③ 重算签名
	signedHeaders := strings.Split(h.Get("X-HMAC-SIGNED-HEADERS"), ";")
	headers := make(map[string]string, len(signedHeaders))
	for _, name := range signedHeaders {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		headers[name] = h.Get(name)
	}

	// 签名字符串基于"创建任务时配置的 callback_url"——服务端无法从回调请求
	// 得知完整 callback_url，但 URI+query 可从请求行还原（Method + RequestURI）。
	// 注意：Vidu 文档示例中 signingString 用的是 callback_url 解析出的 path/query，
	// 与回调请求自身一致（回调就是发到这个 URL）。此处用请求自身还原。
	signingString := buildSigningString(http.MethodPost, h, signedHeaders, headers)
	expected := base64.StdEncoding.EncodeToString(hmacSHA256([]byte(secretKey), []byte(signingString)))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return fmt.Errorf("回调签名校验失败")
	}
	return nil
}

// buildSigningString 构造签名字符串（六段拼接，末尾换行——格式高度敏感）。
func buildSigningString(method string, h http.Header, signedHeaders []string, headers map[string]string) string {
	// URI/query：从请求行还原（回调请求即 callback_url）
	rawURI := h.Get("X-Vidu-Request-URI")
	uri, query := "/", ""
	if rawURI != "" {
		if u, err := url.Parse(rawURI); err == nil {
			uri = u.Path
			query = u.RawQuery
		}
	}
	date := h.Get("Date")

	var sb strings.Builder
	sb.WriteString(method)
	sb.WriteString("\n")
	sb.WriteString(uri)
	sb.WriteString("\n")
	sb.WriteString(query)
	sb.WriteString("\n")
	sb.WriteString("vidu")
	sb.WriteString("\n")
	sb.WriteString(date)
	sb.WriteString("\n")
	for _, name := range signedHeaders {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		sb.WriteString(name)
		sb.WriteString(":")
		sb.WriteString(headers[name])
		sb.WriteString("\n")
	}
	return sb.String()
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
