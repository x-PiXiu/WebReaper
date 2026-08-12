package ai

import (
	"math/rand"
	"time"
)

// ProbeQuestioner 生成多样化探测问法（采样矩阵的"问法维度"）。
//
// 背景：AgentProbe 曾用完全相同提示词采样 N 次 → 结果同质化（"5 次搜索词全一样"）。
// 本实现把 direct_probe.go 原有的 generateProbeQuestions 提取为共享能力：
//   - AgentProbe / DirectProbe 共用（策略复用，删重复实现）
//   - 随机打乱问法顺序（同一关键词多次监测不固定，降低缓存/顺序偏差）
//   - localContext 非空时插入位置型问法（"望京附近有什么川菜馆"——本地生活场景）
type ProbeQuestioner struct {
	rand *rand.Rand
}

// NewProbeQuestioner 创建问法生成器（随机种子按时间，避免固定顺序）。
func NewProbeQuestioner() *ProbeQuestioner {
	return &ProbeQuestioner{rand: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// Questions 生成 count 个多样化问法。
// 基础问法：原词 / 推荐一些X / X哪家好 / 有了解过X吗 / 关于X推荐。
// localContext 非空时前置位置型问法（本地生活 P0 补全）。
// 注意：关键词若已含位置（"北京装修公司哪家好"），本地问法拼接会冗余——
// 沿用原启发式：本地问法仍会生成，由 LLM 自然忽略冗余（保持实现简单）。
func (q *ProbeQuestioner) Questions(keyword string, count int, localContext string) []string {
	base := []string{
		keyword,
		"推荐一些" + keyword,
		keyword + "哪家好",
		"有了解过" + keyword + "吗",
		"关于" + keyword + "，你有什么推荐的",
	}
	if localContext != "" {
		local := []string{
			localContext + "附近有什么" + keyword,
			keyword + "，" + localContext + "哪家好",
			localContext + "有什么值得推荐的" + keyword,
		}
		base = append(local, base...)
	}
	// 随机打乱（每次监测问法顺序不同——降低缓存命中与顺序偏差）
	q.rand.Shuffle(len(base), func(i, j int) { base[i], base[j] = base[j], base[i] })
	if count <= len(base) {
		return base[:count]
	}
	// 超过基础问法数时循环补齐（count 通常 ≤ 10，基础问法 5~8 个）
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, base[i%len(base)])
	}
	return out
}

// replaceForbiddenQuestions 采样层兜底（真实性红线第③道防线）：
// 把含品牌/竞品名的问法逐条替换为模板中性问法（模板只含关键词+位置，天然不点名）。
// 任何上游校验遗漏的点名问法都会诱导 AI 复述名字（回声提及），必须替换。
func replaceForbiddenQuestions(questions, forbidden []string, keyword string, count int, localCtx string, q *ProbeQuestioner) []string {
	if q == nil || len(questions) == 0 {
		return questions
	}
	out := make([]string, 0, len(questions))
	for _, qs := range questions {
		if len(sanitizeQuestions([]string{qs}, forbidden)) == 0 {
			// 点名问法 → 从模板池取第一个中性问法替换
			pool := q.Questions(keyword, count, localCtx)
			replaced := false
			for _, alt := range pool {
				if len(sanitizeQuestions([]string{alt}, forbidden)) > 0 {
					out = append(out, alt)
					replaced = true
					break
				}
			}
			if !replaced {
				out = append(out, qs) // 极端兜底：模板也被污染则保留（正常不会发生）
			}
		} else {
			out = append(out, qs)
		}
	}
	return out
}

// generateProbeQuestions 兼容薄包装（历史测试保留）——委托共享实现。
func generateProbeQuestions(keyword string, count int, localContext string) []string {
	return NewProbeQuestioner().Questions(keyword, count, localContext)
}
