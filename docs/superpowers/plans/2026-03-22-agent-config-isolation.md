# Agent配置隔离与Subagent融合 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将配置生成从项目粒度改为Agent角色粒度，每个Agent角色拥有独立的配置目录，并支持Subagent范式管理。

**Architecture:** 配置生成服务重构为按Agent角色粒度生成，存储在全局配置池 `{data-dir}/agents/{agent-role-id}/.claude/`。新增Subagent模型实现子代理管理，前端添加配置生成入口和子代理管理页面。

**Tech Stack:** Go + Gin + GORM (后端), React + Ant Design + TypeScript (前端)

---

## File Structure

```
isdp/
├── internal/
│   ├── model/
│   │   ├── subagent.go              # [CREATE] Subagent模型
│   │   └── agent_config.go          # [MODIFY] 添加配置生成字段
│   ├── repo/
│   │   ├── subagent.go              # [CREATE] Subagent Repository
│   │   └── agent_subagent_binding.go # [CREATE] Agent-Subagent绑定Repository
│   ├── service/
│   │   ├── configgen/
│   │   │   ├── service.go           # [MODIFY] 重构为Agent角色粒度
│   │   │   └── downloader.go        # [MODIFY] 支持Subagent文件处理
│   │   └── subagent/
│   │       └── service.go           # [CREATE] Subagent服务
│   └── api/
│       ├── configgen_handler.go     # [MODIFY] 添加Agent配置生成API
│       └── subagent_handler.go      # [CREATE] Subagent API处理器
├── pkg/config/
│   └── config.go                    # [MODIFY] 添加AgentConfig配置项
├── sql-change/migrations/
│   └── 202603220001_add_subagent_tables.sql # [CREATE] 数据库迁移
└── web/src/
    ├── api/
    │   └── client.ts                # [MODIFY] 添加Subagent API
    ├── types/
    │   └── index.ts                 # [MODIFY] 添加Subagent类型
    └── pages/
        ├── AgentRoleList.tsx        # [MODIFY] 添加配置生成按钮
        └── SubagentList.tsx         # [CREATE] Subagent管理页面
```

---

## Phase 1: 数据层

### Task 1: 数据库迁移脚本

**Files:**
- Create: `isdp/sql-change/migrations/202603220001_add_subagent_tables.sql`

- [ ] **Step 1: 创建迁移文件**

```sql
-- isdp/sql-change/migrations/202603220001_add_subagent_tables.sql
-- 变更说明：添加子代理相关表和Agent配置生成字段
-- 作者：ISDP Team
-- 日期：2026-03-22

SET NAMES utf8mb4;

-- ----------------------------
-- Subagent 表
-- ----------------------------
DROP TABLE IF EXISTS subagents;
CREATE TABLE subagents (
    id VARCHAR(64) NOT NULL COMMENT 'Subagent唯一标识符',
    name VARCHAR(255) NOT NULL COMMENT 'Subagent名称',
    description TEXT COMMENT '描述',
    content LONGTEXT NOT NULL COMMENT 'Markdown内容',

    -- 来源信息
    skill_id VARCHAR(64) COMMENT '所属技能包ID（可选）',

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_subagents_name (name),
    KEY idx_subagents_skill_id (skill_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='子代理表';

-- ----------------------------
-- Agent-Subagent 绑定表
-- ----------------------------
DROP TABLE IF EXISTS agent_subagent_bindings;
CREATE TABLE agent_subagent_bindings (
    id VARCHAR(64) NOT NULL COMMENT '绑定唯一标识符',
    agent_role_id VARCHAR(64) NOT NULL COMMENT 'AgentRole ID',
    subagent_id VARCHAR(64) NOT NULL COMMENT 'Subagent ID',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',

    PRIMARY KEY (id),
    UNIQUE KEY uk_binding (agent_role_id, subagent_id),
    KEY idx_agent_subagent_bindings_agent_role_id (agent_role_id),
    KEY idx_agent_subagent_bindings_subagent_id (subagent_id),
    CONSTRAINT fk_agent_subagent_agent FOREIGN KEY (agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    CONSTRAINT fk_agent_subagent_subagent FOREIGN KEY (subagent_id) REFERENCES subagents(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Agent与Subagent绑定表';

-- ----------------------------
-- AgentRoleConfig 添加配置生成字段
-- ----------------------------
ALTER TABLE agent_configs
    ADD COLUMN config_generated_at TIMESTAMP NULL COMMENT '配置最后生成时间',
    ADD COLUMN config_path VARCHAR(512) NULL COMMENT '配置目录路径';

-- 回滚语句（如需回滚执行以下语句）
-- DROP TABLE IF EXISTS agent_subagent_bindings;
-- DROP TABLE IF EXISTS subagents;
-- ALTER TABLE agent_configs DROP COLUMN config_generated_at, DROP COLUMN config_path;
```

- [ ] **Step 2: 验证迁移文件**

检查SQL语法正确性，确保与现有表结构兼容。

---

### Task 2: Subagent 模型

**Files:**
- Create: `isdp/internal/model/subagent.go`

- [ ] **Step 1: 创建Subagent模型**

```go
// isdp/internal/model/subagent.go
package model

import (
	"time"

	"github.com/google/uuid"
)

// Subagent 子代理模型
type Subagent struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Content     string    `json:"content"`           // Markdown内容
	SkillID     uuid.UUID `json:"skillId,omitempty"` // 所属技能包ID（可选）
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (s *Subagent) TableName() string {
	return "subagents"
}

// AgentSubagentBinding Agent角色与子代理绑定
type AgentSubagentBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agentRoleId"`
	SubagentID  uuid.UUID `json:"subagentId"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (a *AgentSubagentBinding) TableName() string {
	return "agent_subagent_bindings"
}

// CreateSubagentRequest 创建Subagent请求
type CreateSubagentRequest struct {
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description"`
	Content     string    `json:"content" binding:"required"`
	SkillID     uuid.UUID `json:"skillId"`
}

