# 开发范式融入 ISDP 平台实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现技能库（Skill Library）基础能力，包括数据库表、后端 API、Agent 与 Skill 关联管理、前端页面。

**Architecture:** 采用分层架构：Model → Repo → Service → API Handler。数据存储使用 MySQL，遵循项目现有的命名和结构规范。

**Tech Stack:** Go 1.21+ / Gin / MySQL / React 18 / TypeScript / Ant Design 5

---

## 文件结构

**新增文件：**

```
isdp/
├── sql-change/migrations/
│   └── 202603210001_add_skill_tables.sql      # 数据库迁移脚本
├── internal/
│   ├── model/
│   │   └── skill.go                           # Skill 数据模型
│   ├── repo/
│   │   └── skill.go                           # Skill Repository
│   ├── service/
│   │   └── skill/
│   │       └── service.go                     # Skill Service
│   └── api/
│       └── skill_handler.go                   # Skill API Handler
└── web/src/
    ├── pages/
    │   └── SkillLibrary/
    │       └── index.tsx                      # 技能库页面
    └── types/
        └── index.ts                           # 新增 Skill 类型定义
```

**修改文件：**

```
isdp/
├── internal/
│   └── api/
│       └── router.go (如有)                   # 注册路由
└── web/src/
    ├── App.tsx                                # 添加路由
    ├── api/client.ts                          # 添加 Skill API
    └── layouts/MainLayout.tsx                 # 添加菜单项
```

---

## Task 1: 数据库迁移脚本

**Files:**
- Create: `isdp/sql-change/migrations/202603210001_add_skill_tables.sql`

- [ ] **Step 1: 创建迁移脚本文件**

```sql
-- isdp/sql-change/migrations/202603210001_add_skill_tables.sql
-- 变更说明：添加技能库相关表
-- 作者：ISDP Team
-- 日期：2026-03-21

-- 设置字符集
SET NAMES utf8mb4;

-- ----------------------------
-- Skill 表
-- ----------------------------
DROP TABLE IF EXISTS skills;
CREATE TABLE skills (
    id VARCHAR(64) NOT NULL COMMENT 'Skill唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT 'Skill名称(唯一标识)',
    display_name VARCHAR(255) COMMENT '显示名称',
    description TEXT COMMENT '描述',
    type VARCHAR(50) DEFAULT 'skill' COMMENT '类型(skill/rule)',
    category VARCHAR(100) COMMENT '分类',

    -- 来源信息
    source_type VARCHAR(50) NOT NULL COMMENT '来源类型(built_in/uploaded/federated)',
    source_registry_id VARCHAR(64) COMMENT '联邦来源ID',
    author_id VARCHAR(64) COMMENT '创建者ID',
    project_id VARCHAR(64) COMMENT '所属项目ID',

    -- 安装信息
    install_source JSON COMMENT '不同智能体的安装地址',

    -- 兼容性
    supported_agents JSON COMMENT '支持的智能体列表',

    -- 版本
    version VARCHAR(50) DEFAULT '1.0.0' COMMENT '版本号',

    -- 统计数据
    use_count INT DEFAULT 0 COMMENT '使用次数',
    star_count INT DEFAULT 0 COMMENT '点赞数',
    favorite_count INT DEFAULT 0 COMMENT '收藏数',

    -- 状态
    status VARCHAR(50) DEFAULT 'active' COMMENT '状态(active/deprecated)',
    is_public TINYINT DEFAULT 0 COMMENT '是否公开(0-否,1-是)',

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_skills_name (name),
    KEY idx_skills_type (type),
    KEY idx_skills_source_type (source_type),
    KEY idx_skills_category (category),
    KEY idx_skills_project_id (project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='技能表';

-- ----------------------------
-- AgentRole 与 Skill 关联表
-- ----------------------------
DROP TABLE IF EXISTS agent_skill_bindings;
CREATE TABLE agent_skill_bindings (
    id VARCHAR(64) NOT NULL COMMENT '关联唯一标识符',
    agent_role_id VARCHAR(64) NOT NULL COMMENT 'AgentRole ID',
    skill_id VARCHAR(64) NOT NULL COMMENT 'Skill ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_binding (agent_role_id, skill_id),
    KEY idx_agent_skill_bindings_agent_role_id (agent_role_id),
    KEY idx_agent_skill_bindings_skill_id (skill_id),
    CONSTRAINT fk_binding_agent FOREIGN KEY (agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    CONSTRAINT fk_binding_skill FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Agent与Skill关联表';

-- ----------------------------
-- Skill 收藏记录表
-- ----------------------------
DROP TABLE IF EXISTS skill_favorites;
CREATE TABLE skill_favorites (
    id VARCHAR(64) NOT NULL COMMENT '收藏记录唯一标识符',
    skill_id VARCHAR(64) NOT NULL COMMENT 'Skill ID',
    user_id VARCHAR(64) NOT NULL COMMENT '用户ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_favorite (skill_id, user_id),
    KEY idx_skill_favorites_skill_id (skill_id),
    KEY idx_skill_favorites_user_id (user_id),
    CONSTRAINT fk_favorite_skill FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Skill收藏记录表';

-- 回滚语句（如需回滚执行以下语句）
-- DROP TABLE IF EXISTS skill_favorites;
-- DROP TABLE IF EXISTS agent_skill_bindings;
-- DROP TABLE IF EXISTS skills;
```

- [ ] **Step 2: 执行迁移脚本**

Run: 在 MySQL 客户端执行迁移脚本
Expected: 表创建成功，无错误

- [ ] **Step 3: 提交**

```bash
git add isdp/sql-change/migrations/202603210001_add_skill_tables.sql
git commit -m "feat(db): add skills, agent_skill_bindings, skill_favorites tables"
```

---

## Task 2: Skill 数据模型

**Files:**
- Create: `isdp/internal/model/skill.go`

- [ ] **Step 1: 创建 Skill 模型文件**

