package publish

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"webreaper/internal/adapter/mock"
	zaplogger "webreaper/internal/adapter/logger"
	"webreaper/internal/domain/entity"
)

// newUCForTest 构造一个指向给定 server 的 PublishUseCase（带真实 logger + 指定重试次数）。
func newUCForTest(t *testing.T, serverURL string, maxRetries int) *PublishUseCase {
	t.Helper()
	extSysRepo := mock.NewMockExternalSystemRepository()
	_ = extSysRepo.Save(context.Background(), mustBuildEntity(serverURL))
	dataRepo := mock.NewMockDataItemRepository()
	_ = dataRepo.Save(context.Background(), mustBuildItem())
	uc := NewPublishUseCase(
		extSysRepo,
		mock.NewMockPublishRecordRepository(),
		dataRepo,
		zaplogger.MustNewZapLogger("test"),
	)
	uc.SetMaxRetries(maxRetries)
	// 测试用更短退避，避免等 1s+2s+4s
	uc.httpClient.Timeout = 2 * time.Second
	return uc
}

// mustBuildEntity 构造测试用的外部系统配置（指向 httptest server）。
func mustBuildEntity(serverURL string) entity.ExternalSystem {
	return entity.ExternalSystem{
		Name: "test-sink", Endpoint: serverURL,
		Method: "POST", Mode: entity.PublishModeRaw, Enabled: true,
	}
}

// mustBuildItem 构造测试用的 DataItem。
func mustBuildItem() entity.DataItem {
	return entity.DataItem{
		ID: "test-item", Title: "t", Content: `{"k":"v"}`,
		RawContent: `{"k":"v"}`, Status: entity.ItemStatusApproved,
	}
}

// TestDoHTTPWithRetry_500ThenSuccess 验证：前 2 次返回 500，第 3 次 200 → 最终成功。
// 这是流 C 的核心场景：外部系统偶发 5xx，重试后恢复。
func TestDoHTTPWithRetry_500ThenSuccess(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"ext-1"}`))
	}))
	defer ts.Close()

	uc := newUCForTest(t, ts.URL, 3)
	// 直接测 doHTTPWithRetry（绕过 Publish 的去重/映射，聚焦重试逻辑）
	extID, err := uc.doHTTPWithRetry(context.Background(), ts.URL, "POST", "", map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("期望重试后成功，得到 error: %v", err)
	}
	if extID != "ext-1" {
		t.Errorf("extID = %q, want ext-1", extID)
	}
	if calls := atomic.LoadInt32(&calls); calls != 3 {
		t.Errorf("总调用次数 = %d, want 3（2 次失败 + 1 次成功）", calls)
	}
}

// TestDoHTTPWithRetry_400NoRetry 验证：4xx 客户端错误不重试，立即失败。
func TestDoHTTPWithRetry_400NoRetry(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer ts.Close()

	uc := newUCForTest(t, ts.URL, 3)
	_, err := uc.doHTTPWithRetry(context.Background(), ts.URL, "POST", "", map[string]any{"k": "v"})
	if err == nil {
		t.Fatal("4xx 应立即失败，不应成功")
	}
	if calls := atomic.LoadInt32(&calls); calls != 1 {
		t.Errorf("4xx 不应重试，调用次数 = %d, want 1", calls)
	}
}

// TestDoHTTPWithRetry_429RetryAfter 验证：429 尊重 Retry-After 头。
// Retry-After: 0 → 立即重试（不等指数退避）。
func TestDoHTTPWithRetry_429RetryAfter(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"ext-2"}`))
	}))
	defer ts.Close()

	uc := newUCForTest(t, ts.URL, 2)
	extID, err := uc.doHTTPWithRetry(context.Background(), ts.URL, "POST", "", map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("429 重试后应成功: %v", err)
	}
	if extID != "ext-2" {
		t.Errorf("extID = %q, want ext-2", extID)
	}
}

// TestDoHTTPWithRetry_Exhausted 验证：重试用尽仍失败，返回最后错误。
func TestDoHTTPWithRetry_Exhausted(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway) // 502 可重试
	}))
	defer ts.Close()

	uc := newUCForTest(t, ts.URL, 2) // 2 次重试
	_, err := uc.doHTTPWithRetry(context.Background(), ts.URL, "POST", "", map[string]any{"k": "v"})
	if err == nil {
		t.Fatal("重试用尽应返回 error")
	}
	// maxRetries=2 → 首次 + 2 次重试 = 3 次调用
	if calls := atomic.LoadInt32(&calls); calls != 3 {
		t.Errorf("重试用尽，调用次数 = %d, want 3", calls)
	}
}

// TestDoHTTPWithRetry_SuccessFirstTry 验证：一次成功不重试。
func TestDoHTTPWithRetry_SuccessFirstTry(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"ext-3"}`))
	}))
	defer ts.Close()

	uc := newUCForTest(t, ts.URL, 3)
	extID, err := uc.doHTTPWithRetry(context.Background(), ts.URL, "POST", "", map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("不应失败: %v", err)
	}
	if extID != "ext-3" {
		t.Errorf("extID = %q, want ext-3", extID)
	}
	if calls := atomic.LoadInt32(&calls); calls != 1 {
		t.Errorf("成功不应重试，调用次数 = %d, want 1", calls)
	}
}

// TestIsRetryableError 验证重试策略分类（纯函数，覆盖各状态码）。
func TestIsRetryableError(t *testing.T) {
	cases := []struct {
		status int
		want   bool
	}{
		{0, true},   // 网络错误
		{500, true}, // 服务端错误
		{502, true}, // Bad Gateway
		{503, true}, // Service Unavailable
		{504, true}, // Gateway Timeout
		{429, true}, // 限流
		{400, false}, // Bad Request
		{401, false}, // Unauthorized
		{403, false}, // Forbidden
		{404, false}, // Not Found
		{422, false}, // Unprocessable Entity
		{200, false}, // 成功（不应走到这）
	}
	for _, c := range cases {
		if got := isRetryableError(c.status); got != c.want {
			t.Errorf("isRetryableError(%d) = %v, want %v", c.status, got, c.want)
		}
	}
}

// TestParseRetryAfter 验证 Retry-After 头解析。
func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		in    string
		want  int
		valid bool
	}{
		{"0", 0, true},
		{"5", 5, true},
		{"120", 120, true},
		{"", 0, true},   // 空串视为无延迟（valid，返回 0）
		{"abc", 0, false},   // 非数字
		{"12x", 0, false},   // 部分数字
	}
	for _, c := range cases {
		got, err := parseRetryAfter(c.in)
		if c.valid && err != nil {
			t.Errorf("parseRetryAfter(%q) 意外 error: %v", c.in, err)
		}
		if !c.valid && err == nil {
			t.Errorf("parseRetryAfter(%q) 应返回 error", c.in)
		}
		if c.valid && got != c.want {
			t.Errorf("parseRetryAfter(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