// UpdateSubagentRequest 更新Subagent请求
type UpdateSubagentRequest struct {
	Description string `json:"description"`
	Content     string `json:"content"`
}

// SubagentListQuery Subagent列表查询参数
type SubagentListQuery struct {
	Search   string `form:"search"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// BindSubagentRequest 绑定Subagent请求
type BindSubagentRequest struct {
	SubagentIDs []uuid.UUID `json:"subagent_ids" binding:"required"`
}
```

- [ ] **Step 2: 验证模型编译**

```bash
cd isdp && go build ./internal/model/...
```

---

### Task 3: 扩展 AgentRoleConfig 模型

**Files:**
- Modify: `isdp/internal/model/agent_config.go`

- [ ] **Step 1: 添加配置生成字段**

在 `AgentRoleConfig` 结构体中添加：

```go
// AgentRoleConfig Agent角色配置模型
type AgentRoleConfig struct {
    // ... 现有字段 ...

    // 配置生成相关字段
    ConfigGeneratedAt *time.Time `json:"configGeneratedAt,omitempty"` // 配置最后生成时间
    ConfigPath        string     `json:"configPath,omitempty"`         // 配置目录路径
}
```

- [ ] **Step 2: 验证模型编译**

```bash
cd isdp && go build ./internal/model/...
```

---

### Task 4: Subagent Repository

**Files:**
- Create: `isdp/internal/repo/subagent.go`
- Create: `isdp/internal/repo/agent_subagent_binding.go`

- [ ] **Step 1: 创建Subagent Repository**

```go
// isdp/internal/repo/subagent.go
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// SubagentRepository Subagent数据访问
type SubagentRepository struct {
	db *sql.DB
}

// NewSubagentRepository 创建Subagent Repository
func NewSubagentRepository(db *sql.DB) *SubagentRepository {
	return &SubagentRepository{db: db}
}

// Create 创建Subagent
func (r *SubagentRepository) Create(ctx context.Context, subagent *model.Subagent) error {
	query := `
		INSERT INTO subagents (id, name, description, content, skill_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	var skillID interface{}
	if subagent.SkillID != uuid.Nil {
		skillID = subagent.SkillID.String()
	}
	_, err := r.db.ExecContext(ctx, query,
		subagent.ID.String(), subagent.Name, subagent.Description, subagent.Content, skillID, subagent.CreatedAt, subagent.UpdatedAt,
	)
	return err
}

// FindByID 根据ID查找
func (r *SubagentRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Subagent, error) {
	query := `
		SELECT id, name, description, content, skill_id, created_at, updated_at
		FROM subagents WHERE id = ?
	`
	subagent := &model.Subagent{}
	var idStr string
	var description, skillID sql.NullString
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &subagent.Name, &description, &subagent.Content, &skillID, &subagent.CreatedAt, &subagent.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subagent not found")
		}
		return nil, err
	}
	subagent.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		subagent.Description = description.String
	}
	if skillID.Valid {
		subagent.SkillID, _ = uuid.Parse(skillID.String)
	}
	return subagent, nil
}

// FindByName 根据名称查找
func (r *SubagentRepository) FindByName(ctx context.Context, name string) (*model.Subagent, error) {
	query := `
		SELECT id, name, description, content, skill_id, created_at, updated_at
		FROM subagents WHERE name = ?
	`
	subagent := &model.Subagent{}
	var idStr string
	var description, skillID sql.NullString
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&idStr, &subagent.Name, &description, &subagent.Content, &skillID, &subagent.CreatedAt, &subagent.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("subagent not found")
		}
		return nil, err
	}
	subagent.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		subagent.Description = description.String
	}
	if skillID.Valid {
		subagent.SkillID, _ = uuid.Parse(skillID.String)
	}
	return subagent, nil
}

// List 列出Subagents
func (r *SubagentRepository) List(ctx context.Context, query *model.SubagentListQuery) ([]*model.Subagent, int64, error) {
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

	// Count
	countQuery := "SELECT COUNT(*) FROM subagents " + whereClause
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Pagination
	page := query.Page
	pageSize := query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	// List
	listQuery := `
		SELECT id, name, description, content, skill_id, created_at, updated_at
		FROM subagents ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	subagents := make([]*model.Subagent, 0)
	for rows.Next() {
		subagent := &model.Subagent{}
		var idStr string
		var description, skillID sql.NullString
		if err := rows.Scan(&idStr, &subagent.Name, &description, &subagent.Content, &skillID, &subagent.CreatedAt, &subagent.UpdatedAt); err != nil {
			return nil, 0, err
		}
		subagent.ID, _ = uuid.Parse(idStr)
		if description.Valid {
			subagent.Description = description.String
		}
		if skillID.Valid {
			subagent.SkillID, _ = uuid.Parse(skillID.String)
		}
		subagents = append(subagents, subagent)
	}

	return subagents, total, nil
}

// Update 更新Subagent
func (r *SubagentRepository) Update(ctx context.Context, subagent *model.Subagent) error {
	query := `
		UPDATE subagents
		SET name = ?, description = ?, content = ?, skill_id = ?, updated_at = NOW()
		WHERE id = ?
	`
	var skillID interface{}
	if subagent.SkillID != uuid.Nil {
		skillID = subagent.SkillID.String()
	}
	_, err := r.db.ExecContext(ctx, query,
		subagent.Name, subagent.Description, subagent.Content, skillID, subagent.ID.String(),
	)
	return err
}

// Delete 删除Subagent
func (r *SubagentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM subagents WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}
```

- [ ] **Step 2: 创建Agent-Subagent绑定Repository**

```go
// isdp/internal/repo/agent_subagent_binding.go
package repo

import (
	"context"
	"database/sql"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// AgentSubagentBindingRepository Agent-Subagent绑定数据访问
type AgentSubagentBindingRepository struct {
	db *sql.DB
}

// NewAgentSubagentBindingRepository 创建Repository
func NewAgentSubagentBindingRepository(db *sql.DB) *AgentSubagentBindingRepository {
	return &AgentSubagentBindingRepository{db: db}
}

// Create 创建绑定
func (r *AgentSubagentBindingRepository) Create(ctx context.Context, binding *model.AgentSubagentBinding) error {
	query := `
		INSERT INTO agent_subagent_bindings (id, agent_role_id, subagent_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		binding.ID.String(), binding.AgentRoleID.String(), binding.SubagentID.String(), binding.CreatedAt,
	)
	return err
}

