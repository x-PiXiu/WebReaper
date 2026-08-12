package vidu

import (
	"encoding/base64"
	"net/http"
	"testing"
	"time"
)

// 构造合法回调头（按签名算法生成签名）。
func signedCallbackHeaders(secret, uri, query, date string) http.Header {
	nonce := "test-nonce-123"
	// 签名字符串：POST\nuri\nquery\nvidu\ndate\nDate:...\nx-request-nonce:...
	signing := "POST\n" + uri + "\n" + query + "\nvidu\n" + date + "\n" +
		"Date:" + date + "\nx-request-nonce:" + nonce + "\n"
	sig := base64.StdEncoding.EncodeToString(hmacSHA256([]byte(secret), []byte(signing)))

	h := http.Header{}
	h.Set("X-HMAC-SIGNATURE", sig)
	h.Set("X-HMAC-ACCESS-KEY", "vidu")
	h.Set("X-HMAC-ALGORITHM", "hmac-sha256")
	h.Set("X-HMAC-SIGNED-HEADERS", "Date;x-request-nonce")
	h.Set("Date", date)
	h.Set("x-request-nonce", nonce)
	h.Set("X-Vidu-Request-URI", uri+"?"+query)
	return h
}

func TestVerifyCallbackValid(t *testing.T) {
	secret := "test-secret-key"
	date := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
	h := signedCallbackHeaders(secret, "/api/v1/generation/callback", "name=james", date)
	if err := verifyCallbackSignature(secret, h, []byte(`{"state":"success"}`)); err != nil {
		t.Fatalf("合法签名应通过: %v", err)
	}
}

func TestVerifyCallbackBadSignature(t *testing.T) {
	secret := "test-secret-key"
	date := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
	h := signedCallbackHeaders(secret, "/api/v1/generation/callback", "name=james", date)
	h.Set("X-HMAC-SIGNATURE", "tampered-signature")
	if err := verifyCallbackSignature(secret, h, nil); err == nil {
		t.Fatal("篡改签名应拒绝")
	}
}

func TestVerifyCallbackStaleDate(t *testing.T) {
	secret := "test-secret-key"
	// 10 分钟前的 Date（超 ±5 分钟窗口）
	date := time.Now().UTC().Add(-10 * time.Minute).Format("Mon, 02 Jan 2006 15:04:05 GMT")
	h := signedCallbackHeaders(secret, "/api/v1/generation/callback", "", date)
	if err := verifyCallbackSignature(secret, h, nil); err == nil {
		t.Fatal("过期 Date 应拒绝（重放防护）")
	}
}

func TestVerifyCallbackMissingNonce(t *testing.T) {
	secret := "test-secret-key"
	date := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
	h := signedCallbackHeaders(secret, "/api/v1/generation/callback", "", date)
	h.Del("x-request-nonce")
	if err := verifyCallbackSignature(secret, h, nil); err == nil {
		t.Fatal("缺 nonce 应拒绝")
	}
}
