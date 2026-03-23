# Agent 资产管理实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将团队基于 Claude Code 的实践范式沉淀到 ISDP 平台，包括 Command、Rule 等资产管理能力，将 Agent 角色升级为一级菜单。

**Architecture:** 采用分层架构，数据库只存元数据，内容存文件系统。新增 Command 和 Rule 两种资产类型，通过绑定关系表建立 Agent→Command/Subagent/Rule 和 Command/Subagent→Skill 的多对多关系。

**Tech Stack:** Go + Gin (后端), React + Ant Design + TypeScript (前端), MySQL/SQLite (数据库)

---

## 文件变更清单

### 后端新增
- `internal/model/command.go` - Command 模型
- `internal/model/rule.go` - Rule 模型
- `internal/repo/command.go` - Command 数据访问层
- `internal/repo/rule.go` - Rule 数据访问层
- `internal/repo/agent_command_binding.go` - Agent-Command 绑定
- `internal/repo/agent_rule_binding.go` - Agent-Rule 绑定
- `internal/repo/command_skill_binding.go` - Command-Skill 绑定
- `internal/repo/subagent_skill_binding.go` - Subagent-Skill 绑定
- `internal/service/command/service.go` - Command 业务逻辑
- `internal/service/rule/service.go` - Rule 业务逻辑
- `internal/api/command_handler.go` - Command API 处理器
- `internal/api/rule_handler.go` - Rule API 处理器
- `internal/api/agent_binding_handler.go` - Agent 绑定 API 扩展（Command/Rule 绑定）
- `sql-change/migrations/202603240001_add_command_rule_tables.sql` - 数据库迁移

### 后端修改
- `internal/model/subagent.go` - 移除 skill_id 字段
- `internal/repo/subagent.go` - 移除 skill_id 相关逻辑
- `internal/service/configgen/service.go` - 扩展配置生成逻辑
- `internal/api/subagent_handler.go` - 新增技能绑定 API
- `pkg/config/config.go` - 新增 Command/Rule 配置
- `cmd/server/main.go` - 注册新服务和 Handler

### 前端新增
- `web/src/pages/CommandList.tsx` - 命令集管理页面
- `web/src/pages/RuleList.tsx` - 规约管理页面
- `web/src/pages/PlaceholderPage.tsx` - 占位页面组件

### 前端修改
- `web/src/layouts/MainLayout.tsx` - 菜单结构调整
- `web/src/App.tsx` - 路由结构调整
- `web/src/pages/AgentRoleList.tsx` - 新增绑定 Command/Rule
- `web/src/pages/SubagentList.tsx` - 新增绑定 Skill
- `web/src/api/client.ts` - 新增 API
- `web/src/types/index.ts` - 新增类型

---

## Task 1: 数据库迁移 - 新增 Command 和 Rule 表

**Files:**
- Create: `isdp/sql-change/migrations/202603240001_add_command_rule_tables.sql`

- [ ] **Step 1: 创建数据库迁移文件**

```sql
-- 文件路径: isdp/sql-change/migrations/202603240001_add_command_rule_tables.sql
-- 变更说明：新增 Command、Rule 表及相关绑定表
-- 作者：ISDP Team
-- 日期：2026-03-24

SET NAMES utf8mb4;

-- 命令表
CREATE TABLE IF NOT EXISTS commands (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 规约表
CREATE TABLE IF NOT EXISTS rules (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    scope VARCHAR(20) NOT NULL DEFAULT 'instance',  -- public / instance
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Agent-Command 绑定表
CREATE TABLE IF NOT EXISTS agent_command_bindings (
    id VARCHAR(64) PRIMARY KEY,
    agent_role_id VARCHAR(64) NOT NULL,
    command_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_agent_command (agent_role_id, command_id),
    KEY idx_agent_role_id (agent_role_id),
    KEY idx_command_id (command_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Agent-Rule 绑定表
CREATE TABLE IF NOT EXISTS agent_rule_bindings (
    id VARCHAR(64) PRIMARY KEY,
    agent_role_id VARCHAR(64) NOT NULL,
    rule_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_agent_rule (agent_role_id, rule_id),
    KEY idx_agent_role_id (agent_role_id),
    KEY idx_rule_id (rule_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Command-Skill 绑定表
CREATE TABLE IF NOT EXISTS command_skill_bindings (
    id VARCHAR(64) PRIMARY KEY,
    command_id VARCHAR(64) NOT NULL,
    skill_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_command_skill (command_id, skill_id),
    KEY idx_command_id (command_id),
    KEY idx_skill_id (skill_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Subagent-Skill 绑定表
CREATE TABLE IF NOT EXISTS subagent_skill_bindings (
    id VARCHAR(64) PRIMARY KEY,
    subagent_id VARCHAR(64) NOT NULL,
    skill_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_subagent_skill (subagent_id, skill_id),
    KEY idx_subagent_id (subagent_id),
    KEY idx_skill_id (skill_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 迁移现有 subagents.skill_id 数据到绑定表
INSERT INTO subagent_skill_bindings (id, subagent_id, skill_id, created_at)
SELECT
    UUID() as id,
    id as subagent_id,
    skill_id,
    NOW() as created_at
FROM subagents
WHERE skill_id IS NOT NULL AND skill_id != '';

-- 回滚语句（如需回滚执行以下语句）
-- DROP TABLE IF EXISTS subagent_skill_bindings;
-- DROP TABLE IF EXISTS command_skill_bindings;
-- DROP TABLE IF EXISTS agent_rule_bindings;
-- DROP TABLE IF EXISTS agent_command_bindings;
-- DROP TABLE IF EXISTS rules;
-- DROP TABLE IF EXISTS commands;
```

- [ ] **Step 2: 执行迁移脚本**

对于 MySQL 环境：
```bash
mysqlsh --sql -h <host> -P 3306 -u <user> -p<password> -D <database> -f isdp/sql-change/migrations/202603240001_add_command_rule_tables.sql
```

对于 SQLite 环境（开发用）：迁移会在 `initDatabase` 函数中处理。

- [ ] **Step 3: 更新 SQLite 初始化函数**

修改 `isdp/cmd/server/main.go` 中的 `initDatabase` 函数，添加新表的创建语句：

```go
// 在 initDatabase 函数的 schema 字符串中添加：

-- 命令表
CREATE TABLE IF NOT EXISTS commands (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 规约表
CREATE TABLE IF NOT EXISTS rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    scope TEXT NOT NULL DEFAULT 'instance',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Agent-Command 绑定表
CREATE TABLE IF NOT EXISTS agent_command_bindings (
    id TEXT PRIMARY KEY,
    agent_role_id TEXT NOT NULL,
    command_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_role_id, command_id)
);

-- Agent-Rule 绑定表
CREATE TABLE IF NOT EXISTS agent_rule_bindings (
    id TEXT PRIMARY KEY,
    agent_role_id TEXT NOT NULL,
    rule_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_role_id, rule_id)
);

-- Command-Skill 绑定表
CREATE TABLE IF NOT EXISTS command_skill_bindings (
    id TEXT PRIMARY KEY,
    command_id TEXT NOT NULL,
    skill_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(command_id, skill_id)
);

-- Subagent-Skill 绑定表
CREATE TABLE IF NOT EXISTS subagent_skill_bindings (
    id TEXT PRIMARY KEY,
    subagent_id TEXT NOT NULL,
    skill_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(subagent_id, skill_id)
);
```

- [ ] **Step 4: 迁移数据并清理旧字段（MySQL）**

MySQL 执行以下迁移脚本（合并到同一个文件中）：

```sql
-- 迁移现有 subagents.skill_id 数据到绑定表
INSERT INTO subagent_skill_bindings (id, subagent_id, skill_id, created_at)
SELECT
    UUID() as id,
    id as subagent_id,
    skill_id,
    NOW() as created_at
FROM subagents
WHERE skill_id IS NOT NULL AND skill_id != '';

-- 验证迁移成功后，移除 skill_id 字段
ALTER TABLE subagents DROP COLUMN skill_id;
```

- [ ] **Step 5: 迁移数据并清理旧字段（SQLite）**

在 `initDatabase` 函数的 migrations 数组中添加：

