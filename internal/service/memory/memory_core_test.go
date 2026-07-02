package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

type fakeAgentConfigLister struct {
	configs []*model.AgentRoleConfig
	err     error
}

func (f fakeAgentConfigLister) List(ctx context.Context) ([]*model.AgentRoleConfig, error) {
	return f.configs, f.err
}

func TestPathsAndScopeHelpers(t *testing.T) {
	root := t.TempDir()
	if got := ProjectMemoryPath(root); got != filepath.Join(root, ".colink/project-memory/MEMORY.md") {
		t.Fatalf("ProjectMemoryPath = %q", got)
	}
	if got := ProjectMemoryPath(""); got != "" {
		t.Fatalf("empty ProjectMemoryPath = %q", got)
	}
	if got := TeamMemoryPath(filepath.Join(root, "MEMORY.md"), "team-a"); got != filepath.Join(root, "team-a", "MEMORY.md") {
		t.Fatalf("TeamMemoryPath with file root = %q", got)
	}
	if got := TeamMemoryPath(root, ""); got != "" {
		t.Fatalf("empty team path = %q", got)
	}

	scope := normalizeMemoryScope(MemoryScopeIdentity{TeamID: " team ", ProjectName: " p "}, root)
	if scope.TeamID != "team" || scope.ProjectName != "p" || scope.WorkspacePath != root {
		t.Fatalf("normalized scope = %#v", scope)
	}
	teamScope := scopeForType(scope, MemoryTypeTeam)
	if teamScope.WorkspacePath != "" || teamScope.ProjectName != "" || teamScope.TeamID != "team" {
		t.Fatalf("team scope = %#v", teamScope)
	}
	projectScope := scopeForType(scope, MemoryTypeProject)
	if projectScope.TeamID != "" || projectScope.ProjectName != "p" || projectScope.WorkspacePath != root {
		t.Fatalf("project scope = %#v", projectScope)
	}
}

func TestScrubMemoryContextStreamingAndStatic(t *testing.T) {
	raw := "before <memory-context>\nsecret memory\n</memory-context> after"
	if got := ScrubMemoryContext(raw); got != "before  after" {
		t.Fatalf("ScrubMemoryContext = %q", got)
	}
	if got := ScrubMemoryContext("plain"); got != "plain" {
		t.Fatalf("plain scrub = %q", got)
	}

	s := NewStreamingMemoryScrubber()
	var visible strings.Builder
	visible.WriteString(s.Feed("hello <memory-context>hidden"))
	visible.WriteString(s.Feed("</memory-context> world"))
	visible.WriteString(s.Flush())
	if visible.String() != "hello  world" {
		t.Fatalf("stream visible = %q", visible.String())
	}

	visible.Reset()
	s.Reset()
	visible.WriteString(s.Feed("a <memory-context>leak"))
	visible.WriteString(s.Flush())
	if visible.String() != "a " {
		t.Fatalf("unclosed stream visible = %q", visible.String())
	}
}