```go
// isdp/internal/model/skill.go
package model

import (
	"time"

	"github.com/google/uuid"
)

// SkillType Skill类型
type SkillType string

const (
	SkillTypeSkill SkillType = "skill"
	SkillTypeRule  SkillType = "rule"
)

// SkillSourceType 来源类型
type SkillSourceType string

const (
	SkillSourceBuiltIn   SkillSourceType = "built_in"
	SkillSourceUploaded  SkillSourceType = "uploaded"
	SkillSourceFederated SkillSourceType = "federated"
)

// SkillStatus Skill状态
type SkillStatus string

const (
	SkillStatusActive     SkillStatus = "active"
	SkillStatusDeprecated SkillStatus = "deprecated"
)

// InstallSource 安装源配置
type InstallSource map[string]string // agent_type -> url

// Skill 技能模型
type Skill struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	DisplayName   string          `json:"display_name,omitempty"`
	Description   string          `json:"description,omitempty"`
	Type          SkillType       `json:"type"`
	Category      string          `json:"category,omitempty"`

	// 来源信息
	SourceType       SkillSourceType `json:"source_type"`
	SourceRegistryID uuid.UUID       `json:"source_registry_id,omitempty"`
	AuthorID         uuid.UUID       `json:"author_id,omitempty"`
	ProjectID        uuid.UUID       `json:"project_id,omitempty"`

	// 安装信息
	InstallSource InstallSource `json:"install_source,omitempty"`

	// 兼容性
	SupportedAgents []string `json:"supported_agents,omitempty"`

	// 版本
	Version string `json:"version"`

	// 统计数据
	UseCount      int `json:"use_count"`
	StarCount     int `json:"star_count"`
	FavoriteCount int `json:"favorite_count"`

	// 状态
	Status   string `json:"status"`
	IsPublic bool   `json:"is_public"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Skill) TableName() string {
	return "skills"
}

// AgentSkillBinding Agent与Skill关联
type AgentSkillBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agent_role_id"`
	SkillID     uuid.UUID `json:"skill_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (a *AgentSkillBinding) TableName() string {
	return "agent_skill_bindings"
}

// SkillFavorite Skill收藏记录
type SkillFavorite struct {
	ID        uuid.UUID `json:"id"`
	SkillID   uuid.UUID `json:"skill_id"`
	UserID    uuid.UUID `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *SkillFavorite) TableName() string {
	return "skill_favorites"
}

// CreateSkillRequest 创建Skill请求
type CreateSkillRequest struct {
	Name           string          `json:"name" binding:"required"`
	DisplayName    string          `json:"display_name"`
	Description    string          `json:"description"`
	Type           SkillType       `json:"type"`
	Category       string          `json:"category"`
	SourceType     SkillSourceType `json:"source_type" binding:"required"`
	InstallSource  InstallSource   `json:"install_source"`
	SupportedAgents []string       `json:"supported_agents"`
	Version        string          `json:"version"`
	IsPublic       bool            `json:"is_public"`
}

// UpdateSkillRequest 更新Skill请求
type UpdateSkillRequest struct {
	DisplayName     string         `json:"display_name"`
	Description     string         `json:"description"`
	Type            SkillType      `json:"type"`
	Category        string         `json:"category"`
	InstallSource   InstallSource  `json:"install_source"`
	SupportedAgents []string       `json:"supported_agents"`
	Version         string         `json:"version"`
	Status          string         `json:"status"`
	IsPublic        bool           `json:"is_public"`
}

// BindSkillRequest 绑定Skill请求
type BindSkillRequest struct {
	SkillIDs []uuid.UUID `json:"skill_ids" binding:"required"`
}

// SkillListQuery Skill列表查询参数
type SkillListQuery struct {
	Type       string `form:"type"`
	Category   string `form:"category"`
	SourceType string `form:"source_type"`
	AgentType  string `form:"agent_type"`
	Search     string `form:"search"`
	Page       int    `form:"page"`
	PageSize   int    `form:"page_size"`
}

// SkillWithBindings Skill及其绑定的Agent列表
type SkillWithBindings struct {
	*Skill
	BoundAgents []*AgentRoleConfig `json:"bound_agents,omitempty"`
}
```

- [ ] **Step 2: 验证编译**

Run: `cd D:/00-codes/isdp/isdp && go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交**

```bash
git add isdp/internal/model/skill.go
git commit -m "feat(model): add Skill, AgentSkillBinding, SkillFavorite models"
```

---

## Task 3: Skill Repository

**Files:**
- Create: `isdp/internal/repo/skill.go`

- [ ] **Step 1: 创建 Skill Repository 文件**

```go
// isdp/internal/repo/skill.go
package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// SkillRepository Skill数据访问
type SkillRepository struct {
	db *sql.DB
}

// NewSkillRepository 创建Skill Repository
func NewSkillRepository(db *sql.DB) *SkillRepository {
	return &SkillRepository{db: db}
}

// Create 创建Skill
func (r *SkillRepository) Create(ctx context.Context, skill *model.Skill) error {
	query := `
		INSERT INTO skills (id, name, display_name, description, type, category,
			source_type, source_registry_id, author_id, project_id,
			install_source, supported_agents, version,
			use_count, star_count, favorite_count, status, is_public,
			created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	installSource, _ := json.Marshal(skill.InstallSource)
	supportedAgents, _ := json.Marshal(skill.SupportedAgents)

	var sourceRegistryID, authorID, projectID interface{}
	if skill.SourceRegistryID != uuid.Nil {
		sourceRegistryID = skill.SourceRegistryID.String()
	}
	if skill.AuthorID != uuid.Nil {
		authorID = skill.AuthorID.String()
	}
	if skill.ProjectID != uuid.Nil {
		projectID = skill.ProjectID.String()
	}

	_, err := r.db.ExecContext(ctx, query,
		skill.ID.String(), skill.Name, skill.DisplayName, skill.Description, skill.Type, skill.Category,
		skill.SourceType, sourceRegistryID, authorID, projectID,
		installSource, supportedAgents, skill.Version,
		skill.UseCount, skill.StarCount, skill.FavoriteCount, skill.Status, skill.IsPublic,
		skill.CreatedAt, skill.UpdatedAt,
	)
	return err
}