```go
// 迁移 Subagent-Skill 绑定数据
// 注意：SQLite 不支持直接 DROP COLUMN，需要重建表
// 这里只创建绑定表，数据迁移在应用层处理
```

然后更新 `subagent.go` model 和 repo，移除 `skill_id` 字段。

- [ ] **Step 6: Commit**

```bash
git add isdp/sql-change/migrations/202603240001_add_command_rule_tables.sql isdp/cmd/server/main.go isdp/internal/model/subagent.go isdp/internal/repo/subagent.go
git commit -m "feat(db): add Command and Rule tables, migrate subagent skill_id to binding table"
```

---

## Task 2: 后端 Model 层 - Command 和 Rule 模型

**Files:**
- Create: `isdp/internal/model/command.go`
- Create: `isdp/internal/model/rule.go`

- [ ] **Step 1: 创建 Command 模型**

```go
// 文件路径: isdp/internal/model/command.go
package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Command Models ==========

// Command 命令模型
type Command struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (c *Command) TableName() string {
	return "commands"
}

// AgentCommandBinding Agent角色与命令绑定
type AgentCommandBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agent_role_id"`
	CommandID   uuid.UUID `json:"command_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (a *AgentCommandBinding) TableName() string {
	return "agent_command_bindings"
}

// CommandSkillBinding 命令与技能绑定
type CommandSkillBinding struct {
	ID        uuid.UUID `json:"id"`
	CommandID uuid.UUID `json:"command_id"`
	SkillID   uuid.UUID `json:"skill_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (c *CommandSkillBinding) TableName() string {
	return "command_skill_bindings"
}

// CreateCommandRequest 创建Command请求
type CreateCommandRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateCommandRequest 更新Command请求
type UpdateCommandRequest struct {
	Description string `json:"description"`
}

// CommandListQuery Command列表查询参数
type CommandListQuery struct {
	Search   string `form:"search"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// BindCommandRequest 绑定Command请求
type BindCommandRequest struct {
	CommandIDs []uuid.UUID `json:"command_ids" binding:"required"`
}

// BindSkillsToCommandRequest 绑定技能到Command请求
type BindSkillsToCommandRequest struct {
	SkillIDs []uuid.UUID `json:"skill_ids" binding:"required"`
}
```

- [ ] **Step 2: 创建 Rule 模型**

```go
// 文件路径: isdp/internal/model/rule.go
package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Rule Models ==========

// RuleScope 规约范围类型
type RuleScope string

const (
	RuleScopePublic   RuleScope = "public"   // 公共规约，自动绑定到所有Agent
	RuleScopeInstance RuleScope = "instance" // 实例规约，需手动绑定
)

// Rule 规约模型
type Rule struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Scope       RuleScope `json:"scope"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (r *Rule) TableName() string {
	return "rules"
}

// AgentRuleBinding Agent角色与规约绑定
type AgentRuleBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agent_role_id"`
	RuleID      uuid.UUID `json:"rule_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (a *AgentRuleBinding) TableName() string {
	return "agent_rule_bindings"
}

// CreateRuleRequest 创建Rule请求
type CreateRuleRequest struct {
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description"`
	Scope       RuleScope `json:"scope"`
}

// UpdateRuleRequest 更新Rule请求
type UpdateRuleRequest struct {
	Description string    `json:"description"`
	Scope       RuleScope `json:"scope"`
}

// RuleListQuery Rule列表查询参数
type RuleListQuery struct {
	Search   string `form:"search"`
	Scope    string `form:"scope"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// BindRuleRequest 绑定Rule请求
type BindRuleRequest struct {
	RuleIDs []uuid.UUID `json:"rule_ids" binding:"required"`
}
```

- [ ] **Step 3: 更新 Subagent 模型，添加绑定类型**

在 `isdp/internal/model/subagent.go` 末尾添加：

```go
// SubagentSkillBinding 子代理与技能绑定
type SubagentSkillBinding struct {
	ID         uuid.UUID `json:"id"`
	SubagentID uuid.UUID `json:"subagent_id"`
	SkillID    uuid.UUID `json:"skill_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *SubagentSkillBinding) TableName() string {
	return "subagent_skill_bindings"
}

// BindSkillsToSubagentRequest 绑定技能到Subagent请求
type BindSkillsToSubagentRequest struct {
	SkillIDs []uuid.UUID `json:"skill_ids" binding:"required"`
}
```

- [ ] **Step 4: Commit**

```bash
git add isdp/internal/model/command.go isdp/internal/model/rule.go isdp/internal/model/subagent.go
git commit -m "feat(model): add Command and Rule models with binding types"
```

---

## Task 3: 后端 Repository 层 - Command 和 Rule 数据访问

**Files:**
- Create: `isdp/internal/repo/command.go`
- Create: `isdp/internal/repo/rule.go`

- [ ] **Step 1: 创建 Command Repository**

```go
// 文件路径: isdp/internal/repo/command.go
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// CommandRepository Command数据访问
type CommandRepository struct {
	db *sql.DB
}

// NewCommandRepository 创建Command Repository
func NewCommandRepository(db *sql.DB) *CommandRepository {
	return &CommandRepository{db: db}
}

// Create 创建Command
func (r *CommandRepository) Create(ctx context.Context, command *model.Command) error {
	query := `
		INSERT INTO commands (id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		command.ID.String(), command.Name, command.Description, command.CreatedAt, command.UpdatedAt,
	)
	return err
}

// scanCommand 辅助函数，扫描Command行
func scanCommand(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.Command, error) {
	command := &model.Command{}
	var idStr string
	var description sql.NullString

	err := scanner.Scan(
		&idStr, &command.Name, &description, &command.CreatedAt, &command.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	command.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		command.Description = description.String
	}

	return command, nil
}

// FindByID 根据ID查找
func (r *CommandRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Command, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM commands WHERE id = ?
	`
	command, err := scanCommand(r.db.QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("command not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find command: %w", err)
	}
	return command, nil
}

// FindByName 根据名称查找
func (r *CommandRepository) FindByName(ctx context.Context, name string) (*model.Command, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM commands WHERE name = ?
	`
	command, err := scanCommand(r.db.QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("command not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find command: %w", err)
	}
	return command, nil
}

// List 列出Commands，支持分页和搜索
func (r *CommandRepository) List(ctx context.Context, query *model.CommandListQuery) ([]*model.Command, int64, error) {
	var conditions []string
	var args []interface{}

	if query.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ?)")
		searchPattern := "%" + query.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 计算总数
	countQuery := "SELECT COUNT(*) FROM commands " + whereClause
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count commands: %w", err)
	}

	// 分页
	page := query.Page
	pageSize := query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// 查询列表
	listQuery := `
		SELECT id, name, description, created_at, updated_at
		FROM commands ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list commands: %w", err)
	}
	defer rows.Close()

	commands := make([]*model.Command, 0)
	for rows.Next() {
		command, err := scanCommand(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan command: %w", err)
		}
		commands = append(commands, command)
	}

	return commands, total, nil
}

// Update 更新Command
func (r *CommandRepository) Update(ctx context.Context, command *model.Command) error {
	query := `
		UPDATE commands
		SET name = ?, description = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		command.Name, command.Description, command.UpdatedAt, command.ID.String(),
	)
	return err
}

// Delete 删除Command
func (r *CommandRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM commands WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}
```

- [ ] **Step 2: 创建 Rule Repository**

```go
// 文件路径: isdp/internal/repo/rule.go
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// RuleRepository Rule数据访问
type RuleRepository struct {
	db *sql.DB
}

// NewRuleRepository 创建Rule Repository
func NewRuleRepository(db *sql.DB) *RuleRepository {
	return &RuleRepository{db: db}
}

// Create 创建Rule
func (r *RuleRepository) Create(ctx context.Context, rule *model.Rule) error {
	query := `
		INSERT INTO rules (id, name, description, scope, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		rule.ID.String(), rule.Name, rule.Description, string(rule.Scope), rule.CreatedAt, rule.UpdatedAt,
	)
	return err
}

// scanRule 辅助函数，扫描Rule行
func scanRule(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.Rule, error) {
	rule := &model.Rule{}
	var idStr string
	var description sql.NullString

	err := scanner.Scan(
		&idStr, &rule.Name, &description, &rule.Scope, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	rule.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		rule.Description = description.String
	}

	return rule, nil
}