func TestCandidateClassificationDraftAndHelpers(t *testing.T) {
	if !containsSensitive("api_key=sk-secret-value") || !containsSensitive("ghp_abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("expected sensitive patterns to match")
	}
	if containsSensitive("ordinary project note") {
		t.Fatalf("ordinary note should not be sensitive")
	}

	draft := MemoryDraft{
		Topic:   "Build Rules",
		Summary: "Run go test before merge.\nNever leak context.",
		Facts:   []string{" run go test ./... ", "run go test ./...", "需要我保存吗", strings.Repeat("x", 400)},
		Usage:   []string{"before testing", "before testing"},
	}
	memoryText, usageText, topic, summary, used, ok := normalizeMemoryDraft(draft)
	if !used || !ok || topic != "build_rules" || !strings.Contains(memoryText, "run go test") || strings.Count(memoryText, "run go test") != 1 || usageText != "before testing" {
		t.Fatalf("normalized draft = %q %q %q used=%v ok=%v", memoryText, usageText, topic, used, ok)
	}
	if !strings.Contains(summary, "Run go test") || strings.Contains(summary, "\n") {
		t.Fatalf("summary = %q", summary)
	}

	if _, _, _, _, used, ok := normalizeMemoryDraft(MemoryDraft{Facts: []string{"token=secret"}}); !used || ok {
		t.Fatalf("sensitive draft should be used but rejected")
	}

	if !shouldAcceptManualMemory(AddMemoryCandidateInput{Source: MemorySourceManual, Type: MemoryTypeProject}, "项目端口固定为 8090") {
		t.Fatalf("manual project memory should be accepted")
	}
	if shouldAcceptManualMemory(AddMemoryCandidateInput{Source: MemorySourceAgentObservation, Type: MemoryTypeProject}, "项目端口固定为 8090") {
		t.Fatalf("non-manual memory should not be accepted")
	}

	if classifyMemoryType("项目启动命令使用 go test 和 sqlite 数据库") != MemoryTypeProject {
		t.Fatalf("project classification failed")
	}
	if classifyMemoryType("架构师负责上游下游 Agent 协作") != MemoryTypeTeam {
		t.Fatalf("team classification failed")
	}
	if classifyMemoryType("just a note") != "" {
		t.Fatalf("uncertain classification should be empty")
	}

	id := buildMemoryID("测试端口 MEM_TEST_PORT", []string{"test"})
	if id == "" || len(id) > 48 {
		t.Fatalf("memory id = %q", id)
	}
	tags := inferTags("测试命令 go test 不要使用 3000 端口，需要协作交接")
	for _, want := range []string{"command", "constraint", "test", "port", "collaboration"} {
		if !containsString(tags, want) {
			t.Fatalf("tags %v missing %s", tags, want)
		}
	}
	if buildUsage(MemoryTypeTeam, "角色职责和下游协作") == "" || buildUsage(MemoryTypeProject, "测试命令 go test") == "" {
		t.Fatalf("buildUsage should explain team/project usage")
	}
	if normalizeSource("unknown") != MemorySourceManual || confidenceForSource(MemorySourceCommandResult, false) != MemoryConfidenceMedium || confidenceForSource(MemorySourceAgentObservation, true) != MemoryConfidenceHigh {
		t.Fatalf("source/confidence helpers changed")
	}
}

func TestRoleMemoryAndTopicHelpers(t *testing.T) {
	content := `我是**代码工程师**
我的职责：
- 实现后端 API
- 编写测试
上游来自 @架构师
完成后交接给 @测试工程师`
	role, ok := extractRoleMemory(content)
	if !ok {
		t.Fatalf("role memory not extracted")
	}
	if role.role != "代码工程师" || !strings.Contains(role.memory, "负责实现后端 API、编写测试") || !strings.Contains(role.memory, "架构师") || !strings.Contains(role.memory, "测试工程师") {
		t.Fatalf("role memory = %#v", role)
	}
	if roleMemoryID("架构设计师") != "agent-role-architect" || roleMemoryID("代码工程师") != "agent-role-coder" {
		t.Fatalf("roleMemoryID built-ins failed")
	}

	cases := []struct {
		entry MemoryEntry
		key   string
		title string
	}{
		{MemoryEntry{Topic: "Custom Topic", Memory: "one"}, "custom_topic", "custom topic"},
		{MemoryEntry{Tags: []string{"agent-role"}, Memory: "one"}, "agent_roles", "Agent 角色职责"},
		{MemoryEntry{Tags: []string{"preference"}, Memory: "one"}, "user_preferences", "用户偏好"},
		{MemoryEntry{Tags: []string{"port"}, Memory: "one"}, "port_constraints", "端口约束"},
		{MemoryEntry{Tags: []string{"test"}, Memory: "one"}, "memory_test_rules", "记忆测试协作规则"},
		{MemoryEntry{Tags: []string{"refactor"}, Memory: "one"}, "refactor_collaboration_rules", "重构协作规则"},
		{MemoryEntry{Type: MemoryTypeProject, Memory: "go test ./..."}, "project_commands", "项目命令约定"},
	}
	for _, tt := range cases {
		got := deriveMemoryTopic(tt.entry)
		if got.Key != tt.key || got.Title != tt.title || got.Filename != tt.key+".md" {
			t.Fatalf("deriveMemoryTopic(%#v) = %#v", tt.entry, got)
		}
	}
}

