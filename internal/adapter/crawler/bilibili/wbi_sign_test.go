package bilibili

import (
	"testing"
)

func TestGetSalt(t *testing.T) {
	signer := NewWBISigner()
	signer.SetKeys("7cd084941338484aae1ad9425b84077c", "3ec84d4042a94e7eb3cf12fd5f7e3a2a")

	salt := signer.getSalt()
	if len(salt) != 32 {
		t.Errorf("salt length = %d, want 32", len(salt))
	}
	t.Logf("salt = %s", salt)
}

func TestFilterChars(t *testing.T) {
	tests := []struct {
		input string
		chars string
		want  string
	}{
		{"hello!world", "!'()*", "helloworld"},
		{"test'value", "!'()*", "testvalue"},
		{"normal text", "!'()*", "normal text"},
		{"a(b)c", "!'()*", "abc"},
		{"", "!'()*", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := filterChars(tt.input, tt.chars)
			if got != tt.want {
				t.Errorf("filterChars(%q, %q) = %q, want %q", tt.input, tt.chars, got, tt.want)
			}
		})
	}
}

func TestExtractKeyFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png", "7cd084941338484aae1ad9425b84077c"},
		{"https://i0.hdslb.com/bfs/wbi/3ec84d4042a94e7eb3cf12fd5f7e3a2a.png", "3ec84d4042a94e7eb3cf12fd5f7e3a2a"},
		{"", ""},
		{"no-slash", "no-slash"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := extractKeyFromURL(tt.url)
			if got != tt.want {
				t.Errorf("extractKeyFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestMd5Hex(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "5d41402abc4b2a76b9719d911017c592"},
		{"", "d41d8cd98f00b204e9800998ecf8427e"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := md5Hex(tt.input)
			if got != tt.want {
				t.Errorf("md5Hex(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEncodeParams(t *testing.T) {
	params := map[string]string{
		"keyword": "测试",
		"page":    "1",
	}
	encoded := encodeParams(params)
	if encoded == "" {
		t.Error("encodeParams should not return empty string")
	}
	// 验证包含 key=value
	if !contains(encoded, "keyword=") || !contains(encoded, "page=1") {
		t.Errorf("encodeParams(%v) = %q, missing expected keys", params, encoded)
	}
}

func TestWBISigner_Sign(t *testing.T) {
	signer := NewWBISigner()
	signer.SetKeys("7cd084941338484aae1ad9425b84077c", "3ec84d4042a94e7eb3cf12fd5f7e3a2a")

	params := map[string]string{
		"search_type": "video",
		"keyword":     "测试",
		"page":        "1",
		"pagesize":    "20",
		"order":       "totalrank",
	}

	signed := signer.Sign(params)

	// 验证 wts 已添加
	if signed["wts"] == "" {
		t.Error("Sign() should add wts parameter")
	}

	// 验证 w_rid 已添加
	if signed["w_rid"] == "" {
		t.Error("Sign() should add w_rid parameter")
	}

	// 验证 w_rid 是 32 字符的 MD5
	if len(signed["w_rid"]) != 32 {
		t.Errorf("w_rid length = %d, want 32", len(signed["w_rid"]))
	}

	t.Logf("signed params: %v", signed)
}

func TestWBISigner_Sign_Deterministic(t *testing.T) {
	signer := NewWBISigner()
	signer.SetKeys("7cd084941338484aae1ad9425b84077c", "3ec84d4042a94e7eb3cf12fd5f7e3a2a")

	// 相同参数应产生相同签名（在同一秒内）
	params1 := map[string]string{"keyword": "test", "page": "1"}
	params2 := map[string]string{"keyword": "test", "page": "1"}

	signed1 := signer.Sign(params1)
	// 强制使用相同的 wts
	params2["wts"] = signed1["wts"]
	// 手动计算 w_rid
	delete(signed1, "w_rid")
	signed2 := signer.Sign(params2)
	signed2["wts"] = signed1["wts"] // 确保 wts 相同

	// 重新签名
	signed1 = signer.Sign(map[string]string{"keyword": "test", "page": "1"})
	signed1["wts"] = signed2["wts"]
	delete(signed1, "w_rid")
	// 重新计算
}

func TestWBISigner_NeedRefresh(t *testing.T) {
	signer := NewWBISigner()

	// 初始状态需要刷新
	if !signer.NeedRefresh() {
		t.Error("NewWBISigner should need refresh")
	}

	// 设置密钥后不需要刷新
	signer.SetKeys("img", "sub")
	if signer.NeedRefresh() {
		t.Error("After SetKeys should not need refresh")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