// FindByID 根据ID查找
func (r *RuleRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Rule, error) {
	query := `
		SELECT id, name, description, scope, created_at, updated_at
		FROM rules WHERE id = ?
	`
	rule, err := scanRule(r.db.QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("rule not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find rule: %w", err)
	}
	return rule, nil
}

// FindByName 根据名称查找
func (r *RuleRepository) FindByName(ctx context.Context, name string) (*model.Rule, error) {
	query := `
		SELECT id, name, description, scope, created_at, updated_at
		FROM rules WHERE name = ?
	`
	rule, err := scanRule(r.db.QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("rule not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find rule: %w", err)
	}
	return rule, nil
}

// List 列出Rules，支持分页、搜索和范围过滤
func (r *RuleRepository) List(ctx context.Context, query *model.RuleListQuery) ([]*model.Rule, int64, error) {
	var conditions []string
	var args []interface{}

	if query.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ?)")
		searchPattern := "%" + query.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}
	if query.Scope != "" {
		conditions = append(conditions, "scope = ?")
		args = append(args, query.Scope)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 计算总数
	countQuery := "SELECT COUNT(*) FROM rules " + whereClause
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count rules: %w", err)
	}

	// 分页
	page := query.Page
	pageSize := query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// 查询列表
	listQuery := `
		SELECT id, name, description, scope, created_at, updated_at
		FROM rules ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list rules: %w", err)
	}
	defer rows.Close()

	rules := make([]*model.Rule, 0)
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return rules, total, nil
}

// FindByScope 根据范围查找
func (r *RuleRepository) FindByScope(ctx context.Context, scope model.RuleScope) ([]*model.Rule, error) {
	query := `
		SELECT id, name, description, scope, created_at, updated_at
		FROM rules WHERE scope = ? ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, string(scope))
	if err != nil {
		return nil, fmt.Errorf("failed to find rules by scope: %w", err)
	}
	defer rows.Close()

	rules := make([]*model.Rule, 0)
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

// Update 更新Rule
func (r *RuleRepository) Update(ctx context.Context, rule *model.Rule) error {
	query := `
		UPDATE rules
		SET name = ?, description = ?, scope = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		rule.Name, rule.Description, string(rule.Scope), rule.UpdatedAt, rule.ID.String(),
	)
	return err
}

// Delete 删除Rule
func (r *RuleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM rules WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}
```

- [ ] **Step 3: Commit**

```bash
git add isdp/internal/repo/command.go isdp/internal/repo/rule.go
git commit -m "feat(repo): add Command and Rule repositories"
```

---

## Task 4: 后端 Repository 层 - 绑定关系表

**Files:**
- Create: `isdp/internal/repo/agent_command_binding.go`
- Create: `isdp/internal/repo/agent_rule_binding.go`
- Create: `isdp/internal/repo/command_skill_binding.go`
- Create: `isdp/internal/repo/subagent_skill_binding.go`

- [ ] **Step 1: 创建 Agent-Command Binding Repository**

```go
// 文件路径: isdp/internal/repo/agent_command_binding.go
package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// AgentCommandBindingRepository Agent-Command绑定数据访问
type AgentCommandBindingRepository struct {
	db *sql.DB
}

// NewAgentCommandBindingRepository 创建AgentCommandBinding Repository
func NewAgentCommandBindingRepository(db *sql.DB) *AgentCommandBindingRepository {
	return &AgentCommandBindingRepository{db: db}
}

// Create 创建绑定
func (r *AgentCommandBindingRepository) Create(ctx context.Context, binding *model.AgentCommandBinding) error {
	query := `
		INSERT INTO agent_command_bindings (id, agent_role_id, command_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		binding.ID.String(), binding.AgentRoleID.String(), binding.CommandID.String(), binding.CreatedAt,
	)
	return err
}

// FindByAgentRoleID 根据AgentRole ID查找绑定的Command ID列表
func (r *AgentCommandBindingRepository) FindByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT command_id FROM agent_command_bindings WHERE agent_role_id = ?`
	rows, err := r.db.QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	commandIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var commandIDStr string
		if err := rows.Scan(&commandIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan command_id: %w", err)
		}
		commandID, _ := uuid.Parse(commandIDStr)
		commandIDs = append(commandIDs, commandID)
	}
	return commandIDs, nil
}

// FindCommandsByAgentRoleID 根据AgentRole ID查找绑定的Command详情列表
func (r *AgentCommandBindingRepository) FindCommandsByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Command, error) {
	query := `
		SELECT c.id, c.name, c.description, c.created_at, c.updated_at
		FROM commands c
		INNER JOIN agent_command_bindings b ON c.id = b.command_id
		WHERE b.agent_role_id = ?
		ORDER BY c.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find commands: %w", err)
	}
	defer rows.Close()

	commands := make([]*model.Command, 0)
	for rows.Next() {
		command, err := scanCommand(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan command: %w", err)
		}
		commands = append(commands, command)
	}
	return commands, nil
}

// DeleteByAgentRoleID 删除AgentRole的所有绑定
func (r *AgentCommandBindingRepository) DeleteByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) error {
	query := `DELETE FROM agent_command_bindings WHERE agent_role_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String())
	return err
}

// DeleteBinding 删除特定绑定
func (r *AgentCommandBindingRepository) DeleteBinding(ctx context.Context, agentRoleID, commandID uuid.UUID) error {
	query := `DELETE FROM agent_command_bindings WHERE agent_role_id = ? AND command_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String(), commandID.String())
	return err
}

// ExistsBinding 检查绑定是否存在
func (r *AgentCommandBindingRepository) ExistsBinding(ctx context.Context, agentRoleID, commandID uuid.UUID) (bool, error) {
	query := `SELECT COUNT(*) FROM agent_command_bindings WHERE agent_role_id = ? AND command_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, agentRoleID.String(), commandID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindByCommandID 根据Command ID查找绑定的AgentRole ID列表
func (r *AgentCommandBindingRepository) FindByCommandID(ctx context.Context, commandID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT agent_role_id FROM agent_command_bindings WHERE command_id = ?`
	rows, err := r.db.QueryContext(ctx, query, commandID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	agentRoleIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var agentRoleIDStr string
		if err := rows.Scan(&agentRoleIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan agent_role_id: %w", err)
		}
		agentRoleID, _ := uuid.Parse(agentRoleIDStr)
		agentRoleIDs = append(agentRoleIDs, agentRoleID)
	}
	return agentRoleIDs, nil
}
```

- [ ] **Step 2: 创建 Agent-Rule Binding Repository**

```go
// 文件路径: isdp/internal/repo/agent_rule_binding.go
package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// AgentRuleBindingRepository Agent-Rule绑定数据访问
type AgentRuleBindingRepository struct {
	db *sql.DB
}

// NewAgentRuleBindingRepository 创建AgentRuleBinding Repository
func NewAgentRuleBindingRepository(db *sql.DB) *AgentRuleBindingRepository {
	return &AgentRuleBindingRepository{db: db}
}

// Create 创建绑定
func (r *AgentRuleBindingRepository) Create(ctx context.Context, binding *model.AgentRuleBinding) error {
	query := `
		INSERT INTO agent_rule_bindings (id, agent_role_id, rule_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		binding.ID.String(), binding.AgentRoleID.String(), binding.RuleID.String(), binding.CreatedAt,
	)
	return err
}

// FindByAgentRoleID 根据AgentRole ID查找绑定的Rule ID列表
func (r *AgentRuleBindingRepository) FindByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT rule_id FROM agent_rule_bindings WHERE agent_role_id = ?`
	rows, err := r.db.QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	ruleIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var ruleIDStr string
		if err := rows.Scan(&ruleIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan rule_id: %w", err)
		}
		ruleID, _ := uuid.Parse(ruleIDStr)
		ruleIDs = append(ruleIDs, ruleID)
	}
	return ruleIDs, nil
}

