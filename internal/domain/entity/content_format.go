package entity

// ContentFormat 内容格式预设（P1-5：用户可选生成"SEO 文章/点评/小红书/脚本/FAQ/评测"）。
//
// 设计动机（整洁架构 + 策略模式）：
//   - 内容格式是"会横向扩展的变化点"——未来可能加微信公众号/知乎专栏/LinkedIn 等。
//   - 用 map 预设表 + 纯函数注入 prompt（零仓储依赖），新增格式 = 加一行 map 条目。
//   - 镜像 engine_pref.go 的 BuildEnginePrefInstruction 模式（单一咽喉注入）。
//
// 与 TargetEngine 的区别：
//   - TargetEngine 是"按 AI 引擎偏好调语气"（豆包口语化 vs ChatGPT 正式）
//   - Format 是"用户选内容形态"（长文章 vs 点评短文 vs 视频脚本）
//   - 两者正交、可叠加（同一次生成既按引擎偏好又按格式输出）。

// ContentFormatPreset 格式预设（key + 展示名 + prompt 指令）。
type ContentFormatPreset struct {
	Key         string
	Label       string
	Instruction string // 追加到 systemPrompt 末尾的格式指令
}

// ContentFormats 全部格式预设（key → 预设）。
var ContentFormats = map[string]ContentFormatPreset{
	"article": {
		Key:   "article",
		Label: "SEO 文章",
		Instruction: "输出格式为 SEO 优化文章：标题层级清晰（H1/H2/H3），800-1500 字，" +
			"结构化列表+小标题，关键词自然分布。适合发布到官网/博客/知乎专栏。",
	},
	"review": {
		Key:   "review",
		Label: "点评文案",
		Instruction: "输出格式为大众点评/美团风格的探店点评：200-400 字，第一人称体验感，" +
			"包含环境/口味/服务/性价比评价，结尾带推荐语。口语化、真实感强，适合发到点评平台。",
	},
	"xiaohongshu": {
		Key:   "xiaohongshu",
		Label: "小红书笔记",
		Instruction: "输出格式为小红书种草笔记：【标题硬约束】标题不超过 20 个字，必须精炼完整" +
			"（如「成都美食｜春熙路必吃蜀香居🔥」），绝不能超过 20 字。" +
			"正文 300-500 字，分段短句，多 emoji 点缀（✨😋📍💡等），" +
			"文末带 3-5 个 #话题标签。语气亲切、有画面感。",
	},
	"script": {
		Key:   "script",
		Label: "视频口播脚本",
		Instruction: "输出格式为短视频口播脚本：开头 3 秒 hook（制造好奇/痛点），" +
			"中间痛点+解决方案（200-400 字口播），结尾行动号召（关注/到店/咨询）。" +
			"标注 [镜头提示] 方便拍摄。适合抖音/视频号。",
	},
	"faq": {
		Key:   "faq",
		Label: "FAQ 问答",
		Instruction: "输出格式为 FAQ 问答集：5-8 个常见问题+简明回答（每答 50-100 字），" +
			"覆盖用户搜索意图（是什么/怎么选/多少钱/注意事项）。适合官网帮助中心/知乎。",
	},
	"comparison": {
		Key:   "comparison",
		Label: "对比评测",
		Instruction: "输出格式为产品/服务对比评测：表格对比+文字分析，覆盖功能/价格/优缺点/" +
			"适用场景，客观中立带推荐结论。适合选购决策类内容。",
	},
	"citation": {
		Key:   "citation",
		Label: "高引用结构",
		Instruction: "输出格式为 AI 高引用友好结构（GEO 可引用素材）：" +
			"① 结论前置——开头 1-2 句直接给出核心答案（AI 摘录时优先取结论段）；" +
			"② 每个观点独立成段，段首用小标题（H3）概括该段要点；" +
			"③ 关键数据必须标注来源（如「据 2025 年行业报告」），可被 AI 验证；" +
			"④ 包含 2-3 个 FAQ 式问答对（问题即用户搜索句，答案 50-80 字）；" +
			"⑤ 全文 600-1200 字，段落短（≤4 行），无营销套话。" +
			"目标：让 AI 引擎在生成回答时最容易摘录、引用并标注你为信源。",
	},
}

// BuildFormatInstruction 根据 format key 返回 prompt 格式指令（空=默认 SEO 文章）。
// 纯函数，可独立单测（参照 engine_pref_test.go 模式）。
func BuildFormatInstruction(format string) string {
	if format == "" {
		return ContentFormats["article"].Instruction
	}
	if f, ok := ContentFormats[format]; ok {
		return f.Instruction
	}
	return ContentFormats["article"].Instruction // 未知格式回退默认
}

// CitationStructureToggles 可引用结构开关（v3 P2：四开关可组合，各自映射 prompt 片段——
// 对齐 GEO 内容四项原则"可引用素材结构"；与 format 预设正交叠加，任何格式都能加结构）。
var CitationStructureToggles = map[string]struct{ Label, Instruction string }{
	"conclusion-first": {
		Label: "结论前置",
		Instruction: "① 结论前置：开头 1-2 句直接给出核心答案/推荐结论（AI 摘录回答时优先取结论段，" +
			"结论不在开头的段落几乎不会被引用）。",
	},
	"standalone-paragraphs": {
		Label: "观点独立成段",
		Instruction: "② 观点独立成段：每个观点/卖点单独成段（≤4 行），一段只说一件事，" +
			"段落之间不互相依赖——AI 检索-选取-引用链路按段工作，混合段落无法被单独摘录。",
	},
	"data-cited": {
		Label: "数据标注来源",
		Instruction: "③ 数据标注来源：关键数字/结论必须标注可验证来源（如「据 2025 年行业报告」" +
			"「XX 官方数据」）——可解释性是 AI 选取信源的权重项，无来源数据会被跳过。",
	},
	"subheadings": {
		Label: "小标题分段",
		Instruction: "④ 小标题分段：每个段落用 H2/H3 小标题概括要点（小标题即该段的语义索引，" +
			"AI 按标题定位可引用段落）。",
	},
}

// BuildCitationStructureInstruction 组合已启用的可引用结构片段（保持稳定顺序，空返回 ""）。
// 纯函数——与 BuildFormatInstruction 同模式（单一咽喉注入，新增开关 = 加一行 map 条目）。
func BuildCitationStructureInstruction(toggles []string) string {
	order := []string{"conclusion-first", "standalone-paragraphs", "data-cited", "subheadings"}
	enabled := make(map[string]bool, len(toggles))
	for _, t := range toggles {
		enabled[t] = true
	}
	out := ""
	for _, key := range order {
		if enabled[key] {
			out += CitationStructureToggles[key].Instruction + "\n"
		}
	}
	return out
}