// FindByAgentRoleID 根据AgentRole ID查找绑定的Subagent ID列表
func (r *AgentSubagentBindingRepository) FindByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT subagent_id FROM agent_subagent_bindings WHERE agent_role_id = ?`
	rows, err := r.db.QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var idStr string
		if err := rows.Scan(&idStr); err != nil {
			return nil, err
		}
		id, _ := uuid.Parse(idStr)
		ids = append(ids, id)
	}
	return ids, nil
}

// FindSubagentsByAgentRoleID 根据AgentRole ID查找绑定的Subagent列表
func (r *AgentSubagentBindingRepository) FindSubagentsByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Subagent, error) {
	query := `
		SELECT s.id, s.name, s.description, s.content, s.skill_id, s.created_at, s.updated_at
		FROM subagents s
		INNER JOIN agent_subagent_bindings b ON s.id = b.subagent_id
		WHERE b.agent_role_id = ?
	`
	rows, err := r.db.QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	subagents := make([]*model.Subagent, 0)
	for rows.Next() {
		subagent := &model.Subagent{}
		var idStr string
		var description, skillID sql.NullString
		if err := rows.Scan(&idStr, &subagent.Name, &description, &subagent.Content, &skillID, &subagent.CreatedAt, &subagent.UpdatedAt); err != nil {
			return nil, err
		}
		subagent.ID, _ = uuid.Parse(idStr)
		if description.Valid {
			subagent.Description = description.String
		}
		if skillID.Valid {
			subagent.SkillID, _ = uuid.Parse(skillID.String)
		}
		subagents = append(subagents, subagent)
	}
	return subagents, nil
}

// DeleteByAgentRoleID 删除AgentRole的所有绑定
func (r *AgentSubagentBindingRepository) DeleteByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) error {
	query := `DELETE FROM agent_subagent_bindings WHERE agent_role_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String())
	return err
}

// DeleteBinding 删除特定绑定
func (r *AgentSubagentBindingRepository) DeleteBinding(ctx context.Context, agentRoleID, subagentID uuid.UUID) error {
	query := `DELETE FROM agent_subagent_bindings WHERE agent_role_id = ? AND subagent_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String(), subagentID.String())
	return err
}
```

- [ ] **Step 3: 验证编译**

```bash
cd isdp && go build ./internal/repo/...
```

---

## Phase 2: 服务层

### Task 5: Agent配置服务重构

**Files:**
- Modify: `isdp/internal/service/configgen/service.go`
- Modify: `isdp/internal/service/configgen/downloader.go`
- Modify: `isdp/internal/repo/agent_config.go`

- [ ] **Step 1: 添加配置目录配置项**

在 `pkg/config/config.go` 中添加：

```go
// AgentConfigConfig Agent配置相关配置
type AgentConfigConfig struct {
    // DataDir ISDP数据目录，用于存储Agent配置池
    DataDir string `mapstructure:"data_dir"`
}

// Config 结构体添加字段
type Config struct {
    // ... 现有字段 ...
    AgentConfig AgentConfigConfig `mapstructure:"agent_config"`
}

// setDefaults 添加默认值
func setDefaults() {
    // ... 现有默认值 ...
    viper.SetDefault("agent_config.data_dir", "./data")
}
```

- [ ] **Step 2: 更新 AgentConfigRepository**

在 `repo/agent_config.go` 中添加更新配置生成状态的方法：

```go
// UpdateConfigGeneratedAt 更新配置生成时间和路径
func (r *AgentConfigRepository) UpdateConfigGeneratedAt(ctx context.Context, id uuid.UUID, configPath string) error {
	query := `
		UPDATE agent_configs
		SET config_generated_at = NOW(), config_path = ?, updated_at = NOW()
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query, configPath, id.String())
	return err
}
```

- [ ] **Step 3: 重构 ConfigGen Service**

修改 `service.go` 支持按Agent角色粒度生成配置：

```go
// GenerateAgentConfigRequest Agent配置生成请求
type GenerateAgentConfigRequest struct {
	AgentRoleID  uuid.UUID `json:"agentRoleId"`
	BaseAgentType string   `json:"baseAgentType"` // claude_code | open_code
	CleanExisting bool     `json:"cleanExisting"`
}

// GenerateAgentConfigResult Agent配置生成结果
type GenerateAgentConfigResult struct {
	AgentID        string    `json:"agentId"`
	ConfigPath     string    `json:"configPath"`
	SkillsCount    int       `json:"skillsCount"`
	SubagentsCount int       `json:"subagentsCount"`
	GeneratedAt    time.Time `json:"generatedAt"`
}