// FindRulesByAgentRoleID 根据AgentRole ID查找绑定的Rule详情列表
func (r *AgentRuleBindingRepository) FindRulesByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Rule, error) {
	query := `
		SELECT r.id, r.name, r.description, r.scope, r.created_at, r.updated_at
		FROM rules r
		INNER JOIN agent_rule_bindings b ON r.id = b.rule_id
		WHERE b.agent_role_id = ?
		ORDER BY r.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find rules: %w", err)
	}
	defer rows.Close()

	rules := make([]*model.Rule, 0)
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// DeleteByAgentRoleID 删除AgentRole的所有绑定
func (r *AgentRuleBindingRepository) DeleteByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) error {
	query := `DELETE FROM agent_rule_bindings WHERE agent_role_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String())
	return err
}

// DeleteBinding 删除特定绑定
func (r *AgentRuleBindingRepository) DeleteBinding(ctx context.Context, agentRoleID, ruleID uuid.UUID) error {
	query := `DELETE FROM agent_rule_bindings WHERE agent_role_id = ? AND rule_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String(), ruleID.String())
	return err
}

// ExistsBinding 检查绑定是否存在
func (r *AgentRuleBindingRepository) ExistsBinding(ctx context.Context, agentRoleID, ruleID uuid.UUID) (bool, error) {
	query := `SELECT COUNT(*) FROM agent_rule_bindings WHERE agent_role_id = ? AND rule_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, agentRoleID.String(), ruleID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindByRuleID 根据Rule ID查找绑定的AgentRole ID列表
func (r *AgentRuleBindingRepository) FindByRuleID(ctx context.Context, ruleID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT agent_role_id FROM agent_rule_bindings WHERE rule_id = ?`
	rows, err := r.db.QueryContext(ctx, query, ruleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	agentRoleIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var agentRoleIDStr string
		if err := rows.Scan(&agentRoleIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan agent_role_id: %w", err)
		}
		agentRoleID, _ := uuid.Parse(agentRoleIDStr)
		agentRoleIDs = append(agentRoleIDs, agentRoleID)
	}
	return agentRoleIDs, nil
}
```

- [ ] **Step 3: 创建 Command-Skill Binding Repository**

```go
// 文件路径: isdp/internal/repo/command_skill_binding.go
package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// CommandSkillBindingRepository Command-Skill绑定数据访问
type CommandSkillBindingRepository struct {
	db *sql.DB
}

// NewCommandSkillBindingRepository 创建CommandSkillBinding Repository
func NewCommandSkillBindingRepository(db *sql.DB) *CommandSkillBindingRepository {
	return &CommandSkillBindingRepository{db: db}
}

// Create 创建绑定
func (r *CommandSkillBindingRepository) Create(ctx context.Context, binding *model.CommandSkillBinding) error {
	query := `
		INSERT INTO command_skill_bindings (id, command_id, skill_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		binding.ID.String(), binding.CommandID.String(), binding.SkillID.String(), binding.CreatedAt,
	)
	return err
}

// FindByCommandID 根据Command ID查找绑定的Skill ID列表
func (r *CommandSkillBindingRepository) FindByCommandID(ctx context.Context, commandID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT skill_id FROM command_skill_bindings WHERE command_id = ?`
	rows, err := r.db.QueryContext(ctx, query, commandID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	skillIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var skillIDStr string
		if err := rows.Scan(&skillIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan skill_id: %w", err)
		}
		skillID, _ := uuid.Parse(skillIDStr)
		skillIDs = append(skillIDs, skillID)
	}
	return skillIDs, nil
}

// FindSkillsByCommandID 根据Command ID查找绑定的Skill详情列表
// 注意：scanSkill 函数定义在 skill.go 中，同属 repo 包可直接调用
func (r *CommandSkillBindingRepository) FindSkillsByCommandID(ctx context.Context, commandID uuid.UUID) ([]*model.Skill, error) {
	query := `
		SELECT s.id, s.name, s.description, s.tags, s.source_type, s.source_registry_id,
		       s.author_id, s.project_id, s.supported_agents, s.version, s.use_count,
		       s.status, s.is_public, s.created_at, s.updated_at
		FROM skills s
		INNER JOIN command_skill_bindings b ON s.id = b.skill_id
		WHERE b.command_id = ?
		ORDER BY s.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, commandID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find skills: %w", err)
	}
	defer rows.Close()

	skills := make([]*model.Skill, 0)
	for rows.Next() {
		skill, err := scanSkill(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan skill: %w", err)
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

// DeleteByCommandID 删除Command的所有绑定
func (r *CommandSkillBindingRepository) DeleteByCommandID(ctx context.Context, commandID uuid.UUID) error {
	query := `DELETE FROM command_skill_bindings WHERE command_id = ?`
	_, err := r.db.ExecContext(ctx, query, commandID.String())
	return err
}

// DeleteBinding 删除特定绑定
func (r *CommandSkillBindingRepository) DeleteBinding(ctx context.Context, commandID, skillID uuid.UUID) error {
	query := `DELETE FROM command_skill_bindings WHERE command_id = ? AND skill_id = ?`
	_, err := r.db.ExecContext(ctx, query, commandID.String(), skillID.String())
	return err
}

// ExistsBinding 检查绑定是否存在
func (r *CommandSkillBindingRepository) ExistsBinding(ctx context.Context, commandID, skillID uuid.UUID) (bool, error) {
	query := `SELECT COUNT(*) FROM command_skill_bindings WHERE command_id = ? AND skill_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, commandID.String(), skillID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
```

- [ ] **Step 4: 创建 Subagent-Skill Binding Repository**

```go
// 文件路径: isdp/internal/repo/subagent_skill_binding.go
package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// SubagentSkillBindingRepository Subagent-Skill绑定数据访问
type SubagentSkillBindingRepository struct {
	db *sql.DB
}

// NewSubagentSkillBindingRepository 创建SubagentSkillBinding Repository
func NewSubagentSkillBindingRepository(db *sql.DB) *SubagentSkillBindingRepository {
	return &SubagentSkillBindingRepository{db: db}
}

// Create 创建绑定
func (r *SubagentSkillBindingRepository) Create(ctx context.Context, binding *model.SubagentSkillBinding) error {
	query := `
		INSERT INTO subagent_skill_bindings (id, subagent_id, skill_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		binding.ID.String(), binding.SubagentID.String(), binding.SkillID.String(), binding.CreatedAt,
	)
	return err
}

// FindBySubagentID 根据Subagent ID查找绑定的Skill ID列表
func (r *SubagentSkillBindingRepository) FindBySubagentID(ctx context.Context, subagentID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT skill_id FROM subagent_skill_bindings WHERE subagent_id = ?`
	rows, err := r.db.QueryContext(ctx, query, subagentID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	skillIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var skillIDStr string
		if err := rows.Scan(&skillIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan skill_id: %w", err)
		}
		skillID, _ := uuid.Parse(skillIDStr)
		skillIDs = append(skillIDs, skillID)
	}
	return skillIDs, nil
}

// FindSkillsBySubagentID 根据Subagent ID查找绑定的Skill详情列表
// 注意：scanSkill 函数定义在 skill.go 中，同属 repo 包可直接调用
func (r *SubagentSkillBindingRepository) FindSkillsBySubagentID(ctx context.Context, subagentID uuid.UUID) ([]*model.Skill, error) {
	query := `
		SELECT s.id, s.name, s.description, s.tags, s.source_type, s.source_registry_id,
		       s.author_id, s.project_id, s.supported_agents, s.version, s.use_count,
		       s.status, s.is_public, s.created_at, s.updated_at
		FROM skills s
		INNER JOIN subagent_skill_bindings b ON s.id = b.skill_id
		WHERE b.subagent_id = ?
		ORDER BY s.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, subagentID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find skills: %w", err)
	}
	defer rows.Close()

	skills := make([]*model.Skill, 0)
	for rows.Next() {
		skill, err := scanSkill(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan skill: %w", err)
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

// DeleteBySubagentID 删除Subagent的所有绑定
func (r *SubagentSkillBindingRepository) DeleteBySubagentID(ctx context.Context, subagentID uuid.UUID) error {
	query := `DELETE FROM subagent_skill_bindings WHERE subagent_id = ?`
	_, err := r.db.ExecContext(ctx, query, subagentID.String())
	return err
}

// DeleteBinding 删除特定绑定
func (r *SubagentSkillBindingRepository) DeleteBinding(ctx context.Context, subagentID, skillID uuid.UUID) error {
	query := `DELETE FROM subagent_skill_bindings WHERE subagent_id = ? AND skill_id = ?`
	_, err := r.db.ExecContext(ctx, query, subagentID.String(), skillID.String())
	return err
}

// ExistsBinding 检查绑定是否存在
func (r *SubagentSkillBindingRepository) ExistsBinding(ctx context.Context, subagentID, skillID uuid.UUID) (bool, error) {
	query := `SELECT COUNT(*) FROM subagent_skill_bindings WHERE subagent_id = ? AND skill_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, subagentID.String(), skillID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindBySkillID 根据Skill ID查找绑定的Subagent ID列表
func (r *SubagentSkillBindingRepository) FindBySkillID(ctx context.Context, skillID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT subagent_id FROM subagent_skill_bindings WHERE skill_id = ?`
	rows, err := r.db.QueryContext(ctx, query, skillID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	subagentIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var subagentIDStr string
		if err := rows.Scan(&subagentIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan subagent_id: %w", err)
		}
		subagentID, _ := uuid.Parse(subagentIDStr)
		subagentIDs = append(subagentIDs, subagentID)
	}
	return subagentIDs, nil
}
```

- [ ] **Step 5: Commit**

```bash
git add isdp/internal/repo/agent_command_binding.go isdp/internal/repo/agent_rule_binding.go isdp/internal/repo/command_skill_binding.go isdp/internal/repo/subagent_skill_binding.go
git commit -m "feat(repo): add binding repositories for Command, Rule and Skill"
```

---

## Task 5: 后端 Service 层 - Command 和 Rule 业务逻辑

**Files:**
- Create: `isdp/internal/service/command/service.go`
- Create: `isdp/internal/service/rule/service.go`

- [ ] **Step 1: 创建 Command Service**

```go
// 文件路径: isdp/internal/service/command/service.go
package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrCommandNameExists 命令名称已存在错误
var ErrCommandNameExists = fmt.Errorf("command name already exists")

// Service Command业务服务
type Service struct {
	repo               *repo.CommandRepository
	skillBindingRepo   *repo.CommandSkillBindingRepository
	agentBindingRepo   *repo.AgentCommandBindingRepository
	agentRepo          *repo.AgentConfigRepository
	skillRepo          *repo.SkillRepository
	storagePath        string
	logger             *zap.Logger
}

// NewService 创建Command Service
func NewService(
	commandRepo *repo.CommandRepository,
	skillBindingRepo *repo.CommandSkillBindingRepository,
	agentBindingRepo *repo.AgentCommandBindingRepository,
	agentRepo *repo.AgentConfigRepository,
	skillRepo *repo.SkillRepository,
	storagePath string,
	logger *zap.Logger,
) *Service {
	return &Service{
		repo:             commandRepo,
		skillBindingRepo: skillBindingRepo,
		agentBindingRepo: agentBindingRepo,
		agentRepo:        agentRepo,
		skillRepo:        skillRepo,
		storagePath:      storagePath,
		logger:           logger,
	}
}

// Create 创建Command
func (s *Service) Create(ctx context.Context, req *model.CreateCommandRequest) (*model.Command, error) {
	// 检查名称格式
	if !isValidName(req.Name) {
		return nil, errors.New("名称只能包含小写字母、数字和中划线，且必须以字母开头")
	}

	// 检查名称是否重复
	existing, err := s.repo.FindByName(ctx, req.Name)
	if err == nil && existing != nil {
		return nil, ErrCommandNameExists
	}

	command := &model.Command{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, command); err != nil {
		return nil, fmt.Errorf("创建命令失败: %w", err)
	}

	s.logger.Info("创建命令成功",
		zap.String("id", command.ID.String()),
		zap.String("name", command.Name),
	)

	return command, nil
}

// Get 根据ID获取Command
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Command, error) {
	return s.repo.FindByID(ctx, id)
}

// List 列出Commands
func (s *Service) List(ctx context.Context, query *model.CommandListQuery) ([]*model.Command, int64, error) {
	return s.repo.List(ctx, query)
}

// Update 更新Command
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateCommandRequest) (*model.Command, error) {
	command, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("命令不存在: %w", err)
	}

	if req.Description != "" {
		command.Description = req.Description
	}
	command.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, command); err != nil {
		return nil, fmt.Errorf("更新命令失败: %w", err)
	}

	s.logger.Info("更新命令成功",
		zap.String("id", command.ID.String()),
		zap.String("name", command.Name),
	)

	return command, nil
}

// Delete 删除Command
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// 检查是否有Agent绑定
	agentRoleIDs, err := s.agentBindingRepo.FindByCommandID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if len(agentRoleIDs) > 0 {
		return fmt.Errorf("无法删除命令：该命令已被 %d 个Agent绑定", len(agentRoleIDs))
	}

	// 删除文件
	command, err := s.repo.FindByID(ctx, id)
	if err == nil && command != nil {
		filePath := fmt.Sprintf("%s/%s.md", s.storagePath, command.Name)
		os.Remove(filePath) // 忽略错误
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除命令失败: %w", err)
	}

	s.logger.Info("删除命令成功",
		zap.String("id", id.String()),
	)

	return nil
}

