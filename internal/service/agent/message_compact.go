// Package agent — Message content compaction for incremental A2A context
//
// 目的：把上游 Agent 完整 storedContent 精简为下游 Agent 需要的最小上下文。
//
// 借鉴 clowder-ai HandoffDigestGenerator 的哲学（"下游只需要知道上游想传递什么"），
// 但走本地纯函数路径 —— 不调 LLM，只做结构化提取 + 头尾截断。
//
// 三步预处理（按优先级）：
//   1. 优先 ExtractHandoffBlock —— 有 <a2a-handoff> 结构化块直接用
//   2. 否则剥离 thinking / tool_use / tool_result 类调试块
//   3. 仍然过长时做 head-tail 40/60 截断
//
// 与 legacy BuildChainHistoryLayer (context_builder.go:274-291) 语义对齐：
//   有 handoff → 保留全文；无 handoff → 800 chars 截断。
//   区别是本函数也剥离 thinking / tool 结构块，进一步压缩。
package agent

import (
	"regexp"
	"strings"
)

// 编译一次的正则，避免每条消息都重编译
var (
	thinkingBlockRegex     = regexp.MustCompile(`(?s)<thinking>.*?</thinking>`)
	toolUseBlockRegex      = regexp.MustCompile(`(?s)<tool_use>.*?</tool_use>`)
	toolResultBlockRegex   = regexp.MustCompile(`(?s)<tool_result>.*?</tool_result>`)
	thinkingMarkdownRegex1 = regexp.MustCompile(`(?s)\*\*思考过程\*\*.*?\n\n`)
	thinkingMarkdownRegex2 = regexp.MustCompile(`(?s)## Thinking.*?\n\n`)
	thinkingMarkdownRegex3 = regexp.MustCompile(`(?s)## 思考.*?\n\n`)
)

// CompactMessageContent 对单条 agent 消息内容做精简。
//
// - maxCharsFallback：无 handoff 且剥离后仍超过此长度时，做 head-tail 截断（保尾权重更高）
//
// 处理顺序：
//  1. ExtractHandoffBlock 命中 → 只保留 handoff 内部内容（不带外层标签），走 handoff 路径不截断
//  2. 未命中 handoff：
//     a. 剥离 <thinking>/<tool_use>/<tool_result> 结构化块
//     b. 剥离 markdown 思考标题（"**思考过程**"/"## Thinking"/"## 思考"）
//     c. 若剩余长度 > maxCharsFallback，走 TruncateHeadTail
//
// 空白规范化：多个连续空行合并为一个，避免视觉噪声撑大长度。
func CompactMessageContent(content string, maxCharsFallback int) string {
	// 1) handoff 优先
	if handoff, ok := ExtractHandoffBlock(content); ok {
		return strings.TrimSpace(handoff)
	}

	// 2) 剥离结构化调试块
	stripped := content
	stripped = thinkingBlockRegex.ReplaceAllString(stripped, "")
	stripped = toolUseBlockRegex.ReplaceAllString(stripped, "")
	stripped = toolResultBlockRegex.ReplaceAllString(stripped, "")
	stripped = thinkingMarkdownRegex1.ReplaceAllString(stripped, "")
	stripped = thinkingMarkdownRegex2.ReplaceAllString(stripped, "")
	stripped = thinkingMarkdownRegex3.ReplaceAllString(stripped, "")

	// 3) 空白规范化：连续 3+ 空行折叠为 2
	for strings.Contains(stripped, "\n\n\n") {
		stripped = strings.ReplaceAll(stripped, "\n\n\n", "\n\n")
	}
	stripped = strings.TrimSpace(stripped)

	// 4) 头尾截断兜底
	if maxCharsFallback > 0 && len(stripped) > maxCharsFallback {
		stripped = TruncateHeadTail(stripped, maxCharsFallback)
	}

	return stripped
}
