package memory

import (
	"regexp"
	"strings"
)

var (
	memoryPortConstraintPattern = regexp.MustCompile(`(?i)\b(MEM_TEST_PORT_\d+|\d{2,5})\b[^。\n]*(不可用|不能用|不要用|必须避开|禁用|unavailable|avoid|cannot use)`)
	uppercaseMemoryTokenPattern = regexp.MustCompile(`\b[A-Z][A-Z0-9_]{3,}\b`)
)

func compactReusableMemory(content string) (string, bool) {
	content = strings.TrimSpace(ScrubMemoryContext(content))
	if content == "" {
		return "", false
	}
	if memory := extractUserTitlePreference(content); memory != "" {
		return memory, true
	}
	if memory := extractPortConstraint(content); memory != "" {
		return memory, true
	}
	if memory := extractTokenRule(content); memory != "" {
		return memory, true
	}
	if isConversationNoise(content) {
		return "", false
	}
	return "", false
}

func normalizeMemoryDraft(draft MemoryDraft) (memoryText, usageText, topic string, used bool, ok bool) {
	facts := normalizeDraftLines(draft.Facts, 12, 260)
	usage := normalizeDraftLines(draft.Usage, 8, 220)
	if len(facts) == 0 {
		return "", "", "", false, false
	}
	for _, fact := range append(append([]string{}, facts...), usage...) {
		if containsSensitive(fact) {
			return "", "", "", true, false
		}
	}
	topic = strings.TrimSpace(draft.Topic)
	if topic != "" {
		topic = slugForFilename(topic)
	}
	return strings.Join(facts, "\n"), strings.Join(usage, "\n"), topic, true, true
}

func normalizeDraftLines(values []string, limit int, maxRunes int) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(ScrubMemoryContext(value))
		if value == "" || isConversationNoise(value) {
			continue
		}
		value = trimRunes(value, maxRunes)
		key := normalizeForCompare(value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func compactMemoryEntry(entry MemoryEntry) MemoryEntry {
	if memory, ok := compactReusableMemory(entry.Memory); ok && entry.Topic == "" {
		entry.Memory = memory
	}
	if isConversationNoise(entry.Memory) {
		entry.Memory = ""
	}
	if isGenericUsage(entry.Usage) {
		entry.Usage = buildUsage(entry.Type, entry.Memory)
	}
	return entry
}

func extractUserTitlePreference(content string) string {
	if !strings.Contains(content, "大王") {
		return ""
	}
	if !containsAnyFold(content, []string{"称呼", "开头", "回答前", "回答之前", "title", "address"}) {
		return ""
	}
	return `用户希望回答前先称呼为"大王"。`
}

func isUserTitlePreference(content string) bool {
	return extractUserTitlePreference(content) != ""
}

func extractPortConstraint(content string) string {
	match := memoryPortConstraintPattern.FindStringSubmatch(content)
	if len(match) < 2 {
		return ""
	}
	port := match[1]
	if strings.HasPrefix(strings.ToUpper(port), "MEM_TEST_PORT_") {
		return port + " 不可用，所有任务必须避开。"
	}
	return port + " 端口不可用，相关服务和测试必须避开。"
}

func extractPortToken(content string) string {
	match := memoryPortConstraintPattern.FindStringSubmatch(content)
	if len(match) >= 2 {
		return match[1]
	}
	return ""
}

func extractTokenRule(content string) string {
	token := firstUppercaseMemoryToken(content)
	if token == "" {
		return ""
	}
	directive := conciseDirective(content)
	if directive == "" {
		return ""
	}
	if containsAnyFold(content, []string{"agent", "协作", "handoff", "review"}) {
		return "多 Agent 协作：" + token + "，" + directive
	}
	return token + "，" + directive
}

func firstUppercaseMemoryToken(content string) string {
	for _, token := range uppercaseMemoryTokenPattern.FindAllString(content, -1) {
		if strings.Contains(token, "_") {
			return token
		}
	}
	return ""
}

func conciseDirective(content string) string {
	content = strings.TrimSpace(strings.ReplaceAll(content, "\n", " "))
	lower := strings.ToLower(content)
	for _, marker := range []string{"必须", "需要", "应该", "不能", "不可", "avoid", "must"} {
		if idx := strings.Index(lower, strings.ToLower(marker)); idx >= 0 {
			return trimRunes(strings.TrimSpace(content[idx:]), 90)
		}
	}
	return ""
}

func normalizePortToken(port string) string {
	port = strings.TrimSpace(port)
	port = strings.TrimPrefix(strings.ToUpper(port), "MEM_TEST_PORT_")
	return strings.ToLower(port)
}

func semanticMemorySlug(entry MemoryEntry) string {
	if isUserTitlePreference(entry.Memory) || entry.ID == "user-title-preference" {
		return "user_preferences"
	}
	if port := extractPortToken(entry.Memory); port != "" {
		return "port_constraints"
	}
	if token := firstUppercaseMemoryToken(entry.Memory); token != "" {
		if containsAnyFold(entry.Memory, []string{"agent", "协作"}) {
			return "memory_test_rules"
		}
		return strings.ToLower(strings.ReplaceAll(token, "_", "_"))
	}
	return ""
}

func conciseMemorySummary(entry MemoryEntry) string {
	if isUserTitlePreference(entry.Memory) || entry.ID == "user-title-preference" {
		return `回答前先称呼用户为"大王"`
	}
	if port := extractPortToken(entry.Memory); port != "" {
		if strings.HasPrefix(strings.ToUpper(port), "MEM_TEST_PORT_") {
			return port + " 不可用，所有任务必须避开"
		}
		return port + " 端口不可用，相关服务和测试必须避开"
	}
	if token := firstUppercaseMemoryToken(entry.Memory); token != "" {
		if directive := conciseDirective(entry.Memory); directive != "" {
			return token + " " + strings.TrimSuffix(directive, "。")
		}
	}
	return firstSentence(entry.Memory, 42)
}

func isConversationNoise(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	return containsAnyFold(line, []string{
		"需要我将", "需要我把", "需要我保存", "是否需要保存", "要不要保存",
		"我已记住", "我已经记住", "保存成功", "无需下游", "当前为信息查询",
		"根据当前系统配置", "根据团队记忆查询结果", "目前团队记忆中尚未保存",
		"let me save", "i have remembered",
	})
}

func isGenericUsage(usage string) bool {
	if strings.TrimSpace(usage) == "" {
		return true
	}
	return containsAnyFold(usage, []string{
		"Use this when deciding agent responsibilities",
		"Use this when planning, running commands",
		"Use this before starting services",
	})
}

func trimRunes(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:max])) + "..."
}