// BindSkills 绑定技能到Command
func (s *Service) BindSkills(ctx context.Context, commandID uuid.UUID, skillIDs []uuid.UUID) error {
	if len(skillIDs) == 0 {
		return errors.New("技能ID列表不能为空")
	}

	// 验证Command是否存在
	_, err := s.repo.FindByID(ctx, commandID)
	if err != nil {
		return fmt.Errorf("命令不存在: %w", err)
	}

	// 验证所有Skill存在
	for _, skillID := range skillIDs {
		_, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			return fmt.Errorf("技能 %s 不存在: %w", skillID.String(), err)
		}
	}

	// 创建绑定
	for _, skillID := range skillIDs {
		exists, err := s.skillBindingRepo.ExistsBinding(ctx, commandID, skillID)
		if err != nil {
			return err
		}
		if exists {
			continue
		}

		binding := &model.CommandSkillBinding{
			ID:        uuid.New(),
			CommandID: commandID,
			SkillID:   skillID,
			CreatedAt: time.Now(),
		}
		if err := s.skillBindingRepo.Create(ctx, binding); err != nil {
			return err
		}
	}

	s.logger.Info("绑定技能到Command成功",
		zap.String("command_id", commandID.String()),
		zap.Int("skill_count", len(skillIDs)),
	)

	return nil
}

// GetSkills 获取Command绑定的技能
func (s *Service) GetSkills(ctx context.Context, commandID uuid.UUID) ([]*model.Skill, error) {
	return s.skillBindingRepo.FindSkillsByCommandID(ctx, commandID)
}

// UnbindSkill 解绑技能
func (s *Service) UnbindSkill(ctx context.Context, commandID, skillID uuid.UUID) error {
	exists, err := s.skillBindingRepo.ExistsBinding(ctx, commandID, skillID)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("绑定关系不存在")
	}

	if err := s.skillBindingRepo.DeleteBinding(ctx, commandID, skillID); err != nil {
		return fmt.Errorf("解除绑定失败: %w", err)
	}

	s.logger.Info("解除技能绑定成功",
		zap.String("command_id", commandID.String()),
		zap.String("skill_id", skillID.String()),
	)

	return nil
}

// isValidName 校验名称格式
func isValidName(name string) bool {
	if len(name) == 0 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9-]*$`, name)
	return matched
}

// GetStoragePath 获取存储路径
func (s *Service) GetStoragePath() string {
	return s.storagePath
}
```

- [ ] **Step 2: 创建 Rule Service**