// FindByID 根据ID查找
func (r *SkillRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Skill, error) {
	query := `
		SELECT id, name, display_name, description, type, category,
			source_type, source_registry_id, author_id, project_id,
			install_source, supported_agents, version,
			use_count, star_count, favorite_count, status, is_public,
			created_at, updated_at
		FROM skills WHERE id = ?
	`

	skill := &model.Skill{}
	var idStr string
	var installSource, supportedAgents []byte
	var sourceRegistryID, authorID, projectID, displayName, description, category sql.NullString

	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &skill.Name, &displayName, &description, &skill.Type, &skill.Category,
		&skill.SourceType, &sourceRegistryID, &authorID, &projectID,
		&installSource, &supportedAgents, &skill.Version,
		&skill.UseCount, &skill.StarCount, &skill.FavoriteCount, &skill.Status, &skill.IsPublic,
		&skill.CreatedAt, &skill.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find skill: %w", err)
	}

	skill.ID, _ = uuid.Parse(idStr)
	if displayName.Valid {
		skill.DisplayName = displayName.String
	}
	if description.Valid {
		skill.Description = description.String
	}
	if category.Valid {
		skill.Category = category.String
	}
	if sourceRegistryID.Valid {
		skill.SourceRegistryID, _ = uuid.Parse(sourceRegistryID.String)
	}
	if authorID.Valid {
		skill.AuthorID, _ = uuid.Parse(authorID.String)
	}
	if projectID.Valid {
		skill.ProjectID, _ = uuid.Parse(projectID.String)
	}
	json.Unmarshal(installSource, &skill.InstallSource)
	json.Unmarshal(supportedAgents, &skill.SupportedAgents)

	return skill, nil
}

// FindByName 根据名称查找
func (r *SkillRepository) FindByName(ctx context.Context, name string) (*model.Skill, error) {
	query := `
		SELECT id, name, display_name, description, type, category,
			source_type, source_registry_id, author_id, project_id,
			install_source, supported_agents, version,
			use_count, star_count, favorite_count, status, is_public,
			created_at, updated_at
		FROM skills WHERE name = ?
	`

	skill := &model.Skill{}
	var idStr string
	var installSource, supportedAgents []byte
	var sourceRegistryID, authorID, projectID, displayName, description, category sql.NullString

	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&idStr, &skill.Name, &displayName, &description, &skill.Type, &skill.Category,
		&skill.SourceType, &sourceRegistryID, &authorID, &projectID,
		&installSource, &supportedAgents, &skill.Version,
		&skill.UseCount, &skill.StarCount, &skill.FavoriteCount, &skill.Status, &skill.IsPublic,
		&skill.CreatedAt, &skill.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find skill: %w", err)
	}

	skill.ID, _ = uuid.Parse(idStr)
	if displayName.Valid {
		skill.DisplayName = displayName.String
	}
	if description.Valid {
		skill.Description = description.String
	}
	if category.Valid {
		skill.Category = category.String
	}
	if sourceRegistryID.Valid {
		skill.SourceRegistryID, _ = uuid.Parse(sourceRegistryID.String)
	}
	if authorID.Valid {
		skill.AuthorID, _ = uuid.Parse(authorID.String)
	}
	if projectID.Valid {
		skill.ProjectID, _ = uuid.Parse(projectID.String)
	}
	json.Unmarshal(installSource, &skill.InstallSource)
	json.Unmarshal(supportedAgents, &skill.SupportedAgents)

	return skill, nil
}

// List 列出所有Skill
func (r *SkillRepository) List(ctx context.Context, query *model.SkillListQuery) ([]*model.Skill, int64, error) {
	baseQuery := `
		FROM skills WHERE 1=1
	`
	args := []interface{}{}

	if query.Type != "" {
		baseQuery += " AND type = ?"
		args = append(args, query.Type)
	}
	if query.Category != "" {
		baseQuery += " AND category = ?"
		args = append(args, query.Category)
	}
	if query.SourceType != "" {
		baseQuery += " AND source_type = ?"
		args = append(args, query.SourceType)
	}
	if query.Search != "" {
		baseQuery += " AND (name LIKE ? OR display_name LIKE ? OR description LIKE ?)"
		search := "%" + query.Search + "%"
		args = append(args, search, search, search)
	}

	// 获取总数
	var total int64
	countQuery := "SELECT COUNT(*) " + baseQuery
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 分页
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	selectQuery := `
		SELECT id, name, display_name, description, type, category,
			source_type, source_registry_id, author_id, project_id,
			install_source, supported_agents, version,
			use_count, star_count, favorite_count, status, is_public,
			created_at, updated_at
	` + baseQuery + " ORDER BY use_count DESC, created_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list skills: %w", err)
	}
	defer rows.Close()

	skills := make([]*model.Skill, 0)
	for rows.Next() {
		skill := &model.Skill{}
		var idStr string
		var installSource, supportedAgents []byte
		var sourceRegistryID, authorID, projectID, displayName, description, category sql.NullString

		err := rows.Scan(
			&idStr, &skill.Name, &displayName, &description, &skill.Type, &skill.Category,
			&skill.SourceType, &sourceRegistryID, &authorID, &projectID,
			&installSource, &supportedAgents, &skill.Version,
			&skill.UseCount, &skill.StarCount, &skill.FavoriteCount, &skill.Status, &skill.IsPublic,
			&skill.CreatedAt, &skill.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan skill: %w", err)
		}

		skill.ID, _ = uuid.Parse(idStr)
		if displayName.Valid {
			skill.DisplayName = displayName.String
		}
		if description.Valid {
			skill.Description = description.String
		}
		if category.Valid {
			skill.Category = category.String
		}
		if sourceRegistryID.Valid {
			skill.SourceRegistryID, _ = uuid.Parse(sourceRegistryID.String)
		}
		if authorID.Valid {
			skill.AuthorID, _ = uuid.Parse(authorID.String)
		}
		if projectID.Valid {
			skill.ProjectID, _ = uuid.Parse(projectID.String)
		}
		json.Unmarshal(installSource, &skill.InstallSource)
		json.Unmarshal(supportedAgents, &skill.SupportedAgents)
		skills = append(skills, skill)
	}

	return skills, total, nil
}

