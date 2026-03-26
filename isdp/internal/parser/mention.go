package parser

import (
	"regexp"
	"sort"
	"strings"
)

// A2A 配置常量
const (
	MaxA2AMentionTargets = 2 // 单条消息最多触发的 Agent 数量
)

// Token boundary 正则表达式
var tokenBoundaryRegex = regexp.MustCompile(`^[\s,.:;!?()\[\]{}<>，。！？、：；（）【】《》「」『』〈〉]`)

// handleContinuationRegex 检测是否是 handle token 的延续
var handleContinuationRegex = regexp.MustCompile(`^[a-z0-9_.-]`)

// codeBlockRegex 代码块正则表达式
var codeBlockRegex = regexp.MustCompile("```[\\s\\S]*?```")

// MentionPattern mention 模式条目
type MentionPattern struct {
	Pattern  string   // mention 模式（如 "@developer", "@开发者"）
	AgentID  string   // 对应的 Agent ID（兼容旧接口）
	AgentIDs []string // 匹配的所有 Agent ID 列表（博弈场景支持）
}

// ParseA2AMentions 解析行首 @mention
//
// 规则（参考 Clowder AI 的 F046 简化规则）：
// 1. 剥离代码块 (```...```) 后再解析
// 2. 仅匹配行首 mention（可带前导空白）
// 3. 长匹配优先 + token boundary，避免 `@opus-45` 误命中 `@opus`
// 4. 过滤自调用
// 5. 最多返回 MaxA2AMentionTargets 个目标
func ParseA2AMentions(text string, currentAgentID string, patterns []MentionPattern) []string {
	// 使用新函数获取所有匹配，然后展开为单个 AgentID 列表
	matches := ParseA2AMentionsMulti(text, currentAgentID, patterns)

	result := make([]string, 0)
	seen := make(map[string]bool)

	for _, agentIDs := range matches {
		for _, agentID := range agentIDs {
			if !seen[agentID] {
				seen[agentID] = true
				result = append(result, agentID)
			}
		}
	}

	// 限制最大数量
	if len(result) > MaxA2AMentionTargets {
		result = result[:MaxA2AMentionTargets]
	}

	return result
}

// ParseA2AMentionsMulti 解析行首 @mention，支持博弈场景
// 返回 pattern -> []AgentID 的映射，一个 pattern 可能匹配多个 Agent
func ParseA2AMentionsMulti(text string, currentAgentID string, patterns []MentionPattern) [][]string {
	if text == "" {
		return nil
	}

	// 1. 剥离代码块
	stripped := codeBlockRegex.ReplaceAllString(text, "")

	// 2. 按长度降序排列（长匹配优先）
	sort.Slice(patterns, func(i, j int) bool {
		return len(patterns[i].Pattern) > len(patterns[j].Pattern)
	})

	// 3. 逐行解析
	lines := strings.Split(stripped, "\n")
	found := make([][]string, 0, MaxA2AMentionTargets)
	seenPatterns := make(map[string]bool)

	for _, line := range lines {
		if len(found) >= MaxA2AMentionTargets {
			break
		}

		// 检查行首 mention
		leadingWs := countLeadingWhitespace(line)
		normalized := strings.ToLower(line[leadingWs:])

		if !strings.HasPrefix(normalized, "@") {
			continue
		}

		// 尝试匹配每个 pattern
		for _, entry := range patterns {
			patternLower := strings.ToLower(entry.Pattern)

			if !strings.HasPrefix(normalized, patternLower) {
				continue
			}

			// Token boundary 检查
			charAfter := ""
			if len(normalized) > len(patternLower) {
				charAfter = string(normalized[len(patternLower)])
			}

			// 检查边界
			isBoundary := charAfter == "" ||
				tokenBoundaryRegex.MatchString(charAfter) ||
				!handleContinuationRegex.MatchString(charAfter)

			if !isBoundary {
				continue
			}

			// 获取匹配的 AgentIDs（支持多 Agent）
			agentIDs := entry.AgentIDs
			if len(agentIDs) == 0 && entry.AgentID != "" {
				// 兼容旧数据
				agentIDs = []string{entry.AgentID}
			}

			// 过滤自调用
			filtered := make([]string, 0)
			for _, agentID := range agentIDs {
				if agentID != currentAgentID {
					filtered = append(filtered, agentID)
				}
			}

			// 添加结果
			if len(filtered) > 0 && !seenPatterns[patternLower] {
				seenPatterns[patternLower] = true
				found = append(found, filtered)
			}

			break // 每行最多匹配一个 pattern
		}
	}

	return found
}

// countLeadingWhitespace 计算行首空白字符数
func countLeadingWhitespace(line string) int {
	count := 0
	for _, r := range line {
		if r == ' ' || r == '\t' {
			count++
		} else {
			break
		}
	}
	return count
}

// DetectUserMention 检测是否 @用户
func DetectUserMention(text string) bool {
	lower := strings.ToLower(text)

	patterns := []string{
		"@co-creator",
		"@铲屎官",
		"@用户",
		"@user",
	}

	for _, p := range patterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}

	return false
}

// StripCodeBlocks 剥离代码块
func StripCodeBlocks(text string) string {
	return codeBlockRegex.ReplaceAllString(text, "")
}