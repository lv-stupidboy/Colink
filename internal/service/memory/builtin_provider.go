package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
)

// BuiltinProvider 内置 SQLite 记忆提供者
type BuiltinProvider struct {
	teamRepo    repo.TeamMemoryRepo
	projectRepo repo.ProjectMemoryRepo
	config      ProviderConfig
}

// NewBuiltinProvider 创建内置提供者
func NewBuiltinProvider(db *sql.DB) *BuiltinProvider {
	return &BuiltinProvider{
		teamRepo:    repo.NewSQLiteTeamMemoryRepo(db),
		projectRepo: repo.NewSQLiteProjectMemoryRepo(db),
	}
}

func (p *BuiltinProvider) Name() string {
	return "builtin"
}

func (p *BuiltinProvider) IsAvailable() bool {
	return true // SQLite always available
}

func (p *BuiltinProvider) Initialize(sessionID string, opts ...Option) error {
	p.config = ProviderConfig{SessionID: sessionID}
	for _, opt := range opts {
		opt(&p.config)
	}
	return nil
}

// ========== Tool Schemas ==========

var MEMORY_TOOL_SCHEMA = map[string]any{
	"name": "memory",
	"description": `记忆工具 - 保存、查询、更新、删除记忆内容。
支持四个层级：user（用户级）、agent（Agent级）、team（团队级）、session（会话级）。

ACTIONS:
- add: 添加新记忆条目
- replace: 替换现有条目（通过 old_text 子串匹配）
- remove: 删除条目（通过 old_text 子串匹配）
- search: 搜索记忆内容

WHEN TO SAVE:
- 用户明确指令："记住这个"、"下次记得"
- 用户偏好、习惯、个人细节
- 发现环境事实、项目约定
- 纠正或教训

SKIP: 临时任务状态、容易重新发现的信息`,
	"parameters": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type": "string",
				"enum": []string{"add", "replace", "remove", "search"},
				"description": "操作类型",
			},
			"scope": map[string]any{
				"type": "string",
				"enum": []string{"user", "agent", "team", "session"},
				"description": "记忆层级",
			},
			"scopeId": map[string]any{
				"type": "string",
				"description": "层级 ID（userId/baseAgentId/teamId/threadId）",
			},
			"content": map[string]any{
				"type": "string",
				"description": "记忆内容（add/replace 时必需）",
			},
			"oldText": map[string]any{
				"type": "string",
				"description": "匹配旧条目的子串（replace/remove 时必需）",
			},
			"query": map[string]any{
				"type": "string",
				"description": "搜索查询词（search 时必需）",
			},
			"category": map[string]any{
				"type": "string",
				"enum": []string{"preference", "decision", "convention", "context", "technical"},
				"description": "内容分类（可选）",
			},
		},
		"required": []string{"action", "scope"},
	},
}

func (p *BuiltinProvider) GetToolSchemas() []map[string]any {
	return []map[string]any{MEMORY_TOOL_SCHEMA}
}

// ========== Tool Call Handler ==========

func (p *BuiltinProvider) HandleToolCall(ctx context.Context, name string, args map[string]any) (string, error) {
	// 根据工具名确定 scope
	var scope model.MemoryScope
	switch name {
	case "team_memory":
		scope = model.MemoryScopeTeam
	case "project_memory":
		scope = model.MemoryScopeProject
	default:
		resp := model.MemoryToolResponse{
			Success: false,
			Error:   "Unknown tool: " + name,
		}
		data, _ := json.Marshal(resp)
		return string(data), nil
	}

	action := model.MemoryAction(args["action"].(string))
	scopeID, _ := args["scopeId"].(string)
	// 从工具特定参数获取 scopeId
	if scope == model.MemoryScopeTeam {
		if teamID, ok := args["teamId"].(string); ok && teamID != "" {
			scopeID = teamID
		}
	} else if scope == model.MemoryScopeProject {
		if projectID, ok := args["projectId"].(string); ok && projectID != "" {
			scopeID = projectID
		}
	}
	content, _ := args["content"].(string)
	oldText, _ := args["oldText"].(string)
	query, _ := args["query"].(string)

	var resp model.MemoryToolResponse

	switch action {
	case model.MemoryActionAdd:
		resp = p.handleAdd(ctx, scope, scopeID, content, args)
	case model.MemoryActionReplace:
		resp = p.handleReplace(ctx, scope, scopeID, oldText, content)
	case model.MemoryActionRemove:
		resp = p.handleRemove(ctx, scope, scopeID, oldText)
	case model.MemoryActionSearch:
		resp = p.handleSearch(ctx, scope, scopeID, query)
	default:
		resp = model.MemoryToolResponse{Success: false, Error: "Unknown action: " + string(action)}
	}

	data, err := json.Marshal(resp)
	return string(data), err
}

