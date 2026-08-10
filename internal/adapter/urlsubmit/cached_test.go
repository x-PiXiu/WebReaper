package urlsubmit

import (
	"context"
	"testing"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/usecase/port"
)

// 验证：配置变更后 TTL 过期自动重建（新渠道生效），TTL 内复用旧实例。
func TestCachedProvider_TTLRebuild(t *testing.T) {
	ctx := context.Background()
	buildCount := 0
	p := NewCachedProvider(func(context.Context) (entity.IndexingConfig, error) {
		return entity.IndexingConfig{IndexNowKey: "indexnow-key-1"}, nil
	}, "https://content.example.com")
	// 注入 mock 构建：不碰外网，记录重建次数
	p.buildFn = func(cfg entity.IndexingConfig) (port.URLSubmitter, error) {
		buildCount++
		return &mockSubmitter{name: "mock-" + cfg.IndexNowKey}, nil
	}
	p.ttl = 50 * time.Millisecond // 测试缩短 TTL

	// 首次提交：构建（buildCount=1）
	if err := p.SubmitURLs(ctx, []string{"https://content.example.com/public/articles/1"}); err != nil {
		t.Fatalf("首次提交失败: %v", err)
	}
	if buildCount != 1 || p.cached == nil {
		t.Fatalf("首次应构建一次: buildCount=%d", buildCount)
	}
	first := p.cached

	// TTL 内提交：复用（buildCount 不变）
	if err := p.SubmitURLs(ctx, []string{"https://content.example.com/public/articles/2"}); err != nil {
		t.Fatalf("TTL 内提交失败: %v", err)
	}
	if buildCount != 1 || p.cached != first {
		t.Errorf("TTL 内不应重建: buildCount=%d", buildCount)
	}

	// TTL 过期：重建
	time.Sleep(80 * time.Millisecond)
	if err := p.SubmitURLs(ctx, []string{"https://content.example.com/public/articles/3"}); err != nil {
		t.Fatalf("过期后提交失败: %v", err)
	}
	if buildCount != 2 || p.cached == first {
		t.Errorf("TTL 过期后应重建: buildCount=%d", buildCount)
	}
}

// 验证：无渠道配置 → no-op（空转成功）。
func TestCachedProvider_NoChannels(t *testing.T) {
	ctx := context.Background()
	p := NewCachedProvider(func(context.Context) (entity.IndexingConfig, error) {
		return entity.IndexingConfig{}, nil
	}, "https://content.example.com")

	if err := p.SubmitURLs(ctx, []string{"https://x/a"}); err != nil {
		t.Errorf("无渠道应空转成功: %v", err)
	}
}

// 验证：百度 + IndexNow 双渠道构建（MultiSubmitter 内部）。
func TestCachedProvider_BothChannels(t *testing.T) {
	ctx := context.Background()
	p := NewCachedProvider(func(context.Context) (entity.IndexingConfig, error) {
		return entity.IndexingConfig{
			IndexNowKey: "indexnow-key-1",
			BaiduSite:   "content.example.com",
			BaiduToken:  "baidu-token",
		}, nil
	}, "https://content.example.com")

	// 不真正提交（会打外网）——只验证构建结果类型
	sub, err := p.current(ctx)
	if err != nil {
		t.Fatalf("构建失败: %v", err)
	}
	if _, ok := sub.(*MultiSubmitter); !ok {
		t.Errorf("双渠道应构建 MultiSubmitter，实际 %T", sub)
	}
}

// 验证：配置加载失败返回错误。
func TestCachedProvider_LoadError(t *testing.T) {
	ctx := context.Background()
	p := NewCachedProvider(func(context.Context) (entity.IndexingConfig, error) {
		return entity.IndexingConfig{}, errLoadFailed
	}, "https://content.example.com")
	if err := p.SubmitURLs(ctx, []string{"https://x/a"}); err == nil {
		t.Error("配置加载失败应报错")
	}
}

// errLoadFailed 测试用加载错误。
var errLoadFailed = &loadTestError{}

type loadTestError struct{}

func (*loadTestError) Error() string { return "load failed" }
