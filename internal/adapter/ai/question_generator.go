package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// LLMQuestionGenerator 是 port.ProbeQuestionGenerator 的"LLM 生成问法"实现。
//
// 背景（采样矩阵·问法维度 v2）：模板问法池只有 8 种，采样数大时循环重复——
// 相同 prompt 命中 LLM 缓存会返回一致内容，监测退化为"同答案重复 N 次"。
// 本实现让 LLM 按品牌上下文（定位/卖点/竞品/门店地址）生成真实用户问法：
//   - 融合卖点："望京川菜馆的水煮鱼怎么样"
//   - 融合竞品对比："望京川菜馆和眉州东坡哪个好吃"（真实决策场景）
//   - 融合地址："望京SOHO附近的正宗川菜馆"
// 生成一次（MonitorUseCase 编排），多引擎分片复用（引擎间问法隔离 → 缓存互不命中）。
type LLMQuestionGenerator struct {
	aiGen      port.AIGenerator
	llmConfig  string // 生成用引擎（空=default；与直测引擎可分离）
}

// NewLLMQuestionGenerator 创建 LLM 问法生成器。
func NewLLMQuestionGenerator(ai port.AIGenerator) *LLMQuestionGenerator {
	return &LLMQuestionGenerator{aiGen: ai, llmConfig: ""}
}

// Generate 按品牌上下文结构化生成 count 个问法（失败返回错误——调用方降级模板）。
func (g *LLMQuestionGenerator) Generate(ctx context.Context, in port.QuestionGenInput) ([]string, error) {
	count := in.Count
	if count <= 0 {
		count = 6
	}

	selling := "无"
	if len(in.CoreSelling) > 0 {
		selling = strings.Join(in.CoreSelling, "、")
	}
	comps := "无"
	if len(in.Competitors) > 0 {
		comps = strings.Join(in.Competitors, "、")
	}
	location := in.LocalContext
	if location == "" {
		location = "（未提供门店位置，生成通用问法）"
	}

	systemPrompt := "你是 GEO 监测问法设计专家。模拟真实用户在各种场景下的搜索提问，生成自然、多样的问法。只输出 JSON。"
	userPrompt := fmt.Sprintf(`基于以下品牌信息，生成 %d 个用户在 AI 搜索引擎里可能问的搜索问法。

品牌：%s
定位：%s
核心卖点：%s
竞品：%s
门店位置：%s
监测关键词：%s

要求：
1. 问法必须是完整自然的用户提问（如"推荐几家正宗的川菜馆"），不要用关键词本身直接当问法
2. 覆盖场景：泛化推荐、卖点相关（菜品/服务）、竞品对比（"两家川菜馆怎么选"）、本地场景（含位置）、口碑评价
3. 与品类/卖点/位置强相关，不要生搬模板
4. 问法之间要有明显差异，避免同义重复
5. 【最重要·真实性红线】问法中禁止出现品牌名、门店名、竞品名本身——真实用户不会用"不知道的名字"去搜索。
   品牌信息只用于理解"我们是什么品类、有什么卖点"，问法必须用品类词+场景替代：
   ❌ 错误示例："蜀香居的水煮鱼好吃吗"（品牌名出现在问法里=诱导，AI 必然复述名字，监测失真）
   ✅ 正确示例："春熙路附近水煮鱼做得好的川菜馆"（品类+卖点+位置，不点名）
   ✅ 正确示例："本地川菜馆里招牌菜是水煮鱼的推荐几家"（卖点驱动，不点名）
   竞品对比也不得点名："两家口碑好的川菜馆怎么选"

只输出 JSON：
{"questions":[{"q":"问法1"},{"q":"问法2"}]}`,
		count, in.BrandName, in.Positioning, selling, comps, location, in.Keyword)

	messages := []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	convID := fmt.Sprintf("qgen-%d", time.Now().UnixNano())
	resp, err := g.aiGen.ChatStream(ctx, convID, g.llmConfig, messages, nil)
	if err != nil {
		return nil, fmt.Errorf("问法生成 LLM 调用失败: %w", err)
	}

	// 结构化解析：剥 think + 代码块 + 大括号兜底（复用全局抽取，含 JSON 校验）
	var parsed struct {
		Questions []struct {
			Q string `json:"q"`
		} `json:"questions"`
	}
	if jsonBlock := pkg.ExtractJSONBlock(resp); jsonBlock != "" {
		if err := json.Unmarshal([]byte(jsonBlock), &parsed); err == nil && len(parsed.Questions) > 0 {
			// 真实性红线（校验层）：剔除任何点名品牌/竞品的问法——LLM 提示词已禁止，
			// 此处程序化兜底（不信任 LLM 输出），不足数量由模板问法补齐
			var raw []string
			seen := make(map[string]bool)
			for _, item := range parsed.Questions {
				q := strings.TrimSpace(item.Q)
				if q == "" || seen[q] || len([]rune(q)) < 4 {
					continue
				}
				seen[q] = true
				raw = append(raw, q)
			}
			forbidden := append([]string{in.BrandName}, in.Competitors...)
			out := sanitizeQuestions(raw, forbidden)
			// LLM 常偷懒少生成（Count=8 只回 3-4 个）——池太小分片会重叠；
			// 或点名问法全被红线剔除——统一用模板问法补齐到 count
			// （混合池：LLM 真实问法优先 + 模板兜底；模板问法天然中性）
			if len(out) < count {
				need := count - len(out)
				if need < 0 {
					need = 0
				}
				pool := NewProbeQuestioner().Questions(in.Keyword, count, in.LocalContext)
				for _, q := range pool {
					if seen[q] || len([]rune(q)) < 4 {
						continue
					}
					seen[q] = true
					out = append(out, q)
					if len(out) >= count {
						break
					}
				}
			}
			if len(out) > 0 {
				return out, nil
			}
		}
	}
	return nil, fmt.Errorf("问法生成解析失败（响应无有效问法）")
}

// sanitizeQuestions 问法真实性红线（校验层）：剔除包含品牌名/门店名/竞品名的问法。
// 任何"点名"的问法都是诱导——AI 搜索引擎会复述问题中的名字，导致提及率失真
// （回声提及 echo mention）。双向包含匹配（忽略空白、大小写）；禁词过短(<2字)跳过
// 防误杀。剩余不足由调用方用模板问法补齐（模板问法天然中性：只含关键词+位置）。
func sanitizeQuestions(qs []string, forbidden []string) []string {
	var out []string
	for _, q := range qs {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		ql := strings.ToLower(q)
		bad := false
		for _, f := range forbidden {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			fl := strings.ToLower(f)
			if len([]rune(fl)) >= 2 && (strings.Contains(ql, fl) || strings.Contains(fl, ql)) {
				bad = true
				break
			}
		}
		if !bad {
			out = append(out, q)
		}
	}
	return out
}
