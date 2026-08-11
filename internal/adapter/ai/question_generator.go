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
2. 覆盖场景：泛化推荐、卖点相关（菜品/服务）、竞品对比（"X和Y哪个好"）、本地场景（含位置）、口碑评价
3. 与品牌/卖点/竞品/位置强相关，不要生搬模板
4. 问法之间要有明显差异，避免同义重复

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
			var out []string
			seen := make(map[string]bool)
			for _, item := range parsed.Questions {
				q := strings.TrimSpace(item.Q)
				if q == "" || seen[q] || len([]rune(q)) < 4 {
					continue
				}
				seen[q] = true
				out = append(out, q)
			}
			// LLM 常偷懒少生成（Count=8 只回 3-4 个）——池太小分片会重叠：
			// 用模板问法补齐到 count（混合池：LLM 真实问法优先 + 模板兜底）
			if len(out) > 0 && len(out) < count {
				pool := NewProbeQuestioner().Questions(in.Keyword, count-len(out), in.LocalContext)
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