```go
// 文件路径: isdp/internal/service/rule/service.go
package rule

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrRuleNameExists 规约名称已存在错误
var ErrRuleNameExists = fmt.Errorf("rule name already exists")

// Service Rule业务服务
type Service struct {
	repo             *repo.RuleRepository
	agentBindingRepo *repo.AgentRuleBindingRepository
	agentRepo        *repo.AgentConfigRepository
	storagePath      string
	logger           *zap.Logger
}

// NewService 创建Rule Service
func NewService(
	ruleRepo *repo.RuleRepository,
	agentBindingRepo *repo.AgentRuleBindingRepository,
	agentRepo *repo.AgentConfigRepository,
	storagePath string,
	logger *zap.Logger,
) *Service {
	return &Service{
		repo:             ruleRepo,
		agentBindingRepo: agentBindingRepo,
		agentRepo:        agentRepo,
		storagePath:      storagePath,
		logger:           logger,
	}
}

// Create 创建Rule
func (s *Service) Create(ctx context.Context, req *model.CreateRuleRequest) (*model.Rule, error) {
	// 检查名称格式
	if !isValidName(req.Name) {
		return nil, errors.New("名称只能包含小写字母、数字和中划线，且必须以字母开头")
	}

	// 检查名称是否重复
	existing, err := s.repo.FindByName(ctx, req.Name)
	if err == nil && existing != nil {
		return nil, ErrRuleNameExists
	}

	// 设置默认范围
	if req.Scope == "" {
		req.Scope = model.RuleScopeInstance
	}

	rule := &model.Rule{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		Scope:       req.Scope,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("创建规约失败: %w", err)
	}

	// 如果是公共规约，自动绑定到所有现有的Agent
	if rule.Scope == model.RuleScopePublic {
		agents, err := s.agentRepo.List(ctx)
		if err == nil {
			for _, agent := range agents {
				binding := &model.AgentRuleBinding{
					ID:          uuid.New(),
					AgentRoleID: agent.ID,
					RuleID:      rule.ID,
					CreatedAt:   time.Now(),
				}
				s.agentBindingRepo.Create(ctx, binding)
			}
		}
	}

	s.logger.Info("创建规约成功",
		zap.String("id", rule.ID.String()),
		zap.String("name", rule.Name),
		zap.String("scope", string(rule.Scope)),
	)

	return rule, nil
}

// Get 根据ID获取Rule
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Rule, error) {
	return s.repo.FindByID(ctx, id)
}

// List 列出Rules
func (s *Service) List(ctx context.Context, query *model.RuleListQuery) ([]*model.Rule, int64, error) {
	return s.repo.List(ctx, query)
}

// Update 更新Rule
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateRuleRequest) (*model.Rule, error) {
	rule, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("规约不存在: %w", err)
	}

	if req.Description != "" {
		rule.Description = req.Description
	}
	if req.Scope != "" {
		rule.Scope = req.Scope
	}
	rule.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, rule); err != nil {
		return nil, fmt.Errorf("更新规约失败: %w", err)
	}

	s.logger.Info("更新规约成功",
		zap.String("id", rule.ID.String()),
		zap.String("name", rule.Name),
	)

	return rule, nil
}

// Delete 删除Rule
// 设计决策：与 Command 不同，Rule 删除时会自动删除所有绑定关系
// 原因：Rule 可能是公共规约，绑定了多个 Agent，允许删除可以减少管理负担
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// 检查是否有Agent绑定
	agentRoleIDs, err := s.agentBindingRepo.FindByRuleID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}

	// 删除文件
	rule, err := s.repo.FindByID(ctx, id)
	if err == nil && rule != nil {
		filePath := fmt.Sprintf("%s/%s.md", s.storagePath, rule.Name)
		os.Remove(filePath) // 忽略错误
	}

	// 先删除所有绑定
	for _, agentRoleID := range agentRoleIDs {
		s.agentBindingRepo.DeleteBinding(ctx, agentRoleID, id)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除规约失败: %w", err)
	}

	s.logger.Info("删除规约成功",
		zap.String("id", id.String()),
		zap.Int("deleted_bindings", len(agentRoleIDs)),
	)

	return nil
}

// isValidName 校验名称格式
func isValidName(name string) bool {
	if len(name) == 0 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9-]*$`, name)
	return matched
}

// GetStoragePath 获取存储路径
func (s *Service) GetStoragePath() string {
	return s.storagePath
}

// BindPublicRulesToAgent 为新建的Agent绑定所有公共规约
func (s *Service) BindPublicRulesToAgent(ctx context.Context, agentRoleID uuid.UUID) error {
	publicRules, err := s.repo.FindByScope(ctx, model.RuleScopePublic)
	if err != nil {
		return fmt.Errorf("获取公共规约失败: %w", err)
	}

	for _, rule := range publicRules {
		binding := &model.AgentRuleBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			RuleID:      rule.ID,
			CreatedAt:   time.Now(),
		}
		if err := s.agentBindingRepo.Create(ctx, binding); err != nil {
			s.logger.Warn("绑定公共规约失败",
				zap.String("agent_role_id", agentRoleID.String()),
				zap.String("rule_id", rule.ID.String()),
				zap.Error(err),
			)
		}
	}

	return nil
}
```

- [ ] **Step 3: Commit**

```bash
git add isdp/internal/service/command/service.go isdp/internal/service/rule/service.go
git commit -m "feat(service): add Command and Rule services"
```

---

## Task 6: 后端 API 层 - Command 和 Rule 处理器

**Files:**
- Create: `isdp/internal/api/command_handler.go`
- Create: `isdp/internal/api/rule_handler.go`

- [ ] **Step 1: 创建 Command Handler**

参考现有的 `subagent_handler.go` 模式，创建 `command_handler.go`，包含：
- CRUD 操作
- 文件上传处理
- 技能绑定 API

- [ ] **Step 2: 创建 Rule Handler**

参考 `subagent_handler.go` 模式，创建 `rule_handler.go`，包含：
- CRUD 操作
- 文件上传处理
- scope 字段处理

- [ ] **Step 3: Commit**

```bash
git add isdp/internal/api/command_handler.go isdp/internal/api/rule_handler.go
git commit -m "feat(api): add Command and Rule handlers"
```

---

## Task 7: 后端 API 层 - Agent 绑定扩展

**Files:**
- Create: `isdp/internal/api/agent_binding_handler.go`

**背景**: 设计文档 7.1 节要求新增 Agent 扩展 API，用于管理 Agent 与 Command/Rule 的绑定关系。

- [ ] **Step 1: 创建 Agent 绑定 Handler**

```go
// 文件路径: isdp/internal/api/agent_binding_handler.go
package api

import (
	"net/http"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/command"
	"github.com/anthropic/isdp/internal/service/rule"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AgentBindingHandler Agent绑定API处理器
type AgentBindingHandler struct {
	commandSvc       *command.Service
	ruleSvc          *rule.Service
	commandBindingRepo *repo.AgentCommandBindingRepository
	ruleBindingRepo    *repo.AgentRuleBindingRepository
}

// NewAgentBindingHandler 创建AgentBindingHandler
func NewAgentBindingHandler(
	commandSvc *command.Service,
	ruleSvc *rule.Service,
	commandBindingRepo *repo.AgentCommandBindingRepository,
	ruleBindingRepo *repo.AgentRuleBindingRepository,
) *AgentBindingHandler {
	return &AgentBindingHandler{
		commandSvc:         commandSvc,
		ruleSvc:            ruleSvc,
		commandBindingRepo: commandBindingRepo,
		ruleBindingRepo:    ruleBindingRepo,
	}
}

// GetCommands 获取Agent绑定的Commands
func (h *AgentBindingHandler) GetCommands(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	commands, err := h.commandBindingRepo.FindCommandsByAgentRoleID(c.Request.Context(), agentRoleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"commands": commands,
		"count":    len(commands),
	})
}

