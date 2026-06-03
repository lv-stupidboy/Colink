package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const memoryDateLayout = "2006-01-02"

var memoryIndexLinkPattern = regexp.MustCompile(`\[[^\]]+\]\(([^)]+\.md)\)`)

type MarkdownStore struct {
	mu sync.Mutex
}

func NewMarkdownStore() *MarkdownStore {
	return &MarkdownStore{}
}

func (s *MarkdownStore) Load(path string) ([]MemoryEntry, error) {
	if path == "" {
		return nil, fmt.Errorf("memory path is empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s.loadLegacySiblings(path)
		}
		return nil, err
	}
	raw := string(data)
	links := parseMemoryIndexLinks(raw)
	if len(links) == 0 {
		return parseMemoryMarkdown(raw), nil
	}
	var entries []MemoryEntry
	for _, link := range links {
		entryPath := filepath.Join(filepath.Dir(path), filepath.FromSlash(link))
		entryData, err := os.ReadFile(entryPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		entries = append(entries, parseMemoryMarkdown(string(entryData))...)
	}
	return entries, nil
}

func (s *MarkdownStore) Save(path, title string, entries []MemoryEntry, scope MemoryScopeIdentity) error {
	if path == "" {
		return fmt.Errorf("memory path is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	entries = prepareTopicEntries(entries)
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Status == entries[j].Status {
			return entries[i].Updated.After(entries[j].Updated)
		}
		return entries[i].Status == MemoryStatusActive
	})

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	activeFiles := make(map[string]bool)
	for _, entry := range entries {
		if strings.TrimSpace(entry.Memory) == "" {
			continue
		}
		filename := memoryEntryFilename(entry)
		activeFiles[filename] = true
		if err := os.WriteFile(filepath.Join(dir, filename), []byte(renderMemoryEntryMarkdown(entry)), 0644); err != nil {
			return err
		}
	}
	if err := removeStaleMemoryFiles(path, activeFiles); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(renderMemoryIndexMarkdown(entries, scope)), 0644)
}

func (s *MarkdownStore) Append(path, title string, entry MemoryEntry, scope MemoryScopeIdentity) error {
	entries, err := s.Load(path)
	if err != nil {
		return err
	}
	entries = append(entries, entry)
	return s.Save(path, title, entries, scope)
}

func (s *MarkdownStore) Replace(path, title string, entries []MemoryEntry, scope MemoryScopeIdentity) error {
	return s.Save(path, title, entries, scope)
}

