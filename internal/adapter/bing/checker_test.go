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

// TestParseResponse 直接测响应解析语义——样本全部来自 2026-08-14 真实 API 实测。
func TestParseResponse(t *testing.T) {
	c := NewChecker("k", "s")
	cases := []struct {
		body string
		want string
	}{
		// 已抓取：LastCrawledDate 有效（真实样本：oc-1786626290004634303）
		{`{"d":{"__type":"UrlInfo:#Microsoft.Bing.Webmaster.Api","DiscoveryDate":"/Date(1786604400000-0700)/","LastCrawledDate":"/Date(1786626352000)/","HttpStatus":0,"IsPage":true,"Url":"https://geo.zhichen.chat/public/articles/oc-1786626290004634303"}}`, "indexed"},
		// 从未抓取：LastCrawledDate 为 .NET MinValue（真实样本：不存在的 URL）
		{`{"d":{"__type":"UrlInfo:#Microsoft.Bing.Webmaster.Api","DiscoveryDate":"/Date(-62135568000000-0800)/","LastCrawledDate":"/Date(-62135568000000-0800)/","HttpStatus":0,"IsPage":true}}`, "pending"},
		// API 错误：密钥无效（真实样本：{"ErrorCode":3,...}——注意不是字符串 "Error"）
		{`{"ErrorCode":3,"Message":"ERROR!!! InvalidApiKey"}`, "error"},
		// d 为 null：Bing 从未发现该 URL
		{`{"d":null}`, "pending"},
		// 空对象（无 d 无 ErrorCode）按 pending 处理
		{`{}`, "pending"},
		// 非 JSON（旧端点返回的 HTML 错误页）→ error
		{`<html>Endpoint not found.</html>`, "error"},
	}
	for _, tc := range cases {
		got := c.parseBody([]byte(tc.body))
		if got != tc.want {
			t.Errorf("body=%s 期望 %q 得到 %q", tc.body, tc.want, got)
		}
	}
}

// TestDotnetMillis .NET 日期解析边界：时区后缀可选、MinValue/非法值归零。
func TestDotnetMillis(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{`/Date(1786626352000)/`, 1786626352000},                 // 无时区后缀（合法样本）
		{`/Date(1786604400000-0700)/`, 1786604400000},            // 带时区后缀
		{`/Date(-62135568000000-0800)/`, 0},                      // MinValue = 从未抓取
		{`/Date(0)/`, 0},                                         // 零值
		{``, 0},                                                  // 空
		{`1786626352000`, 0},                                     // 非 .NET 格式
	}
	for _, tc := range cases {
		if got := dotnetMillis(tc.in); got != tc.want {
			t.Errorf("dotnetMillis(%q) = %d, 期望 %d", tc.in, got, tc.want)
		}
	}
}
