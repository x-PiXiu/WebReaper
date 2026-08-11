package entity

import "time"

// GEO 领域实体：围绕"品牌 → 关键词 → AI 引擎监测 → 内容优化"组织。
//
// 设计动机（整洁架构）：
//   - 这些是 GEO 业务的核心概念，去掉软件也存在（品牌要在 AI 搜索里被引用）。
//   - 全部带 TenantID，多租户隔离的根。仓储层强制按 TenantID 过滤。
//   - 零框架依赖：纯 struct + 领域规则，不 import gorm/jwt/llm。

// Brand 是商户的品牌资产（聚合根）。
// 一个商户可有多品牌（如装修公司有"家装""工装"两个品牌线）。
type Brand struct {
	ID           string
	TenantID     string   // 租户隔离
	Name         string   // 品牌名（如"某装修公司"）
	Positioning  string   // 品牌定位（生成内容的提示词语料）
	CoreSelling  []string // 核心卖点（优化内容的权威性维度）
	Competitors  []string // 竞品名清单（监测时对比）
	CreatedAt    time.Time
}

// IsValid 领域规则：品牌必须有 ID、TenantID、名称。
func (b Brand) IsValid() bool {
	return b.ID != "" && b.TenantID != "" && b.Name != ""
}

// Keyword 是商户要监测/优化的搜索词。
type Keyword struct {
	ID        string
	TenantID  string
	BrandID   string // 所属品牌
	Term      string // 关键词（如"北京装修公司哪家好"）
	Intent    string // 搜索意图：informational/transactional/local
	CreatedAt time.Time
}

// IsValid 领域规则。
func (k Keyword) IsValid() bool {
	return k.ID != "" && k.TenantID != "" && k.BrandID != "" && k.Term != ""
}

// MonitoringResult 是一次 AI 引擎探测的快照（GEO 最有价值的数据资产）。
//
// 设计要点（采样降噪——实体层业务规则，不可漏）：
//   - AI 回答有随机性，单次不可信。一次监测对同一引擎采样多次。
//   - 提及情况用 MentionRate（提及率）而非布尔值。
type MonitoringResult struct {
	ID           string
	TenantID     string
	BrandID      string
	KeywordID    string
	EngineName   string  // 探测的 AI 引擎（对应 LLMConfig.Name）
	SampleCount  int     // 采样次数
	MentionCount int     // 提到品牌的次数
	MentionRate  float64 // 提及率 = MentionCount/SampleCount
	AvgPosition  int     // 平均排名（1=最靠前，0=未被提及）
	Sentiment    string  // positive/neutral/negative
	Competitors  []string // 同次回答里提到的竞品（去重清单）
	// CompetitorRates 竞品提及率（{竞品名: 提及率}）——付费说服力核心：
	// 用户需要坐标系"我 45% vs 竞品 80%"才知道自己好不好。
	// 与 Competitors 同源（探测时统计），落库时按采样数归一化。
	CompetitorRates map[string]float64
	Confidence   float64 // 置信度（采样次数少则低）
	ProbedAt     time.Time
	RawSample    string  // 原始回答摘录（留证，便于复核；不存全量以省空间）
}

// MentionRateLabel 领域规则：把提及率映射为可读等级（纯函数，零依赖）。
func (m MonitoringResult) MentionRateLabel() string {
	switch {
	case m.MentionRate >= 0.8:
		return "强势"
	case m.MentionRate >= 0.5:
		return "稳定"
	case m.MentionRate >= 0.2:
		return "偶尔"
	default:
		return "缺席"
	}
}