func (p *BuiltinProvider) handleAdd(ctx context.Context, scope model.MemoryScope, scopeID, content string, args map[string]any) model.MemoryToolResponse {
	if content == "" {
		return model.MemoryToolResponse{Success: false, Error: "content is required for add"}
	}
	if scopeID == "" {
		scopeID = p.resolveScopeID(scope)
	}

	category, _ := args["category"].(string)

	switch scope {
	case model.MemoryScopeTeam:
		mem := &model.TeamMemory{
			TeamID:   scopeID,
			Content:  content,
			Category: model.MemoryCategory(category),
		}
		err := p.teamRepo.Create(ctx, mem)
		if err != nil {
			return model.MemoryToolResponse{Success: false, Error: err.Error()}
		}
		return model.MemoryToolResponse{
			Success: true,
			Message: "Team memory added",
			Entries: []string{mem.ID},
		}
	case model.MemoryScopeProject:
		mem := &model.ProjectMemory{
			ProjectID: scopeID,
			Content:   content,
			Category:  model.MemoryCategory(category),
		}
		err := p.projectRepo.Create(ctx, mem)
		if err != nil {
			return model.MemoryToolResponse{Success: false, Error: err.Error()}
		}
		return model.MemoryToolResponse{
			Success: true,
			Message: "Project memory added",
			Entries: []string{mem.ID},
		}
	}
	return model.MemoryToolResponse{Success: false, Error: "Invalid scope: only team and project are supported"}
}

func (p *BuiltinProvider) handleReplace(ctx context.Context, scope model.MemoryScope, scopeID, oldText, content string) model.MemoryToolResponse {
	if oldText == "" || content == "" {
		return model.MemoryToolResponse{Success: false, Error: "oldText and content required for replace"}
	}
	if scopeID == "" {
		scopeID = p.resolveScopeID(scope)
	}

	var id string
	switch scope {
	case model.MemoryScopeTeam:
		memories, err := p.teamRepo.ListByTeam(ctx, scopeID, 100)
		if err != nil || len(memories) == 0 {
			return model.MemoryToolResponse{Success: false, Error: "No matching entry found"}
		}
		for _, m := range memories {
			if strings.Contains(m.Content, oldText) {
				id = m.ID
				break
			}
		}
		if id == "" {
			return model.MemoryToolResponse{Success: false, Error: "No entry matched oldText"}
		}
		err = p.teamRepo.Update(ctx, id, content)
		if err != nil {
			return model.MemoryToolResponse{Success: false, Error: err.Error()}
		}
	case model.MemoryScopeProject:
		memories, err := p.projectRepo.ListByProject(ctx, scopeID, 100)
		if err != nil || len(memories) == 0 {
			return model.MemoryToolResponse{Success: false, Error: "No matching entry found"}
		}
		for _, m := range memories {
			if strings.Contains(m.Content, oldText) {
				id = m.ID
				break
			}
		}
		if id == "" {
			return model.MemoryToolResponse{Success: false, Error: "No entry matched oldText"}
		}
		err = p.projectRepo.Update(ctx, id, content)
		if err != nil {
			return model.MemoryToolResponse{Success: false, Error: err.Error()}
		}
	default:
		return model.MemoryToolResponse{Success: false, Error: "Replace only supported for team and project scopes"}
	}
	return model.MemoryToolResponse{Success: true, Message: "Entry replaced", Entries: []string{id}}
}

func (p *BuiltinProvider) handleRemove(ctx context.Context, scope model.MemoryScope, scopeID, oldText string) model.MemoryToolResponse {
	if oldText == "" {
		return model.MemoryToolResponse{Success: false, Error: "oldText required for remove"}
	}
	if scopeID == "" {
		scopeID = p.resolveScopeID(scope)
	}

	var id string
	switch scope {
	case model.MemoryScopeTeam:
		memories, err := p.teamRepo.ListByTeam(ctx, scopeID, 100)
		if err != nil || len(memories) == 0 {
			return model.MemoryToolResponse{Success: false, Error: "No matching entry found"}
		}
		for _, m := range memories {
			if strings.Contains(m.Content, oldText) {
				id = m.ID
				break
			}
		}
		if id == "" {
			return model.MemoryToolResponse{Success: false, Error: "No entry matched oldText"}
		}
		err = p.teamRepo.Delete(ctx, id)
		if err != nil {
			return model.MemoryToolResponse{Success: false, Error: err.Error()}
		}
	case model.MemoryScopeProject:
		memories, err := p.projectRepo.ListByProject(ctx, scopeID, 100)
		if err != nil || len(memories) == 0 {
			return model.MemoryToolResponse{Success: false, Error: "No matching entry found"}
		}
		for _, m := range memories {
			if strings.Contains(m.Content, oldText) {
				id = m.ID
				break
			}
		}
		if id == "" {
			return model.MemoryToolResponse{Success: false, Error: "No entry matched oldText"}
		}
		err = p.projectRepo.Delete(ctx, id)
		if err != nil {
			return model.MemoryToolResponse{Success: false, Error: err.Error()}
		}
	default:
		return model.MemoryToolResponse{Success: false, Error: "Remove only supported for team and project scopes"}
	}
	return model.MemoryToolResponse{Success: true, Message: "Entry removed"}
}