// Update 更新Skill
func (r *SkillRepository) Update(ctx context.Context, skill *model.Skill) error {
	query := `
		UPDATE skills SET
			display_name = ?, description = ?, type = ?, category = ?,
			install_source = ?, supported_agents = ?, version = ?,
			status = ?, is_public = ?, updated_at = ?
		WHERE id = ?
	`

	installSource, _ := json.Marshal(skill.InstallSource)
	supportedAgents, _ := json.Marshal(skill.SupportedAgents)
	skill.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, query,
		skill.DisplayName, skill.Description, skill.Type, skill.Category,
		installSource, supportedAgents, skill.Version,
		skill.Status, skill.IsPublic, skill.UpdatedAt,
		skill.ID.String(),
	)
	return err
}

// Delete 删除Skill
func (r *SkillRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM skills WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}

// IncrementUseCount 增加使用次数
func (r *SkillRepository) IncrementUseCount(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE skills SET use_count = use_count + 1 WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}

// IncrementStarCount 增加点赞数
func (r *SkillRepository) IncrementStarCount(ctx context.Context, id uuid.UUID, delta int) error {
	query := `UPDATE skills SET star_count = star_count + ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, delta, id.String())
	return err
}

// IncrementFavoriteCount 增加收藏数
func (r *SkillRepository) IncrementFavoriteCount(ctx context.Context, id uuid.UUID, delta int) error {
	query := `UPDATE skills SET favorite_count = favorite_count + ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, delta, id.String())
	return err
}
```

- [ ] **Step 2: 验证编译**

Run: `cd D:/00-codes/isdp/isdp && go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交**

```bash
git add isdp/internal/repo/skill.go
git commit -m "feat(repo): add SkillRepository with CRUD operations"
```

---

## Task 4: Agent-Skill Binding Repository

**Files:**
- Modify: `isdp/internal/repo/skill.go` (追加内容)

- [ ] **Step 1: 在 skill.go 中添加关联表操作方法**

在文件末尾追加以下代码：

```go
// ========== Agent-Skill Binding ==========

// AgentSkillBindingRepository Agent-Skill绑定数据访问
type AgentSkillBindingRepository struct {
	db *sql.DB
}

// NewAgentSkillBindingRepository 创建AgentSkillBinding Repository
func NewAgentSkillBindingRepository(db *sql.DB) *AgentSkillBindingRepository {
	return &AgentSkillBindingRepository{db: db}
}

// Create 创建绑定
func (r *AgentSkillBindingRepository) Create(ctx context.Context, binding *model.AgentSkillBinding) error {
	query := `
		INSERT INTO agent_skill_bindings (id, agent_role_id, skill_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		binding.ID.String(), binding.AgentRoleID.String(), binding.SkillID.String(), binding.CreatedAt,
	)
	return err
}

// FindByAgentRoleID 根据AgentRole ID查找绑定的Skill ID列表
func (r *AgentSkillBindingRepository) FindByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT skill_id FROM agent_skill_bindings WHERE agent_role_id = ?`
	rows, err := r.db.QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	skillIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var skillIDStr string
		if err := rows.Scan(&skillIDStr); err != nil {
			return nil, err
		}
		skillID, _ := uuid.Parse(skillIDStr)
		skillIDs = append(skillIDs, skillID)
	}
	return skillIDs, nil
}

// FindBySkillID 根据Skill ID查找绑定的AgentRole ID列表
func (r *AgentSkillBindingRepository) FindBySkillID(ctx context.Context, skillID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT agent_role_id FROM agent_skill_bindings WHERE skill_id = ?`
	rows, err := r.db.QueryContext(ctx, query, skillID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	agentRoleIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var agentRoleIDStr string
		if err := rows.Scan(&agentRoleIDStr); err != nil {
			return nil, err
		}
		agentRoleID, _ := uuid.Parse(agentRoleIDStr)
		agentRoleIDs = append(agentRoleIDs, agentRoleID)
	}
	return agentRoleIDs, nil
}

// DeleteByAgentRoleID 删除AgentRole的所有绑定
func (r *AgentSkillBindingRepository) DeleteByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) error {
	query := `DELETE FROM agent_skill_bindings WHERE agent_role_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String())
	return err
}

// DeleteBinding 删除特定绑定
func (r *AgentSkillBindingRepository) DeleteBinding(ctx context.Context, agentRoleID, skillID uuid.UUID) error {
	query := `DELETE FROM agent_skill_bindings WHERE agent_role_id = ? AND skill_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String(), skillID.String())
	return err
}

