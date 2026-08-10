package indexnow

import (
	"context"
	"strings"
	"testing"
)

// NewSubmitter 构造测试：从 baseURL 解析 host。
func TestNewSubmitter(t *testing.T) {
	s, err := NewSubmitter("https://content.example.com", "my-key", "https://content.example.com/public/indexnow-key.txt")
	if err != nil {
		t.Fatalf("NewSubmitter error: %v", err)
	}
	if s.host != "content.example.com" {
		t.Errorf("host = %s, want content.example.com", s.host)
	}
	if s.key != "my-key" {
		t.Errorf("key 未保存")
	}
	if !strings.Contains(s.keyLocation, "indexnow-key.txt") {
		t.Errorf("keyLocation 不正确: %s", s.keyLocation)
	}
}

// 非法 baseURL 应报错。
func TestNewSubmitter_InvalidBaseURL(t *testing.T) {
	if _, err := NewSubmitter("not-a-url", "k", "loc"); err == nil {
		t.Error("非法 baseURL 应报错")
	}
}

// SubmitURLs 空列表不调用网络。
func TestSubmitter_EmptyURLs(t *testing.T) {
	s, _ := NewSubmitter("https://content.example.com", "k", "loc")
	if err := s.SubmitURLs(context.Background(), nil); err != nil {
		t.Errorf("空列表不应报错: %v", err)
	}
	if err := s.SubmitURLs(context.Background(), []string{}); err != nil {
		t.Errorf("空列表不应报错: %v", err)
	}
}

// 请求体序列化正确（host/key/keyLocation/urlList 结构）。
func TestSubmitter_RequestShape(t *testing.T) {
	s, _ := NewSubmitter("https://content.example.com", "k-123", "https://content.example.com/public/indexnow-key.txt")
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
	if !strings.Contains(out, `"key":"k-123"`) {
		t.Errorf("key 字段错误: %s", out)
	}
	if !strings.Contains(out, `"urlList":["https://content.example.com/public/articles/oc-1"]`) {
		t.Errorf("urlList 字段错误: %s", out)
	}
}
