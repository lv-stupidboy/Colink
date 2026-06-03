package memory

import (
	"regexp"
	"strings"
)

var agentMentionPattern = regexp.MustCompile(`@([^\s` + "`" + `，。！？；：、]+)`)

type roleMemoryCandidate struct {
	role   string
	memory string
}

func extractRoleMemory(content string) (roleMemoryCandidate, bool) {
	normalized := strings.TrimSpace(ScrubMemoryContext(content))
	if normalized == "" {
		return roleMemoryCandidate{}, false
	}
	if !containsAnyFold(normalized, []string{"我是", "我的职责", "职责是", "上游", "下游", "触发下游", "交接给"}) {
		return roleMemoryCandidate{}, false
	}

	role := extractRoleName(normalized)
	responsibilities := extractResponsibilities(normalized)
	if role == "" || len(responsibilities) == 0 {
		return roleMemoryCandidate{}, false
	}
	upstream := extractMentionNear(normalized, []string{"上游", "来自", "由"})
	downstream := extractMentionNear(normalized, []string{"下游", "交接给", "触发下游", "通过 @"})

	var lines []string
	lines = append(lines, role+"：负责"+strings.Join(responsibilities, "、")+"。")
	if len(upstream) > 0 {
		lines = append(lines, "上游："+strings.Join(upstream, "、")+"。")
	} else {
		lines = append(lines, "上游：未配置。")
	}
	if len(downstream) > 0 {
		lines = append(lines, "下游："+strings.Join(downstream, "、")+"。")
	} else {
		lines = append(lines, "下游：未配置。")
	}

	return roleMemoryCandidate{
		role:   role,
		memory: strings.Join(lines, "\n"),
	}, true
}

func extractRoleName(content string) string {
	for _, marker := range []string{"我是**", "我是"} {
		idx := strings.Index(content, marker)
		if idx < 0 {
			continue
		}
		value := content[idx+len(marker):]
		value = strings.TrimLeft(value, "* ：:")
		return cleanRoleName(readUntilAny(value, []string{"**", "。", "\n", "，", ","}))
	}
	return ""
}

func cleanRoleName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "* ：:。")
	return value
}

func extractResponsibilities(content string) []string {
	lines := strings.Split(content, "\n")
	var result []string
	inResponsibilities := false
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if strings.Contains(line, "职责") {
			inResponsibilities = true
			continue
		}
		if inResponsibilities && containsAnyFold(line, []string{"完成", "交接给", "触发下游", "下游", "上游", "无需下游"}) {
			break
		}
		if !inResponsibilities {
			continue
		}
		line = strings.TrimLeft(line, "-*•0123456789.、 ")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		result = append(result, strings.TrimRight(line, "。"))
		if len(result) >= 6 {
			break
		}
	}
	return normalizeStrings(result)
}

func extractMentionNear(content string, markers []string) []string {
	var result []string
	for _, marker := range markers {
		idx := strings.Index(content, marker)
		if idx < 0 {
			continue
		}
		runes := []rune(content)
		start := len([]rune(content[:idx])) - 80
		if start < 0 {
			start = 0
		}
		end := len([]rune(content[:idx])) + 140
		if end > len(runes) {
			end = len(runes)
		}
		window := string(runes[start:end])
		for _, match := range agentMentionPattern.FindAllStringSubmatch(window, -1) {
			if len(match) >= 2 {
				result = append(result, strings.TrimSpace(match[1]))
			}
		}
	}
	return normalizeStrings(result)
}

func readUntilAny(value string, stops []string) string {
	end := len(value)
	for _, stop := range stops {
		if idx := strings.Index(value, stop); idx >= 0 && idx < end {
			end = idx
		}
	}
	return value[:end]
}

func roleMemoryID(role string) string {
	if containsAnyFold(role, []string{"架构设计师", "架构工程师", "architecture designer", "architect"}) {
		return "agent-role-architect"
	}
	if containsAnyFold(role, []string{"代码工程师", "开发工程师", "coder", "developer"}) {
		return "agent-role-coder"
	}
	slug := slugify(role)
	if slug == "" {
		slug = "agent-role-" + shortHash(role)
	}
	return "agent-role-" + slug
}
