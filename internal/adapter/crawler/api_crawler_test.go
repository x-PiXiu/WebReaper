package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPICrawler_Name(t *testing.T) {
	c := NewAPICrawler()
	if c.Name() != "api_crawler" {
		t.Errorf("Name = %q, want api_crawler", c.Name())
	}
}

func TestAPICrawler_Description(t *testing.T) {
	c := NewAPICrawler()
	if c.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestAPICrawler_Execute_Success(t *testing.T) {
	// 起一个本地 API 服务器返回测试 JSON
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "conv-123",
			"messages": []map[string]string{
				{"role": "user", "content": "什么是 goroutine"},
				{"role": "assistant", "content": "goroutine 是轻量级线程..."},
			},
		})
	}))
	defer ts.Close()

	c := NewAPICrawler()
	args, _ := json.Marshal(map[string]any{
		"url":     ts.URL,
		"headers": map[string]string{"Authorization": "Bearer test-token"},
	})

	result, err := c.Execute(context.Background(), string(args))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.SourceURL != ts.URL {
		t.Errorf("SourceURL = %q", result.SourceURL)
	}
	if result.Metadata["crawler_type"] != "api" {
		t.Errorf("CrawlerType = %q", result.Metadata["crawler_type"])
	}
	// RawContent 应含 messages
	if !contains(result.RawContent, "goroutine") {
		t.Errorf("RawContent missing data: %s", result.RawContent)
	}
}

func TestAPICrawler_Execute_MissingURL(t *testing.T) {
	c := NewAPICrawler()
	_, err := c.Execute(context.Background(), `{}`)
	if err == nil {
		t.Error("expected error for missing url")
	}
}

func TestAPICrawler_Execute_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	c := NewAPICrawler()
	args := `{"url":"` + ts.URL + `"}`
	_, err := c.Execute(context.Background(), args)
	if err == nil {
		t.Error("expected error for 401")
	}
}

func TestAPICrawler_Execute_DefaultMethod(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c := NewAPICrawler()
	args := `{"url":"` + ts.URL + `"}`
	result, err := c.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !contains(result.RawContent, "ok") {
		t.Errorf("RawContent = %s", result.RawContent)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
