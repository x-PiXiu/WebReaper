package pkg

import "strings"

// ExtractJSONBlock 从 LLM 输出中提取 JSON 块（兼容 ```json 包裹）。
// 结构化输出模式下引擎一般直接输出纯 JSON；个别模型带代码块包裹时去包裹。
// 返回空串表示未找到有效 JSON 块。
func ExtractJSONBlock(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			return strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	return s
}