// GenerateAgentConfig 为单个Agent角色生成配置
func (s *Service) GenerateAgentConfig(ctx context.Context, req *GenerateAgentConfigRequest) (*GenerateAgentConfigResult, error) {
	// 1. 获取Agent角色
	agent, err := s.agentRepo.FindByID(ctx, req.AgentRoleID)
	if err != nil {
		return nil, fmt.Errorf("Agent角色不存在: %w", err)
	}

	// 2. 确定配置目录
	configPath := filepath.Join(s.dataDir, "agents", req.AgentRoleID.String(), ".claude")

	// 3. 清理现有配置（可选）
	if req.CleanExisting {
		os.RemoveAll(configPath)
	}

	// 4. 创建目录结构
	skillsDir := filepath.Join(configPath, "skills")
	agentsDir := filepath.Join(configPath, "agents")
	os.MkdirAll(skillsDir, 0755)
	os.MkdirAll(agentsDir, 0755)

	// 5. 生成 settings.json
	if err := s.generateSettingsJSON(configPath, agent); err != nil {
		return nil, err
	}

	// 6. 生成 CLAUDE.md
	if err := s.generateCLAUDEMd(configPath, agent); err != nil {
		return nil, err
	}

	// 7. 复制绑定的技能文件
	skillIDs, err := s.bindingRepo.FindByAgentRoleID(ctx, req.AgentRoleID)
	skillsCount := 0
	if err == nil && len(skillIDs) > 0 {
		for _, skillID := range skillIDs {
			skill, err := s.skillRepo.FindByID(ctx, skillID)
			if err != nil {
				continue
			}
			if err := s.downloader.CopySkillToDir(skill, skillsDir); err != nil {
				s.logger.Warn("复制技能文件失败", zap.Error(err))
			} else {
				skillsCount++
			}
		}
	}

	// 8. 复制绑定的子代理文件
	subagentIDs, err := s.subagentBindingRepo.FindByAgentRoleID(ctx, req.AgentRoleID)
	subagentsCount := 0
	if err == nil && len(subagentIDs) > 0 {
		for _, subagentID := range subagentIDs {
			subagent, err := s.subagentRepo.FindByID(ctx, subagentID)
			if err != nil {
				continue
			}
			if err := s.downloader.CopySubagentToDir(subagent, agentsDir); err != nil {
				s.logger.Warn("复制子代理文件失败", zap.Error(err))
			} else {
				subagentsCount++
			}
		}
	}

	// 9. 更新Agent配置生成时间
	now := time.Now()
	s.agentRepo.UpdateConfigGeneratedAt(ctx, req.AgentRoleID, configPath)

	return &GenerateAgentConfigResult{
		AgentID:        req.AgentRoleID.String(),
		ConfigPath:     configPath,
		SkillsCount:    skillsCount,
		SubagentsCount: subagentsCount,
		GeneratedAt:    now,
	}, nil
}

// generateSettingsJSON 生成 settings.json 文件
func (s *Service) generateSettingsJSON(configPath string, agent *model.AgentRoleConfig) error {
	settings := map[string]interface{}{
		"model": "claude-sonnet-4-6",
		"permissions": map[string]interface{}{
			"allow": []string{
				"Read(*)",
				"Write(*)",
				"Edit(*)",
				"Bash(npm *)",
				"Bash(git *)",
			},
			"deny": []string{},
		},
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(configPath, "settings.json"), data, 0644)
}

// generateCLAUDEMd 生成 CLAUDE.md 文件
func (s *Service) generateCLAUDEMd(configPath string, agent *model.AgentRoleConfig) error {
	content := fmt.Sprintf("# %s\n\n%s", agent.Name, agent.SystemPrompt)
	return os.WriteFile(filepath.Join(configPath, "CLAUDE.md"), []byte(content), 0644)
}
```

- [ ] **Step 4: 更新 Downloader 支持子代理**

在 `downloader.go` 中添加：

```go
// CopySubagentToDir 复制子代理文件到目标目录
func (d *Downloader) CopySubagentToDir(subagent *model.Subagent, targetDir string) error {
	// 文件名使用子代理名称，确保有效文件名
	filename := strings.ReplaceAll(subagent.Name, " ", "-") + ".md"
	if !isValidFilename(filename) {
		filename = subagent.ID.String() + ".md"
	}

	filePath := filepath.Join(targetDir, filename)

	// 构建子代理文件内容
	content := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n%s",
		subagent.Name,
		subagent.Description,
		subagent.Content,
	)

	return os.WriteFile(filePath, []byte(content), 0644)
}
```

- [ ] **Step 5: 验证编译**

```bash
cd isdp && go build ./internal/service/configgen/...
```

---

### Task 6: Subagent Service

**Files:**
- Create: `isdp/internal/service/subagent/service.go`

- [ ] **Step 1: 创建Subagent服务**

```go
// isdp/internal/service/subagent/service.go
package subagent

import (
	"context"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service Subagent服务
type Service struct {
	repo          *repo.SubagentRepository
	bindingRepo   *repo.AgentSubagentBindingRepository
	agentRepo     *repo.AgentConfigRepository
	logger        *zap.Logger
}

// NewService 创建服务
func NewService(
	repo *repo.SubagentRepository,
	bindingRepo *repo.AgentSubagentBindingRepository,
	agentRepo *repo.AgentConfigRepository,
	logger *zap.Logger,
) *Service {
	return &Service{
		repo:        repo,
		bindingRepo: bindingRepo,
		agentRepo:   agentRepo,
		logger:      logger,
	}
}

// Create 创建Subagent
func (s *Service) Create(ctx context.Context, req *model.CreateSubagentRequest) (*model.Subagent, error) {
	// 检查名称是否已存在
	if _, err := s.repo.FindByName(ctx, req.Name); err == nil {
		return nil, ErrSubagentNameExists
	}

	subagent := &model.Subagent{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		SkillID:     req.SkillID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, subagent); err != nil {
		return nil, err
	}

	return subagent, nil
}

// Get 获取Subagent
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Subagent, error) {
	return s.repo.FindByID(ctx, id)
}

// List 列出Subagents
func (s *Service) List(ctx context.Context, query *model.SubagentListQuery) ([]*model.Subagent, int64, error) {
	return s.repo.List(ctx, query)
}

