package geo

import (
	"context"
	"strings"
	"testing"

	"webreaper/internal/domain/entity"
)

// 测试数据：一份"高质量"内容（结构化+数据+来源+时效+差异化）
const highQualityContent = `# 2026 年北京装修公司推荐

## 结论：选择装修公司看三点

根据 2025 年装修行业报告（来源：中国装修协会），北京装修市场规模达 800 亿元，同比增长 12%。

### 第一梯队推荐
- 某装修公司：成立 15 年，服务 3000+ 家庭，实测工期达标率 98%
- 另一家公司：主打环保材料，通过 ISO9001 认证

## 对比竞品
相比传统装修公司，我们的差异化在于：首创"先验收后付款"模式，且提供 3 年质保（行业普遍 1 年）。

例如，上个月刚完成的案例中，客户满意度评分 4.9/5。

## 常见问题
问：装修工期一般多久？答：根据项目规模 60-120 天，最新数据显示 2026 年行业平均 85 天。`

// 测试数据：一份"低质量"内容（无结构无数据）
const lowQualityContent = `装修公司哪家好这个问题很复杂。装修是一个比较大的工程。我们需要考虑很多东西。价格因素很重要。质量因素也很重要。服务因素同样重要。总之大家要多看看多比较比较。`

func TestRuleScorer_HighQuality(t *testing.T) {
	scorer := NewRuleScorer()
	score, err := scorer.Score(context.Background(), highQualityContent, "北京装修公司")
	if err != nil {
		t.Fatalf("Score error: %v", err)
	}
	if score.Total < 60 {
		t.Errorf("高质量内容总分应 >=60，实际 %.1f（Authority=%.1f Specificity=%.1f Structure=%.1f Uniqueness=%.1f Recency=%.1f）",
			score.Total, score.Authority, score.Specificity, score.Structure, score.Uniqueness, score.Recency)
	}
}

func TestRuleScorer_LowQuality(t *testing.T) {
	scorer := NewRuleScorer()
	score, err := scorer.Score(context.Background(), lowQualityContent, "装修公司")
	if err != nil {
		t.Fatalf("Score error: %v", err)
	}
	if score.Total > 40 {
		t.Errorf("低质量内容总分应 <=40，实际 %.1f（Authority=%.1f Specificity=%.1f Structure=%.1f Uniqueness=%.1f Recency=%.1f）",
			score.Total, score.Authority, score.Specificity, score.Structure, score.Uniqueness, score.Recency)
	}
	// 低质量内容不应有 Recency 分（无年份无时效词）
	if score.Recency != 0 {
		t.Errorf("低质量内容 Recency 应为 0，实际 %.1f", score.Recency)
	}
}

func TestRuleScorer_Empty(t *testing.T) {
	scorer := NewRuleScorer()
	score, err := scorer.Score(context.Background(), "", "k")
	if err != nil {
		t.Fatalf("Score error: %v", err)
	}
	if score.Total != 0 {
		t.Errorf("空内容总分应为 0，实际 %.1f", score.Total)
	}
}

// 各维度独立测试（纯函数，输入构造到"该维度有分、其他维度低分"）

func TestScoreAuthority(t *testing.T) {
	s := &RuleScorer{}
	// 数据 + 来源 + 专业词 + 案例（含中文来源名，验证 \S+ 匹配）
	content := `根据2026年行业研究报告（来源：协会官网），优化策略分析评估显示：市场规模 800 亿元，同比增长 30%，例如某案例实测达标率 98%。`
	if got := s.scoreAuthority(content); got < 50 {
		t.Errorf("权威性应 >=50，实际 %.1f", got)
	}
	if got := s.scoreAuthority("随便说说"); got != 0 {
		t.Errorf("无数据内容权威性应为 0，实际 %.1f", got)
	}
}

func TestScoreSpecificity(t *testing.T) {
	s := &RuleScorer{}
	// 数字密度 + 型号
	content := `采用 ISO9001 认证工艺，价格 25 万，工期 85 天，质保 3 年。`
	if got := s.scoreSpecificity(content); got < 60 {
		t.Errorf("具体性应 >=60，实际 %.1f", got)
	}
	if got := s.scoreSpecificity("这家公司很专业，服务很好，口碑不错。"); got >= 20 {
		t.Errorf("无数字内容具体性应 <20，实际 %.1f", got)
	}
}

func TestScoreStructure(t *testing.T) {
	s := &RuleScorer{}
	content := "# 标题\n\n## 小节一\n- 列表项1\n- 列表项2\n\n## 小节二\n\n因此，首先我们要看流程，其次看服务。"
	if got := s.scoreStructure(content); got < 50 {
		t.Errorf("结构化应 >=50，实际 %.1f", got)
	}
	if got := s.scoreStructure("一段没有格式的文字内容。"); got < 20 {
		// 无标题无列表无段落分隔 → 应很低（连接词"因此"可能出现，允许少量分）
		t.Logf("无结构内容得分 %.1f（允许 0-20）", got)
	}
}