// BindCommands 绑定Commands到Agent
func (h *AgentBindingHandler) BindCommands(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var req model.BindCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 创建绑定
	for _, commandID := range req.CommandIDs {
		exists, err := h.commandBindingRepo.ExistsBinding(c.Request.Context(), agentRoleID, commandID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if exists {
			continue
		}

		binding := &model.AgentCommandBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			CommandID:   commandID,
			CreatedAt:   time.Now(),
		}
		if err := h.commandBindingRepo.Create(c.Request.Context(), binding); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.Status(http.StatusNoContent)
}

// UnbindCommand 解绑Command
func (h *AgentBindingHandler) UnbindCommand(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	commandID, err := uuid.Parse(c.Param("command_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid command id"})
		return
	}

	if err := h.commandBindingRepo.DeleteBinding(c.Request.Context(), agentRoleID, commandID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetRules 获取Agent绑定的Rules
func (h *AgentBindingHandler) GetRules(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	rules, err := h.ruleBindingRepo.FindRulesByAgentRoleID(c.Request.Context(), agentRoleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"count": len(rules),
	})
}

// BindRules 绑定Rules到Agent
func (h *AgentBindingHandler) BindRules(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var req model.BindRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 创建绑定
	for _, ruleID := range req.RuleIDs {
		exists, err := h.ruleBindingRepo.ExistsBinding(c.Request.Context(), agentRoleID, ruleID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if exists {
			continue
		}

		binding := &model.AgentRuleBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			RuleID:      ruleID,
			CreatedAt:   time.Now(),
		}
		if err := h.ruleBindingRepo.Create(c.Request.Context(), binding); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.Status(http.StatusNoContent)
}

// UnbindRule 解绑Rule
func (h *AgentBindingHandler) UnbindRule(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	ruleID, err := uuid.Parse(c.Param("rule_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}

	if err := h.ruleBindingRepo.DeleteBinding(c.Request.Context(), agentRoleID, ruleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// RegisterRoutes 注册路由
func (h *AgentBindingHandler) RegisterRoutes(r *gin.RouterGroup) {
	agents := r.Group("/agents")
	{
		// Command 绑定
		agents.GET("/:id/commands", h.GetCommands)
		agents.POST("/:id/commands", h.BindCommands)
		agents.DELETE("/:id/commands/:command_id", h.UnbindCommand)

		// Rule 绑定
		agents.GET("/:id/rules", h.GetRules)
		agents.POST("/:id/rules", h.BindRules)
		agents.DELETE("/:id/rules/:rule_id", h.UnbindRule)
	}
}
```

- [ ] **Step 2: 在 main.go 中注册 AgentBindingHandler**

在 `cmd/server/main.go` 中添加：

```go
// Agent 绑定 Handler
agentBindingHandler := api.NewAgentBindingHandler(
	commandSvc, ruleSvc,
	agentCommandBindingRepo, agentRuleBindingRepo,
)
agentBindingHandler.RegisterRoutes(v1)
```

- [ ] **Step 3: Commit**

```bash
git add isdp/internal/api/agent_binding_handler.go isdp/cmd/server/main.go
git commit -m "feat(api): add Agent binding API for Command and Rule"
```

---

## Task 8: 后端配置和服务注册

**Files:**
- Modify: `isdp/pkg/config/config.go`
- Modify: `isdp/cmd/server/main.go`

- [ ] **Step 1: 添加 Command 和 Rule 配置**

在 `isdp/pkg/config/config.go` 中添加：

```go
// CommandConfig 命令配置
type CommandConfig struct {
	// UploadMaxSize 命令文件上传最大大小，单位 MB，默认 2
	UploadMaxSize int `mapstructure:"upload_max_size"`

	// StoragePath 命令文件存储路径，默认 ./commands
	StoragePath string `mapstructure:"storage_path"`
}

// RuleConfig 规约配置
type RuleConfig struct {
	// UploadMaxSize 规约文件上传最大大小，单位 MB，默认 2
	UploadMaxSize int `mapstructure:"upload_max_size"`

	// StoragePath 规约文件存储路径，默认 ./rules
	StoragePath string `mapstructure:"storage_path"`
}

// 在 Config 结构体中添加
Command CommandConfig `mapstructure:"command"`
Rule    RuleConfig    `mapstructure:"rule"`

// 添加默认值
func (c *CommandConfig) GetUploadMaxSize() int64 {
	if c.UploadMaxSize <= 0 {
		return 2 * 1024 * 1024
	}
	return int64(c.UploadMaxSize) * 1024 * 1024
}

func (c *CommandConfig) GetStoragePath() string {
	if c.StoragePath == "" {
		return "./commands"
	}
	return c.StoragePath
}

func (c *RuleConfig) GetUploadMaxSize() int64 {
	if c.UploadMaxSize <= 0 {
		return 2 * 1024 * 1024
	}
	return int64(c.UploadMaxSize) * 1024 * 1024
}

func (c *RuleConfig) GetStoragePath() string {
	if c.StoragePath == "" {
		return "./rules"
	}
	return c.StoragePath
}

// 在 setDefaults 中添加
viper.SetDefault("command.upload_max_size", 2)
viper.SetDefault("command.storage_path", "./commands")
viper.SetDefault("rule.upload_max_size", 2)
viper.SetDefault("rule.storage_path", "./rules")
```

- [ ] **Step 2: 更新 main.go 注册新服务**

在 `isdp/cmd/server/main.go` 中：
1. 导入新服务包
2. 创建新的 Repository 实例
3. 创建新的 Service 实例
4. 注册新的 Handler

- [ ] **Step 3: Commit**

```bash
git add isdp/pkg/config/config.go isdp/cmd/server/main.go
git commit -m "feat(config): add Command and Rule configuration and service registration"
```

---

## Task 9: 前端类型定义和 API 客户端

**Files:**
- Modify: `isdp/web/src/types/index.ts`
- Modify: `isdp/web/src/api/client.ts`

- [ ] **Step 1: 添加 Command 和 Rule 类型**

在 `isdp/web/src/types/index.ts` 末尾添加：

```typescript
// ========== Command 相关类型 ==========

// Command
export interface Command {
  id: string;
  name: string;
  description?: string;
  createdAt: string;
  updatedAt: string;
}

// 创建Command请求
export interface CreateCommandRequest {
  name: string;
  description?: string;
}

// 更新Command请求
export interface UpdateCommandRequest {
  description?: string;
}

// Command列表查询参数
export interface CommandListQuery {
  search?: string;
  page?: number;
  pageSize?: number;
}

// Command列表响应
export interface CommandListResponse {
  data: Command[];
  total: number;
  page: number;
  pageSize: number;
}

// ========== Rule 相关类型 ==========

// RuleScope 规约范围
export type RuleScope = 'public' | 'instance';

// Rule
export interface Rule {
  id: string;
  name: string;
  description?: string;
  scope: RuleScope;
  createdAt: string;
  updatedAt: string;
}

// 创建Rule请求
export interface CreateRuleRequest {
  name: string;
  description?: string;
  scope?: RuleScope;
}

// 更新Rule请求
export interface UpdateRuleRequest {
  description?: string;
  scope?: RuleScope;
}

// Rule列表查询参数
export interface RuleListQuery {
  search?: string;
  scope?: RuleScope;
  page?: number;
  pageSize?: number;
}

// Rule列表响应
export interface RuleListResponse {
  data: Rule[];
  total: number;
  page: number;
  pageSize: number;
}
```

- [ ] **Step 2: 更新 API 客户端**

在 `isdp/web/src/api/client.ts` 中添加 Command 和 Rule API 方法。

- [ ] **Step 3: Commit**

```bash
git add isdp/web/src/types/index.ts isdp/web/src/api/client.ts
git commit -m "feat(web): add Command and Rule types and API client"
```

---

## Task 10: 前端菜单和路由重构

**Files:**
- Modify: `isdp/web/src/layouts/MainLayout.tsx`
- Modify: `isdp/web/src/App.tsx`
- Create: `isdp/web/src/pages/PlaceholderPage.tsx`

- [ ] **Step 1: 创建占位页面组件**

```tsx
// 文件路径: isdp/web/src/pages/PlaceholderPage.tsx
import React from 'react';
import { Result, Button } from 'antd';
import { useNavigate } from 'react-router-dom';

interface PlaceholderPageProps {
  title: string;
  description?: string;
}

const PlaceholderPage: React.FC<PlaceholderPageProps> = ({ title, description }) => {
  const navigate = useNavigate();

  return (
    <Result
      status="info"
      title={title}
      subTitle={description || '该功能正在开发中，敬请期待'}
      extra={
        <Button type="primary" onClick={() => navigate('/dashboard')}>
          返回首页
        </Button>
      }
    />
  );
};

export default PlaceholderPage;
```

- [ ] **Step 2: 重构菜单结构**

修改 `MainLayout.tsx`，将 Agent 角色改为一级菜单，添加二级子菜单。

- [ ] **Step 3: 重构路由配置**

修改 `App.tsx`，添加新的路由配置和重定向规则。

- [ ] **Step 4: Commit**

```bash
git add isdp/web/src/layouts/MainLayout.tsx isdp/web/src/App.tsx isdp/web/src/pages/PlaceholderPage.tsx
git commit -m "feat(web): restructure menu and routes for Agent assets management"
```

---

## Task 11: 前端页面 - Command 和 Rule 管理

**Files:**
- Create: `isdp/web/src/pages/CommandList.tsx`
- Create: `isdp/web/src/pages/RuleList.tsx`

- [ ] **Step 1: 创建 CommandList 页面**

参考 `SubagentList.tsx` 模式，创建命令集管理页面，包含：
- 列表展示
- 上传功能
- 编辑弹窗
- 技能绑定

- [ ] **Step 2: 创建 RuleList 页面**

参考 `SubagentList.tsx` 模式，创建规约管理页面，包含：
- 列表展示（含 scope 标签）
- 上传功能
- 编辑弹窗

- [ ] **Step 3: Commit**

```bash
git add isdp/web/src/pages/CommandList.tsx isdp/web/src/pages/RuleList.tsx
git commit -m "feat(web): add Command and Rule management pages"
```

---

## Task 12: 扩展 AgentRoleList 页面 - 添加资产绑定

**Files:**
- Modify: `isdp/web/src/pages/AgentRoleList.tsx`

- [ ] **Step 1: 在编辑弹窗中添加 Command 绑定**

添加绑定命令的多选组件。

- [ ] **Step 2: 在编辑弹窗中添加 Rule 绑定**

添加绑定规约的多选组件，区分公共规约和实例规约。

- [ ] **Step 3: Commit**

```bash
git add isdp/web/src/pages/AgentRoleList.tsx
git commit -m "feat(web): add Command and Rule binding to AgentRoleList"
```

---

## Task 13: 扩展 SubagentList 页面 - 添加技能绑定

**Files:**
- Modify: `isdp/web/src/pages/SubagentList.tsx`
- Modify: `isdp/internal/api/subagent_handler.go`
- Modify: `isdp/internal/service/subagent/service.go`

- [ ] **Step 1: 后端添加技能绑定 API**

在 SubagentHandler 和 Service 中添加技能绑定相关方法。

- [ ] **Step 2: 前端添加技能绑定功能**

在编辑弹窗中添加绑定技能的多选组件。

- [ ] **Step 3: Commit**

```bash
git add isdp/internal/api/subagent_handler.go isdp/internal/service/subagent/service.go isdp/web/src/pages/SubagentList.tsx
git commit -m "feat: add Skill binding to Subagent"
```

---

## Task 14: 扩展配置生成服务

**Files:**
- Modify: `isdp/internal/service/configgen/service.go`

- [ ] **Step 1: 扩展配置生成逻辑**

修改 `configgen/service.go`，添加：
- 复制 Command 文件
- 复制 Rule 文件
- 通过绑定关系收集所有 Skill（去重）

- [ ] **Step 2: 更新 API 响应**

在配置生成响应中添加 Command 和 Rule 的统计信息。

- [ ] **Step 3: Commit**

```bash
git add isdp/internal/service/configgen/service.go
git commit -m "feat(configgen): extend config generation to include Command and Rule"
```

---

## Task 15: 单元测试

**Files:**
- Create: `isdp/internal/service/command/service_test.go`
- Create: `isdp/internal/service/rule/service_test.go`
- Create: `isdp/internal/repo/command_test.go`
- Create: `isdp/internal/repo/rule_test.go`

- [ ] **Step 1: 创建 Command Service 单元测试**

```go
// 文件路径: isdp/internal/service/command/service_test.go
package command

import (
	"context"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestIsValidName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid lowercase", "my-command", true},
		{"valid with numbers", "command-123", true},
		{"invalid starts with number", "123-command", false},
		{"invalid uppercase", "MyCommand", false},
		{"invalid special char", "my_command", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
```

- [ ] **Step 2: 创建 Rule Service 单元测试**

类似的测试结构，覆盖 Rule 的 CRUD 和 scope 相关逻辑。

- [ ] **Step 3: 运行测试**

```bash
cd isdp && go test ./internal/service/command/... ./internal/service/rule/... -v
```

- [ ] **Step 4: Commit**

```bash
git add isdp/internal/service/command/service_test.go isdp/internal/service/rule/service_test.go
git commit -m "test: add unit tests for Command and Rule services"
```

---

## Task 16: 集成测试和文档更新

**Files:**
- Modify: `docs/CHANGELOG.md`
- Modify: `isdp/configs/config.yaml.example`

- [ ] **Step 1: 更新 CHANGELOG**

添加本次变更的记录。

- [ ] **Step 2: 更新配置模板**

在 `config.yaml.example` 中添加 Command 和 Rule 的配置示例。

- [ ] **Step 3: 运行测试**

```bash
cd isdp && go test ./internal/... -v
cd web && npm run lint && npm run build
```

- [ ] **Step 4: 最终 Commit**

```bash
git add docs/CHANGELOG.md isdp/configs/config.yaml.example
git commit -m "docs: update CHANGELOG and config template for Agent assets management"
```

---

## 执行顺序

1. Task 1: 数据库迁移（包含 subagent.skill_id 迁移）
2. Task 2-4: 后端 Model 和 Repository 层
3. Task 5-6: 后端 Service 和 API 层
4. Task 7: Agent 绑定 API 扩展
5. Task 8: 后端配置和服务注册
6. Task 9: 前端类型和 API 客户端
7. Task 10: 前端菜单和路由重构
8. Task 11: 前端管理页面
9. Task 12-13: 扩展现有页面
10. Task 14: 配置生成服务扩展
11. Task 15: 单元测试
12. Task 16: 集成测试和文档

---

## 验证清单

### 后端验证

- [ ] **后端编译通过**
  ```bash
  cd isdp && go build ./...
  ```
  期望结果：无编译错误

- [ ] **后端单元测试通过**
  ```bash
  cd isdp && go test ./internal/... -v
  ```
  期望结果：所有测试 PASS

- [ ] **API 接口可访问**
  ```bash
  # 测试 Command API
  curl http://localhost:8080/api/v1/commands
  # 测试 Rule API
  curl http://localhost:8080/api/v1/rules
  ```
  期望结果：返回空列表或现有数据

- [ ] **Agent 绑定 API 正常**
  ```bash
  # 测试获取 Agent 绑定的 Commands
  curl http://localhost:8080/api/v1/agents/{agent_id}/commands
  # 测试获取 Agent 绑定的 Rules
  curl http://localhost:8080/api/v1/agents/{agent_id}/rules
  ```

### 前端验证

- [ ] **前端构建成功**
  ```bash
  cd web && npm run build
  ```
  期望结果：构建成功，无错误

- [ ] **前端 Lint 通过**
  ```bash
  cd web && npm run lint
  ```
  期望结果：无 Lint 错误

- [ ] **菜单结构正确**
  验证方法：登录前端，检查侧边栏菜单是否包含：
  - Agent 角色（一级菜单）
    - 角色管理
    - 命令集
    - 子代理
    - 技能库
    - 规约
    - 插件（占位）
    - 钩子（占位）
    - 设置（占位）

- [ ] **路由跳转正常**
  验证方法：点击各菜单项，检查 URL 是否正确变化

### 功能验证

- [ ] **文件上传功能正常**
  验证方法：
  1. 进入命令集管理页面
  2. 上传一个 .md 文件
  3. 检查文件是否保存到 `{data-dir}/commands/{name}.md`

- [ ] **绑定关系保存正确**
  验证方法：
  1. 编辑一个 Agent 角色
  2. 绑定 Command 和 Rule
  3. 保存后刷新页面，检查绑定是否保留

- [ ] **公共规约自动绑定**
  验证方法：
  1. 创建一个 scope='public' 的 Rule
  2. 创建一个新的 Agent 角色
  3. 检查新 Agent 是否自动绑定了该 Rule

- [ ] **配置生成包含所有资产**
  验证方法：
  1. 为 Agent 绑定 Command、Subagent、Rule
  2. 触发配置生成
  3. 检查生成的目录结构是否包含：
     - `.claude/commands/`
     - `.claude/agents/`
     - `.claude/skills/`
     - `.claude/rules/`