// ExistsBinding 检查绑定是否存在
func (r *AgentSkillBindingRepository) ExistsBinding(ctx context.Context, agentRoleID, skillID uuid.UUID) (bool, error) {
	query := `SELECT COUNT(*) FROM agent_skill_bindings WHERE agent_role_id = ? AND skill_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, agentRoleID.String(), skillID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
```

- [ ] **Step 2: 验证编译**

Run: `cd D:/00-codes/isdp/isdp && go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交**

```bash
git add isdp/internal/repo/skill.go
git commit -m "feat(repo): add AgentSkillBindingRepository for agent-skill relations"
```

---

## Task 5: Skill Service

**Files:**
- Create: `isdp/internal/service/skill/service.go`

- [ ] **Step 1: 创建 Skill Service 文件**

```go
// isdp/internal/service/skill/service.go
package skill

import (
	"context"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// Service Skill服务
type Service struct {
	skillRepo    *repo.SkillRepository
	bindingRepo  *repo.AgentSkillBindingRepository
	agentRepo    *repo.AgentConfigRepository
}

// NewService 创建Skill服务
func NewService(
	skillRepo *repo.SkillRepository,
	bindingRepo *repo.AgentSkillBindingRepository,
	agentRepo *repo.AgentConfigRepository,
) *Service {
	return &Service{
		skillRepo:   skillRepo,
		bindingRepo: bindingRepo,
		agentRepo:   agentRepo,
	}
}

// Create 创建Skill
func (s *Service) Create(ctx context.Context, req *model.CreateSkillRequest) (*model.Skill, error) {
	// 检查名称是否已存在
	_, err := s.skillRepo.FindByName(ctx, req.Name)
	if err == nil {
		return nil, fmt.Errorf("skill with name '%s' already exists", req.Name)
	}

	skill := &model.Skill{
		ID:              uuid.New(),
		Name:            req.Name,
		DisplayName:     req.DisplayName,
		Description:     req.Description,
		Type:            req.Type,
		Category:        req.Category,
		SourceType:      req.SourceType,
		InstallSource:   req.InstallSource,
		SupportedAgents: req.SupportedAgents,
		Version:         req.Version,
		Status:          string(model.SkillStatusActive),
		IsPublic:        req.IsPublic,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if skill.Type == "" {
		skill.Type = model.SkillTypeSkill
	}
	if skill.Version == "" {
		skill.Version = "1.0.0"
	}

	if err := s.skillRepo.Create(ctx, skill); err != nil {
		return nil, fmt.Errorf("failed to create skill: %w", err)
	}

	return skill, nil
}

// GetByID 根据ID获取Skill
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.Skill, error) {
	return s.skillRepo.FindByID(ctx, id)
}

// GetByName 根据名称获取Skill
func (s *Service) GetByName(ctx context.Context, name string) (*model.Skill, error) {
	return s.skillRepo.FindByName(ctx, name)
}

// List 列出Skill
func (s *Service) List(ctx context.Context, query *model.SkillListQuery) ([]*model.Skill, int64, error) {
	return s.skillRepo.List(ctx, query)
}

// Update 更新Skill
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateSkillRequest) (*model.Skill, error) {
	skill, err := s.skillRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("skill not found: %w", err)
	}

	if req.DisplayName != "" {
		skill.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		skill.Description = req.Description
	}
	if req.Type != "" {
		skill.Type = req.Type
	}
	if req.Category != "" {
		skill.Category = req.Category
	}
	if req.InstallSource != nil {
		skill.InstallSource = req.InstallSource
	}
	if req.SupportedAgents != nil {
		skill.SupportedAgents = req.SupportedAgents
	}
	if req.Version != "" {
		skill.Version = req.Version
	}
	if req.Status != "" {
		skill.Status = req.Status
	}
	skill.IsPublic = req.IsPublic

	if err := s.skillRepo.Update(ctx, skill); err != nil {
		return nil, fmt.Errorf("failed to update skill: %w", err)
	}

	return skill, nil
}

// Delete 删除Skill
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// 检查是否有Agent绑定
	agentIDs, err := s.bindingRepo.FindBySkillID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to check bindings: %w", err)
	}
	if len(agentIDs) > 0 {
		return fmt.Errorf("skill is bound to %d agents, cannot delete", len(agentIDs))
	}

	return s.skillRepo.Delete(ctx, id)
}

// BindSkills 绑定Skills到Agent
func (s *Service) BindSkills(ctx context.Context, agentRoleID uuid.UUID, skillIDs []uuid.UUID) error {
	// 验证Agent存在
	_, err := s.agentRepo.FindByID(ctx, agentRoleID)
	if err != nil {
		return fmt.Errorf("agent role not found: %w", err)
	}

	// 验证所有Skill存在
	for _, skillID := range skillIDs {
		_, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			return fmt.Errorf("skill %s not found: %w", skillID, err)
		}

		// 检查是否已绑定
		exists, err := s.bindingRepo.ExistsBinding(ctx, agentRoleID, skillID)
		if err != nil {
			return err
		}
		if exists {
			continue // 已绑定，跳过
		}

		// 创建绑定
		binding := &model.AgentSkillBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			SkillID:     skillID,
			CreatedAt:   time.Now(),
		}
		if err := s.bindingRepo.Create(ctx, binding); err != nil {
			return fmt.Errorf("failed to create binding: %w", err)
		}
	}

	return nil
}

// UnbindSkill 解绑Skill
func (s *Service) UnbindSkill(ctx context.Context, agentRoleID, skillID uuid.UUID) error {
	return s.bindingRepo.DeleteBinding(ctx, agentRoleID, skillID)
}