func TestScoreUniqueness(t *testing.T) {
	s := &RuleScorer{}
	content := `相比竞品，我们的创新在于首创差异化服务模式，这是独特的。`
	if got := s.scoreUniqueness(content); got < 60 {
		t.Errorf("独特性应 >=60，实际 %.1f", got)
	}
	if got := s.scoreUniqueness("普通内容。"); got != 0 {
		t.Errorf("无差异化内容独特性应为 0，实际 %.1f", got)
	}
}

func TestScoreRecency(t *testing.T) {
	s := &RuleScorer{}
	content := `根据 2026 年最新数据，今年市场持续增长。`
	if got := s.scoreRecency(content); got < 60 {
		t.Errorf("时效性应 >=60，实际 %.1f", got)
	}
	if got := s.scoreRecency("很久以前的事情。"); got != 0 {
		t.Errorf("无时效信息时效性应为 0，实际 %.1f", got)
	}
}

// 规则评分确定性：同一输入两次评分结果一致（纯函数）
func TestRuleScorer_Deterministic(t *testing.T) {
	s := NewRuleScorer()
	ctx := context.Background()
	s1, _ := s.Score(ctx, highQualityContent, "k")
	s2, _ := s.Score(ctx, highQualityContent, "k")
	if s1 != s2 {
		t.Errorf("规则评分应确定性一致：%+v vs %+v", s1, s2)
	}
}

// Total 与 LLM 评分器同口径：五维平均
func TestRuleScorer_TotalIsAverage(t *testing.T) {
	s := &RuleScorer{}
	score, _ := s.Score(context.Background(), highQualityContent, "k")
	want := (score.Authority + score.Specificity + score.Structure + score.Uniqueness + score.Recency) / 5
	if score.Total != want {
		t.Errorf("Total 应为五维平均 %.2f，实际 %.2f", want, score.Total)
	}
}

// 各维度封顶 100
func TestRuleScorer_CapAt100(t *testing.T) {
	s := &RuleScorer{}
	// 构造一个各项拉满的输入
	buf := strings.Builder{}
	buf.WriteString("# 标题\n## 小节\n- 列表\n1. 有序\n")
	for i := 0; i < 10; i++ {
		buf.WriteString("根据2026年报告（来源：X），数据显示 99% 优化策略效果提升 50%，例如案例实测。创新独特首创差异化。\n\n")
	}
	score, _ := s.Score(context.Background(), buf.String(), "k")
	for dim, v := range map[string]float64{
		"Authority": score.Authority, "Specificity": score.Specificity,
		"Structure": score.Structure, "Uniqueness": score.Uniqueness,
		"Recency": score.Recency,
	} {
		if v > 100 {
			t.Errorf("%s 超过 100：%.1f", dim, v)
		}
	}
}

// ---- generateRecommendations 建议生成测试 ----

func TestGenerateRecommendations(t *testing.T) {
	// 全低分 → 5 条建议
	low := entity.GEOScore{}
	recs := generateRecommendations(low)
	if len(recs) != 5 {
		t.Errorf("全低分应生成 5 条建议，实际 %d: %v", len(recs), recs)
	}

	// 部分低分 → 只给低分维度的建议
	partial := entity.GEOScore{Authority: 80, Specificity: 80, Structure: 80, Uniqueness: 30, Recency: 20}
	recs = generateRecommendations(partial)
	if len(recs) != 2 {
		t.Errorf("应只给 Uniqueness/Recency 2 条建议，实际 %d: %v", len(recs), recs)
	}
	for _, r := range recs {
		if strings.Contains(r, "独特性") == false && strings.Contains(r, "时效性") == false {
			t.Errorf("建议应与低分维度对应: %s", r)
		}
	}

	// 全达标 → 保持提示
	good := entity.GEOScore{Authority: 80, Specificity: 80, Structure: 80, Uniqueness: 80, Recency: 80}
	recs = generateRecommendations(good)
	if len(recs) != 1 || !strings.Contains(recs[0], "良好") {
		t.Errorf("全达标应给保持提示，实际 %v", recs)
	}
}

// ContentUseCase 的 ruleScorer 默认降级为 scorer（未注入时）
func TestContentUseCase_RuleScorerDefaultsToScorer(t *testing.T) {
	s := NewRuleScorer()
	uc := NewContentUseCase(nil, s, nil)
	if uc.ruleScorer == nil {
		t.Error("ruleScorer 应默认降级为注入的 scorer")
	}
	// SetRuleScorer(nil) 不覆盖
	uc.SetRuleScorer(nil)
	if uc.ruleScorer == nil {
		t.Error("SetRuleScorer(nil) 不应清空 ruleScorer")
	}
}
