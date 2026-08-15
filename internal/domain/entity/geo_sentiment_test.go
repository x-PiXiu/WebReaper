package entity

import "testing"

func TestNormalizeSentiment(t *testing.T) {
	cases := map[string]string{
		"positive": "positive",
		"negative": "negative",
		"neutral":  "neutral",
		"":         "neutral",  // 探测降级路径的空值
		"AWESOME":  "neutral",  // LLM 返回任意字符串不可透传落库
		"Positive": "neutral",  // 大小写敏感——非法即归一
		"很好":       "neutral",
	}
	for in, want := range cases {
		if got := NormalizeSentiment(in); got != want {
			t.Errorf("NormalizeSentiment(%q) = %q, want %q", in, got, want)
		}
	}
}
