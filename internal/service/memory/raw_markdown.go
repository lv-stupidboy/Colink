package memory

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type RawMarkdownFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

type RawMarkdownGroup struct {
	Type        MemoryType          `json:"type"`
	IndexPath   string              `json:"indexPath"`
	IndexExists bool                `json:"indexExists"`
	Index       string              `json:"index"`
	Files       []RawMarkdownFile   `json:"files"`
	Missing     []string            `json:"missing"`
	Scope       MemoryScopeIdentity `json:"scope"`
}

func (m *MemoryManager) ReadRawMarkdown(memoryType MemoryType, scope MemoryScopeIdentity) (RawMarkdownGroup, error) {
	scope = normalizeMemoryScope(scope, scope.WorkspacePath)
	group := RawMarkdownGroup{
		Type:  memoryType,
		Files: make([]RawMarkdownFile, 0),
		Scope: scopeForType(scope, memoryType),
	}

	indexPath := m.rawMarkdownIndexPath(memoryType, scope)
	group.IndexPath = indexPath
	if indexPath == "" {
		return group, nil
	}

	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return group, nil
		}
		return group, err
	}
	group.IndexExists = true
	group.Index = string(indexData)

	links := parseMemoryIndexLinks(group.Index)
	if len(links) == 0 {
		return m.appendRawMarkdownSiblings(group, nil)
	}

	baseDir := filepath.Dir(indexPath)
	seen := make(map[string]bool)
	for _, link := range links {
		normalized := filepath.ToSlash(strings.TrimSpace(link))
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		filePath := filepath.Join(baseDir, filepath.FromSlash(normalized))
		file, err := rawMarkdownFileFromPath(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				group.Missing = append(group.Missing, normalized)
				continue
			}
			return group, err
		}
		group.Files = append(group.Files, file)
	}

	return m.appendRawMarkdownSiblings(group, seen)
}

func rawMarkdownFileFromPath(path string) (RawMarkdownFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RawMarkdownFile{}, err
	}
	return RawMarkdownFile{
		Name:    filepath.Base(path),
		Path:    path,
		Content: string(data),
	}, nil
}

func (m *MemoryManager) appendRawMarkdownSiblings(group RawMarkdownGroup, seen map[string]bool) (RawMarkdownGroup, error) {
	if group.IndexPath == "" {
		return group, nil
	}
	if seen == nil {
		seen = make(map[string]bool)
	}
	for _, file := range group.Files {
		seen[filepath.ToSlash(file.Name)] = true
	}

	pattern := filepath.Join(filepath.Dir(group.IndexPath), "*.md")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return group, err
	}
	for _, path := range paths {
		name := filepath.Base(path)
		if strings.EqualFold(name, memoryIndexFile) || seen[filepath.ToSlash(name)] {
			continue
		}
		file, err := rawMarkdownFileFromPath(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return group, err
		}
		group.Files = append(group.Files, file)
		seen[filepath.ToSlash(name)] = true
	}
	return group, nil
}

func (m *MemoryManager) rawMarkdownIndexPath(memoryType MemoryType, scope MemoryScopeIdentity) string {
	indexPath := m.pathForType(memoryType, scope)
	if memoryType != MemoryTypeTeam || indexPath == "" || fileExists(indexPath) {
		return indexPath
	}
	if runtime.GOOS != "windows" || strings.TrimSpace(scope.TeamID) == "" {
		return indexPath
	}

	matches, err := filepath.Glob(filepath.Join(`C:\Users`, `*`, teamMemoryRelDir, scope.TeamID, memoryIndexFile))
	if err != nil || len(matches) == 0 {
		return indexPath
	}
	return matches[0]
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
