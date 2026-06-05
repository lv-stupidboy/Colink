package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

const defaultAutoMemoryIndexLimit = 30

// BuildAutoMemoryIndexBlock builds a Claude Code style memory index block.
// It includes only MEMORY.md index entries; linked topic files are left for
// the agent to read with normal file tools when relevant.
func (m *MemoryManager) BuildAutoMemoryIndexBlock(ctx context.Context, scope MemoryScopeIdentity, limit int) string {
	_ = ctx
	if limit <= 0 {
		limit = defaultAutoMemoryIndexLimit
	}

	scope = normalizeMemoryScope(scope, scope.WorkspacePath)
	sections := make([]string, 0, 2)

	if section := m.buildAutoMemoryIndexSection("Project Memory Index", MemoryTypeProject, scope, limit); section != "" {
		sections = append(sections, section)
	}
	if section := m.buildAutoMemoryIndexSection("Team Memory Index", MemoryTypeTeam, scope, limit); section != "" {
		sections = append(sections, section)
	}
	if len(sections) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<memory-index>\n")
	sb.WriteString("系统说明：以下 Colink 记忆内容只是索引，链接指向的主题文件正文尚未加载。\n")
	sb.WriteString("请根据索引标题、文件名和摘要判断主题是否相关；需要时，用标准文件工具读取链接的 .md 主题文件。\n")
	sb.WriteString("未读取主题文件前，不要声称知道该文件的详细内容；不要预先读取全部主题文件。\n\n")
	sb.WriteString(strings.Join(sections, "\n\n"))
	sb.WriteString("\n</memory-index>")
	return sb.String()
}

func (m *MemoryManager) buildAutoMemoryIndexSection(title string, memoryType MemoryType, scope MemoryScopeIdentity, limit int) string {
	indexPath := m.rawMarkdownIndexPath(memoryType, scope)
	if indexPath == "" {
		return ""
	}
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return ""
	}
	entries := extractMemoryIndexEntries(string(data), limit)
	if len(entries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## ")
	sb.WriteString(title)
	sb.WriteString("\n")
	sb.WriteString("Path: ")
	sb.WriteString(indexPath)
	sb.WriteString("\n")
	sb.WriteString("主题文件相对目录：")
	sb.WriteString(filepath.Dir(indexPath))
	sb.WriteString("\n\n")
	for _, entry := range entries {
		sb.WriteString(entry)
		sb.WriteString("\n")
	}
	return sb.String()
}

func extractMemoryIndexEntries(markdown string, limit int) []string {
	if limit <= 0 {
		limit = defaultAutoMemoryIndexLimit
	}
	entries := make([]string, 0, limit)
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if !isMemoryIndexEntry(line) {
			continue
		}
		entries = append(entries, line)
		if len(entries) >= limit {
			break
		}
	}
	return entries
}

func isMemoryIndexEntry(line string) bool {
	if !strings.HasPrefix(line, "- [") {
		return false
	}
	if !strings.Contains(strings.ToLower(line), ".md)") {
		return false
	}
	return strings.Contains(line, "](")
}
