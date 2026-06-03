package memory_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropic/isdp/internal/service/memory"
)

func testScope(teamID, teamName, workspace string) memory.MemoryScopeIdentity {
	return memory.MemoryScopeIdentity{
		TeamID:        teamID,
		TeamName:      teamName,
		ProjectID:     "project-1",
		ProjectName:   "Colink",
		WorkspacePath: workspace,
	}
}

func TestTeamMemory_IsolatedByTeamIDAndIndexContainsScope(t *testing.T) {
	tempDir := t.TempDir()
	teamRoot := filepath.Join(tempDir, "team-memory")
	manager := memory.NewMemoryManagerWithTeamPath(nil, teamRoot)
	workspace := filepath.Join(tempDir, "workspace")

	teamA := testScope("team-a", "架构团队", workspace)
	teamB := testScope("team-b", "测试团队", workspace)

	result, err := manager.AddMemoryCandidate(memory.AddMemoryCandidateInput{
		Content: "团队采用多智能体协同工作模式，各 Agent 分工协作完成任务。",
		Source:  memory.MemorySourceManual,
		Type:    memory.MemoryTypeTeam,
		Scope:   teamA,
	})
	if err != nil {
		t.Fatalf("AddMemoryCandidate() error = %v", err)
	}
	if !result.Written {
		t.Fatalf("expected team memory write, got %+v", result)
	}

	teamAIndex := filepath.Join(teamRoot, "team-a", "MEMORY.md")
	indexData, err := os.ReadFile(teamAIndex)
	if err != nil {
		t.Fatalf("expected team A index: %v", err)
	}
	index := string(indexData)
	for _, want := range []string{"## Scope", "- Type: team", "- Team ID: team-a", "- Team Name: 架构团队", "## Team Memory"} {
		if !strings.Contains(index, want) {
			t.Fatalf("expected %q in team index, got:\n%s", want, index)
		}
	}

	resultsA, err := manager.SearchMemory(memory.SearchMemoryInput{
		Scope:           teamA,
		Type:            memory.MemoryTypeTeam,
		Query:           "多智能体",
		IncludeInactive: true,
	})
	if err != nil {
		t.Fatalf("SearchMemory(teamA) error = %v", err)
	}
	if len(resultsA) != 1 {
		t.Fatalf("expected team A memory, got %+v", resultsA)
	}

	resultsB, err := manager.SearchMemory(memory.SearchMemoryInput{
		Scope:           teamB,
		Type:            memory.MemoryTypeTeam,
		Query:           "多智能体",
		IncludeInactive: true,
	})
	if err != nil {
		t.Fatalf("SearchMemory(teamB) error = %v", err)
	}
	if len(resultsB) != 0 {
		t.Fatalf("expected team B to be isolated, got %+v", resultsB)
	}
}

func TestProjectMemory_UsesWorkspaceScope(t *testing.T) {
	tempDir := t.TempDir()
	manager := memory.NewMemoryManagerWithTeamPath(nil, filepath.Join(tempDir, "team-memory"))
	workspaceA := filepath.Join(tempDir, "workspace-a")
	workspaceB := filepath.Join(tempDir, "workspace-b")

	result, err := manager.AddMemoryCandidate(memory.AddMemoryCandidateInput{
		Content:       "这个项目 8080 端口不可用，相关服务和测试必须避开。",
		Source:        memory.MemorySourceManual,
		WorkspacePath: workspaceA,
		Type:          memory.MemoryTypeProject,
		Scope:         testScope("team-a", "架构团队", workspaceA),
	})
	if err != nil {
		t.Fatalf("AddMemoryCandidate() error = %v", err)
	}
	if !result.Written {
		t.Fatalf("expected project memory write, got %+v", result)
	}

	resultsA, err := manager.SearchMemory(memory.SearchMemoryInput{
		Scope:           testScope("team-a", "架构团队", workspaceA),
		Type:            memory.MemoryTypeProject,
		Query:           "8080",
		IncludeInactive: true,
	})
	if err != nil || len(resultsA) != 1 {
		t.Fatalf("expected project A memory, got results=%+v err=%v", resultsA, err)
	}

	resultsB, err := manager.SearchMemory(memory.SearchMemoryInput{
		Scope:           testScope("team-a", "架构团队", workspaceB),
		Type:            memory.MemoryTypeProject,
		Query:           "8080",
		IncludeInactive: true,
	})
	if err != nil {
		t.Fatalf("SearchMemory(projectB) error = %v", err)
	}
	if len(resultsB) != 0 {
		t.Fatalf("expected project B to be isolated, got %+v", resultsB)
	}
}

func TestMemoryToolAdd_RequiresTeamIdentityAndSavesWithScope(t *testing.T) {
	tempDir := t.TempDir()
	teamRoot := filepath.Join(tempDir, "team-memory")
	manager := memory.NewMemoryManagerWithTeamPath(nil, teamRoot)

	raw, err := manager.HandleToolCall(nil, "memory.add", map[string]any{
		"type":    "team",
		"content": "团队采用多智能体协同工作模式，各 Agent 分工协作完成任务。",
	})
	if err != nil {
		t.Fatalf("HandleToolCall() error = %v", err)
	}
	var resp memory.MemoryToolResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal() error = %v, raw=%s", err, raw)
	}
	if resp.Success || !strings.Contains(resp.Error, "team identity is required") {
		t.Fatalf("expected team identity failure, got %+v", resp)
	}

	raw, err = manager.HandleToolCall(nil, "memory.add", map[string]any{
		"type":     "team",
		"teamId":   "team-a",
		"teamName": "架构团队",
		"content":  "团队采用多智能体协同工作模式，各 Agent 分工协作完成任务。",
	})
	if err != nil {
		t.Fatalf("HandleToolCall() scoped error = %v", err)
	}
	resp = memory.MemoryToolResponse{}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal scoped error = %v, raw=%s", err, raw)
	}
	if !resp.Success {
		t.Fatalf("expected scoped team memory to be saved, got %+v", resp)
	}
	if _, err := os.Stat(filepath.Join(teamRoot, "team-a", "MEMORY.md")); err != nil {
		t.Fatalf("expected scoped team memory file: %v", err)
	}
}

func TestMarkdownMemory_ReadsLegacyProjectFormat(t *testing.T) {
	tempDir := t.TempDir()
	workspace := filepath.Join(tempDir, "workspace")
	projectFile := memory.ProjectMemoryPath(workspace)
	if err := os.MkdirAll(filepath.Dir(projectFile), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	legacy := `# Colink Project Memory

## legacy-port-memory

- Type: project
- Source: manual
- Confidence: high
- Status: active
- Tags: port
- Created: 2026-06-02
- Updated: 2026-06-02

### Memory

Port 8080 is unavailable.

### Usage

Use another port when starting local services.
`
	if err := os.WriteFile(projectFile, []byte(legacy), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manager := memory.NewMemoryManagerWithTeamPath(nil, filepath.Join(tempDir, "team-memory"))
	results, err := manager.SearchMemory(memory.SearchMemoryInput{
		Scope:           testScope("team-a", "架构团队", workspace),
		Query:           "8080",
		Type:            memory.MemoryTypeProject,
		IncludeInactive: true,
	})
	if err != nil {
		t.Fatalf("SearchMemory() error = %v", err)
	}
	if len(results) != 1 || results[0].ID != "legacy-port-memory" {
		t.Fatalf("expected legacy memory to be parsed, got %+v", results)
	}
}