// Update 更新Subagent
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateSubagentRequest) (*model.Subagent, error) {
	subagent, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Description != "" {
		subagent.Description = req.Description
	}
	if req.Content != "" {
		subagent.Content = req.Content
	}
	subagent.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, subagent); err != nil {
		return nil, err
	}

	return subagent, nil
}

// Delete 删除Subagent
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// BindSubagents 绑定Subagents到Agent
func (s *Service) BindSubagents(ctx context.Context, agentRoleID uuid.UUID, subagentIDs []uuid.UUID) error {
	// 先删除现有绑定
	if err := s.bindingRepo.DeleteByAgentRoleID(ctx, agentRoleID); err != nil {
		return err
	}

	// 创建新绑定
	for _, subagentID := range subagentIDs {
		binding := &model.AgentSubagentBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			SubagentID:  subagentID,
			CreatedAt:   time.Now(),
		}
		if err := s.bindingRepo.Create(ctx, binding); err != nil {
			s.logger.Warn("创建绑定失败", zap.Error(err))
		}
	}

	return nil
}

// GetAgentSubagents 获取Agent绑定的Subagents
func (s *Service) GetAgentSubagents(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Subagent, error) {
	return s.bindingRepo.FindSubagentsByAgentRoleID(ctx, agentRoleID)
}

// UnbindSubagent 解绑Subagent
func (s *Service) UnbindSubagent(ctx context.Context, agentRoleID, subagentID uuid.UUID) error {
	return s.bindingRepo.DeleteBinding(ctx, agentRoleID, subagentID)
}

var ErrSubagentNameExists = fmt.Errorf("subagent name already exists")
```

- [ ] **Step 2: 验证编译**

```bash
cd isdp && go build ./internal/service/subagent/...
```

---

## Phase 3: API层

### Task 7: Agent配置生成API

**Files:**
- Modify: `isdp/internal/api/configgen_handler.go`

- [ ] **Step 1: 添加Agent配置生成API**

在 `configgen_handler.go` 中添加：

```go
// GenerateAgentConfigRequest 生成Agent配置请求
type GenerateAgentConfigRequest struct {
	BaseAgentType string `json:"baseAgentType" binding:"required"` // claude_code | open_code
	CleanExisting bool   `json:"cleanExisting"`
}

// GenerateAgentConfig 生成Agent角色配置
// POST /agents/:id/config/generate
func (h *ConfigGenHandler) GenerateAgentConfig(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少Agent ID"})
		return
	}

	agentUUID, err := uuid.Parse(agentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的Agent ID"})
		return
	}

	var req GenerateAgentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if req.BaseAgentType != "claude_code" && req.BaseAgentType != "open_code" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "baseAgentType 必须是 claude_code 或 open_code"})
		return
	}

	result, err := h.configGenSvc.GenerateAgentConfig(c.Request.Context(), &configgen.GenerateAgentConfigRequest{
		AgentRoleID:   agentUUID,
		BaseAgentType: req.BaseAgentType,
		CleanExisting: req.CleanExisting,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "配置生成成功",
		"agentId":       result.AgentID,
		"configPath":    result.ConfigPath,
		"skillsCount":   result.SkillsCount,
		"subagentsCount": result.SubagentsCount,
		"generatedAt":   result.GeneratedAt,
	})
}

// RegisterRoutes 更新路由注册
func (h *ConfigGenHandler) RegisterRoutes(r *gin.RouterGroup) {
	// 项目级配置同步（保留兼容）
	projects := r.Group("/projects")
	{
		projects.POST("/:id/config/sync", h.SyncConfig)
	}

	// Agent级配置生成
	agents := r.Group("/agents")
	{
		agents.POST("/:id/config/generate", h.GenerateAgentConfig)
	}
}
```

- [ ] **Step 2: 验证编译**

```bash
cd isdp && go build ./internal/api/...
```

---

### Task 8: Subagent API Handler

**Files:**
- Create: `isdp/internal/api/subagent_handler.go`

- [ ] **Step 1: 创建Subagent API处理器**

```go
// isdp/internal/api/subagent_handler.go
package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/subagent"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SubagentHandler Subagent API处理器
type SubagentHandler struct {
	svc *subagent.Service
}

// NewSubagentHandler 创建处理器
func NewSubagentHandler(svc *subagent.Service) *SubagentHandler {
	return &SubagentHandler{svc: svc}
}

// List 列出Subagents
// GET /subagents
func (h *SubagentHandler) List(c *gin.Context) {
	var query model.SubagentListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	subagents, total, err := h.svc.List(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  subagents,
		"total": total,
		"page":  query.Page,
		"size":  query.PageSize,
	})
}

// Get 获取Subagent
// GET /subagents/:id
func (h *SubagentHandler) Get(c *gin.Context) {
	id := c.Param("id")
	uuid, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	subagent, err := h.svc.Get(c.Request.Context(), uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, subagent)
}

// Create 创建Subagent
// POST /subagents
func (h *SubagentHandler) Create(c *gin.Context) {
	var req model.CreateSubagentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	subagent, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, subagent)
}

// Update 更新Subagent
// PUT /subagents/:id
func (h *SubagentHandler) Update(c *gin.Context) {
	id := c.Param("id")
	uuid, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var req model.UpdateSubagentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	subagent, err := h.svc.Update(c.Request.Context(), uuid, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, subagent)
}

// Delete 删除Subagent
// DELETE /subagents/:id
func (h *SubagentHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	uuid, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), uuid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetAgentSubagents 获取Agent绑定的Subagents
// GET /agents/:id/subagents
func (h *SubagentHandler) GetAgentSubagents(c *gin.Context) {
	id := c.Param("id")
	uuid, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	subagents, err := h.svc.GetAgentSubagents(c.Request.Context(), uuid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"subagents": subagents})
}

