package ai

import (
	"strings"
	"testing"
)

// 本地监测问法（本地生活 P0 补全）+ 采样矩阵问法维度测试。
//
// 注意（采样矩阵 P0-1 重构后语义）：
//   - 问法池 = 3 本地 + 5 基础 = 8 条，生成时随机打乱（无固定顺序）
//   - count 超过池大小 → 循环补齐到 count（不再截断）
//   - 原 generateProbeQuestions 保留为兼容薄包装（委托共享 ProbeQuestioner）

func TestGenerateProbeQuestions_LocalContext(t *testing.T) {
	qs := generateProbeQuestions("川菜馆", 5, "朝阳区望京")
	if len(qs) != 5 {
		t.Fatalf("问法数 = %d, want 5", len(qs))
	}
	// 随机打乱后不再保证本地问法在前——只要池子里存在本地问法即可
	localCount := 0
	for _, q := range qs {
		if strings.Contains(q, "朝阳区望京") {
			localCount++
		}
		// 全部问法含关键词
		if !strings.Contains(q, "川菜馆") {
			t.Errorf("问法缺少关键词: %s", q)
		}
	}
	if localCount == 0 {
		t.Error("有本地上下文时应包含本地问法")
	}
}

func TestGenerateProbeQuestions_NoLocalContext(t *testing.T) {
	qs := generateProbeQuestions("装修公司", 5, "")
	if len(qs) != 5 {
		t.Fatalf("问法数 = %d, want 5", len(qs))
	}
	for _, q := range qs {
		if strings.Contains(q, "附近") {
			t.Errorf("无位置时不应有本地问法: %s", q)
		}
		if !strings.Contains(q, "装修公司") {
			t.Errorf("问法缺少关键词: %s", q)
		}
	}
}

func TestGenerateProbeQuestions_CountOverflow(t *testing.T) {
	// 采样数超过问法池（8）：循环补齐到 count（不越界、不截断）
	qs := generateProbeQuestions("川菜馆", 20, "朝阳区望京")
	if len(qs) != 20 {
		t.Errorf("应循环补齐到 20 条: %d", len(qs))
	}
	for _, q := range qs {
		if !strings.Contains(q, "川菜馆") {
			t.Errorf("问法缺少关键词: %s", q)
		}
	}
}

func TestBuildSearchTask(t *testing.T) {
	p := &AgentProbe{}
	base := p.buildSearchTask("川菜馆")
	if !strings.Contains(base, "用户问：川菜馆") {
		t.Errorf("搜索任务应包含问法: %s", base)
	}
	if !strings.Contains(base, "搜索") {
		t.Errorf("搜索任务应引导联网搜索: %s", base)
	}
	local := p.buildSearchTask("朝阳区望京附近有什么川菜馆")
	if !strings.Contains(local, "朝阳区望京附近有什么川菜馆") {
		t.Errorf("本地问法包装错误: %s", local)
	}
}
