package pkg

import "strings"

// StripThinkBlocks 剥离 LLM 思考块 <think>...</think>（可能多段/未闭合）。
// 部分模型忽略 DisableThinking 选项仍输出思考块——解析 JSON 前必须先剥离，
// 否则 ```json 前缀判断失败、JSON 内容被当普通文本。
func StripThinkBlocks(s string) string {
	for {
		i := strings.Index(s, "<think>")
		if i < 0 {
			break
		}
		j := strings.Index(s[i:], "</think>")
		if j < 0 {
			s = s[:i] // 未闭合的 think 块：截断保留前缀
			break
		}
		s = s[:i] + s[i+j+len("</think>"):]
	}
	return strings.TrimSpace(s)
}

// ExtractJSONBlock 从 LLM 输出中提取 JSON 块。
// 兼容：<think> 思考块包裹、```json 代码块包裹、前后附带说明文字（大括号兜底）。
// 结构化输出模式下引擎一般直接输出纯 JSON；个别模型带包裹时去包裹。
// 返回空串表示未找到有效 JSON 块。
func ExtractJSONBlock(s string) string {
	s = StripThinkBlocks(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			s = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	// 大括号兜底：取第一个 { 到最后一个 }（JSON 前/后的说明文字、```json 标记一并剔除）
	if i := strings.IndexByte(s, '{'); i >= 0 {
		if j := strings.LastIndexByte(s, '}'); j > i {
			return s[i : j+1]
		}
	}
	return s
}
