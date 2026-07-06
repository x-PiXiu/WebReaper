package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestStaticCrawler_Execute_404 验证：HTTP 4xx 不应把错误页正文当正常内容返回。
// 原缺陷：static_crawler 无 OnError/OnResponse 状态码检查，
// 404 页面若有 <body> 仍会返回 DataItem（错误页正文被当正常采集结果）。
func TestStaticCrawler_Execute_404(t *testing.T) {
	// 起一个返回 404 的本地服务器（带错误页 body）
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`<html><body><h1>404 Not Found</h1></body></html>`))
	}))
	defer ts.Close()

	c := NewStaticCrawler()
	args, _ := json.Marshal(map[string]any{"url": ts.URL})

	result, err := c.Execute(context.Background(), string(args))
	if err == nil {
		t.Fatalf("期望 404 返回 error，但得到 DataItem（Content=%q）——错误页被当正常内容", result.Content)
	}
	// 错误信息应含状态码
	if !contains(err.Error(), "404") {
		t.Errorf("错误信息应含状态码 404，得到: %v", err)
	}
}

// TestStaticCrawler_Execute_500 验证：HTTP 5xx 同样应返回错误。
func TestStaticCrawler_Execute_500(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`<html><body><h1>500 Internal Server Error</h1></body></html>`))
	}))
	defer ts.Close()

	c := NewStaticCrawler()
	args, _ := json.Marshal(map[string]any{"url": ts.URL})

	_, err := c.Execute(context.Background(), string(args))
	if err == nil {
		t.Fatal("期望 500 返回 error")
	}
}

// TestStaticCrawler_Execute_Success 验证：2xx 正常页仍能采集到正文。
func TestStaticCrawler_Execute_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><body><h1>Go 语言教程</h1><p>goroutine 是轻量级线程</p></body></html>`))
	}))
	defer ts.Close()

	c := NewStaticCrawler()
	args, _ := json.Marshal(map[string]any{"url": ts.URL})

	result, err := c.Execute(context.Background(), string(args))
	if err != nil {
		t.Fatalf("正常页应采集成功，得到 error: %v", err)
	}
	if !contains(result.Content, "goroutine") {
		t.Errorf("Content 应含正文，得到: %q", result.Content)
	}
	if result.Metadata["crawler_type"] != "static" {
		t.Errorf("crawler_type 应为 static，得到 %q", result.Metadata["crawler_type"])
	}
}

// TestSearchCrawler_Execute_NonOKStatus 无法用 httptest 直接验证：
// search_crawler 的 searchURL 硬编码指向 DuckDuckGo，无法重定向到本地服务器。
// 该路径的 status 守卫行为由 TestCrawlErr + 代码审查共同覆盖（crawlErr + status 检查组合）。

// TestCrawlErr 验证统一错误构造器对不同场景的输出。
func TestCrawlErr(t *testing.T) {
	// 纯网络错误（status=0）
	err := crawlErr("http://x", 0, http.ErrAbortHandler)
	if err == nil {
		t.Error("网络错误应返回 error")
	}

	// HTTP 状态码错误
	err = crawlErr("http://x", 404, nil)
	if err == nil || !contains(err.Error(), "404") {
		t.Errorf("状态码错误应含 404，得到: %v", err)
	}

	// 带 cause 的状态码错误
	err = crawlErr("http://x", 502, http.ErrServerClosed)
	if err == nil || !contains(err.Error(), "502") {
		t.Errorf("应含 502，得到: %v", err)
	}
}
