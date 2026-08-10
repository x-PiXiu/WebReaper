package bing

import (
	"context"
	"testing"
)

func TestCheckURLsEmptyConfig(t *testing.T) {
	// 空配置（未配 BING_API_KEY / BING_SITE_URL）：任务空转，不报错
	empty := NewChecker("", "")
	got, err := empty.CheckURLs(context.Background(), []string{"https://a.com/x"})
	if err != nil {
		t.Fatalf("空配置不应报错: %v", err)
	}
	if got != nil {
		t.Fatalf("空配置应返回 nil（空转）: %v", got)
	}
}

// TestParseResponse 直接测响应解析语义（null/Error/对象）。
func TestParseResponse(t *testing.T) {
	c := NewChecker("k", "s")
	cases := []struct {
		body string
		want string
	}{
		{`null`, "pending"},                    // 已提交未收录
		{`"Error"`, "error"},                   // 查询失败
		{`{"d":"ok"}`, "indexed"},              // 返回对象 = 已收录
		{`{"d":{"UrlSubmissionStatus":{}}}`, "indexed"},
		{`not-json`, "error"},                  // 解析失败按 error 处理
	}
	for _, tc := range cases {
		got := c.parseBody([]byte(tc.body))
		if got != tc.want {
			t.Errorf("body=%q 期望 %q 得到 %q", tc.body, tc.want, got)
		}
	}
}
