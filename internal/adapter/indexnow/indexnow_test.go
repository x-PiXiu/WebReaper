package indexnow

import (
	"context"
	"strings"
	"testing"
)

// NewSubmitter 构造测试：从 baseURL 解析 host。
func TestNewSubmitter(t *testing.T) {
	s, err := NewSubmitter("https://content.example.com", "my-test-key-1", "https://content.example.com/public/indexnow-key.txt")
	if err != nil {
		t.Fatalf("NewSubmitter error: %v", err)
	}
	if s.host != "content.example.com" {
		t.Errorf("host = %s, want content.example.com", s.host)
	}
	if s.key != "my-test-key-1" {
		t.Errorf("key 未保存")
	}
	if !strings.Contains(s.keyLocation, "indexnow-key.txt") {
		t.Errorf("keyLocation 不正确: %s", s.keyLocation)
	}
}

// 非法 baseURL 应报错。
func TestNewSubmitter_InvalidBaseURL(t *testing.T) {
	if _, err := NewSubmitter("not-a-url", "abcdefgh", "loc"); err == nil {
		t.Error("非法 baseURL 应报错")
	}
}

// key 格式校验（IndexNow 文档：8-128 个 a-zA-Z0-9-）。
func TestNewSubmitter_KeyValidation(t *testing.T) {
	valid := []string{
		"abcdefgh",            // 8 位
		strings.Repeat("k", 128), // 128 位
		"ABC-def-123-xyz",     // 连字符
	}
	for _, k := range valid {
		if _, err := NewSubmitter("https://content.example.com", k, "loc"); err != nil {
			t.Errorf("合法 key %q 不应报错: %v", k, err)
		}
	}
	invalid := []string{
		"",         // 空
		"short",    // <8
		strings.Repeat("k", 129), // >128
		"key with space",
		"key_with_underscore",
		"密钥中文",
	}
	for _, k := range invalid {
		if _, err := NewSubmitter("https://content.example.com", k, "loc"); err == nil {
			t.Errorf("非法 key %q 应报错", k)
		}
	}
}

// SubmitURLs 空列表不调用网络。
func TestSubmitter_EmptyURLs(t *testing.T) {
	s, _ := NewSubmitter("https://content.example.com", "my-test-key", "loc")
	if err := s.SubmitURLs(context.Background(), nil); err != nil {
		t.Errorf("空列表不应报错: %v", err)
	}
	if err := s.SubmitURLs(context.Background(), []string{}); err != nil {
		t.Errorf("空列表不应报错: %v", err)
	}
}

// 请求体序列化正确（host/key/keyLocation/urlList 结构）。
func TestSubmitter_RequestShape(t *testing.T) {
	s, _ := NewSubmitter("https://content.example.com", "k-123-abcd", "https://content.example.com/public/indexnow-key.txt")
	req := indexNowRequest{
		Host:        s.host,
		Key:         s.key,
		KeyLocation: s.keyLocation,
		URLList:     []string{"https://content.example.com/public/articles/oc-1"},
	}
	out, err := jsonMarshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if !strings.Contains(out, `"host":"content.example.com"`) {
		t.Errorf("host 字段错误: %s", out)
	}
	if !strings.Contains(out, `"key":"k-123-abcd"`) {
		t.Errorf("key 字段错误: %s", out)
	}
	if !strings.Contains(out, `"urlList":["https://content.example.com/public/articles/oc-1"]`) {
		t.Errorf("urlList 字段错误: %s", out)
	}
}
