package urlsubmit

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// ---- BaiduSubmitter 构造校验 ----

func TestNewBaiduSubmitter_Validation(t *testing.T) {
	if _, err := NewBaiduSubmitter("", "tok"); err == nil {
		t.Error("空 site 应报错")
	}
	if _, err := NewBaiduSubmitter("content.example.com", ""); err == nil {
		t.Error("空 token 应报错")
	}
	s, err := NewBaiduSubmitter("content.example.com", "tok-123")
	if err != nil {
		t.Fatalf("合法配置应成功: %v", err)
	}
	if s.site != "content.example.com" || s.token != "tok-123" {
		t.Errorf("site/token 未保存")
	}
}

// ---- chunkURLs 分片逻辑 ----

func TestChunkURLs(t *testing.T) {
	urls := []string{"a", "b", "c", "d", "e"}
	chunks := chunkURLs(urls, 2)
	if len(chunks) != 3 {
		t.Fatalf("5 条按 2 分片应 3 片，实际 %d", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[2]) != 1 {
		t.Errorf("分片大小错误: %v", chunks)
	}

	// 恰好整除
	if got := chunkURLs([]string{"a", "b", "c", "d"}, 2); len(got) != 2 {
		t.Errorf("4 条按 2 分片应 2 片，实际 %d", len(got))
	}
	// 空列表
	if got := chunkURLs(nil, 2000); len(got) != 0 {
		t.Errorf("空列表应 0 片，实际 %d", len(got))
	}
	// 单条不超过上限
	if got := chunkURLs([]string{"a"}, 2000); len(got) != 1 {
		t.Errorf("单条应 1 片，实际 %d", len(got))
	}
}

// ---- MultiSubmitter 组合行为 ----

// mockSubmitter 可控的测试提交器。
type mockSubmitter struct {
	name    string
	fail    bool
	calls   int
	urls    []string
}

func (m *mockSubmitter) SubmitURLs(_ context.Context, urls []string) error {
	m.calls++
	m.urls = append(m.urls, urls...)
	if m.fail {
		return errors.New(m.name + " 失败")
	}
	return nil
}

func TestMultiSubmitter_AllSuccess(t *testing.T) {
	a := &mockSubmitter{name: "a"}
	b := &mockSubmitter{name: "b"}
	m := NewMultiSubmitter(a, b)
	urls := []string{"https://x/public/articles/1"}

	if err := m.SubmitURLs(context.Background(), urls); err != nil {
		t.Fatalf("全部成功不应报错: %v", err)
	}
	if a.calls != 1 || b.calls != 1 {
		t.Errorf("两个渠道都应被调用: a=%d b=%d", a.calls, b.calls)
	}
	if len(a.urls) != 1 || a.urls[0] != urls[0] {
		t.Errorf("URL 传递错误: %v", a.urls)
	}
}

func TestMultiSubmitter_PartialFailure(t *testing.T) {
	a := &mockSubmitter{name: "a"}          // 成功
	b := &mockSubmitter{name: "b", fail: true} // 失败
	m := NewMultiSubmitter(a, b)

	err := m.SubmitURLs(context.Background(), []string{"https://x/public/articles/1"})
	if err == nil {
		t.Fatal("部分失败应返回错误")
	}
	if !strings.Contains(err.Error(), "b 失败") {
		t.Errorf("错误应包含失败渠道信息: %v", err)
	}
	// 失败不影响其他渠道执行
	if a.calls != 1 {
		t.Errorf("成功渠道仍应被执行: a.calls=%d", a.calls)
	}
}

func TestMultiSubmitter_Empty(t *testing.T) {
	// 无渠道：直接成功
	if err := NewMultiSubmitter().SubmitURLs(context.Background(), []string{"x"}); err != nil {
		t.Errorf("无渠道应成功: %v", err)
	}
	// 空 URL：不调用任何渠道
	a := &mockSubmitter{name: "a"}
	if err := NewMultiSubmitter(a).SubmitURLs(context.Background(), nil); err != nil {
		t.Errorf("空 URL 应成功: %v", err)
	}
	if a.calls != 0 {
		t.Errorf("空 URL 不应调用渠道: %d", a.calls)
	}
}
