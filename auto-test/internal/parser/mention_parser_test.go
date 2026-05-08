// auto-test/internal/parser/mention_parser_test.go
package parser_test

import (
	"testing"

	"github.com/anthropic/isdp/internal/parser"
	"github.com/stretchr/testify/assert"
)

/**
 * SV-02: A2A Mention Parser 测试
 * P0 用例：SV-02-01, SV-02-07
 */

// 测试用 patterns
var testPatterns = []parser.MentionPattern{
	{Pattern: "@backend", AgentID: "backend_developer"},
	{Pattern: "@后端", AgentID: "backend_developer"},
	{Pattern: "@architect", AgentID: "architect"},
	{Pattern: "@架构师", AgentID: "architect"},
	{Pattern: "@code_reviewer", AgentID: "code_reviewer"},
	{Pattern: "@reviewer", AgentID: "code_reviewer"},
	{Pattern: "@sre", AgentID: "sre_engineer"},
}

// @feature F003 - 多 Agent 协作
// @priority P0
// @id SV-02-01
func TestParseA2AMentions_Core(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		currentCatID string
		wantEmpty    bool // 期望空结果
		want         []string
	}{
		{
			name:         "single mention at line start",
			text:         "@backend 请实现这个功能",
			currentCatID: "architect",
			wantEmpty:    false,
			want:         []string{"backend_developer"},
		},
		{
			name:         "multiple mentions on separate lines",
			text:         "@backend 请实现\n@architect 请设计",
			currentCatID: "code_reviewer",
			wantEmpty:    false,
			want:         []string{"backend_developer", "architect"},
		},
		{
			name:         "filter self mention",
			text:         "@backend 请实现\n@architect 请设计",
			currentCatID: "backend_developer",
			wantEmpty:    false,
			want:         []string{"architect"},
		},
		{
			name:         "mention inside code block ignored",
			text:         "```\n@backend\n```\n@architect this one counts",
			currentCatID: "backend_developer",
			wantEmpty:    false,
			want:         []string{"architect"},
		},
		{
			name:         "mention not at line start ignored",
			text:         "hello @backend not at start",
			currentCatID: "architect",
			wantEmpty:    true,
		},
		{
			name:         "empty text returns empty",
			text:         "",
			currentCatID: "architect",
			wantEmpty:    true,
		},
		{
			name:         "text without mentions returns empty",
			text:         "这是一条普通消息",
			currentCatID: "architect",
			wantEmpty:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.ParseA2AMentions(tt.text, tt.currentCatID, testPatterns)
			if tt.wantEmpty {
				assert.Empty(t, got, "Expected empty result")
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// @feature F003 - 多 Agent 协作
// @priority P0
// @id SV-02-07
func TestParseA2AMentions_BoundaryCheck(t *testing.T) {
	// 测试最多 2 个目标限制
	text := "@backend 请实现\n@architect 请设计\n@code_reviewer 第三行"
	currentCatID := "sre_engineer"

	got := parser.ParseA2AMentions(text, currentCatID, testPatterns)
	assert.Len(t, got, 2, "最多只应该返回 2 个目标")
}

// @feature F003 - 多 Agent 协作
// @priority P1
// @id SV-02-08
func TestParseA2AMentions_SelfMentionFilter(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		currentCatID string
		wantEmpty    bool
		want         []string
	}{
		{
			name:         "single self mention returns empty",
			text:         "@backend 自己给自己发消息",
			currentCatID: "backend_developer",
			wantEmpty:    true,
		},
		{
			name:         "self mention with others returns only others",
			text:         "@backend 请实现\n@architect 请设计",
			currentCatID: "backend_developer",
			wantEmpty:    false,
			want:         []string{"architect"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.ParseA2AMentions(tt.text, tt.currentCatID, testPatterns)
			if tt.wantEmpty {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// @feature F003 - 多 Agent 协作
// @priority P1
// @id SV-02-09
func TestParseA2AMentions_CodeBlockFilter(t *testing.T) {
	text := "请看这段代码：\n```python\n@backend decorator\n```\n@architect 这才是真正的提及"
	currentCatID := "backend_developer"

	got := parser.ParseA2AMentions(text, currentCatID, testPatterns)
	assert.Equal(t, []string{"architect"}, got, "代码块内的 mention 应被过滤")
}

// @feature F003 - 多 Agent 协作
// @priority P1
// @id SV-02-10
func TestParseA2AMentions_NonLineStartFilter(t *testing.T) {
	text := "hello @backend\n@architect this one is at line start"
	currentCatID := "backend_developer"

	got := parser.ParseA2AMentions(text, currentCatID, testPatterns)
	assert.Equal(t, []string{"architect"}, got, "非行首的 mention 应被过滤")
}

// @feature F003 - 多 Agent 协作
// @priority P1
// @id SV-02-11
func TestParseA2AMentions_ChineseAlias(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		currentCatID string
		want         []string
	}{
		{
			name:         "chinese alias backend",
			text:         "@后端 请实现",
			currentCatID: "architect",
			want:         []string{"backend_developer"},
		},
		{
			name:         "chinese alias architect",
			text:         "@架构师 请设计",
			currentCatID: "backend_developer",
			want:         []string{"architect"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.ParseA2AMentions(tt.text, tt.currentCatID, testPatterns)
			assert.Equal(t, tt.want, got)
		})
	}
}

// @feature F003 - 多 Agent 协作
// @priority P1
// @id SV-02-12
func TestParseA2AMentions_TokenBoundary(t *testing.T) {
	// 测试 token boundary - @backend-xxx 不应该匹配 @backend
	text := "@backend-extra 这是另一个 Agent"
	currentCatID := "architect"

	got := parser.ParseA2AMentions(text, currentCatID, testPatterns)
	assert.Empty(t, got, "@backend-extra 不应该匹配 @backend pattern")
}

// @feature F003 - 多 Agent 协作
// @priority P2
// @id SV-02-13
func TestDetectUserMention(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"@co-creator detected", "@co-creator 请确认", true},
		{"@铲屎官 detected", "@铲屎官 看一下", true},
		{"@用户 detected", "@用户 检查", true},
		{"@user detected", "@user help", true},
		{"no mention", "这是一条普通消息", false},
		{"mention in sentence", "hello @co-creator", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.DetectUserMention(tt.text)
			assert.Equal(t, tt.want, got)
		})
	}
}

// @feature F003 - 多 Agent 协作
// @priority P2
// @id SV-02-14
func TestStripCodeBlocks(t *testing.T) {
	text := "前文\n```python\nprint('hello')\n```\n后文"
	got := parser.StripCodeBlocks(text)
	assert.Equal(t, "前文\n\n后文", got, "代码块应被剥离")
}