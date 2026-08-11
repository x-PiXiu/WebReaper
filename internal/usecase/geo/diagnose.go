package geo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"webreaper/internal/domain/entity"
	"webreaper/internal/pkg"
	"webreaper/internal/usecase/port"
)

// ============ GEO 诊断用例 ============
//
// 诊断回答商户最关心的问题："我的品牌为什么没被 AI 提及？"
// 数据驱动（基于监测结果 + RAG 检索源真实情况），不是泛泛的"多发文章"。

// DiagnoseUseCase GEO 诊断用例。
type DiagnoseUseCase struct {
	brandRepo  port.BrandRepository
	resultRepo port.MonitoringResultRepository
	aiGen      port.AIGenerator // 生成自然语言改进建议
	quotaGate  port.QuotaStore  // 配额检查门（可选；X-01：diagnose 场景按次限额）
}

func NewDiagnoseUseCase(br port.BrandRepository, rr port.MonitoringResultRepository, ai port.AIGenerator) *DiagnoseUseCase {
	return &DiagnoseUseCase{brandRepo: br, resultRepo: rr, aiGen: ai}
}

// SetQuotaGate 注入配额检查门（可选；X-01 经济系统收口——诊断烧 token（RAG+LLM 建议），
// 超限返回 ErrQuotaExceeded → HTTP 402）。注意：内容生成的"按诊断建议生成"走
// content-gen 配额，独立诊断端点（/geo/brands/:id/diagnose）走 diagnose 配额。
func (uc *DiagnoseUseCase) SetQuotaGate(g port.QuotaStore) {
	if g != nil {
		uc.quotaGate = g
	}
}

// DiagnoseInput 诊断输入。
type DiagnoseInput struct {
	TenantID  string
	BrandID   string
	KeywordID string // 可选：诊断特定关键词；空则诊断品牌整体
}

// Diagnose 执行诊断：聚合监测结果 → 分析问题 → 生成改进建议。
func (uc *DiagnoseUseCase) Diagnose(ctx context.Context, in DiagnoseInput) (entity.DiagnoseReport, error) {
	// 配额检查（X-01：diagnose 场景；超限 402）
	if uc.quotaGate != nil {
		if err := uc.quotaGate.Check(ctx, in.TenantID, "diagnose"); err != nil {
			return entity.DiagnoseReport{}, err
		}
	}
	// 用量计量上下文（X-01）：诊断内部的 LLM 调用（llmSuggestions）落 usages 表，
	// 场景=diagnose——配额计数与成本统计同源。
	ctx = port.WithUsageContext(ctx, in.TenantID, "diagnose")

	brand, err := uc.brandRepo.FindByID(ctx, in.TenantID, in.BrandID)
	if err != nil {
		return entity.DiagnoseReport{}, fmt.Errorf("品牌不存在: %w", err)
	}

	// 取近期监测数据（趋势）
	results, err := uc.resultRepo.Trend(ctx, in.TenantID, in.BrandID, 20)
	if err != nil {
		results = nil
	}

	// 聚合分析
	var totalSource int
	var sumRate float64
	competitorStats := make(map[string]*entity.CompetitorStat)
	count := 0
	for _, r := range results {
		if in.KeywordID != "" && r.KeywordID != in.KeywordID {
			continue
		}
		totalSource += r.SampleCount
		sumRate += r.MentionRate
		count++
		// 竞品统计
		for _, comp := range r.Competitors {
			if competitorStats[comp] == nil {
				competitorStats[comp] = &entity.CompetitorStat{Name: comp}
			}
			competitorStats[comp].AppearanceRate += 1
		}
	}

	avgRate := 0.0
	if count > 0 {
		avgRate = sumRate / float64(count)
	}

	// 竞品统计转切片 + 算出现率
	var compStats []entity.CompetitorStat
	for _, cs := range competitorStats {
		if count > 0 {
			cs.AppearanceRate = cs.AppearanceRate / float64(count)
		}
		compStats = append(compStats, *cs)
	}

	// 生成改进建议（LLM 基于真实数据）
	suggestions := uc.generateSuggestions(ctx, brand, avgRate, totalSource, compStats)

	return entity.DiagnoseReport{
		BrandID:             in.BrandID,
		KeywordID:           in.KeywordID,
		ContentCoverage:     totalSource,
		BrandAppearanceRate: avgRate,
		CompetitorStats:     compStats,
		Suggestions:         suggestions,
	}, nil
}

