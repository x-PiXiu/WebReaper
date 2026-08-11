package ai

import (
	"strings"
	"testing"
)

// 本地监测问法（本地生活 P0 补全）测试。

func TestGenerateProbeQuestions_LocalContext(t *testing.T) {
	qs := generateProbeQuestions("川菜馆", 5, "朝阳区望京")
	if len(qs) != 5 {
		t.Fatalf("问法数 = %d, want 5", len(qs))
	}
	// 本地问法优先（前 3 条含位置）
	if !strings.Contains(qs[0], "朝阳区望京") {
		t.Errorf("首位应为本地问法: %s", qs[0])
	}
	if !strings.Contains(qs[1], "朝阳区望京") || !strings.Contains(qs[1], "川菜馆") {
		t.Errorf("第二位应为本地问法: %s", qs[1])
	}
	// 全部问法含关键词
	for _, q := range qs {
		if !strings.Contains(q, "川菜馆") {
			t.Errorf("问法缺少关键词: %s", q)
		}
	}
}

func TestGenerateProbeQuestions_NoLocalContext(t *testing.T) {
	// 无位置时行为与改造前一致（首位=原词）
	qs := generateProbeQuestions("装修公司", 5, "")
	if qs[0] != "装修公司" {
		t.Errorf("无位置时首位应为原词: %s", qs[0])
	}
	if strings.Contains(qs[0], "附近") {
		t.Errorf("无位置时不应有本地问法: %s", qs[0])
	}
}

func TestGenerateProbeQuestions_CountOverflow(t *testing.T) {
	// 采样数超过问法池：轮换使用（不越界）
	qs := generateProbeQuestions("川菜馆", 20, "朝阳区望京")
	if len(qs) != 8 { // 3 本地 + 5 基础
		t.Errorf("问法池应 8 条: %d", len(qs))
	}
}

func TestBuildSearchContext_Local(t *testing.T) {
	p := &AgentProbe{}
	base := p.buildSearchContext("川菜馆", "")
	if strings.Contains(base, "附近") {
		t.Errorf("无位置时不应有本地限定: %s", base)
	}
	local := p.buildSearchContext("川菜馆", "朝阳区望京")
	if !strings.Contains(local, "朝阳区望京附近有什么川菜馆") {
		t.Errorf("本地搜索任务构造错误: %s", local)
	}
}