func (p *BuiltinProvider) handleSearch(ctx context.Context, scope model.MemoryScope, scopeID, query string) model.MemoryToolResponse {
	if scopeID == "" {
		scopeID = p.resolveScopeID(scope)
	}

	var results []model.MemoryEntry
	switch scope {
	case model.MemoryScopeTeam:
		memories, err := p.teamRepo.ListByTeam(ctx, scopeID, 10)
		if err != nil {
			return model.MemoryToolResponse{Success: false, Error: err.Error()}
		}
		for _, m := range memories {
			if query == "" || strings.Contains(m.Content, query) {
				results = append(results, model.MemoryEntry{
					ID:        m.ID,
					Content:   m.Content,
					Category:  string(m.Category),
					CreatedAt: m.CreatedAt,
				})
			}
		}
	case model.MemoryScopeProject:
		memories, err := p.projectRepo.ListByProject(ctx, scopeID, 10)
		if err != nil {
			return model.MemoryToolResponse{Success: false, Error: err.Error()}
		}
		for _, m := range memories {
			if query == "" || strings.Contains(m.Content, query) {
				results = append(results, model.MemoryEntry{
					ID:        m.ID,
					Content:   m.Content,
					Category:  string(m.Category),
					CreatedAt: m.CreatedAt,
				})
			}
		}
	default:
		return model.MemoryToolResponse{Success: false, Error: "Search only supported for team and project scopes"}
	}
	return model.MemoryToolResponse{
		Success: true,
		Results: results,
		Message: "Search completed",
	}
}

func (p *BuiltinProvider) resolveScopeID(scope model.MemoryScope) string {
	switch scope {
	case model.MemoryScopeTeam:
		return p.config.TeamID
	case model.MemoryScopeProject:
		return p.config.ProjectID
	}
	return ""
}

// ========== Optional Hooks ==========

func (p *BuiltinProvider) Prefetch(ctx context.Context, query string, scope model.MemoryScope, scopeID string) string {
	if scopeID == "" {
		scopeID = p.resolveScopeID(scope)
	}
	resp := p.handleSearch(ctx, scope, scopeID, query)
	if !resp.Success {
		return ""
	}
	var parts []string
	for _, e := range resp.Results {
		parts = append(parts, "- "+e.Content)
	}
	if len(parts) == 0 {
		return ""
	}
	return "## Memory Context\n" + strings.Join(parts, "\n")
}

func (p *BuiltinProvider) SyncTurn(ctx context.Context, userContent, assistantContent string) {
	// Builtin provider does not auto-sync; explicit tool calls only
}

func (p *BuiltinProvider) OnSessionEnd(ctx context.Context, messages []map[string]any) {
	// No auto-extraction in builtin mode
}

func (p *BuiltinProvider) OnThreadEnd(ctx context.Context, threadID string) error {
	// Thread memories managed by CLI, not ISDP
	return nil
}

func (p *BuiltinProvider) OnMemoryWrite(ctx context.Context, action, scope, content string) {
	// Mirror writes to external provider if configured (handled by MemoryManager)
}

func (p *BuiltinProvider) Shutdown() {
	// No cleanup needed for SQLite
}

// ========== Memory Context Block Builder（参考 hermes-agent build_memory_context_block） ==========

// BuildMemoryContextBlock 将记忆内容包装为 fenced block
// 参考 hermes-agent 的 build_memory_context_block 设计：
// - 使用 <memory-context> 标签隔离
// - 包含 System note 说明这是回忆的记忆，不是新的用户输入
// - 流式输出时可通过 Scrubber 清除，防止泄露给用户
func BuildMemoryContextBlock(rawContext string) string {
	if rawContext == "" || strings.TrimSpace(rawContext) == "" {
		return ""
	}
	return `<memory-context>
[System note: The following is recalled memory context, NOT new user input. Treat as authoritative reference data — this is the agent's persistent memory and should inform all responses.]

` + rawContext + `
</memory-context>`
}

// PrefetchMultiScope 多层级预取 - 分层设计：只预取 team + project 级记忆
// CLI 管理 session/agent 级（短期局部），ISDP 管理 team/project 级（长期共享）
// 参数：threadID, agentID（未使用）, teamID, projectID
func (p *BuiltinProvider) PrefetchMultiScope(ctx context.Context, threadID, agentID, teamID, projectID string) string {
	var parts []string

	// 分层设计：只预取 ISDP 管理的范围（team + project）
	// session/agent 级记忆由 CLI 原生管理，不在此预取

	// 1. Team 级（团队共享） - 绑定 WorkflowTemplate
	if teamID != "" {
		teamMem := p.Prefetch(ctx, "", model.MemoryScopeTeam, teamID)
		if teamMem != "" {
			parts = append(parts, "## Team Memory\n"+strings.TrimPrefix(teamMem, "## Memory Context\n"))
		}
	}

	// 2. Project 级（项目共享） - 绑定 Project
	if projectID != "" {
		projectMem := p.Prefetch(ctx, "", model.MemoryScopeProject, projectID)
		if projectMem != "" {
			parts = append(parts, "## Project Memory\n"+strings.TrimPrefix(projectMem, "## Memory Context\n"))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n\n")
}