package model

import (
	"time"
)

// ========== Memory Scope Types ==========

// MemoryScope 记忆层级类型
// 分层设计：CLI 管理 session/agent 级（短期局部），ISDP 管理 team/project 级（长期共享）
type MemoryScope string

const (
	// MemoryScopeTeam 团队级记忆 - 多 Agent 协作共享，绑定 WorkflowTemplate
	MemoryScopeTeam MemoryScope = "team"
	// MemoryScopeProject 项目级记忆 - 跨团队共享，绑定 Project
	MemoryScopeProject MemoryScope = "project"
)

// MemoryCategory 记忆内容分类
type MemoryCategory string

const (
	MemoryCategoryPreference  MemoryCategory = "preference"  // 用户偏好
	MemoryCategoryDecision    MemoryCategory = "decision"    // 决策记录
	MemoryCategoryConvention  MemoryCategory = "convention"  // 团队约定
	MemoryCategoryContext     MemoryCategory = "context"     // 临时上下文
	MemoryCategoryTechnical   MemoryCategory = "technical"   // 技术事实
)

// ========== Memory Models ==========

// TeamMemory 团队级记忆模型（team_memories 表）
// 绑定 WorkflowTemplate（工作流模板），同一团队的所有 Agent 可见
type TeamMemory struct {
	ID        string         `json:"id"`
	TeamID    string         `json:"teamId"`     // 关联 workflow_templates.id
	Content   string         `json:"content"`
	Category  MemoryCategory `json:"category,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

func (m *TeamMemory) TableName() string {
	return "team_memories"
}

// ProjectMemory 项目级记忆模型（project_memories 表）
// 绑定 Project（项目），当前项目下的所有团队可见
// 用于保存：项目规范、技术栈约定、架构决策、代码风格约定
type ProjectMemory struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"projectId"`   // 关联 projects.id
	Content   string         `json:"content"`
	Category  MemoryCategory `json:"category,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

func (m *ProjectMemory) TableName() string {
	return "project_memories"
}

// ========== Memory Request Models ==========

// MemoryAction 记忆操作类型
type MemoryAction string

const (
	MemoryActionAdd     MemoryAction = "add"
	MemoryActionReplace MemoryAction = "replace"
	MemoryActionRemove  MemoryAction = "remove"
	MemoryActionSearch  MemoryAction = "search"
)

// MemoryToolRequest 记忆工具请求
type MemoryToolRequest struct {
	Action   MemoryAction `json:"action"`
	Scope    MemoryScope  `json:"scope"`
	ScopeID  string       `json:"scopeId,omitempty"`
	Content  string       `json:"content,omitempty"`
	OldText  string       `json:"oldText,omitempty"`
	Query    string       `json:"query,omitempty"`
	Category MemoryCategory `json:"category,omitempty"`
}

// MemoryToolResponse 记忆工具响应
type MemoryToolResponse struct {
	Success   bool          `json:"success"`
	Message   string        `json:"message,omitempty"`
	Error     string        `json:"error,omitempty"`
	Entries   []string      `json:"entries,omitempty"`
	Results   []MemoryEntry `json:"results,omitempty"`
	Usage     string        `json:"usage,omitempty"`
}

// MemoryEntry 记忆条目（统一返回格式）
type MemoryEntry struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Category  string    `json:"category,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}