package memory

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

var (
	sensitivePattern       = regexp.MustCompile(`(?i)(api[_-]?key|token|password|passwd|secret|authorization|bearer)\s*[:=]\s*["']?[^\s"']+`)
	secretLikePattern      = regexp.MustCompile(`(?i)\b(sk-[a-z0-9_-]{12,}|xox[baprs]-[a-z0-9-]{10,}|gh[pousr]_[a-z0-9_]{20,})\b`)
	portUnavailablePattern = regexp.MustCompile(`(?i)(MEM_TEST_PORT_\d+|\d{2,5}).*(不可用|不能用|不要用|必须避开|禁用|unavailable|avoid|cannot)`)
)

func (m *MemoryManager) AddMemoryCandidate(input AddMemoryCandidateInput) (*AddMemoryCandidateResult, error) {
	content := strings.TrimSpace(ScrubMemoryContext(input.Content))
	if content == "" && len(input.Draft.Facts) == 0 {
		return &AddMemoryCandidateResult{Written: false, Reason: "content is empty"}, nil
	}
	if containsSensitive(content) {
		return &AddMemoryCandidateResult{Written: false, Reason: "content contains sensitive credential-like text"}, nil
	}

	memoryText, usageText, topic, usedDraft, ok := normalizeMemoryDraft(input.Draft)
	roleMemory, isRoleMemory := extractRoleMemory(content)
	if !usedDraft {
		if isRoleMemory {
			memoryText = roleMemory.memory
			ok = true
		} else {
			memoryText, ok = compactReusableMemory(content)
			if !ok && shouldAcceptManualMemory(input, content) {
				memoryText = trimRunes(content, 300)
				ok = true
			}
		}
		if !ok {
			return &AddMemoryCandidateResult{Written: false, Reason: "no reusable stable memory candidate found"}, nil
		}
	}

	memoryType := input.Type
	if isRoleMemory {
		memoryType = MemoryTypeTeam
	}
	if memoryType == "" {
		memoryType = classifyMemoryType(memoryText)
	}
	if memoryType == "" {
		return &AddMemoryCandidateResult{Written: false, Reason: "candidate type is uncertain"}, nil
	}
	scope := normalizeMemoryScope(input.Scope, input.WorkspacePath)
	if memoryType == MemoryTypeProject && strings.TrimSpace(scope.WorkspacePath) == "" {
		return &AddMemoryCandidateResult{Written: false, Type: memoryType, Reason: "workspacePath is required for project memory"}, nil
	}
	if memoryType == MemoryTypeTeam && strings.TrimSpace(scope.TeamID) == "" {
		return &AddMemoryCandidateResult{Written: false, Type: memoryType, Reason: "team identity is required for team memory"}, nil
	}

	now := time.Now()
	entry := MemoryEntry{
		ID:         buildMemoryID(memoryText, input.Tags),
		Type:       memoryType,
		Source:     normalizeSource(input.Source),
		Confidence: confidenceForSource(input.Source, usedDraft),
		Status:     MemoryStatusActive,
		Tags:       mergeStrings(input.Tags, inferTags(memoryText)),
		Topic:      topic,
		Created:    now,
		Updated:    now,
		Memory:     memoryText,
		Usage:      buildUsage(memoryType, memoryText),
	}
	if usedDraft {
		entry.Usage = usageText
	}
	if isRoleMemory {
		entry.ID = roleMemoryID(roleMemory.role)
		entry.Topic = "agent_roles"
		entry.Tags = mergeStrings(entry.Tags, []string{"agent-role"})
		entry.Usage = "调度或交接任务前，检查该角色的职责、上游和下游关系。"
	}

	targetFile := m.pathForType(memoryType, scope)
	result, err := m.addEntry(scope, targetFile, titleForType(memoryType), entry)
	if result != nil {
		result.Type = memoryType
		result.TargetFile = targetFile
	}
	return result, err
}

func containsSensitive(content string) bool {
	return sensitivePattern.MatchString(content) || secretLikePattern.MatchString(content)
}

func shouldAcceptManualMemory(input AddMemoryCandidateInput, content string) bool {
	if normalizeSource(input.Source) != MemorySourceManual {
		return false
	}
	if input.Type != MemoryTypeTeam && input.Type != MemoryTypeProject {
		return false
	}
	if isConversationNoise(content) || containsSensitive(content) {
		return false
	}
	runeLen := len([]rune(strings.TrimSpace(content)))
	return runeLen >= 6 && runeLen <= 300
}