// BindSubagents 绑定Subagents到Agent
// POST /agents/:id/subagents
func (h *SubagentHandler) BindSubagents(c *gin.Context) {
	id := c.Param("id")
	uuid, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var req model.BindSubagentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.BindSubagents(c.Request.Context(), uuid, req.SubagentIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "绑定成功"})
}

// UnbindSubagent 解绑Subagent
// DELETE /agents/:id/subagents/:subagent_id
func (h *SubagentHandler) UnbindSubagent(c *gin.Context) {
	agentID := c.Param("id")
	subagentID := c.Param("subagent_id")

	agentUUID, err := uuid.Parse(agentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的Agent ID"})
		return
	}

	subagentUUID, err := uuid.Parse(subagentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的Subagent ID"})
		return
	}

	if err := h.svc.UnbindSubagent(c.Request.Context(), agentUUID, subagentUUID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "解绑成功"})
}

// RegisterRoutes 注册路由
func (h *SubagentHandler) RegisterRoutes(r *gin.RouterGroup) {
	subagents := r.Group("/subagents")
	{
		subagents.GET("", h.List)
		subagents.GET("/:id", h.Get)
		subagents.POST("", h.Create)
		subagents.PUT("/:id", h.Update)
		subagents.DELETE("/:id", h.Delete)
	}

	agents := r.Group("/agents")
	{
		agents.GET("/:id/subagents", h.GetAgentSubagents)
		agents.POST("/:id/subagents", h.BindSubagents)
		agents.DELETE("/:id/subagents/:subagent_id", h.UnbindSubagent)
	}
}
```

- [ ] **Step 2: 验证编译**

```bash
cd isdp && go build ./internal/api/...
```

---

### Task 9: 集成到主程序

**Files:**
- Modify: `isdp/cmd/server/main.go`

- [ ] **Step 1: 注册新服务和Handler**

在 `main.go` 中添加初始化代码：

```go
// 创建 Subagent Repository
subagentRepo := repo.NewSubagentRepository(db)
agentSubagentBindingRepo := repo.NewAgentSubagentBindingRepository(db)

// 创建 Subagent Service
subagentSvc := subagent.NewService(subagentRepo, agentSubagentBindingRepo, agentConfigRepo, logger)

// 更新 ConfigGen Service（添加新的依赖）
configGenSvc := configgen.NewService(
    projectRepo, agentConfigRepo, skillRepo, agentSkillBindingRepo,
    subagentRepo, agentSubagentBindingRepo,
    cfg.AgentConfig.DataDir, cfg.Skill.StoragePath,
    logger,
)

