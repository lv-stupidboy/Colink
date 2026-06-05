package memory_test

import (
	"encoding/json"
	"fmt"
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

func TestMemoryIndex_UsesTopicSummaryWithoutEllipsis(t *testing.T) {
	tempDir := t.TempDir()
	teamRoot := filepath.Join(tempDir, "team-memory")
	manager := memory.NewMemoryManagerWithTeamPath(nil, teamRoot)
	workspace := filepath.Join(tempDir, "workspace")
	scope := testScope("team-a", "架构团队", workspace)

	result, err := manager.AddMemoryCandidate(memory.AddMemoryCandidateInput{
		Content: "架构设计师负责团队技术方案。",
		Source:  memory.MemorySourceAgentObservation,
		Type:    memory.MemoryTypeTeam,
		Scope:   scope,
		Draft: memory.MemoryDraft{
			Topic:   "agent_roles",
			Summary: "记录架构设计师、代码工程师的职责与上下游关系",
			Facts: []string{
				"架构设计师负责方案设计、技术选型、模块拆分、关键风险识别、设计文档输出和后续实现交接。",
				"代码工程师负责根据架构设计完成代码实现、单元测试、问题修复和实现状态反馈。",
				"架构设计师的下游是代码工程师，代码工程师完成实现后需要反馈阻塞与验证结果。",
			},
			Usage: []string{"回答团队分工、角色职责或调度下游 Agent 前先检查。"},
		},
	})
	if err != nil {
		t.Fatalf("AddMemoryCandidate(team) error = %v", err)
	}
	if !result.Written {
		t.Fatalf("expected team memory write, got %+v", result)
	}

	teamIndexData, err := os.ReadFile(filepath.Join(teamRoot, "team-a", "MEMORY.md"))
	if err != nil {
		t.Fatalf("ReadFile(team index) error = %v", err)
	}
	teamIndex := string(teamIndexData)
	if strings.Contains(teamIndex, "...") {
		t.Fatalf("team index should not contain ellipsis truncation:\n%s", teamIndex)
	}
	if want := "- [Agent 角色职责](agent_roles.md) — 记录架构设计师、代码工程师的职责与上下游关系"; !strings.Contains(teamIndex, want) {
		t.Fatalf("expected topic summary %q in team index:\n%s", want, teamIndex)
	}

	result, err = manager.AddMemoryCandidate(memory.AddMemoryCandidateInput{
		Content:       "项目常用命令。",
		Source:        memory.MemorySourceAgentObservation,
		WorkspacePath: workspace,
		Type:          memory.MemoryTypeProject,
		Scope:         scope,
		Draft: memory.MemoryDraft{
			Topic:   "project_commands",
			Summary: "记录2条项目命令约定",
			Facts: []string{
				"go test ./internal/service/memory ./internal/mcp-server/... ./internal/api ./cmd/server -run TestNonExistent 用于快速验证记忆服务与 MCP 工具编译。",
				"go test ./auto-test/internal/service/memory 用于验证 Markdown 记忆索引、隔离和检索行为。",
			},
			Usage: []string{"修改记忆落盘或注入逻辑后运行。"},
		},
	})
	if err != nil {
		t.Fatalf("AddMemoryCandidate(project) error = %v", err)
	}
	if !result.Written {
		t.Fatalf("expected project memory write, got %+v", result)
	}

	projectIndexData, err := os.ReadFile(memory.ProjectMemoryPath(workspace))
	if err != nil {
		t.Fatalf("ReadFile(project index) error = %v", err)
	}
	projectIndex := string(projectIndexData)
	if strings.Contains(projectIndex, "...") {
		t.Fatalf("project index should not contain ellipsis truncation:\n%s", projectIndex)
	}
	if want := "- [项目命令约定](project_commands.md) — 记录2条项目命令约定"; !strings.Contains(projectIndex, want) {
		t.Fatalf("expected topic summary %q in project index:\n%s", want, projectIndex)
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

func TestAutoMemoryIndexBlock_InjectsOnlyFirstThirtyIndexEntries(t *testing.T) {
	tempDir := t.TempDir()
	workspace := filepath.Join(tempDir, "workspace")
	projectIndex := memory.ProjectMemoryPath(workspace)
	if err := os.MkdirAll(filepath.Dir(projectIndex), 0755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	var project strings.Builder
	project.WriteString("# Memory Index\n\n## Project Memory\n")
	for i := 1; i <= 35; i++ {
		project.WriteString(fmt.Sprintf("- [project topic %02d](project_topic_%02d.md) — summary %02d\n", i, i, i))
	}
	if err := os.WriteFile(projectIndex, []byte(project.String()), 0644); err != nil {
		t.Fatalf("WriteFile(project index) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(projectIndex), "project_topic_01.md"), []byte("FULL PROJECT TOPIC BODY"), 0644); err != nil {
		t.Fatalf("WriteFile(project topic) error = %v", err)
	}

	teamRoot := filepath.Join(tempDir, "team-memory")
	teamIndex := filepath.Join(teamRoot, "team-a", "MEMORY.md")
	if err := os.MkdirAll(filepath.Dir(teamIndex), 0755); err != nil {
		t.Fatalf("MkdirAll(team) error = %v", err)
	}
	var team strings.Builder
	team.WriteString("# Memory Index\n\n## Team Memory\n")
	for i := 1; i <= 35; i++ {
		team.WriteString(fmt.Sprintf("- [team topic %02d](team_topic_%02d.md) — summary %02d\n", i, i, i))
	}
	if err := os.WriteFile(teamIndex, []byte(team.String()), 0644); err != nil {
		t.Fatalf("WriteFile(team index) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(teamIndex), "team_topic_01.md"), []byte("FULL TEAM TOPIC BODY"), 0644); err != nil {
		t.Fatalf("WriteFile(team topic) error = %v", err)
	}

	manager := memory.NewMemoryManagerWithTeamPath(nil, teamRoot)
	block := manager.BuildAutoMemoryIndexBlock(nil, testScope("team-a", "架构团队", workspace), 30)

	for _, want := range []string{
		"<memory-index>",
		"## Project Memory Index",
		"## Team Memory Index",
		"- [project topic 30](project_topic_30.md)",
		"- [team topic 30](team_topic_30.md)",
		projectIndex,
		teamIndex,
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("expected %q in auto memory block:\n%s", want, block)
		}
	}
	for _, unwanted := range []string{
		"- [project topic 31](project_topic_31.md)",
		"- [team topic 31](team_topic_31.md)",
		"FULL PROJECT TOPIC BODY",
		"FULL TEAM TOPIC BODY",
	} {
		if strings.Contains(block, unwanted) {
			t.Fatalf("did not expect %q in auto memory block:\n%s", unwanted, block)
		}
	}
}
