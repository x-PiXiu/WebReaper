package ai

import "testing"

// 竞品情感跨采样多数投票（A1：与自家情感的投票口径统一）。
func TestVoteCompetitorSentiments(t *testing.T) {
	votes := map[string]map[string]int{
		"甲": {"positive": 2, "negative": 1}, // 多数正面
		"乙": {"negative": 2, "positive": 1}, // 多数负面
		"丙": {"positive": 1, "negative": 1}, // 平票 → 不写入（按中性展示）
		"丁": {"neutral": 3},                 // 中性多数 → 不写入
	}
	got := voteCompetitorSentiments(votes)
	if got["甲"] != "positive" {
		t.Errorf("甲 = %q, want positive", got["甲"])
	}
	if got["乙"] != "negative" {
		t.Errorf("乙 = %q, want negative", got["乙"])
	}
	if _, ok := got["丙"]; ok {
		t.Errorf("平票不应写入, got %q", got["丙"])
	}
	if _, ok := got["丁"]; ok {
		t.Errorf("中性多数不应写入, got %q", got["丁"])
	}
}

func TestMajoritySentiment(t *testing.T) {
	cases := []struct {
		votes map[string]int
		want  string
	}{
		{map[string]int{"positive": 3, "negative": 1, "neutral": 1}, "positive"},
		{map[string]int{"negative": 2, "positive": 1}, "negative"},
		{map[string]int{"positive": 1, "negative": 1}, ""},
		{map[string]int{"neutral": 5}, ""},
		{map[string]int{}, ""},
	}
	for _, c := range cases {
		if got := majoritySentiment(c.votes); got != c.want {
			t.Errorf("majoritySentiment(%v) = %q, want %q", c.votes, got, c.want)
		}
	}
}

// 语义降级标记：LLM 返回非 JSON → fallbackStringMatch 兜底 → Degraded 必须为 true
//（此前情感/位次静默缺失，商户看到的"中性/未提及"可能只是降级产物）。
func TestMatchBrandFromListMarksDegradedOnBadJSON(t *testing.T) {
	ma := matchBrandFromList("品牌很好，但这不是 JSON：{{{", "品牌", nil, []string{"竞品"})
	if !ma.Degraded {
		t.Error("JSON 解析失败走兜底时应标记 Degraded")
	}
	if !ma.Mentioned {
		t.Error("兜底字符串匹配：回答含品牌名应判定提及")
	}
	if ma.Sentiment == "positive" || ma.Sentiment == "negative" {
		t.Errorf("兜底路径不应产出语义情感, got %q", ma.Sentiment)
	}
}

// 正常解析路径不应标记降级。
func TestMatchBrandFromListNotDegraded(t *testing.T) {
	resp := `{"brands":[{"name":"品牌","position":1,"sentiment":"positive"}],"sources":[]}`
	ma := matchBrandFromList(resp, "品牌", nil, nil)
	if ma.Degraded {
		t.Error("正常 LLM JSON 路径不应标记 Degraded")
	}
	if !ma.Mentioned || ma.Position != 1 {
		t.Errorf("正常解析失败: %+v", ma)
	}
}

func TestProbeConfidenceDegraded(t *testing.T) {
	if got := probeConfidenceDegraded(0.9, 0); got != 0.9 {
		t.Errorf("无降级不打折: %v", got)
	}
	if got := probeConfidenceDegraded(0.9, 2); got <= 0.6 || got >= 0.7 {
		t.Errorf("降级应打 7 折: %v", got)
	}
}
