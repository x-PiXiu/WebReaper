package port

import (
	"context"

	"webreaper/internal/domain/entity"
)

// ---- GEO 能力接口（监测/评分/内容生成）----

// ProbeInput 是探测一个关键词在某 AI 引擎的表现的输入。
type ProbeInput struct {
	TenantID    string   // 租户 ID（多租户隔离：临时向量 id 带 tenant 前缀，避免并发碰撞）
	Keyword     string   // 用户可能的搜索词（如"北京装修公司哪家好"）
	EngineName  string   // 探测的 AI 引擎（对应 LLMConfig.Name）
	BrandName   string   // 要监测的品牌名
	Aliases     []string // 品牌别名（AI 可能用简称/全称，监测时一起匹配）
	Competitors []string // 竞品名（同时监测对比）
	SampleSize  int      // 采样次数（默认 5，越多越准但越贵）
	// SelfBaseDomain 自营公开站域名（如 content.example.com，归因生命线 P5-01）。
	// 非空时探测统计 AI 回答引用的来源里包含该域名的次数（self_source_count）——
	// 回答"我们做的内容到底有没有被 AI 引用"这个续费生命线问题。
	SelfBaseDomain string
	// LocalContext 本地位置上下文（如"朝阳区望京"；本地生活 P0 补全，可选）。
	// 非空时探测问法加入位置型提问（"望京附近有什么川菜馆"）——测的是"本地生意"
	// 而非泛化品牌声量。由 MonitorUseCase 从门店档案注入。
	LocalContext string
	// AnalyzerName 解析用 LLM 引擎（P1-3：防自评 bias）。
	// 空 = default（与直测引擎分离——"豆包监测豆包回答"时用 default 解析，
	// 避免模型给自己/同类打高分的主观偏差）。
	AnalyzerName string
	// Questions 预生成问法池（采样矩阵·问法维度 v2：LLM 按品牌/卖点/竞品/地址生成，
	// 一次生成多引擎分片使用——见 ProbeQuestionGenerator）。
	// 非空时 Probe 实现按引擎分片随机抽取（同引擎不同采样不同问法、不同引擎问法隔离，
	// 避免相同 prompt 命中 LLM 缓存返回一致内容）；空则 Probe 内部模板问法兜底。
	Questions []string
}

// ProbeQuestionGenerator 问法池生成器（采样矩阵·问法维度，策略接口）。
// 由 MonitorUseCase 编排（一次监测一次生成，多引擎分片复用）；实现：
//   - LLMQuestionGenerator：按品牌定位/核心卖点/竞品/门店地址结构化生成真实用户问法
//   - TemplateQuestionGenerator：固定模板（ProbeQuestioner，兜底/未配置 AI 时）
//
// 设计动机（依赖倒置 + 策略模式）：问法生成是"监测采样质量"的核心变化点——
// 生成方式演进（模板 → LLM → 未来按用户画像）不应影响 Probe 实现与监测主流程。
type ProbeQuestionGenerator interface {
	// Generate 生成 count 个监测问法（结构化输出失败返回错误，由调用方降级模板）。
	Generate(ctx context.Context, in QuestionGenInput) ([]string, error)
}

// QuestionGenInput 问法生成输入（品牌上下文 + 采样规模）。
type QuestionGenInput struct {
	BrandName    string   // 品牌名（如"望京川菜馆"）
	Positioning  string   // 品牌定位（如"正宗川菜，主打水煮鱼"）
	CoreSelling  []string // 核心卖点
	Competitors  []string // 竞品名（生成对比问法："望京川菜馆和眉州东坡哪个好"）
	Keyword      string   // 监测关键词
	LocalContext string   // 门店位置（商圈/区/城市——"望京附近…"问法）
	Count        int      // 生成数量（问法池大小）
}

// ShardQuestions 问法池引擎分片：第 engineIdx 个引擎取 count 个问法（环形）。
//
// 采样矩阵·去缓存核心：LLM 生成一次问法池 → 每个引擎取不同子集——
//   - 同引擎多次采样：不同问法（池 > 采样数时）
//   - 不同引擎：问法隔离（相同 prompt 不跨引擎命中 LLM 缓存，内容互不相同）
// 池大小不足 count 时以引擎偏移环形循环（仍保证引擎间错位）。
func ShardQuestions(pool []string, engineIdx, count int) []string {
	if len(pool) == 0 || count <= 0 {
		return nil
	}
	// 引擎偏移：不同引擎从池的不同位置起取（错位 7 步，简单素数步长）
	start := (engineIdx * 7) % len(pool)
	out := make([]string, 0, count)
	for j := 0; j < count; j++ {
		out = append(out, pool[(start+j)%len(pool)])
	}
	return out
}

// ProbeResult 是一次探测的统计结果。
type ProbeResult struct {
	SampleCount  int
	MentionCount int     // 提到品牌的次数
	MentionRate  float64 // 提及率
	AvgPosition  int     // 平均排名
	Sentiment    string  // positive/neutral/negative
	Competitors  map[string]int // 竞品名 → 被提及次数
	RawSample    string  // 原始回答摘录
	SourceCount  int     // 搜索源文章数
	BrandAppearanceCount int // 品牌在检索源里出现的文章数
	Confidence   float64 // 置信度（由 Probe 实现按信息量计算，不再固定 sampleCount/5）
	// Sources 回答中提到的来源（链接/平台名，去重；P5-01 引用来源追踪）。
	// 归因：AI"提到你"≠"引用你的内容"——有了来源才能回答"我的文章被引用了几次"。
	Sources []string
	// SelfSourceCount 来源中包含自营公开站域名的次数（P5-01）。
	// >0 意味着 AI 回答实际引用了我们发布的内容——这是内容 GEO 的直接效果证据。
	SelfSourceCount int
}

// AIEngineProbe 是 AI 引擎监测适配器的接口（边界）。
// 用例层声明，适配器层实现（复用 port.AIGenerator 调 LLM，但把"问问题"包装成"探测"）。
//
// 设计动机（依赖倒置）：
//   - 用例层不关心怎么调 LLM、怎么解析回答，只依赖此接口。
//   - 换 AI 引擎 = 换 LLMConfig，探测实现零改动。
type AIEngineProbe interface {
	// Probe 对一个关键词问 AI 引擎，采样 N 次，解析品牌提及情况。
	Probe(ctx context.Context, in ProbeInput) (ProbeResult, error)
}

// GEOScorer 是 GEO 评分器接口（边界）。
// 实现：规则匹配 + LLM 评估混合。用例层只依赖此接口。
type GEOScorer interface {
	// Score 给内容打 GEO 分。
	Score(ctx context.Context, content string, keyword string) (entity.GEOScore, error)
}
