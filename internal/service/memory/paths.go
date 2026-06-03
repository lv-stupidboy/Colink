package memory

import (
	"os"
	"path/filepath"
)

const (
	memoryIndexFile      = "MEMORY.md"
	projectMemoryRelPath = ".colink/project-memory/MEMORY.md"
	teamMemoryRelDir     = ".colink/team-memory"
)

func ProjectMemoryPath(workspacePath string) string {
	if workspacePath == "" {
		return ""
	}
	return filepath.Join(workspacePath, projectMemoryRelPath)
}

func DefaultTeamMemoryRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", teamMemoryRelDir)
	}
	return filepath.Join(home, teamMemoryRelDir)
}

func DefaultTeamMemoryPath() string {
	return filepath.Join(DefaultTeamMemoryRoot(), memoryIndexFile)
}

func TeamMemoryPath(teamRoot, teamID string) string {
	if teamID == "" {
		return ""
	}
	if teamRoot == "" {
		teamRoot = DefaultTeamMemoryRoot()
	}
	if filepath.Base(teamRoot) == memoryIndexFile {
		teamRoot = filepath.Dir(teamRoot)
	}
	return filepath.Join(teamRoot, teamID, memoryIndexFile)
}

func memoryPathForType(memoryType MemoryType, scope MemoryScopeIdentity, teamMemoryRoot string) string {
	workspacePath := scope.WorkspacePath
	switch memoryType {
	case MemoryTypeTeam:
		return TeamMemoryPath(teamMemoryRoot, scope.TeamID)
	case MemoryTypeProject:
		return ProjectMemoryPath(workspacePath)
	default:
		return ""
	}
}
