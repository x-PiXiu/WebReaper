package urlsubmit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---- BingSubmitter 构造校验 ----

func TestNewBingSubmitter_Validation(t *testing.T) {
	if _, err := NewBingSubmitter("", "https://x.com"); err == nil {
		t.Error("空 apiKey 应报错")
	}
	if _, err := NewBingSubmitter("key-123", ""); err == nil {
		t.Error("空 site 应报错")
	}
	s, err := NewBingSubmitter("key-123", "https://x.com")
	if err != nil {
		t.Fatalf("合法配置应成功: %v", err)
	}
	if s.apiKey != "key-123" || s.site != "https://x.com" {
		t.Errorf("apiKey/site 未保存")
	}
}

// ---- 提交行为（httptest 模拟 Bing 端点）----

// TestBingSubmit_OK 200 = 已受理：请求体/URL 正确，提交成功。
func TestBingSubmit_OK(t *testing.T) {
	var gotURL string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	old := bingSubmitEndpoint
	bingSubmitEndpoint = srv.URL // 测试注入端点（包内常量可替换）
	defer func() { bingSubmitEndpoint = old }()

	s, _ := NewBingSubmitter("key-123", "https://x.com")
	urls := []string{"https://x.com/public/articles/1", "https://x.com/public/articles/2"}
	if err := s.SubmitURLs(context.Background(), urls); err != nil {
		t.Fatalf("200 应成功: %v", err)
	}

	if !strings.Contains(gotURL, "apikey=key-123") {
		t.Errorf("请求应带 apikey: %s", gotURL)
	}
	var req bingSubmitRequest
	if err := json.Unmarshal(gotBody, &req); err != nil {
		t.Fatalf("请求体应可解析为 JSON: %v", err)
	}
	if req.SiteURL != "https://x.com" || len(req.URLList) != 2 || req.URLList[1] != urls[1] {
		t.Errorf("请求体错误: %+v", req)
	}
}

// TestBingSubmit_Non200 非 200（含 429 配额用尽）应报错。
func TestBingSubmit_Non200(t *testing.T) {
	for _, code := range []int{429, 500} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		old := bingSubmitEndpoint
		bingSubmitEndpoint = srv.URL
		s, _ := NewBingSubmitter("key-123", "https://x.com")
		err := s.SubmitURLs(context.Background(), []string{"https://x.com/public/articles/1"})
		srv.Close()
		bingSubmitEndpoint = old
		if err == nil {
			t.Errorf("HTTP %d 应报错", code)
		}
	}
}

// TestBingSubmit_Chunking 超过 100 条自动分片（每片独立请求）。
func TestBingSubmit_Chunking(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	old := bingSubmitEndpoint
	bingSubmitEndpoint = srv.URL
	defer func() { bingSubmitEndpoint = old }()

	s, _ := NewBingSubmitter("key-123", "https://x.com")
	urls := make([]string, 250)
	for i := range urls {
		urls[i] = "https://x.com/public/articles/x"
	}
	if err := s.SubmitURLs(context.Background(), urls); err != nil {
		t.Fatalf("分片提交应成功: %v", err)
	}
	if calls != 3 {
		t.Errorf("250 条按 100 分片应 3 次请求，实际 %d", calls)
	}
}

// TestBingSubmit_Empty 空 URL 列表直接成功（不发起请求）。
func TestBingSubmit_Empty(t *testing.T) {
	s, _ := NewBingSubmitter("key-123", "https://x.com")
	if err := s.SubmitURLs(context.Background(), nil); err != nil {
		t.Errorf("空列表应成功: %v", err)
	}
}
