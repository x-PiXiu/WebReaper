package generation

import (
	"context"
	"testing"

	"webreaper/internal/domain/entity"
)

func TestClassifyError(t *testing.T) {
	cases := []struct {
		code string
		want RetryClass
	}{
		{"TooManyRequests", RetryAuto},
		{"SystemThrottling", RetryAuto},
		{"InternalServiceFailure", RetryAuto},
		{"CreditInsufficient", RetryManual},
		{"TaskPromptPolicyViolation", RetryManual},
		{"ImageCheckFaceFailed", RetryTerminal},
		{"VideoFormatInvalid", RetryTerminal},
	}
	for _, c := range cases {
		if got := ClassifyError(c.code); got != c.want {
			t.Errorf("ClassifyError(%q) = %v, want %v", c.code, got, c.want)
		}
	}
}

func TestCanAutoRetry(t *testing.T) {
	if !CanAutoRetry("TooManyRequests", 0) {
		t.Error("限流首次应可自动重试")
	}
	if CanAutoRetry("TooManyRequests", 3) {
		t.Error("超过 3 次不应再自动重试")
	}
	if CanAutoRetry("CreditInsufficient", 0) {
		t.Error("积分不足不应自动重试")
	}
}

func TestParamsHash(t *testing.T) {
	p1 := entity.GenerationParams{"prompt": "测试", "duration": 5, "images": []string{"a", "b"}}
	p2 := entity.GenerationParams{"images": []string{"a", "b"}, "duration": 5, "prompt": "测试"}
	h1 := paramsHash("text2video", "viduq3-pro", p1)
	h2 := paramsHash("text2video", "viduq3-pro", p2)
	if h1 != h2 {
		t.Errorf("参数顺序不同哈希应一致: %s vs %s", h1, h2)
	}
	h3 := paramsHash("text2video", "viduq2", p1)
	if h1 == h3 {
		t.Error("模型不同哈希应不同")
	}
}

func TestIsTerminal(t *testing.T) {
	if !entity.IsTerminal(entity.TaskStateSuccess) || !entity.IsTerminal(entity.TaskStateFailed) {
		t.Error("success/failed 应为终态")
	}
	if entity.IsTerminal(entity.TaskStateProcessing) {
		t.Error("processing 不应为终态")
	}
}

var _ = context.Background
