package memory

import (
	"fmt"
	"strings"
)

func normalizeMemoryDraft(draft MemoryDraft) (memoryText, usageText, topic, summary string, used bool, ok bool) {
	facts := normalizeDraftLines(draft.Facts, 12, 260)
	usage := normalizeDraftLines(draft.Usage, 8, 220)
	if len(facts) == 0 {
		return "", "", "", "", false, false
	}
	for _, fact := range append(append([]string{}, facts...), usage...) {
		if containsSensitive(fact) {
			return "", "", "", "", true, false
		}
	}
	topic = strings.TrimSpace(draft.Topic)
	if topic != "" {
		topic = slugForFilename(topic)
	}
	summary = normalizeSummary(draft.Summary)
	return strings.Join(facts, "\n"), strings.Join(usage, "\n"), topic, summary, true, true
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

func normalizeSummary(value string) string {
	value = strings.TrimSpace(ScrubMemoryContext(value))
	if value == "" || isConversationNoise(value) || containsSensitive(value) {
		return ""
	}
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return trimRunes(value, 120)
}

func compactMemoryEntry(entry MemoryEntry) MemoryEntry {
	entry.Summary = normalizeSummary(entry.Summary)
	if isConversationNoise(entry.Memory) {
		entry.Memory = ""
	}
	if isGenericUsage(entry.Usage) {
		entry.Usage = ""
	}
	return entry
}

func conciseMemorySummary(entry MemoryEntry) string {
	if summary := normalizeSummary(entry.Summary); summary != "" {
		return summary
	}
	return fallbackMemorySummary(entry)
}

func fallbackMemorySummary(entry MemoryEntry) string {
	facts := splitMemoryFacts(entry.Memory)
	count := len(facts)
	if count == 0 {
		return "记录该主题的长期记忆"
	}
	subject := topicTitleFromKey(strings.TrimSpace(entry.Topic))
	if subject == "" {
		subject = memoryTitle(entry)
	}
	if count <= 1 {
		return "记录1条" + subject
	}
	return "记录" + fmt.Sprintf("%d", count) + "条" + subject
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
	if max <= 0 || len(runes) <= max {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:max]))
}