func classifyMemoryType(content string) MemoryType {
	if isUserTitlePreference(content) {
		return MemoryTypeTeam
	}
	projectScore := countSignals(content, []string{"项目", "workspace", "repo", "端口", "port", "pnpm", "npm", "yarn", "make ", "go test", "测试", "构建", "启动", "命令", "数据库", "mysql", "sqlite", "目录", "路径"})
	teamScore := countSignals(content, []string{"agent", "团队", "工程师", "架构师", "代码工程师", "测试工程师", "职责", "角色", "上游", "下游", "协作", "交接", "分工"})
	if projectScore == 0 && teamScore == 0 {
		return ""
	}
	if projectScore > teamScore {
		return MemoryTypeProject
	}
	return MemoryTypeTeam
}

func countSignals(content string, signals []string) int {
	count := 0
	for _, signal := range signals {
		if containsAnyFold(content, []string{signal}) {
			count++
		}
	}
	return count
}

func buildMemoryID(content string, tags []string) string {
	if isUserTitlePreference(content) {
		return "user-title-preference"
	}
	if m := portUnavailablePattern.FindStringSubmatch(content); len(m) >= 2 {
		return "port-" + normalizePortToken(m[1]) + "-unavailable"
	}
	if token := firstUppercaseMemoryToken(content); token != "" {
		prefix := ""
		if containsAnyFold(content, []string{"agent", "协作"}) {
			prefix = "agent-"
		}
		return prefix + strings.ToLower(strings.ReplaceAll(token, "_", "-"))
	}
	seed := content
	if len(tags) > 0 {
		seed = strings.Join(tags, "-") + "-" + content
	}
	slug := slugify(seed)
	if slug == "" {
		slug = "memory-" + shortHash(content)
	}
	if len(slug) > 48 {
		slug = strings.Trim(slug[:48], "-")
	}
	return slug
}

func slugify(value string) string {
	var sb strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			lastDash = false
			continue
		}
		if unicode.IsSpace(r) || r == '-' || r == '_' || r == '.' || r == '/' {
			if !lastDash && sb.Len() > 0 {
				sb.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(sb.String(), "-")
}

func shortHash(value string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(value))
	return fmt.Sprintf("%08x", h.Sum32())
}

func inferTags(content string) []string {
	tagSignals := map[string][]string{
		"port":          {"端口", "port", "MEM_TEST_PORT"},
		"constraint":    {"不能", "不可", "不要", "avoid", "禁用"},
		"command":       {"命令", "go test", "npm", "pnpm", "make"},
		"test":          {"测试", "test"},
		"agent-role":    {"角色", "职责", "上游", "下游"},
		"collaboration": {"协作", "交接", "分工"},
	}
	var tags []string
	for tag, signals := range tagSignals {
		if containsAnyFold(content, signals) {
			tags = append(tags, tag)
		}
	}
	sort.Strings(tags)
	return tags
}

func buildUsage(memoryType MemoryType, content string) string {
	if isUserTitlePreference(content) {
		return "回答用户前先检查称呼偏好。"
	}
	if port := extractPortToken(content); port != "" {
		return "配置端口、启动服务或编写端口相关测试前，确认没有使用 " + port + "。"
	}
	if memoryType == MemoryTypeTeam && containsAnyFold(content, []string{"角色", "职责", "上游", "下游", "协作"}) {
		return "分配任务、触发下游 Agent 或解释团队分工前先检查。"
	}
	if memoryType == MemoryTypeProject && containsAnyFold(content, []string{"命令", "测试", "构建", "启动"}) {
		return "运行命令、编写测试或构建项目前先检查。"
	}
	return ""
}

func titleForType(memoryType MemoryType) string {
	if memoryType == MemoryTypeTeam {
		return "Colink Team Memory"
	}
	return "Colink Project Memory"
}

func normalizeSource(source MemorySource) MemorySource {
	switch source {
	case MemorySourceUserMessage, MemorySourceAgentObservation, MemorySourceCommandResult, MemorySourceManual:
		return source
	default:
		return MemorySourceManual
	}
}

func confidenceForSource(source MemorySource, usedDraft bool) MemoryConfidence {
	if usedDraft {
		return MemoryConfidenceHigh
	}
	switch normalizeSource(source) {
	case MemorySourceUserMessage, MemorySourceManual:
		return MemoryConfidenceHigh
	case MemorySourceCommandResult:
		return MemoryConfidenceMedium
	default:
		return MemoryConfidenceMedium
	}
}

func containsAnyFold(content string, needles []string) bool {
	lower := strings.ToLower(content)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func normalizeStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}
	return result
}

func mergeStrings(left, right []string) []string {
	return normalizeStrings(append(append([]string{}, left...), right...))
}

func containsString(values []string, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == target {
			return true
		}
	}
	return false
}