// generateSuggestions 基于监测数据生成可执行改进建议。
// 优先用 LLM 生成自然语言建议；LLM 不可用时降级到规则建议。
func (uc *DiagnoseUseCase) generateSuggestions(ctx context.Context, brand entity.Brand, avgRate float64, sourceCount int, competitors []entity.CompetitorStat) []string {
	// 规则建议（基础，总是生成）
	var suggestions []string
	if sourceCount < 10 {
		suggestions = append(suggestions, fmt.Sprintf("全网相关内容不足（仅检索到 %d 篇），建议在知乎、掘金、CSDN 等平台多发「%s」相关文章，扩大内容覆盖面", sourceCount, brand.Name))
	}
	if avgRate < 0.3 {
		suggestions = append(suggestions, fmt.Sprintf("品牌「%s」的 AI 提及率偏低（%.0f%%），说明现有内容未被 AI 充分引用。建议优化内容质量：补充具体数据、案例、对比分析", brand.Name, avgRate*100))
	}
	if len(competitors) > 0 {
		topComp := competitors[0]
		if topComp.AppearanceRate > avgRate {
			suggestions = append(suggestions, fmt.Sprintf("竞品「%s」的可见度（%.0f%%）高于本品牌（%.0f%%），建议分析其内容策略并差异化竞争", topComp.Name, topComp.AppearanceRate*100, avgRate*100))
		}
	}
	if len(brand.Competitors) == 0 {
		suggestions = append(suggestions, "尚未配置竞品，建议补充竞品信息以便对比分析和针对性优化")
	}

	// LLM 生成更具体的建议（可选，增强）
	if uc.aiGen != nil && len(suggestions) > 0 {
		llmSugg := uc.llmSuggestions(ctx, brand, avgRate, sourceCount, competitors, suggestions)
		if len(llmSugg) > 0 {
			return llmSugg
		}
	}
	return suggestions
}

// llmSuggestions 让 LLM 基于监测数据生成更具体的改进建议。
func (uc *DiagnoseUseCase) llmSuggestions(ctx context.Context, brand entity.Brand, avgRate float64, sourceCount int, competitors []entity.CompetitorStat, ruleSuggestions []string) []string {
	compInfo := "无竞品数据"
	if len(competitors) > 0 {
		var parts []string
		for _, c := range competitors {
			parts = append(parts, fmt.Sprintf("%s(%.0f%%)", c.Name, c.AppearanceRate*100))
		}
		compInfo = strings.Join(parts, ", ")
	}

	systemPrompt := "你是 GEO 优化顾问。基于监测数据，给商户 3-5 条具体可执行的改进建议。每条建议一行，不要编号。"
	userPrompt := fmt.Sprintf(`品牌：%s
定位：%s
当前 AI 提及率：%.0f%%
全网相关内容数：%d 篇
竞品可见度：%s

基础分析：%s

请给出更具体、可执行的 GEO 改进建议（如具体发什么内容、发到哪个平台、怎么优化）：`,
		brand.Name, brand.Positioning, avgRate*100, sourceCount, compInfo, strings.Join(ruleSuggestions, "; "))

	messages := []port.ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	convID := fmt.Sprintf("diagnose-%d", time.Now().UnixNano())
	resp, err := uc.aiGen.ChatStream(ctx, convID, "", messages, nil)
	if err != nil {
		return nil
	}
	// 部分模型输出 <think> 思考块——剥离后再按行解析，避免碎片混入建议
	resp = pkg.StripThinkTags(resp)
	var out []string
	for _, line := range strings.Split(resp, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "0123456789.、) ")
		if line != "" && len([]rune(line)) >= 5 {
			out = append(out, line)
		}
	}
	return out
}