func (s *MarkdownStore) loadLegacySiblings(path string) ([]MemoryEntry, error) {
	var entries []MemoryEntry
	legacyPaths := []string{
		filepath.Join(filepath.Dir(path), "project.md"),
		filepath.Join(filepath.Dir(path), "team.md"),
		filepath.Join(filepath.Dir(filepath.Dir(path)), "memory", "project.md"),
		filepath.Join(filepath.Dir(filepath.Dir(path)), "memory", "team.md"),
	}
	for _, legacyPath := range legacyPaths {
		data, err := os.ReadFile(legacyPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		entries = append(entries, parseMemoryMarkdown(string(data))...)
	}
	return entries, nil
}

func prepareTopicEntries(entries []MemoryEntry) []MemoryEntry {
	grouped := make(map[string]MemoryEntry)
	order := make([]string, 0, len(entries))
	for _, entry := range entries {
		entry = compactMemoryEntry(entry)
		if strings.TrimSpace(entry.Memory) == "" {
			continue
		}
		topic := deriveMemoryTopic(entry)
		entry.Topic = topic.Key
		entry.ID = topic.Key
		key := string(entry.Type) + ":" + topic.Key
		if existing, ok := grouped[key]; ok {
			grouped[key] = mergeTopicEntry(existing, entry)
			continue
		}
		if entry.Created.IsZero() {
			entry.Created = time.Now()
		}
		if entry.Updated.IsZero() {
			entry.Updated = entry.Created
		}
		grouped[key] = entry
		order = append(order, key)
	}
	result := make([]MemoryEntry, 0, len(order))
	for _, key := range order {
		result = append(result, grouped[key])
	}
	return result
}

func mergeTopicEntry(existing, next MemoryEntry) MemoryEntry {
	existing.Memory = mergeMemoryText(existing.Memory, next.Memory)
	existing.Usage = mergeMemoryText(existing.Usage, next.Usage)
	existing.Tags = mergeStrings(existing.Tags, next.Tags)
	if existing.Created.IsZero() || (!next.Created.IsZero() && next.Created.Before(existing.Created)) {
		existing.Created = next.Created
	}
	if next.Updated.After(existing.Updated) {
		existing.Updated = next.Updated
	}
	if existing.Status != MemoryStatusActive && next.Status == MemoryStatusActive {
		existing.Status = next.Status
	}
	if existing.Confidence != MemoryConfidenceHigh && next.Confidence == MemoryConfidenceHigh {
		existing.Confidence = next.Confidence
	}
	return existing
}

func mergeMemoryText(left, right string) string {
	var parts []string
	seen := make(map[string]bool)
	for _, value := range []string{left, right} {
		for _, part := range splitMemoryFacts(value) {
			key := normalizeForCompare(part)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n")
}

func splitMemoryFacts(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	var result []string
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func renderMemoryIndexMarkdown(entries []MemoryEntry, scope MemoryScopeIdentity) string {
	var sb strings.Builder
	sb.WriteString("# Memory Index\n\n")
	writeMemoryScope(&sb, scope)
	writeMemoryIndex(&sb, "Project Memory", entries, MemoryTypeProject)
	writeMemoryIndex(&sb, "Team Memory", entries, MemoryTypeTeam)
	return sb.String()
}

func writeMemoryScope(sb *strings.Builder, scope MemoryScopeIdentity) {
	scope = normalizeMemoryScope(scope, scope.WorkspacePath)
	if scope.TeamID == "" && scope.ProjectID == "" && scope.WorkspacePath == "" {
		return
	}
	sb.WriteString("## Scope\n\n")
	if scope.TeamID != "" {
		sb.WriteString("- Type: team\n")
		sb.WriteString("- Team ID: ")
		sb.WriteString(scope.TeamID)
		sb.WriteString("\n")
		if scope.TeamName != "" {
			sb.WriteString("- Team Name: ")
			sb.WriteString(scope.TeamName)
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("- Type: project\n")
		if scope.ProjectID != "" {
			sb.WriteString("- Project ID: ")
			sb.WriteString(scope.ProjectID)
			sb.WriteString("\n")
		}
		if scope.ProjectName != "" {
			sb.WriteString("- Project Name: ")
			sb.WriteString(scope.ProjectName)
			sb.WriteString("\n")
		}
		if scope.WorkspacePath != "" {
			sb.WriteString("- Workspace: ")
			sb.WriteString(scope.WorkspacePath)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")
}

func writeMemoryIndex(sb *strings.Builder, heading string, entries []MemoryEntry, memoryType MemoryType) {
	wroteHeading := false
	for _, entry := range entries {
		if entry.Type != memoryType {
			continue
		}
		if !wroteHeading {
			sb.WriteString("## ")
			sb.WriteString(heading)
			sb.WriteString("\n")
			wroteHeading = true
		}
		topic := deriveMemoryTopic(entry)
		sb.WriteString("- [")
		sb.WriteString(topic.Title)
		sb.WriteString("](")
		sb.WriteString(topic.Filename)
		sb.WriteString(") — ")
		sb.WriteString(topic.Summary)
		sb.WriteString("\n")
	}
	if wroteHeading {
		sb.WriteString("\n")
	}
}

func renderMemoryEntryMarkdown(entry MemoryEntry) string {
	topic := deriveMemoryTopic(entry)
	var sb strings.Builder
	sb.WriteString("---\n")
	writeFrontmatterField(&sb, "name", entry.ID)
	writeFrontmatterField(&sb, "description", memoryDescription(entry))
	sb.WriteString("metadata:\n")
	writeMetadataField(&sb, "node_type", "memory")
	writeMetadataField(&sb, "topic", topic.Key)
	writeMetadataField(&sb, "type", string(entry.Type))
	writeMetadataField(&sb, "source", string(entry.Source))
	writeMetadataField(&sb, "confidence", string(entry.Confidence))
	writeMetadataField(&sb, "status", string(entry.Status))
	writeMetadataField(&sb, "tags", strings.Join(entry.Tags, ", "))
	writeMetadataField(&sb, "created", formatMemoryDate(entry.Created))
	writeMetadataField(&sb, "updated", formatMemoryDate(entry.Updated))
	sb.WriteString("---\n\n")
	sb.WriteString("# ")
	sb.WriteString(topic.Title)
	sb.WriteString("\n\n## Memory\n\n")
	for _, fact := range splitMemoryFacts(entry.Memory) {
		sb.WriteString("- ")
		sb.WriteString(fact)
		sb.WriteString("\n")
	}
	if strings.TrimSpace(entry.Usage) != "" {
		sb.WriteString("\n## How to apply\n\n")
		for _, usage := range splitMemoryFacts(entry.Usage) {
			sb.WriteString("- ")
			sb.WriteString(usage)
			sb.WriteString("\n")
		}
	}
	if related := memoryRelated(entry); related != "" {
		sb.WriteString("\n**Related**: ")
		sb.WriteString(related)
		sb.WriteString("\n")
	}
	return sb.String()
}

func parseMemoryIndexLinks(raw string) []string {
	matches := memoryIndexLinkPattern.FindAllStringSubmatch(raw, -1)
	links := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		link := filepath.ToSlash(strings.TrimSpace(match[1]))
		if link == "" || filepath.IsAbs(link) || strings.Contains(link, "..") || strings.EqualFold(link, memoryIndexFile) {
			continue
		}
		if !seen[link] {
			seen[link] = true
			links = append(links, link)
		}
	}
	return links
}

func memoryEntryFilename(entry MemoryEntry) string {
	if topic := deriveMemoryTopic(entry); topic.Filename != "" {
		return topic.Filename
	}
	if slug := semanticMemorySlug(entry); slug != "" {
		return slug + ".md"
	}
	slug := slugForFilename(memoryTitle(entry))
	if slug == "" {
		slug = "general_memory"
	}
	return slug + ".md"
}

func slugForFilename(value string) string {
	var sb strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if r == '-' || r == '_' || unicodeIsFileSeparator(r) {
			if !lastUnderscore && sb.Len() > 0 {
				sb.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(sb.String(), "_")
}

func unicodeIsFileSeparator(r rune) bool {
	return r == ' ' || r == '\t' || r == '.' || r == '/'
}

func removeStaleMemoryFiles(indexPath string, activeFiles map[string]bool) error {
	dir := filepath.Dir(indexPath)
	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return err
	}
	for _, file := range files {
		name := filepath.Base(file)
		if strings.EqualFold(name, memoryIndexFile) || activeFiles[name] || name == "project.md" || name == "team.md" {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), "node_type: memory") {
			if err := os.Remove(file); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeFrontmatterField(sb *strings.Builder, key, value string) {
	sb.WriteString(key)
	sb.WriteString(": ")
	sb.WriteString(formatFrontmatterValue(value))
	sb.WriteString("\n")
}

func writeMetadataField(sb *strings.Builder, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	sb.WriteString("  ")
	sb.WriteString(key)
	sb.WriteString(": ")
	sb.WriteString(formatFrontmatterValue(value))
	sb.WriteString("\n")
}

func formatFrontmatterValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, "\":") || strings.HasPrefix(value, "[") || strings.HasPrefix(value, "{") {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return value
}

func memoryTitle(entry MemoryEntry) string {
	if isUserTitlePreference(entry.Memory) || entry.ID == "user-title-preference" {
		return "用户偏好"
	}
	if extractPortToken(entry.Memory) != "" || strings.Contains(entry.ID, "-unavailable") {
		return "端口约束"
	}
	if tag := firstNonEmpty(entry.Tags); tag != "" {
		return humanizeID(tag)
	}
	if entry.ID != "" {
		return humanizeID(entry.ID)
	}
	return firstSentence(entry.Memory, 24)
}

func memoryDescription(entry MemoryEntry) string {
	title := memoryTitle(entry)
	summary := memorySummary(entry)
	if summary == "" || summary == title {
		return title
	}
	return title + " - " + summary
}

func memorySummary(entry MemoryEntry) string {
	return conciseMemorySummary(entry)
}

func memoryRelated(entry MemoryEntry) string {
	var items []string
	seen := make(map[string]bool)
	for _, value := range entry.Tags {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		items = append(items, "[["+value+"]]")
	}
	return strings.Join(items, ", ")
}

func firstNonEmpty(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func humanizeID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "mem-")
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	return strings.Join(strings.Fields(value), " ")
}

func firstSentence(value string, maxRunes int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if value == "" {
		return ""
	}
	for _, sep := range []string{"。", "！", "？", ". ", "! ", "? "} {
		if idx := strings.Index(value, sep); idx >= 0 {
			value = strings.TrimSpace(value[:idx+len(strings.TrimRight(sep, " "))])
			break
		}
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "..."
}

func parseMemoryMarkdown(raw string) []MemoryEntry {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if strings.Contains(normalized, "\n---\n") || strings.HasPrefix(normalized, "---\n") {
		if entries := parseFrontmatterMemoryMarkdown(normalized); len(entries) > 0 {
			return entries
		}
	}
	return parseLegacyMemoryMarkdown(normalized)
}

func parseFrontmatterMemoryMarkdown(raw string) []MemoryEntry {
	lines := strings.Split(raw, "\n")
	var entries []MemoryEntry
	for i := 0; i < len(lines); {
		if strings.TrimSpace(lines[i]) != "---" {
			i++
			continue
		}
		frontmatterStart := i + 1
		i++
		for i < len(lines) && strings.TrimSpace(lines[i]) != "---" {
			i++
		}
		if i >= len(lines) {
			break
		}
		frontmatter := lines[frontmatterStart:i]
		bodyStart := i + 1
		i++
		for i < len(lines) && strings.TrimSpace(lines[i]) != "---" {
			i++
		}
		if entry, ok := parseFrontmatterMemoryBlock(frontmatter, lines[bodyStart:i]); ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func parseFrontmatterMemoryBlock(frontmatter, body []string) (MemoryEntry, bool) {
	entry := MemoryEntry{Status: MemoryStatusActive, Confidence: MemoryConfidenceMedium}
	inMetadata := false
	for _, rawLine := range frontmatter {
		line := strings.TrimRight(rawLine, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "metadata:" {
			inMetadata = true
			continue
		}
		if inMetadata {
			trimmed = strings.TrimSpace(trimmed)
		}
		key, value, ok := splitFrontmatterField(trimmed)
		if !ok {
			continue
		}
		switch key {
		case "name":
			entry.ID = value
		case "type":
			entry.Type = MemoryType(value)
		case "source":
			entry.Source = MemorySource(value)
		case "confidence":
			entry.Confidence = MemoryConfidence(value)
		case "status":
			entry.Status = MemoryStatus(value)
		case "tags":
			entry.Tags = splitCSV(value)
		case "topic":
			entry.Topic = value
		case "created":
			entry.Created = parseMemoryDate(value)
		case "updated":
			entry.Updated = parseMemoryDate(value)
		}
	}
	if entry.ID == "" {
		return MemoryEntry{}, false
	}
	var memoryLines, usageLines []string
	section := "memory"
	for _, rawLine := range body {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "# ") || strings.HasPrefix(strings.ToLower(line), "**related**:") {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			heading := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			switch heading {
			case "memory", "memories", "记忆条目":
				section = "memory"
			case "how to apply", "usage", "如何应用":
				section = "usage"
			}
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if value == "" {
			continue
		}
		if section == "usage" {
			usageLines = append(usageLines, value)
		} else {
			memoryLines = append(memoryLines, value)
		}
	}
	entry.Memory = strings.TrimSpace(strings.Join(memoryLines, "\n"))
	entry.Usage = strings.TrimSpace(strings.Join(usageLines, "\n"))
	if entry.Created.IsZero() {
		entry.Created = time.Now()
	}
	if entry.Updated.IsZero() {
		entry.Updated = entry.Created
	}
	return entry, entry.Memory != ""
}

func splitFrontmatterField(line string) (string, string, bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+1:])
	value = strings.Trim(value, `"`)
	value = strings.ReplaceAll(value, `\"`, `"`)
	return key, value, true
}

func parseLegacyMemoryMarkdown(raw string) []MemoryEntry {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var entries []MemoryEntry
	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			i++
			continue
		}
		start := i
		i++
		for i < len(lines) {
			next := strings.TrimSpace(lines[i])
			if strings.HasPrefix(next, "## ") && !strings.HasPrefix(next, "### ") {
				break
			}
			i++
		}
		if entry, ok := parseMemoryBlock(lines[start:i]); ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func parseMemoryBlock(lines []string) (MemoryEntry, bool) {
	if len(lines) == 0 {
		return MemoryEntry{}, false
	}
	entry := MemoryEntry{
		ID:         strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[0]), "## ")),
		Status:     MemoryStatusActive,
		Confidence: MemoryConfidenceMedium,
	}
	section := ""
	var memoryLines, usageLines []string
	for _, rawLine := range lines[1:] {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "- ") && section == "" {
			key, value, ok := splitField(line)
			if !ok {
				continue
			}
			switch strings.ToLower(key) {
			case "type":
				entry.Type = MemoryType(value)
			case "source":
				entry.Source = MemorySource(value)
			case "confidence":
				entry.Confidence = MemoryConfidence(value)
			case "status":
				entry.Status = MemoryStatus(value)
			case "tags":
				entry.Tags = splitCSV(value)
			case "created":
				entry.Created = parseMemoryDate(value)
			case "updated":
				entry.Updated = parseMemoryDate(value)
			}
			continue
		}
		if strings.HasPrefix(line, "### ") {
			section = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "### ")))
			continue
		}
		switch section {
		case "memory":
			memoryLines = append(memoryLines, rawLine)
		case "usage":
			usageLines = append(usageLines, rawLine)
		}
	}
	entry.Memory = strings.TrimSpace(strings.Join(memoryLines, "\n"))
	entry.Usage = strings.TrimSpace(strings.Join(usageLines, "\n"))
	if entry.Created.IsZero() {
		entry.Created = time.Now()
	}
	if entry.Updated.IsZero() {
		entry.Updated = entry.Created
	}
	return entry, entry.ID != "" && entry.Memory != ""
}

func splitField(line string) (string, string, bool) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var result []string
	seen := make(map[string]bool)
	for _, part := range strings.Split(value, ",") {
		item := strings.TrimSpace(part)
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

func formatMemoryDate(t time.Time) string {
	if t.IsZero() {
		return time.Now().Format(memoryDateLayout)
	}
	return t.Format(memoryDateLayout)
}

func parseMemoryDate(value string) time.Time {
	t, err := time.ParseInLocation(memoryDateLayout, strings.TrimSpace(value), time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}
