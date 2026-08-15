package cache

import (
	"context"
	"testing"
	"time"
)

// 三防纯函数测试（Redis 交互依赖真连接，不做集成——核心保证逻辑在此验证）。

// 防雪崩：JitterTTL 结果 ∈ [base, base*1.25)，且多次调用有分散性。
func TestJitterTLLOnlyAddsBoundedRandom(t *testing.T) {
	base := 60 * time.Second
	for i := 0; i < 200; i++ {
		got := JitterTTL(base)
		if got < base || got >= base+base/4+1 {
			t.Fatalf("JitterTTL(%v) = %v，应在 [%v, %v) 内", base, got, base, base+base/4)
		}
	}
	if JitterTTL(0) != 0 || JitterTTL(-time.Second) != -time.Second {
		t.Error("非正基准 TTL 应原样返回")
	}
}

// 防雪崩分散性：100 次采样至少出现多种不同 TTL（不是常数偏移）。
func TestJitterTLLOrStaysVaried(t *testing.T) {
	base := time.Second
	seen := map[time.Duration]bool{}
	for i := 0; i < 100; i++ {
		seen[JitterTTL(base)] = true
	}
	if len(seen) < 10 {
		t.Errorf("抖动应产生分散 TTL，仅出现 %d 种", len(seen))
	}
}

func TestRandomTokenUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		tok := RandomToken()
		if len(tok) != 32 {
			t.Fatalf("token 长度 = %d, want 32（16字节 hex）", len(tok))
		}
		if seen[tok] {
			t.Fatal("token 重复")
		}
		seen[tok] = true
	}
}

// 防穿透/防重放的内存实现（nonce 语义与 usecase 包内实现一致——此处验证 adapter 版本）。
func TestMemoryNonceStore(t *testing.T) {
	s := NewMemoryNonceStore()
	ctx := context.Background()
	if !s.Seen(ctx, "n1") {
		t.Error("首次 nonce 应放行")
	}
	if s.Seen(ctx, "n1") {
		t.Error("重复 nonce 应拒绝")
	}
	if !s.Seen(ctx, "n2") {
		t.Error("不同 nonce 互不影响")
	}
}