// ComputeConfidenceEx 基于实际信息量计算置信度（推荐使用）。
// 不再只看采样次数——而是看这次监测的"证据强度"：
//   - 回答长度（长回答 = Agent 搜索到了足够内容 = 可信）
//   - 采样成功数（成功次数/期望次数）
//   - 搜索源数（爬到了几篇内容）
func ComputeConfidenceEx(answerLength, sampleCount, sourceCount int) float64 {
	if sampleCount <= 0 {
		return 0
	}
	// 基础分：采样成功 = 60 分起步
	score := 0.6
	// 回答长度加分（500字以上=满分，线性递减）
	if answerLength > 500 {
		score += 0.3
	} else if answerLength > 0 {
		score += 0.3 * float64(answerLength) / 500.0
	}
	// 搜索源加分（有内容来源=额外可信）
	if sourceCount > 0 {
		score += 0.1
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// GEOScore 是内容的 GEO 可见度评分（核心领域逻辑）。
//
// 五个维度（与具体 AI 引擎无关）：
//   1. Authority    权威性：有无数据/案例/资质支撑
//   2. Specificity  具体性：数字、细节、可验证信息
//   3. Structure    结构化：标题层级、列表、FAQ
//   4. Uniqueness   独特性：与全网内容的差异化
//   5. Recency      时效性：信息是否最新
//
// 发布门槛（保护公开站整体权重——低质量内容会拖累全站收录）：
//   - Score.Total < MinPublishScore（30）：拒绝发布
//   - 30 ≤ Score.Total < WarnPublishScore（50）：允许但记日志警告
//   - Score.Total == 0：跳过检查（兼容无评分的历史数据）
const (
	MinPublishScore  = 30.0 // 低于此分拒绝发布（硬门槛）
	WarnPublishScore = 50.0 // 低于此分允许但警告（软门槛）
)

type GEOScore struct {
	Total       float64
	Authority   float64
	Specificity float64
	Structure   float64
	Uniqueness  float64
	Recency     float64
}

// Level 领域规则：把总分映射为等级（纯函数）。
func (s GEOScore) Level() string {
	switch {
	case s.Total >= 80:
		return "A" // 优秀：高度可能被引用
	case s.Total >= 65:
		return "B" // 良好
	case s.Total >= 50:
		return "C" // 及格
	default:
		return "D" // 待优化
	}
}

// OptimizedContent 是优化后的内容（带版本，支持 A/B 对比）。
type OptimizedContent struct {
	ID            string
	TenantID      string
	BrandID       string
	KeywordID     string
	Title         string   // 内容标题（AI 生成/优化时从正文提取，发布到平台用）
	OriginalText  string   // 原始素材
	OptimizedText string   // AI 优化后的内容
	Version       int      // 版本号
	Score         GEOScore
	Status        string    // draft/approved/published
	IndexStatus   string    // 收录状态：pending（已提交未收录）/ indexed（已收录）/ error（查询失败）
	IndexedAt     time.Time // 收录确认时间
	CreatedAt     time.Time
}

// 收录状态常量（收录验证任务写入，公开站点/管理后台展示）。
const (
	IndexStatusPending = "pending" // 已提交 IndexNow，尚未被收录
	IndexStatusIndexed = "indexed" // 已被 Bing 收录
	IndexStatusError   = "error"   // 查询失败（网络/key 问题），保留上次状态
)

// IsValid 领域规则。
func (o OptimizedContent) IsValid() bool {
	return o.ID != "" && o.TenantID != "" && o.OptimizedText != ""
}

// DiagnoseReport 是 GEO 诊断报告（回答"品牌为什么没被 AI 提及"）。
//
// 诊断维度（数据驱动，基于 RAG 检索源的真实情况）：
//   - ContentCoverage 内容覆盖度：全网相关文章数量（爬到了几篇）
//   - BrandAppearanceRate 品牌出现率：检索源里提到品牌的比例
//   - CompetitorGap 竞品对比：竞品在检索源里的出现情况（谁盖过你）
//   - Suggestions 改进建议：基于上述给出可执行建议
type DiagnoseReport struct {
	BrandID             string
	KeywordID           string
	ContentCoverage     int      // 全网相关文章数量
	BrandAppearanceRate float64  // 检索源里提到品牌的比例（0-1）
	CompetitorStats     []CompetitorStat // 竞品出现统计
	Suggestions         []string // 可执行改进建议
	DiagnosedAt         time.Time
}

// CompetitorStat 竞品在检索源中的出现统计。
type CompetitorStat struct {
	Name            string  // 竞品名
	AppearanceRate  float64 // 出现率（0-1）
	AvgPosition     int     // 平均排名
}
