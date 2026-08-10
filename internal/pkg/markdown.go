package pkg

import "regexp"

// MarkdownToPlainText 将 Markdown 转为纯文本。
// 去掉所有 Markdown 格式符号（#、*、-、`、>、| 等），保留正文文字。
// 用于发布到社媒平台（知乎/小红书不支持 Markdown 渲染）。
func MarkdownToPlainText(md string) string {
	text := md

	// 去掉代码块 ```...```
	codeBlockRegex := regexp.MustCompile("(?s)```[a-zA-Z]*\n(.*?)```")
	text = codeBlockRegex.ReplaceAllString(text, "$1")

	// 去掉行内代码 `code`
	text = regexp.MustCompile("`([^`]+)`").ReplaceAllString(text, "$1")

	// 去掉标题标记 # ## ### 等（保留标题文字）
	text = regexp.MustCompile(`(?m)^#{1,6}\s+`).ReplaceAllString(text, "")

	// 去掉粗体 **text** 或 __text__
	text = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`__([^_]+)__`).ReplaceAllString(text, "$1")

	// 去掉斜体 *text* 或 _text_
	// 注意：Go 的 regexp（RE2）不支持 (?<!\w) 后行断言，用 \b 替代
	text = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`\b_([^_]+)_\b`).ReplaceAllString(text, "$1")

	// 去掉删除线 ~~text~~
	text = regexp.MustCompile(`~~([^~]+)~~`).ReplaceAllString(text, "$1")

	// 去掉列表标记 - * + （行首）
	text = regexp.MustCompile(`(?m)^[\s]*[-*+]\s+`).ReplaceAllString(text, "")

	// 去掉有序列表标记 1. 2. 等（行首）
	text = regexp.MustCompile(`(?m)^[\s]*\d+\.\s+`).ReplaceAllString(text, "")

	// 去掉引用 > 
	text = regexp.MustCompile(`(?m)^>\s?`).ReplaceAllString(text, "")

	// 去掉表格分隔行 |---|---|
	text = regexp.MustCompile(`(?m)^\|[\s\-:|]+\|?\s*$`).ReplaceAllString(text, "")

	// 去掉表格管道符 |，保留单元格内容
	text = regexp.MustCompile(`\|`).ReplaceAllString(text, " ")

	// 去掉链接 [text](url) → 只保留 text
	text = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(text, "$1")

	// 去掉图片 ![alt](url)
	text = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`).ReplaceAllString(text, "$1")

	// 去掉水平线 --- ***
	text = regexp.MustCompile(`(?m)^[\s]*[-*]{3,}[\s]*$`).ReplaceAllString(text, "")

	// 多余空行压缩为单个空行
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")

	return text
}

// ExtractTitle 从正文内容提取标题。
// 优先取第一个非空行（通常是标题行），去掉 Markdown 标题符号。
// 如果正文没有明确的标题行，取前 50 个字符。
func ExtractTitle(content string) string {
	lines := regexp.MustCompile(`\n`).Split(content, -1)
	for _, line := range lines {
		line = regexp.MustCompile(`(?m)^#{1,6}\s+`).ReplaceAllString(line, "")
		line = regexp.MustCompile(`\s+`).ReplaceAllString(line, " ")
		if len(line) > 3 {
			return line
		}
	}
	// 降级：取前 50 字符
	if len(content) > 50 {
		return content[:50]
	}
	return content
}
