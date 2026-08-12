package ai

import (
	"strings"
	"testing"
)

// 真实性红线测试：问法不得点名品牌/竞品（回声提及会导致监测失真）。

func TestSanitizeQuestionsRemovesBrandName(t *testing.T) {
	qs := []string{
		"蜀香居川菜馆的水煮鱼好吃吗", // 点名品牌 → 剔除
		"春熙路附近水煮鱼做得好的川菜馆", // 中性 → 保留
		"蜀香居和卢记正街饭店哪个好",   // 点名品牌+竞品 → 剔除
		"推荐几家正宗的川菜馆",       // 中性 → 保留
		"两家口碑好的川菜馆怎么选",     // 竞品对比但不点名 → 保留
	}
	out := sanitizeQuestions(qs, []string{"蜀香居川菜馆", "卢记正街饭店"})
	if len(out) != 3 {
		t.Fatalf("期望保留 3 个中性问法，实际 %d: %v", len(out), out)
	}
	for _, q := range out {
		if strings.Contains(q, "蜀香居") || strings.Contains(q, "卢记") {
			t.Fatalf("问法仍含品牌/竞品名: %q", q)
		}
	}
}

func TestSanitizeQuestionsPartialMatch(t *testing.T) {
	// 品牌名完整出现（LLM 生成的问法用输入全名）必须剔除；通用品类词问法保留
	qs := []string{"蜀香居川菜馆怎么样", "川菜馆推荐", "春熙路附近有什么川菜馆"}
	out := sanitizeQuestions(qs, []string{"蜀香居川菜馆"})
	if len(out) != 2 {
		t.Fatalf("期望保留 2 个，实际 %d: %v", len(out), out)
	}
	if out[0] != "川菜馆推荐" || out[1] != "春熙路附近有什么川菜馆" {
		t.Fatalf("保留的问法不对: %v", out)
	}
}

func TestReplaceForbiddenQuestionsKeepsLength(t *testing.T) {
	q := NewProbeQuestioner()
	qs := []string{
		"蜀香居川菜馆的水煮鱼好吃吗",
		"推荐几家正宗的川菜馆",
		"蜀香居川菜馆的招牌菜是什么",
	}
	out := replaceForbiddenQuestions(qs, []string{"蜀香居川菜馆"}, "成都川菜馆推荐", 5, "春熙路", q)
	if len(out) != len(qs) {
		t.Fatalf("替换后数量必须与原来一致：%d != %d", len(out), len(qs))
	}
	for _, qs2 := range out {
		if strings.Contains(qs2, "蜀香居") {
			t.Fatalf("替换后仍含品牌名: %q", qs2)
		}
	}
}

func TestReplaceForbiddenQuestionsCleanPoolUntouched(t *testing.T) {
	q := NewProbeQuestioner()
	qs := []string{"推荐几家正宗的川菜馆", "春熙路附近有什么好吃的"}
	out := replaceForbiddenQuestions(qs, []string{"蜀香居川菜馆"}, "成都川菜馆推荐", 5, "春熙路", q)
	if len(out) != 2 || out[0] != qs[0] || out[1] != qs[1] {
		t.Fatalf("中性问法池不应被改动: %v", out)
	}
}
