package memory

import (
	"regexp"
	"strings"
)

// ========== Memory Context Scrubber（参考 hermes-agent StreamingContextScrubber） ==========

// MemoryContextScrubber 清除流式输出中的 memory-context 标签
// 参考 hermes-agent StreamingContextScrubber 设计：
// - 防止 <memory-context> 内容泄露给用户
// - 支持流式处理（标签可能跨多个 chunk）
var (
	// 匹配完整的 memory-context block
	memoryContextBlockRegex = regexp.MustCompile(`(?s)<memory-context>[\s\S]*?</memory-context>`)
	// 匹配开始的标签
	memoryContextOpenRegex  = regexp.MustCompile(`<memory-context>`)
	// 匹配结束的标签
	memoryContextCloseRegex = regexp.MustCompile(`</memory-context>`)
	// 匹配 System note 行
	memorySystemNoteRegex   = regexp.MustCompile(`\[System note: The following is recalled memory context[^\]]*\]\s*`)
)

// ScrubMemoryContext 从文本中清除 memory-context 标签及其内容
// 用于 WebSocket 流式输出，防止用户看到记忆上下文
func ScrubMemoryContext(text string) string {
	if text == "" {
		return ""
	}

	// 如果不包含 memory-context 标签，直接返回
	if !strings.Contains(text, "<memory-context>") && !strings.Contains(text, "</memory-context>") {
		return text
	}

	// 清除完整的 memory-context block
	text = memoryContextBlockRegex.ReplaceAllString(text, "")

	// 清除残留的开始/结束标签（可能跨 chunk）
	text = memoryContextOpenRegex.ReplaceAllString(text, "")
	text = memoryContextCloseRegex.ReplaceAllString(text, "")

	// 清除 System note 行
	text = memorySystemNoteRegex.ReplaceAllString(text, "")

	// 清理多余空白
	text = strings.TrimSpace(text)

	return text
}

// StreamingMemoryScrubber 流式记忆清除器（参考 hermes-agent StreamingContextScrubber）
// 处理标签跨 chunk 边界的情况
type StreamingMemoryScrubber struct {
	inSpan bool   // 是否在 memory-context span 内
	buffer string // 缓存未处理的文本
}

// NewStreamingMemoryScrubber 创建流式清除器
func NewStreamingMemoryScrubber() *StreamingMemoryScrubber {
	return &StreamingMemoryScrubber{
		inSpan: false,
		buffer: "",
	}
}

// Feed 处理流式输入，返回清除后的可见文本
// 参考 hermes-agent StreamingContextScrubber.feed()
func (s *StreamingMemoryScrubber) Feed(text string) string {
	if text == "" {
		return ""
	}

	buf := s.buffer + text
	s.buffer = ""
	var result []string

	i := 0
	for i < len(buf) {
		if s.inSpan {
			// 在 span 内，寻找结束标签
			closeIdx := strings.Index(buf[i:], "</memory-context>")
			if closeIdx == -1 {
				// 结束标签未找到，缓存剩余部分
				s.buffer = buf[i:]
				return strings.Join(result, "")
			}
			// 找到结束标签，跳过 span 内容
			i += closeIdx + len("</memory-context>")
			s.inSpan = false
		} else {
			// 不在 span 内，寻找开始标签
			openIdx := strings.Index(buf[i:], "<memory-context>")
			if openIdx == -1 {
				// 开始标签未找到，输出剩余部分
				result = append(result, buf[i:])
				return strings.Join(result, "")
			}
			// 输出开始标签前的内容
			if openIdx > 0 {
				result = append(result, buf[i:i+openIdx])
			}
			i += openIdx + len("<memory-context>")
			s.inSpan = true
		}
	}

	return strings.Join(result, "")
}

// Flush 在流结束时调用，返回剩余可见文本
// 如果仍在 span 内，丢弃内容（防止泄露部分记忆）
func (s *StreamingMemoryScrubber) Flush() string {
	if s.inSpan {
		// 仍在 span 内，丢弃
		s.inSpan = false
		s.buffer = ""
		return ""
	}
	result := s.buffer
	s.buffer = ""
	return result
}

// Reset 重置清除器状态
func (s *StreamingMemoryScrubber) Reset() {
	s.inSpan = false
	s.buffer = ""
}