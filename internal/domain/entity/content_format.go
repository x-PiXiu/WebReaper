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
		Instruction: "输出格式为小红书种草笔记：标题吸睛（带 emoji），300-500 字，分段短句，" +
			"多 emoji 点缀（✨😋📍💡等），文末带 3-5 个 #话题标签。语气亲切、有画面感。",
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