// 注册 Handler
subagentHandler := api.NewSubagentHandler(subagentSvc)
subagentHandler.RegisterRoutes(apiGroup)
configGenHandler.RegisterRoutes(apiGroup) // 已更新路由
```

- [ ] **Step 2: 验证编译和启动**

```bash
cd isdp && go build ./cmd/server && ./bin/isdp
```

---

## Phase 4: 前端

### Task 10: 前端类型定义

**Files:**
- Modify: `isdp/web/src/types/index.ts`

- [ ] **Step 1: 添加Subagent类型**

```typescript
// Subagent 类型
export interface Subagent {
  id: string;
  name: string;
  description?: string;
  content: string;
  skillId?: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateSubagentRequest {
  name: string;
  description?: string;
  content: string;
  skillId?: string;
}

export interface UpdateSubagentRequest {
  description?: string;
  content: string;
}

export interface SubagentListQuery {
  search?: string;
  page?: number;
  pageSize?: number;
}

export interface SubagentListResponse {
  data: Subagent[];
  total: number;
  page: number;
  size: number;
}

// 扩展 AgentConfig 类型
export interface AgentConfig {
  // ... 现有字段 ...
  configGeneratedAt?: string;
  configPath?: string;
}
```

- [ ] **Step 2: 验证TypeScript编译**

```bash
cd web && npm run build
```

---

### Task 11: 前端 API Client

**Files:**
- Modify: `isdp/web/src/api/client.ts`

- [ ] **Step 1: 添加Subagent API**

```typescript
// 在 APIClient 类中添加

// Subagent API
subagents = {
  list: (query?: SubagentListQuery): Promise<SubagentListResponse> => {
    const params = new URLSearchParams();
    if (query?.search) params.append('search', query.search);
    if (query?.page) params.append('page', query.page.toString());
    if (query?.pageSize) params.append('page_size', query.pageSize.toString());

    const url = params.toString() ? `/subagents?${params.toString()}` : '/subagents';
    return this.request(url, 'GET');
  },
  get: (id: string): Promise<Subagent> =>
    this.request(`/subagents/${id}`, 'GET'),
  create: (data: CreateSubagentRequest): Promise<Subagent> =>
    this.request('/subagents', 'POST', data),
  update: (id: string, data: UpdateSubagentRequest): Promise<Subagent> =>
    this.request(`/subagents/${id}`, 'PUT', data),
  delete: (id: string): Promise<void> =>
    this.request(`/subagents/${id}`, 'DELETE'),
};

// Agent 配置生成 API（扩展 agents 对象）
agents = {
  // ... 现有方法 ...
  generateConfig: (id: string, baseAgentType: string, cleanExisting?: boolean): Promise<{
    message: string;
    agentId: string;
    configPath: string;
    skillsCount: number;
    subagentsCount: number;
    generatedAt: string;
  }> => this.request(`/agents/${id}/config/generate`, 'POST', {
    baseAgentType,
    cleanExisting: cleanExisting || false,
  }),
  getSubagents: (id: string): Promise<{ subagents: Subagent[] }> =>
    this.request(`/agents/${id}/subagents`, 'GET'),
  bindSubagents: (id: string, subagentIds: string[]): Promise<{ message: string }> =>
    this.request(`/agents/${id}/subagents`, 'POST', { subagent_ids: subagentIds }),
  unbindSubagent: (id: string, subagentId: string): Promise<{ message: string }> =>
    this.request(`/agents/${id}/subagents/${subagentId}`, 'DELETE'),
};
```

- [ ] **Step 2: 验证编译**

```bash
cd web && npm run build
```

---

### Task 12: Agent列表页添加配置生成

**Files:**
- Modify: `isdp/web/src/pages/AgentRoleList.tsx`

- [ ] **Step 1: 添加配置生成按钮和状态列**

在表格列中添加：

```tsx
{
  title: '配置状态',
  key: 'configStatus',
  width: 150,
  render: (_: unknown, record: AgentConfig) => (
    record.configGeneratedAt ? (
      <Tooltip title={`路径: ${record.configPath}`}>
        <Tag color="green">
          已生成 ({new Date(record.configGeneratedAt).toLocaleDateString()})
        </Tag>
      </Tooltip>
    ) : (
      <Tag color="default">未生成</Tag>
    )
  ),
},
{
  title: '操作',
  key: 'actions',
  width: 350,
  fixed: 'right' as const,
  render: (_: unknown, record: AgentConfig) => (
    <Space size="small">
      <Button
        type="link"
        size="small"
        icon={<SettingOutlined />}
        onClick={() => handleGenerateConfig(record)}
      >
        生成配置
      </Button>
      {/* ... 其他按钮 ... */}
    </Space>
  ),
},
```

- [ ] **Step 2: 添加配置生成处理函数**

```tsx
const [generateLoading, setGenerateLoading] = useState<string | null>(null);

const handleGenerateConfig = (record: AgentConfig) => {
  Modal.confirm({
    title: '生成配置',
    content: (
      <div>
        <p>确定要为Agent「{record.name}」生成配置吗？</p>
        <p>这将创建独立的配置目录，包含绑定的技能和子代理。</p>
      </div>
    ),
    onOk: async () => {
      setGenerateLoading(record.id);
      try {
        const result = await api.agents.generateConfig(record.id, 'claude_code');
        message.success(`配置生成成功，包含 ${result.skillsCount} 个技能和 ${result.subagentsCount} 个子代理`);
        loadConfigs();
      } catch (error) {
        message.error('配置生成失败');
      } finally {
        setGenerateLoading(null);
      }
    },
  });
};
```

- [ ] **Step 3: 验证前端运行**

```bash
cd web && npm run dev
```

---

### Task 13: Subagent管理页面

**Files:**
- Create: `isdp/web/src/pages/SubagentList.tsx`
- Modify: `isdp/web/src/App.tsx` (路由)
- Modify: `isdp/web/src/layouts/MainLayout.tsx` (菜单)

- [ ] **Step 1: 创建Subagent管理页面**

```tsx
// isdp/web/src/pages/SubagentList.tsx
import React, { useEffect, useState } from 'react';
import { Table, Button, Card, Modal, Form, Input, message, Space, Typography, Tag } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import api from '@/api/client';
import type { Subagent } from '@/types';

const { Title, Text } = Typography;
const { TextArea } = Input;

const SubagentList: React.FC = () => {
  const [subagents, setSubagents] = useState<Subagent[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingSubagent, setEditingSubagent] = useState<Subagent | null>(null);
  const [form] = Form.useForm();
  const [pagination, setPagination] = useState({ current: 1, pageSize: 20, total: 0 });

  useEffect(() => {
    loadSubagents();
  }, [pagination.current, pagination.pageSize]);

  const loadSubagents = async () => {
    setLoading(true);
    try {
      const result = await api.subagents.list({
        page: pagination.current,
        pageSize: pagination.pageSize,
      });
      setSubagents(result.data);
      setPagination(prev => ({ ...prev, total: result.total }));
    } catch (error) {
      message.error('加载子代理列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingSubagent(null);
    form.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: Subagent) => {
    setEditingSubagent(record);
    form.setFieldsValue(record);
    setModalVisible(true);
  };

  const handleDelete = (record: Subagent) => {
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除子代理「${record.name}」吗？`,
      okText: '确认删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await api.subagents.delete(record.id);
          message.success('删除成功');
          loadSubagents();
        } catch (error) {
          message.error('删除失败');
        }
      },
    });
  };

  const handleSubmit = async (values: any) => {
    try {
      if (editingSubagent) {
        await api.subagents.update(editingSubagent.id, values);
        message.success('更新成功');
      } else {
        await api.subagents.create(values);
        message.success('创建成功');
      }
      setModalVisible(false);
      loadSubagents();
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
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 180,
      render: (date: string) => new Date(date).toLocaleString(),
    },
    {
      title: '操作',
      key: 'actions',
      width: 150,
      render: (_: unknown, record: Subagent) => (
        <Space size="small">
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
            编辑
          </Button>
          <Button type="link" size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record)}>
            删除
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 12 }}>
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <Title level={2} style={{ margin: 0 }}>子代理管理</Title>
          <Text type="secondary">管理可委派任务的独立子代理</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新建子代理
        </Button>
      </div>

      <Card>
        <Table
          dataSource={subagents}
          columns={columns}
          rowKey="id"
          loading={loading}
          pagination={{
            ...pagination,
            onChange: (page, pageSize) => setPagination(prev => ({ ...prev, current: page, pageSize })),
          }}
        />
      </Card>

      <Modal
        title={editingSubagent ? '编辑子代理' : '新建子代理'}
        open={modalVisible}
        onOk={() => form.submit()}
        onCancel={() => setModalVisible(false)}
        width={800}
      >
        <Form form={form} layout="vertical" onFinish={handleSubmit}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="子代理名称" disabled={!!editingSubagent} />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input placeholder="子代理描述" />
          </Form.Item>
          <Form.Item name="content" label="内容" rules={[{ required: true, message: '请输入内容' }]}>
            <TextArea rows={15} placeholder="Markdown格式的子代理内容" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default SubagentList;
```

- [ ] **Step 2: 添加路由**

```tsx
// App.tsx 添加路由
import SubagentList from './pages/SubagentList';

// 在路由配置中添加
<Route path="/subagents" element={<SubagentList />} />
```

- [ ] **Step 3: 添加菜单**

```tsx
// MainLayout.tsx 添加菜单项
{
  key: '/subagents',
  icon: <RobotOutlined />,
  label: '子代理管理',
}
```

- [ ] **Step 4: 验证前端运行**

```bash
cd web && npm run dev
```

---

### Task 14: Agent编辑页添加子代理绑定

**Files:**
- Modify: `isdp/web/src/pages/AgentRoleList.tsx`

- [ ] **Step 1: 添加子代理绑定选择**

在Modal表单中添加：

```tsx
const [selectedSubagentIds, setSelectedSubagentIds] = useState<string[]>([]);
const [subagents, setSubagents] = useState<Subagent[]>([]);

// 加载子代理列表
const loadSubagents = async () => {
  try {
    const result = await api.subagents.list({ pageSize: 100 });
    setSubagents(result.data);
  } catch (error) {
    console.error('加载子代理列表失败', error);
  }
};

// 加载Agent绑定的子代理
const loadAgentSubagents = async (agentId: string) => {
  try {
    const result = await api.agents.getSubagents(agentId);
    setSelectedSubagentIds(result.subagents.map(s => s.id));
  } catch (error) {
    console.error('加载Agent绑定的子代理失败', error);
    setSelectedSubagentIds([]);
  }
};

// 在 handleEdit 中调用
const handleEdit = async (record: AgentConfig) => {
  setEditingConfig(record);
  form.setFieldsValue(record);
  await loadAgentSkills(record.id);
  await loadAgentSubagents(record.id);
  setModalVisible(true);
};

// 在 Form 中添加子代理选择
<Form.Item label="绑定子代理">
  <Select
    mode="multiple"
    placeholder="选择要绑定的子代理"
    value={selectedSubagentIds}
    onChange={setSelectedSubagentIds}
    style={{ width: '100%' }}
    options={subagents.map(s => ({
      label: s.name,
      value: s.id,
      desc: s.description || '暂无描述',
    }))}
  />
</Form.Item>
```

- [ ] **Step 2: 更新提交逻辑**

```tsx
const handleSubmit = async (values: Partial<AgentConfig>) => {
  try {
    if (editingConfig) {
      await api.agents.update(editingConfig.id, values);
      // 更新技能绑定
      if (selectedSkillIds.length > 0) {
        await api.agents.bindSkills(editingConfig.id, selectedSkillIds);
      }
      // 更新子代理绑定
      if (selectedSubagentIds.length > 0) {
        await api.agents.bindSubagents(editingConfig.id, selectedSubagentIds);
      }
      message.success('更新成功');
    } else {
      const newAgent = await api.agents.create(values);
      if (selectedSkillIds.length > 0) {
        await api.agents.bindSkills(newAgent.id, selectedSkillIds);
      }
      if (selectedSubagentIds.length > 0) {
        await api.agents.bindSubagents(newAgent.id, selectedSubagentIds);
      }
      message.success('创建成功');
    }
    setModalVisible(false);
    loadConfigs();
  } catch (error) {
    message.error('操作失败');
  }
};
```

- [ ] **Step 3: 验证前端运行**

```bash
cd web && npm run dev
```

---

## Phase 5: 配置文件更新

### Task 15: 更新配置模板

**Files:**
- Modify: `isdp/configs/config.yaml.example`

- [ ] **Step 1: 添加Agent配置相关配置项**

```yaml
# Agent配置相关
agent_config:
  # ISDP数据目录，用于存储Agent配置池
  # 默认值: ./data
  data_dir: ./data
```

- [ ] **Step 2: 更新本地配置**

同步更新 `configs/config.yaml`

---

## 验证清单

完成所有任务后执行以下验证：

1. **数据库验证**
   ```bash
   mysql -u isdp -p -D dev_ji -e "SHOW TABLES LIKE '%subagent%'"
   ```

2. **后端编译**
   ```bash
   cd isdp && go build ./cmd/server
   ```

3. **前端编译**
   ```bash
   cd web && npm run build
   ```

4. **功能验证**
   - 创建Agent角色
   - 绑定技能和子代理
   - 点击"生成配置"按钮
   - 检查 `./data/agents/{agent-id}/.claude/` 目录结构
   - 使用 `CLAUDE_CONFIG_DIR=./data/agents/{agent-id}/.claude claude` 启动验证

---

## 提交计划

每个Phase完成后创建一个提交：

```bash
# Phase 1
git add isdp/sql-change/migrations/202603220001_add_subagent_tables.sql
git add isdp/internal/model/subagent.go
git add isdp/internal/model/agent_config.go
git add isdp/internal/repo/subagent.go
git add isdp/internal/repo/agent_subagent_binding.go
git commit -m "feat(data): add subagent models and repositories"

# Phase 2
git add isdp/pkg/config/config.go
git add isdp/internal/service/configgen/
git add isdp/internal/service/subagent/
git commit -m "feat(service): refactor configgen for agent-level and add subagent service"

# Phase 3
git add isdp/internal/api/configgen_handler.go
git add isdp/internal/api/subagent_handler.go
git add isdp/cmd/server/main.go
git commit -m "feat(api): add agent config generation and subagent APIs"

# Phase 4
git add isdp/web/src/
git commit -m "feat(web): add subagent management and config generation UI"

# Phase 5
git add isdp/configs/
git commit -m "chore(config): add agent_config data_dir setting"
```