package a2a

import (
	"github.com/anthropic/isdp/internal/parser"
)

// A2A 配置常量
const (
	MaxA2AMentionTargets = 2  // 单条消息最多触发的 Agent 数量
	MaxA2ADepth          = 10 // A2A 链最大深度
)

// MentionPattern mention 模式条目
type MentionPattern = parser.MentionPattern

// DetectUserMention 检测是否 @用户
func DetectUserMention(text string) bool {
	return parser.DetectUserMention(text)
}

// StripCodeBlocks 剥离代码块
func StripCodeBlocks(text string) string {
	return parser.StripCodeBlocks(text)
}