func TestMarkdownStoreSaveLoadAndParse(t *testing.T) {
	store := NewMarkdownStore()
	root := t.TempDir()
	index := filepath.Join(root, "MEMORY.md")
	now := time.Date(2026, 6, 29, 0, 0, 0, 0, time.Local)
	entries := []MemoryEntry{
		{
			ID:         "project-command",
			Type:       MemoryTypeProject,
			Source:     MemorySourceManual,
			Confidence: MemoryConfidenceHigh,
			Status:     MemoryStatusActive,
			Tags:       []string{"command", "test"},
			Summary:    "Run tests before merge",
			Created:    now,
			Updated:    now,
			Memory:     "go test ./...\ngo vet ./...",
			Usage:      "Before merging",
		},
		{
			ID:         "agent-role-coder",
			Type:       MemoryTypeTeam,
			Source:     MemorySourceManual,
			Confidence: MemoryConfidenceHigh,
			Status:     MemoryStatusOutdated,
			Tags:       []string{"agent-role"},
			Created:    now,
			Updated:    now.Add(time.Hour),
			Memory:     "代码工程师：负责实现功能。",
		},
	}

	if err := store.Save(index, "Memory", entries, MemoryScopeIdentity{ProjectName: "Colink", WorkspacePath: root}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	rawIndex, err := os.ReadFile(index)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if !strings.Contains(string(rawIndex), "Project Memory") || !strings.Contains(string(rawIndex), "Team Memory") {
		t.Fatalf("index markdown = %s", rawIndex)
	}
	loaded, err := store.Load(index)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded entries = %#v", loaded)
	}
	if loaded[0].Memory == "" || loaded[1].Memory == "" {
		t.Fatalf("loaded memory should not be empty: %#v", loaded)
	}

	links := parseMemoryIndexLinks("- [ok](topic.md)\n- [bad](/abs.md)\n- [bad](../escape.md)\n- [dup](topic.md)\n- [self](MEMORY.md)")
	if len(links) != 1 || links[0] != "topic.md" {
		t.Fatalf("links = %#v", links)
	}

	legacy := `## old-memory
- Type: project
- Source: manual
- Confidence: high
- Status: active
- Tags: command, test
### Memory
run make test
### Usage
before release`
	parsed := parseMemoryMarkdown(legacy)
	if len(parsed) != 1 || parsed[0].ID != "old-memory" || parsed[0].Usage != "before release" {
		t.Fatalf("legacy parsed = %#v", parsed)
	}

	stalePath := filepath.Join(root, "stale.md")
	if err := os.WriteFile(stalePath, []byte("node_type: memory\nold"), 0644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	if err := store.Replace(index, "Memory", entries[:1], MemoryScopeIdentity{WorkspacePath: root}); err != nil {
		t.Fatalf("Replace returned error: %v", err)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("stale memory file should be removed, err=%v", err)
	}
}

func TestReadRawMarkdownIndexLinksMissingAndSiblings(t *testing.T) {
	workspace := t.TempDir()
	indexDir := filepath.Join(workspace, ".colink", "project-memory")
	if err := os.MkdirAll(filepath.Join(indexDir, "nested"), 0755); err != nil {
		t.Fatalf("mkdir memory dirs: %v", err)
	}
	index := filepath.Join(indexDir, memoryIndexFile)
	if err := os.WriteFile(index, []byte("- [Alpha](alpha.md)\n- [Beta](nested/beta.md)\n- [Missing](missing.md)\n- [Duplicate](alpha.md)\n"), 0644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(indexDir, "alpha.md"), []byte("alpha memory"), 0644); err != nil {
		t.Fatalf("write alpha: %v", err)
	}
	if err := os.WriteFile(filepath.Join(indexDir, "nested", "beta.md"), []byte("beta memory"), 0644); err != nil {
		t.Fatalf("write beta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(indexDir, "sibling.md"), []byte("sibling memory"), 0644); err != nil {
		t.Fatalf("write sibling: %v", err)
	}

	manager := NewMemoryManager(nil)
	group, err := manager.ReadRawMarkdown(MemoryTypeProject, MemoryScopeIdentity{WorkspacePath: workspace, ProjectName: "Colink"})
	if err != nil {
		t.Fatalf("ReadRawMarkdown() error = %v", err)
	}
	if !group.IndexExists || group.IndexPath != index || !strings.Contains(group.Index, "Alpha") {
		t.Fatalf("raw group index fields = %+v", group)
	}
	if len(group.Missing) != 1 || group.Missing[0] != "missing.md" {
		t.Fatalf("missing = %#v", group.Missing)
	}
	if !rawMarkdownGroupHasFile(group, "alpha.md", "alpha memory") ||
		!rawMarkdownGroupHasFile(group, "beta.md", "beta memory") ||
		!rawMarkdownGroupHasFile(group, "sibling.md", "sibling memory") {
		t.Fatalf("raw group files = %#v", group.Files)
	}

	empty, err := manager.ReadRawMarkdown(MemoryTypeProject, MemoryScopeIdentity{WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatalf("ReadRawMarkdown(missing index) error = %v", err)
	}
	if empty.IndexExists || len(empty.Files) != 0 || len(empty.Missing) != 0 {
		t.Fatalf("missing index group = %+v", empty)
	}
	if _, err := rawMarkdownFileFromPath(filepath.Join(workspace, "missing.md")); err == nil {
		t.Fatal("rawMarkdownFileFromPath(missing) error = nil, want error")
	}
	if got := manager.rawMarkdownIndexPath(MemoryType("unknown"), MemoryScopeIdentity{}); got != "" {
		t.Fatalf("rawMarkdownIndexPath(unknown) = %q, want empty", got)
	}
}

func rawMarkdownGroupHasFile(group RawMarkdownGroup, name, content string) bool {
	for _, file := range group.Files {
		if file.Name == name && strings.Contains(file.Content, content) {
			return true
		}
	}
	return false
}

func TestMemoryManagerAddSearchReplaceRemoveAndToolCalls(t *testing.T) {
	workspace := t.TempDir()
	teamRoot := filepath.Join(t.TempDir(), "team-memory")
	manager := NewMemoryManagerWithTeamPath(nil, teamRoot)
	scope := MemoryScopeIdentity{TeamID: "team-a", TeamName: "Team A", ProjectID: "project-a", ProjectName: "Project A", WorkspacePath: workspace}

	result, err := manager.AddMemoryCandidate(AddMemoryCandidateInput{
		Content:       "项目启动命令使用 go test ./...，sqlite 数据库路径固定。",
		Source:        MemorySourceManual,
		WorkspacePath: workspace,
		Scope:         scope,
		Type:          MemoryTypeProject,
		Tags:          []string{"command"},
	})
	if err != nil {
		t.Fatalf("AddMemoryCandidate project error: %v", err)
	}
	if !result.Written || result.Type != MemoryTypeProject || result.TargetFile != ProjectMemoryPath(workspace) {
		t.Fatalf("project add result = %#v", result)
	}
	duplicate, err := manager.AddMemoryCandidate(AddMemoryCandidateInput{
		Content:       "项目启动命令使用 go test ./...，sqlite 数据库路径固定。",
		Source:        MemorySourceManual,
		WorkspacePath: workspace,
		Scope:         scope,
		Type:          MemoryTypeProject,
	})
	if err != nil {
		t.Fatalf("duplicate add error: %v", err)
	}
	if duplicate.Written || !strings.Contains(duplicate.Reason, "duplicate") {
		t.Fatalf("duplicate result = %#v", duplicate)
	}

	teamResult, err := manager.AddMemoryCandidate(AddMemoryCandidateInput{
		Content: `我是代码工程师
我的职责：
- 实现 API
上游 @架构师 下游 @测试工程师`,
		Source: MemorySourceManual,
		Scope:  scope,
	})
	if err != nil {
		t.Fatalf("AddMemoryCandidate team error: %v", err)
	}
	if !teamResult.Written || teamResult.Type != MemoryTypeTeam {
		t.Fatalf("team add result = %#v", teamResult)
	}

	results, err := manager.SearchMemory(SearchMemoryInput{Scope: scope, Query: "sqlite", IncludeInactive: false})
	if err != nil {
		t.Fatalf("SearchMemory returned error: %v", err)
	}
	if len(results) != 1 || !strings.Contains(results[0].Memory, "sqlite") {
		t.Fatalf("search results = %#v", results)
	}

	id, err := manager.ReplaceMemory(scope, MemoryTypeProject, "sqlite", "项目启动命令使用 go test ./...，数据库改为 postgres。")
	if err != nil {
		t.Fatalf("ReplaceMemory returned error: %v", err)
	}
	if id == "" {
		t.Fatalf("ReplaceMemory id empty")
	}
	if err := manager.UpdateMemoryStatus(scope, MemoryTypeProject, "postgres", MemoryStatusOutdated); err != nil {
		t.Fatalf("UpdateMemoryStatus returned error: %v", err)
	}
	inactive, err := manager.SearchMemory(SearchMemoryInput{Scope: scope, Query: "postgres", Type: MemoryTypeProject, IncludeInactive: true})
	if err != nil || len(inactive) != 1 || inactive[0].Status != MemoryStatusOutdated {
		t.Fatalf("inactive search = %#v err=%v", inactive, err)
	}
	if err := manager.RemoveMemory(scope, MemoryTypeProject, "postgres"); err != nil {
		t.Fatalf("RemoveMemory returned error: %v", err)
	}
	if _, err := manager.ReplaceMemory(scope, MemoryTypeProject, "missing", "replacement"); err == nil {
		t.Fatalf("missing replace should fail")
	}
	if err := manager.UpdateMemoryStatus(scope, "", "x", MemoryStatusActive); err == nil {
		t.Fatalf("missing memory type should fail")
	}

	addJSON, err := manager.HandleToolCall(context.Background(), "memory.add", map[string]any{
		"workspacePath": workspace,
		"teamId":        "team-a",
		"type":          "project",
		"content":       "项目测试命令使用 go test ./internal/service/memory",
	})
	if err != nil {
		t.Fatalf("HandleToolCall add error: %v", err)
	}
	var addResp MemoryToolResponse
	if err := json.Unmarshal([]byte(addJSON), &addResp); err != nil || !addResp.Success {
		t.Fatalf("add response = %s err=%v", addJSON, err)
	}
	searchResp := manager.handleToolCall(context.Background(), "memory.search", map[string]any{
		"workspacePath": workspace,
		"teamId":        "team-a",
		"type":          "project",
		"query":         "memory",
	})
	if !searchResp.Success || len(searchResp.Results) == 0 {
		t.Fatalf("search tool response = %#v", searchResp)
	}
	if resp := manager.handleToolCall(context.Background(), "memory.remove", map[string]any{"action": "remove"}); resp.Success || !strings.Contains(resp.Error, "oldText") {
		t.Fatalf("remove validation response = %#v", resp)
	}
	if resp := manager.handleToolCall(context.Background(), "memory.unknown", map[string]any{"action": "wat"}); resp.Success || !strings.Contains(resp.Error, "unknown action") {
		t.Fatalf("unknown action response = %#v", resp)
	}

	prefetch := manager.PrefetchMultiScope(context.Background(), "thread", "agent", "team-a", "project-a", workspace)
	if !strings.Contains(prefetch, "Project Memory") || !strings.Contains(prefetch, "Team Memory") {
		t.Fatalf("prefetch = %q", prefetch)
	}
	block := manager.BuildMemoryContextBlock("remember this")
	if !strings.Contains(block, "<memory-context>") || !strings.Contains(block, "remember this") {
		t.Fatalf("context block = %q", block)
	}
	manager.SyncTurn(context.Background(), "u", "a")
	if err := manager.OnThreadEnd(context.Background(), "thread"); err != nil {
		t.Fatalf("OnThreadEnd returned error: %v", err)
	}
	manager.Shutdown()
}

func TestAgentDiscoveryAndAutoMemoryIndex(t *testing.T) {
	lister := fakeAgentConfigLister{configs: []*model.AgentRoleConfig{
		nil,
		{ID: uuid.New(), Name: "前端工程师", Role: model.AgentRoleAgent, Description: "负责 UI 组件和 router", SystemPrompt: "使用 vite"},
		{ID: uuid.New(), Name: "人工审批", Role: model.AgentRoleHuman, Description: "human"},
		{ID: uuid.New(), Name: "DB Reviewer", Role: model.AgentRoleReviewer, Description: "review sql database"},
	}}
	manager := NewMemoryManagerWithTeamPath(lister, t.TempDir())
	agents, err := manager.ListTeamAgents(context.Background(), "")
	if err != nil {
		t.Fatalf("ListTeamAgents returned error: %v", err)
	}
	if len(agents) != 2 || agents[0].Name != "DB Reviewer" || agents[1].Name != "前端工程师" {
		t.Fatalf("agents = %#v", agents)
	}
	if !containsString(agents[0].Capabilities, "database") || !containsString(agents[0].Capabilities, "review") {
		t.Fatalf("db reviewer capabilities = %#v", agents[0].Capabilities)
	}
	if !containsString(agents[1].Capabilities, "ui") || !containsString(agents[1].Capabilities, "router") || !containsString(agents[1].Capabilities, "frontend-build") {
		t.Fatalf("frontend capabilities = %#v", agents[1].Capabilities)
	}

	workspace := t.TempDir()
	scope := MemoryScopeIdentity{TeamID: "team-a", WorkspacePath: workspace}
	if _, err := manager.AddMemoryCandidate(AddMemoryCandidateInput{
		Content:       "项目测试命令使用 go test ./...",
		Source:        MemorySourceManual,
		WorkspacePath: workspace,
		Scope:         scope,
		Type:          MemoryTypeProject,
	}); err != nil {
		t.Fatalf("add project memory: %v", err)
	}
	if _, err := manager.AddMemoryCandidate(AddMemoryCandidateInput{
		Content: "团队职责由架构师负责上游下游协作。",
		Source:  MemorySourceManual,
		Scope:   scope,
		Type:    MemoryTypeTeam,
	}); err != nil {
		t.Fatalf("add team memory: %v", err)
	}
	index := manager.BuildAutoMemoryIndexBlock(context.Background(), scope, 10)
	if !strings.Contains(index, "<memory-index>") || !strings.Contains(index, "Project Memory Index") || !strings.Contains(index, "Team Memory Index") || !strings.Contains(index, "MEMORY.md") {
		t.Fatalf("auto memory index = %q", index)
	}
	entries := extractMemoryIndexEntries("- [A](a.md)\nnot an entry\n- [B](b.md)\n- [C](c.txt)", 1)
	if len(entries) != 1 || entries[0] != "- [A](a.md)" || !isMemoryIndexEntry("- [A](a.md)") || isMemoryIndexEntry("- nope") {
		t.Fatalf("index entries = %#v", entries)
	}
}