// GetBoundSkills 获取Agent绑定的Skills
func (s *Service) GetBoundSkills(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Skill, error) {
	skillIDs, err := s.bindingRepo.FindByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		return nil, err
	}

	skills := make([]*model.Skill, 0, len(skillIDs))
	for _, skillID := range skillIDs {
		skill, err := s.skillRepo.FindByID(ctx, skillID)
		if err != nil {
			continue // 忽略已删除的skill
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

// GetBoundAgents 获取Skill绑定的Agents
func (s *Service) GetBoundAgents(ctx context.Context, skillID uuid.UUID) ([]*model.AgentRoleConfig, error) {
	agentIDs, err := s.bindingRepo.FindBySkillID(ctx, skillID)
	if err != nil {
		return nil, err
	}

	agents := make([]*model.AgentRoleConfig, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		agent, err := s.agentRepo.FindByID(ctx, agentID)
		if err != nil {
			continue
		}
		agents = append(agents, agent)
	}
	return agents, nil
}

// IncrementUse 增加使用次数
func (s *Service) IncrementUse(ctx context.Context, id uuid.UUID) error {
	return s.skillRepo.IncrementUseCount(ctx, id)
}
```

- [ ] **Step 2: 验证编译**

Run: `cd D:/00-codes/isdp/isdp && go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交**

```bash
git add isdp/internal/service/skill/service.go
git commit -m "feat(service): add SkillService with CRUD and binding operations"
```

---

## Task 6: Skill API Handler

**Files:**
- Create: `isdp/internal/api/skill_handler.go`

- [ ] **Step 1: 创建 Skill API Handler 文件**

```go
// isdp/internal/api/skill_handler.go
package api

import (
	"net/http"
	"strconv"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/skill"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SkillHandler Skill API处理器
type SkillHandler struct {
	skillSvc *skill.Service
}

// NewSkillHandler 创建处理器
func NewSkillHandler(skillSvc *skill.Service) *SkillHandler {
	return &SkillHandler{
		skillSvc: skillSvc,
	}
}

// List 列出所有Skill
func (h *SkillHandler) List(c *gin.Context) {
	var query model.SkillListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	skills, total, err := h.skillSvc.List(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  skills,
		"total": total,
		"page":  query.Page,
		"page_size": query.PageSize,
	})
}

// Get 获取单个Skill
func (h *SkillHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	skill, err := h.skillSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "skill not found"})
		return
	}

	c.JSON(http.StatusOK, skill)
}

// Create 创建Skill
func (h *SkillHandler) Create(c *gin.Context) {
	var req model.CreateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	skill, err := h.skillSvc.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, skill)
}

// Update 更新Skill
func (h *SkillHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.UpdateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	skill, err := h.skillSvc.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, skill)
}

// Delete 删除Skill
func (h *SkillHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.skillSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetBoundAgents 获取Skill绑定的Agent列表
func (h *SkillHandler) GetBoundAgents(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	agents, err := h.skillSvc.GetBoundAgents(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"count":  len(agents),
	})
}

// BindSkillsRequest 绑定Skill请求
type BindSkillsRequest struct {
	SkillIDs []string `json:"skill_ids" binding:"required"`
}

// BindSkills 绑定Skills到Agent
func (h *SkillHandler) BindSkills(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("agentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var req BindSkillsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	skillIDs := make([]uuid.UUID, 0, len(req.SkillIDs))
	for _, idStr := range req.SkillIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid skill id: " + idStr})
			return
		}
		skillIDs = append(skillIDs, id)
	}

	if err := h.skillSvc.BindSkills(c.Request.Context(), agentID, skillIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "skills bound successfully"})
}

// UnbindSkill 解绑Skill
func (h *SkillHandler) UnbindSkill(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("agentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	skillID, err := uuid.Parse(c.Param("skillId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid skill id"})
		return
	}

	if err := h.skillSvc.UnbindSkill(c.Request.Context(), agentID, skillID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "skill unbound successfully"})
}

// GetAgentSkills 获取Agent绑定的Skills
func (h *SkillHandler) GetAgentSkills(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("agentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	skills, err := h.skillSvc.GetBoundSkills(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"skills": skills,
		"count":  len(skills),
	})
}

// RegisterRoutes 注册路由
func (h *SkillHandler) RegisterRoutes(r *gin.RouterGroup) {
	skills := r.Group("/skills")
	{
		skills.GET("", h.List)
		skills.POST("", h.Create)
		skills.GET("/:id", h.Get)
		skills.PUT("/:id", h.Update)
		skills.DELETE("/:id", h.Delete)
		skills.GET("/:id/agents", h.GetBoundAgents)
	}

	// Agent-Skill 绑定路由
	agents := r.Group("/agents")
	{
		agents.GET("/:agentId/skills", h.GetAgentSkills)
		agents.POST("/:agentId/skills", h.BindSkills)
		agents.DELETE("/:agentId/skills/:skillId", h.UnbindSkill)
	}
}
```

- [ ] **Step 2: 验证编译**

Run: `cd D:/00-codes/isdp/isdp && go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 3: 提交**

```bash
git add isdp/internal/api/skill_handler.go
git commit -m "feat(api): add SkillHandler with CRUD and binding endpoints"
```

---

## Task 7: 集成到主程序

**Files:**
- Modify: `isdp/cmd/server/main.go` (或相应的入口文件)

- [ ] **Step 1: 查找入口文件并添加依赖注入**

先查看入口文件结构：

```bash
# 找到 main.go 并查看其结构
```

在 main.go 中添加 SkillHandler 的初始化和路由注册。具体修改取决于现有的依赖注入模式。

- [ ] **Step 2: 验证编译和启动**

Run: `cd D:/00-codes/isdp/isdp && go build ./cmd/server`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add isdp/cmd/server/main.go
git commit -m "feat: integrate SkillHandler into main server"
```

---

## Task 8: 前端类型定义

**Files:**
- Modify: `isdp/web/src/types/index.ts`

- [ ] **Step 1: 添加 Skill 类型定义**

在 `index.ts` 文件末尾追加：

```typescript
// ========== Skill 相关类型 ==========

// Skill类型
export type SkillType = 'skill' | 'rule';

// Skill来源类型
export type SkillSourceType = 'built_in' | 'uploaded' | 'federated';

// Skill状态
export type SkillStatus = 'active' | 'deprecated';

// 安装源配置
export type InstallSource = Record<string, string>;

// Skill
export interface Skill {
  id: string;
  name: string;
  displayName?: string;
  description?: string;
  type: SkillType;
  category?: string;
  sourceType: SkillSourceType;
  sourceRegistryId?: string;
  authorId?: string;
  projectId?: string;
  installSource?: InstallSource;
  supportedAgents?: string[];
  version: string;
  useCount: number;
  starCount: number;
  favoriteCount: number;
  status: SkillStatus;
  isPublic: boolean;
  createdAt: string;
  updatedAt: string;
}

// 创建Skill请求
export interface CreateSkillRequest {
  name: string;
  displayName?: string;
  description?: string;
  type?: SkillType;
  category?: string;
  sourceType: SkillSourceType;
  installSource?: InstallSource;
  supportedAgents?: string[];
  version?: string;
  isPublic?: boolean;
}

// 更新Skill请求
export interface UpdateSkillRequest {
  displayName?: string;
  description?: string;
  type?: SkillType;
  category?: string;
  installSource?: InstallSource;
  supportedAgents?: string[];
  version?: string;
  status?: SkillStatus;
  isPublic?: boolean;
}

// Skill列表查询参数
export interface SkillListQuery {
  type?: string;
  category?: string;
  sourceType?: string;
  agentType?: string;
  search?: string;
  page?: number;
  pageSize?: number;
}

// Skill列表响应
export interface SkillListResponse {
  data: Skill[];
  total: number;
  page: number;
  pageSize: number;
}

// Agent-Skill绑定请求
export interface BindSkillsRequest {
  skillIds: string[];
}

// Agent绑定的Skills响应
export interface AgentSkillsResponse {
  skills: Skill[];
  count: number;
}

// Skill绑定的Agents响应
export interface SkillAgentsResponse {
  agents: AgentConfig[];
  count: number;
}
```

- [ ] **Step 2: 验证 TypeScript 编译**

Run: `cd D:/00-codes/isdp/isdp/web && npm run build`
Expected: 编译成功，无类型错误

- [ ] **Step 3: 提交**

```bash
git add isdp/web/src/types/index.ts
git commit -m "feat(types): add Skill related TypeScript types"
```

---

## Task 9: 前端 API Client

**Files:**
- Modify: `isdp/web/src/api/client.ts`

- [ ] **Step 1: 添加 Skill API 方法**

在 APIClient 类中添加 skills 属性：

```typescript
import type {
  // ... 现有导入
  Skill,
  CreateSkillRequest,
  UpdateSkillRequest,
  SkillListQuery,
  SkillListResponse,
  BindSkillsRequest,
  AgentSkillsResponse,
  SkillAgentsResponse,
} from '@/types';

// 在 APIClient 类中添加：

// Skill API
skills = {
  list: (query?: SkillListQuery): Promise<SkillListResponse> => {
    const params = new URLSearchParams();
    if (query?.type) params.append('type', query.type);
    if (query?.category) params.append('category', query.category);
    if (query?.sourceType) params.append('source_type', query.sourceType);
    if (query?.agentType) params.append('agent_type', query.agentType);
    if (query?.search) params.append('search', query.search);
    if (query?.page) params.append('page', query.page.toString());
    if (query?.pageSize) params.append('page_size', query.pageSize.toString());

    const url = params.toString() ? `/skills?${params.toString()}` : '/skills';
    return this.request(url, 'GET');
  },

  get: (id: string): Promise<Skill> =>
    this.request(`/skills/${id}`, 'GET'),

  create: (data: CreateSkillRequest): Promise<Skill> =>
    this.request('/skills', 'POST', data),

  update: (id: string, data: UpdateSkillRequest): Promise<Skill> =>
    this.request(`/skills/${id}`, 'PUT', data),

  delete: (id: string): Promise<void> =>
    this.request(`/skills/${id}`, 'DELETE'),

  getBoundAgents: (id: string): Promise<SkillAgentsResponse> =>
    this.request(`/skills/${id}/agents`, 'GET'),
};

// 同时修改 agents 属性，添加 skill 相关方法
// 在现有的 agents 对象中添加：

getSkills: (agentId: string): Promise<AgentSkillsResponse> =>
  this.request(`/agents/${agentId}/skills`, 'GET'),

bindSkills: (agentId: string, skillIds: string[]): Promise<{ message: string }> =>
  this.request(`/agents/${agentId}/skills`, 'POST', { skill_ids: skillIds }),

unbindSkill: (agentId: string, skillId: string): Promise<{ message: string }> =>
  this.request(`/agents/${agentId}/skills/${skillId}`, 'DELETE'),
```

- [ ] **Step 2: 验证编译**

Run: `cd D:/00-codes/isdp/isdp/web && npm run build`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add isdp/web/src/api/client.ts
git commit -m "feat(api): add Skill API client methods"
```

---

## Task 10: 技能库前端页面

**Files:**
- Create: `isdp/web/src/pages/SkillLibrary/index.tsx`

- [ ] **Step 1: 创建技能库页面组件**

```tsx
// isdp/web/src/pages/SkillLibrary/index.tsx
import React, { useEffect, useState, useCallback } from 'react';
import {
  Table, Button, Card, Modal, Form, Input, Select, message, Space, Tag, Typography,
  Tooltip, Popconfirm, Tabs, Badge, Statistic, Row, Col
} from 'antd';
import {
  PlusOutlined, EditOutlined, DeleteOutlined, SearchOutlined,
  StarOutlined, HeartOutlined, BookOutlined, CodeOutlined
} from '@ant-design/icons';
import api from '@/api/client';
import type { Skill, SkillType, SkillSourceType } from '@/types';

const { Title, Text } = Typography;
const { TabPane } = Tabs;

const SkillLibrary: React.FC = () => {
  const [skills, setSkills] = useState<Skill[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingSkill, setEditingSkill] = useState<Skill | null>(null);
  const [searchText, setSearchText] = useState('');
  const [typeFilter, setTypeFilter] = useState<string>('');
  const [sourceFilter, setSourceFilter] = useState<string>('');
  const [form] = Form.useForm();

  const loadSkills = useCallback(async () => {
    setLoading(true);
    try {
      const result = await api.skills.list({
        page,
        pageSize,
        search: searchText,
        type: typeFilter,
        sourceType: sourceFilter,
      });
      setSkills(result.data);
      setTotal(result.total);
    } catch (error) {
      message.error('加载技能列表失败');
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, searchText, typeFilter, sourceFilter]);

  useEffect(() => {
    loadSkills();
  }, [loadSkills]);

  const handleCreate = () => {
    setEditingSkill(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: Skill) => {
    setEditingSkill(record);
    form.setFieldsValue(record);
    setModalVisible(true);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.skills.delete(id);
      message.success('删除成功');
      loadSkills();
    } catch (error: any) {
      const errorData = error.response?.data;
      if (errorData?.error) {
        message.error(errorData.error);
      } else {
        message.error('删除失败');
      }
    }
  };

  const handleSubmit = async (values: any) => {
    try {
      if (editingSkill) {
        await api.skills.update(editingSkill.id, values);
        message.success('更新成功');
      } else {
        await api.skills.create(values);
        message.success('创建成功');
      }
      setModalVisible(false);
      loadSkills();
    } catch (error) {
      message.error('操作失败');
    }
  };

  const columns = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      render: (name: string, record: Skill) => (
        <Space>
          {record.type === 'rule' ? <BookOutlined /> : <CodeOutlined />}
          <span>{record.displayName || name}</span>
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 80,
      render: (type: SkillType) => (
        <Tag color={type === 'rule' ? 'purple' : 'blue'}>
          {type === 'rule' ? '规则' : '技能'}
        </Tag>
      ),
    },
    {
      title: '分类',
      dataIndex: 'category',
      key: 'category',
      width: 120,
      render: (category: string) => category || '-',
    },
    {
      title: '来源',
      dataIndex: 'sourceType',
      key: 'sourceType',
      width: 100,
      render: (sourceType: SkillSourceType) => {
        const colorMap: Record<string, string> = {
          built_in: 'green',
          uploaded: 'orange',
          federated: 'cyan',
        };
        const labelMap: Record<string, string> = {
          built_in: '内置',
          uploaded: '上传',
          federated: '联邦',
        };
        return <Tag color={colorMap[sourceType]}>{labelMap[sourceType]}</Tag>;
      },
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: (desc: string) => (
        <Tooltip title={desc}>
          {desc || '-'}
        </Tooltip>
      ),
    },
    {
      title: '统计',
      key: 'stats',
      width: 180,
      render: (_: unknown, record: Skill) => (
        <Space size="small">
          <Tooltip title="使用次数">
            <Badge count={record.useCount} showZero style={{ backgroundColor: '#52c41a' }} />
          </Tooltip>
          <Tooltip title="点赞">
            <StarOutlined /> {record.starCount}
          </Tooltip>
          <Tooltip title="收藏">
            <HeartOutlined /> {record.favoriteCount}
          </Tooltip>
        </Space>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 150,
      fixed: 'right' as const,
      render: (_: unknown, record: Skill) => (
        <Space size="small">
          <Button type="link" size="small" onClick={() => handleEdit(record)}>
            编辑
          </Button>
          <Popconfirm
            title="确定要删除这个技能吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="link" size="small" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div className="skill-library">
      <div style={{ marginBottom: 24 }}>
        <Title level={2}>技能库</Title>
        <Text type="secondary">管理可复用的技能和规则</Text>
      </div>

      <Card>
        <Tabs defaultActiveKey="all" onChange={(key) => setSourceFilter(key === 'all' ? '' : key)}>
          <TabPane tab="全部" key="all" />
          <TabPane tab="内置" key="built_in" />
          <TabPane tab="上传" key="uploaded" />
          <TabPane tab="联邦" key="federated" />
        </Tabs>

        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
          <Space>
            <Select
              placeholder="类型筛选"
              style={{ width: 120 }}
              allowClear
              onChange={(value) => setTypeFilter(value || '')}
            >
              <Select.Option value="skill">技能</Select.Option>
              <Select.Option value="rule">规则</Select.Option>
            </Select>
            <Input.Search
              placeholder="搜索技能..."
              allowClear
              style={{ width: 200 }}
              onSearch={(value) => setSearchText(value)}
            />
          </Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            新建技能
          </Button>
        </div>

        <Table
          dataSource={skills}
          columns={columns}
          rowKey="id"
          loading={loading}
          scroll={{ x: 'max-content' }}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 条`,
            onChange: (p, ps) => {
              setPage(p);
              setPageSize(ps);
            },
          }}
        />
      </Card>

      <Modal
        title={editingSkill ? '编辑技能' : '新建技能'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={600}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          initialValues={{ type: 'skill', sourceType: 'uploaded', version: '1.0.0' }}
        >
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="技能唯一标识（如 java-coding-standards）" disabled={!!editingSkill} />
          </Form.Item>

          <Form.Item name="displayName" label="显示名称">
            <Input placeholder="显示名称" />
          </Form.Item>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="type" label="类型">
                <Select>
                  <Select.Option value="skill">技能</Select.Option>
                  <Select.Option value="rule">规则</Select.Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="sourceType" label="来源">
                <Select disabled={!!editingSkill}>
                  <Select.Option value="built_in">内置</Select.Option>
                  <Select.Option value="uploaded">上传</Select.Option>
                  <Select.Option value="federated">联邦</Select.Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Form.Item name="category" label="分类">
            <Input placeholder="如：开发规范、中间件、前端" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="技能描述" />
          </Form.Item>

          <Form.Item name="version" label="版本">
            <Input placeholder="如：1.0.0" />
          </Form.Item>

          <Form.Item name="isPublic" label="公开" valuePropName="checked">
            <Select>
              <Select.Option value={false}>私有</Select.Option>
              <Select.Option value={true}>公开</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default SkillLibrary;
```

- [ ] **Step 2: 验证编译**

Run: `cd D:/00-codes/isdp/isdp/web && npm run build`
Expected: 编译成功

- [ ] **Step 3: 提交**

```bash
git add isdp/web/src/pages/SkillLibrary/index.tsx
git commit -m "feat(web): add SkillLibrary page with CRUD UI"
```

---

## Task 11: 添加路由和菜单

**Files:**
- Modify: `isdp/web/src/App.tsx`
- Modify: `isdp/web/src/layouts/MainLayout.tsx`

- [ ] **Step 1: 在 App.tsx 添加路由**

```tsx
// 在 App.tsx 中导入
import SkillLibrary from '@/pages/SkillLibrary';

// 在 Routes 中添加
<Route path="skills" element={<SkillLibrary />} />
```

- [ ] **Step 2: 在 MainLayout.tsx 添加菜单项**

在侧边菜单配置中添加技能库菜单项：

```tsx
{
  key: '/skills',
  icon: <BookOutlined />,
  label: '技能库',
}
```

- [ ] **Step 3: 验证功能**

Run: `cd D:/00-codes/isdp/isdp/web && npm run dev`
Expected: 页面可访问，菜单显示正常

- [ ] **Step 4: 提交**

```bash
git add isdp/web/src/App.tsx isdp/web/src/layouts/MainLayout.tsx
git commit -m "feat(web): add SkillLibrary route and menu item"
```

---

## Task 12: Agent 编辑页面集成 Skill 绑定

**Files:**
- Modify: `isdp/web/src/pages/AgentRoleList.tsx`

- [ ] **Step 1: 在 Agent 编辑模态框中添加 Skill 选择**

在 AgentRoleList.tsx 的编辑模态框中添加 Skill 多选组件，允许用户选择要绑定的 Skill。

- [ ] **Step 2: 实现绑定逻辑**

在提交时调用 `api.agents.bindSkills` 方法。

- [ ] **Step 3: 提交**

```bash
git add isdp/web/src/pages/AgentRoleList.tsx
git commit -m "feat(web): integrate skill binding in AgentRoleList"
```

---

## 执行选择

**计划完成并保存到 `docs/superpowers/plans/2026-03-21-dev-paradigm-integration.md`。两种执行方式：**

**1. Subagent-Driven (推荐)** - 我为每个任务派遣一个新的子代理，任务之间进行审查，快速迭代

**2. Inline Execution** - 在此会话中使用 executing-plans 批量执行，带检查点审查

**选择哪种方式？**