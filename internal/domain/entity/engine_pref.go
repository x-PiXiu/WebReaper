package entity

import "strings"

// EnginePref 是某目标 AI 引擎的内容格式偏好（GEO 优化侧差异，不是接入侧差异）。
//
// 设计动机（让内容"更可能被目标引擎引用"）：
//   各 AI 引擎在综合回答时对内容格式有不同偏好（如 Perplexity 偏好简洁+来源链接、
//   ChatGPT 偏好结构化小节、豆包偏好口语化接地气）。这些偏好是公开可观察的行为特征，
//   属于领域规则（去掉软件也存在），放实体层。
//
// 用法：ContentUseCase 优化/生成时按 TargetEngine 取预设，把指令注入 prompt。
// 注意：这是"概率优化"，不是"保证引用"——引用前提仍是公开可访问 + 通用信号。
type EnginePref struct {
	Label            string   // 引擎显示名
	ContentStyle     string   // 内容风格要求
	SectionHint      string   // 章节结构要求
	CitationHint     string   // 引用/来源要求
	CustomInstruction string  // 附加指令（可空）
}

// EnginePrefs 目标引擎偏好预设表。
var EnginePrefs = map[string]EnginePref{
	"chatgpt": {
		Label:         "ChatGPT",
		ContentStyle:  "professional, structured, clear heading levels",
		SectionHint:   "use concise heading hierarchy and bullet lists",
		CitationHint:  "include data and citations where appropriate",
	},
	"perplexity": {
		Label:         "Perplexity",
		ContentStyle:  "concise, direct answer first",
		SectionHint:   "put the key conclusion in the first sentence",
		CitationHint:  "prefer citing authoritative sources with specific numbers",
	},
	"kimi": {
		Label:         "Kimi",
		ContentStyle:  "clear and readable, moderate length",
		SectionHint:   "use clear section headings",
		CitationHint:  "include verifiable facts and figures",
	},
	"doubao": {
		Label:         "豆包",
		ContentStyle:  "自然口语化、接地气，但保持专业",
		SectionHint:   "用小标题和列表让结构清晰",
		CitationHint:  "给出具体数字和可验证细节",
	},
	"generic": {
		Label:         "通用",
		ContentStyle:  "专业、结构化、结论前置",
		SectionHint:   "标题层级清晰、适当使用列表",
		CitationHint:  "包含具体数据与来源",
	},
}

// BuildEnginePrefInstruction 生成目标引擎的优化指令片段（纯函数，可单测）。
// engine 为空或未收录时返回通用指令。
func BuildEnginePrefInstruction(engine string) string {
	pref, ok := EnginePrefs[strings.ToLower(engine)]
	if !ok {
		pref = EnginePrefs["generic"]
	}
	var sb strings.Builder
	sb.WriteString("目标引擎「" + pref.Label + "」的引用偏好：")
	sb.WriteString(pref.ContentStyle)
	if pref.SectionHint != "" {
		sb.WriteString("；结构：" + pref.SectionHint)
	}
	if pref.CitationHint != "" {
		sb.WriteString("；引用：" + pref.CitationHint)
	}
	return sb.String()
}

// EnginePrefLabel 返回引擎显示名（未收录返回"通用"）。
func EnginePrefLabel(engine string) string {
	if pref, ok := EnginePrefs[strings.ToLower(engine)]; ok {
		return pref.Label
	}
	return EnginePrefs["generic"].Label
}
