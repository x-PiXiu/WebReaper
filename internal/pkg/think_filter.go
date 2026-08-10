package pkg

import "regexp"

// thinkTagRegex 匹配 <think>...</think> 块（含可能的属性、跨行、大小写不敏感）。
var thinkTagRegex = regexp.MustCompile(`(?is)<think[^>]*>.*?</think>`)

// partialThinkStartRegex 匹配未闭合的 <think> 开头（流式场景：think 块还没结束）。
var partialThinkStartRegex = regexp.MustCompile(`(?is)<think[^>]*>.*`)

// partialThinkEndRegex 匹配未开头的 </think> 结尾（流式场景：think 块刚开始就被截断）。
var partialThinkEndRegex = regexp.MustCompile(`(?is)^.*</think>`)

// StripThinkTags 从文本中移除 <think>...</think> 块，只保留实际内容。
// 用于过滤 LLM 推理模型的思考过程，防止泄漏给用户。
//
// 适用场景：
//   - 非流式最终结果（完整文本，直接正则移除）
//   - 流式增量文本（可能不完整，用 StripThinkTagsStream 处理）
func StripThinkTags(text string) string {
	// 移除完整的 <think>...</think> 块
	result := thinkTagRegex.ReplaceAllString(text, "")
	// 移除未闭合的 <think> 开头（流式最后一片可能没收到 </think>）
	result = partialThinkStartRegex.ReplaceAllString(result, "")
	// 移除孤立的 </think> 结尾（之前的 <think> 被分片过滤了）
	result = regexp.MustCompile(`(?i)</think>`).ReplaceAllString(result, "")
	return result
}

// thinkState 流式过滤状态。
type thinkState struct {
	inThinkBlock bool // 是否正在 <think> 块内
	buffer       string // 缓冲区（处理跨分片的标签）
}

// NewThinkStreamFilter 创建一个流式 <think> 过滤器。
// 用于处理流式 LLM 输出中可能跨多个 text-delta 分片的 <think> 块。
//
// 用法：
//   filter := pkg.NewThinkStreamFilter()
//   for delta := range stream {
//       clean := filter.Filter(delta)
//       if clean != "" { onDelta(clean) }
//   }
//   clean := filter.Flush()  // 处理最后残留的缓冲区
func NewThinkStreamFilter() *ThinkStreamFilter {
	return &ThinkStreamFilter{state: &thinkState{}}
}

// ThinkStreamFilter 流式 <think> 块过滤器。
type ThinkStreamFilter struct {
	state *thinkState
}

// Filter 处理一个流式增量文本，返回应该推给用户的部分。
// 可能返回空字符串（整个增量在 think 块内或标签被截断时）。
func (f *ThinkStreamFilter) Filter(delta string) string {
	f.state.buffer += delta
	var output string

	for len(f.state.buffer) > 0 {
		if f.state.inThinkBlock {
			// 在 think 块内，寻找 </think>
			endIdx := indexOfCI(f.state.buffer, "</think>")
			if endIdx >= 0 {
				// 找到结束标签，跳过 think 内容
				f.state.buffer = f.state.buffer[endIdx+8:] // 8 = len("</think>")
				f.state.inThinkBlock = false
			} else {
				// 还没找到结束标签，继续缓冲
				break
			}
		} else {
			// 不在 think 块内，寻找 <think
			startIdx := indexOfCI(f.state.buffer, "<think")
			if startIdx >= 0 {
				// 找到开始标签，输出之前的内容
				output += f.state.buffer[:startIdx]
				rest := f.state.buffer[startIdx:]

				// 检查是否有完整的 <think> 标签（含 >）
				gtIdx := indexOfCI(rest, ">")
				if gtIdx >= 0 {
					// 标签完整，进入 think 块
					f.state.buffer = rest[gtIdx+1:]
					f.state.inThinkBlock = true
				} else {
					// 标签被截断（如 "<thi"），缓冲等待下一个 delta
					f.state.buffer = rest
					break
				}
			} else {
				// 没有找到 <think，但缓冲区末尾可能是截断的标签开头
				// 检查缓冲区末尾是否有 "<" 开头的部分标签
				cutAt := findPartialTagStart(f.state.buffer)
				if cutAt >= 0 && cutAt < len(f.state.buffer) {
					output += f.state.buffer[:cutAt]
					f.state.buffer = f.state.buffer[cutAt:]
					break
				}
				// 没有截断的标签，全部输出
				output += f.state.buffer
				f.state.buffer = ""
			}
		}
	}

	return output
}

// Flush 处理缓冲区中残留的内容（流结束时调用）。
func (f *ThinkStreamFilter) Flush() string {
	if f.state.inThinkBlock {
		// 流结束时仍在 think 块内，丢弃全部缓冲
		f.state.buffer = ""
		f.state.inThinkBlock = false
		return ""
	}
	result := f.state.buffer
	f.state.buffer = ""
	return result
}

// indexOfCI 大小写不敏感查找子串位置。
func indexOfCI(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			a, b := s[i+j], substr[j]
			if a >= 'A' && a <= 'Z' { a += 32 }
			if b >= 'A' && b <= 'Z' { b += 32 }
			if a != b { match = false; break }
		}
		if match { return i }
	}
	return -1
}

// findPartialTagStart 查找缓冲区末尾可能是截断标签开头的位置。
// 例如缓冲区末尾是 "<thi"，返回 "<thi" 的起始位置。
func findPartialTagStart(s string) int {
	tag := "<think"
	for i := len(tag) - 1; i > 0; i-- {
		if len(s) >= i && indexOfCI(s[len(s)-i:], tag[:i]) == 0 {
			return len(s) - i
		}
	}
	return -1
}
