# 资产包系统实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现资产包系统，支持多种资产的批量导入导出和版本管理能力。

**Architecture:**
- 资产包作为传输载体（ZIP格式），导入后资产散开管理
- Settings 新增为资产类型（目录形式），统一管理 CLI 配置和自定义配置
- Subagent 从数据库存储迁移到文件系统存储
- 配置生成服务集成 Settings，生成时拷贝到 Agent 配置目录

**Tech Stack:** Go (Gin + SQL), React (Ant Design), ZIP (archive/zip + JSZip)

---

## 文件结构概览

### 新增文件

| 文件 | 负责内容 |
|------|----------|
| `isdp/internal/model/settings.go` | Settings 模型定义 |
| `isdp/internal/model/asset_package.go` | AssetPackage 模型定义 |
| `isdp/internal/repo/settings.go` | Settings 数据访问 |
| `isdp/internal/repo/asset_package.go` | AssetPackage 数据访问 |
| `isdp/internal/repo/agent_settings_binding.go` | Agent-Settings 绑定数据访问 |
| `isdp/internal/service/settings/service.go` | Settings 业务服务 |
| `isdp/internal/service/assetpackage/service.go` | 资产包导入导出服务 |
| `isdp/internal/api/settings_handler.go` | Settings HTTP 处理器 |
| `isdp/internal/api/asset_package_handler.go` | 资产包 HTTP 处理器 |
| `isdp/web/src/pages/AssetPackage/index.tsx` | 资产包管理页面 |
| `isdp/web/src/api/assetPackage.ts` | 资产包前端 API |

### 修改文件

| 文件 | 改动内容 |
|------|----------|
| `isdp/internal/model/command.go` | 添加 Version 字段 |
| `isdp/internal/model/subagent.go` | 添加 Version 字段，移除 Content 字段 |
| `isdp/internal/model/rule.go` | 添加 Version 字段 |
| `isdp/internal/repo/command.go` | 适配 Version 字段 |
| `isdp/internal/repo/subagent.go` | 适配 Version 字段，移除 Content |
| `isdp/internal/repo/rule.go` | 适配 Version 字段 |
| `isdp/internal/service/command/service.go` | 适配 Version |
| `isdp/internal/service/subagent/service.go` | 文件系统存储逻辑 |
| `isdp/internal/service/rule/service.go` | 适配 Version |
| `isdp/internal/service/configgen/service.go` | 集成 Settings 配置生成 |
| `isdp/internal/api/command_handler.go` | 适配 Version |
| `isdp/internal/api/subagent_handler.go` | 文件上传改为目录上传 |
| `isdp/cmd/server/main.go` | 注册新路由和服务 |
| `isdp/web/src/api/client.ts` | 添加资产包 API |
| `isdp/web/src/App.tsx` | 添加资产包路由 |

### 数据库迁移

| 文件 | 内容 |
|------|------|
| `isdp/sql-change/migrations/202603310001_add_asset_package_tables.sql` | 新增 asset_packages, settings, agent_settings_bindings 表 |
| `isdp/sql-change/migrations/202603310002_add_version_fields.sql` | 添加 version 字段到 commands, subagents, rules |

---

## Task 1: 数据库迁移脚本

**Files:**
- Create: `isdp/sql-change/migrations/202603310001_add_asset_package_tables.sql`
- Create: `isdp/sql-change/migrations/202603310002_add_version_fields.sql`

- [ ] **Step 1: 创建资产包相关表迁移脚本**

```sql
-- 文件路径: isdp/sql-change/migrations/202603310001_add_asset_package_tables.sql
-- 变更说明：新增资产包系统相关表
-- 作者：AI Assistant
-- 日期：2026-03-31

SET NAMES utf8mb4;

-- 资产包表
CREATE TABLE IF NOT EXISTS asset_packages (
  id VARCHAR(36) PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  version VARCHAR(50) NOT NULL,
  description TEXT,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Settings 表
CREATE TABLE IF NOT EXISTS settings (
  id VARCHAR(36) PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  directory_path VARCHAR(500),
  version VARCHAR(20),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Agent-Settings 绑定表
CREATE TABLE IF NOT EXISTS agent_settings_bindings (
  id VARCHAR(36) PRIMARY KEY,
  agent_role_id VARCHAR(36) NOT NULL,
  settings_id VARCHAR(36) NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (agent_role_id) REFERENCES agent_role_configs(id) ON DELETE CASCADE,
  FOREIGN KEY (settings_id) REFERENCES settings(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 回滚语句
-- DROP TABLE IF EXISTS agent_settings_bindings;
-- DROP TABLE IF EXISTS settings;
-- DROP TABLE IF EXISTS asset_packages;
```

- [ ] **Step 2: 创建版本字段迁移脚本**

```sql
-- 文件路径: isdp/sql-change/migrations/202603310002_add_version_fields.sql
-- 变更说明：为 commands, subagents, rules 表添加 version 字段
-- 作者：AI Assistant
-- 日期：2026-03-31

SET NAMES utf8mb4;

-- 为 commands 添加 version 字段
ALTER TABLE commands ADD COLUMN version VARCHAR(20) DEFAULT '1.0.0';

-- 为 subagents 添加 version 字段
ALTER TABLE subagents ADD COLUMN version VARCHAR(20) DEFAULT '1.0.0';

-- 为 rules 添加 version 字段
ALTER TABLE rules ADD COLUMN version VARCHAR(20) DEFAULT '1.0.0';

-- 回滚语句
-- ALTER TABLE commands DROP COLUMN version;
-- ALTER TABLE subagents DROP COLUMN version;
-- ALTER TABLE rules DROP COLUMN version;
```

- [ ] **Step 3: 执行迁移脚本验证**

Run: 在本地数据库执行迁移脚本，验证表结构正确

- [ ] **Step 4: 提交数据库变更**

```bash
git add isdp/sql-change/migrations/202603310001_add_asset_package_tables.sql
git add isdp/sql-change/migrations/202603310002_add_version_fields.sql
git commit -m "feat(db): 添加资产包系统数据库表和版本字段"
```

---

## Task 2: Settings 模型和 Repository

**Files:**
- Create: `isdp/internal/model/settings.go`
- Create: `isdp/internal/repo/settings.go`
- Create: `isdp/internal/repo/agent_settings_binding.go`

- [ ] **Step 1: 创建 Settings 模型**

```go
// 文件路径: isdp/internal/model/settings.go
package model

import (
	"time"
	"github.com/google/uuid"
)

// Settings 配置目录资产模型
type Settings struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	DirectoryPath string    `json:"directoryPath,omitempty"` // 存储路径
	Version       string    `json:"version"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (s *Settings) TableName() string {
	return "settings"
}

// AgentSettingsBinding Agent角色与Settings绑定
type AgentSettingsBinding struct {
	ID          uuid.UUID `json:"id"`
	AgentRoleID uuid.UUID `json:"agentRoleId"`
	SettingsID  uuid.UUID `json:"settingsId"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (a *AgentSettingsBinding) TableName() string {
	return "agent_settings_bindings"
}

// CreateSettingsRequest 创建Settings请求
type CreateSettingsRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateSettingsRequest 更新Settings请求
type UpdateSettingsRequest struct {
	Description string `json:"description"`
	Version     string `json:"version"`
}

// SettingsListQuery Settings列表查询参数
type SettingsListQuery struct {
	Search   string `form:"search"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// BindSettingsRequest 绑定Settings请求
type BindSettingsRequest struct {
	SettingsIDs []uuid.UUID `json:"settingsIds" binding:"required"`
}
```

- [ ] **Step 2: 创建 Settings Repository**

```go
// 文件路径: isdp/internal/repo/settings.go
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

type SettingsRepository struct {
	db *sql.DB
}

func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

func (r *SettingsRepository) Create(ctx context.Context, settings *model.Settings) error {
	query := `
		INSERT INTO settings (id, name, description, directory_path, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		settings.ID.String(), settings.Name, settings.Description,
		settings.DirectoryPath, settings.Version, settings.CreatedAt, settings.UpdatedAt,
	)
	return err
}

func scanSettings(scanner interface{ Scan(dest ...interface{}) error }) (*model.Settings, error) {
	settings := &model.Settings{}
	var idStr string
	var description, directoryPath sql.NullString
	var version sql.NullString

	err := scanner.Scan(
		&idStr, &settings.Name, &description, &directoryPath, &version,
		&settings.CreatedAt, &settings.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	settings.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		settings.Description = description.String
	}
	if directoryPath.Valid {
		settings.DirectoryPath = directoryPath.String
	}
	if version.Valid {
		settings.Version = version.String
	} else {
		settings.Version = "1.0.0"
	}

	return settings, nil
}

func (r *SettingsRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Settings, error) {
	query := `SELECT id, name, description, directory_path, version, created_at, updated_at FROM settings WHERE id = ?`
	settings, err := scanSettings(r.db.QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("settings not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find settings: %w", err)
	}
	return settings, nil
}

func (r *SettingsRepository) FindByName(ctx context.Context, name string) (*model.Settings, error) {
	query := `SELECT id, name, description, directory_path, version, created_at, updated_at FROM settings WHERE name = ?`
	settings, err := scanSettings(r.db.QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("settings not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find settings: %w", err)
	}
	return settings, nil
}

func (r *SettingsRepository) List(ctx context.Context, query *model.SettingsListQuery) ([]*model.Settings, int64, error) {
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

	countQuery := "SELECT COUNT(*) FROM settings " + whereClause
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count settings: %w", err)
	}

	page := query.Page
	pageSize := query.PageSize
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }
	if pageSize > 100 { pageSize = 100 }
	offset := (page - 1) * pageSize

	listQuery := `SELECT id, name, description, directory_path, version, created_at, updated_at FROM settings ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list settings: %w", err)
	}
	defer rows.Close()

	settingsList := make([]*model.Settings, 0)
	for rows.Next() {
		s, err := scanSettings(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan settings: %w", err)
		}
		settingsList = append(settingsList, s)
	}

	return settingsList, total, nil
}

func (r *SettingsRepository) Update(ctx context.Context, settings *model.Settings) error {
	query := `UPDATE settings SET name = ?, description = ?, directory_path = ?, version = ?, updated_at = NOW() WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query,
		settings.Name, settings.Description, settings.DirectoryPath, settings.Version, settings.ID.String(),
	)
	return err
}

func (r *SettingsRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM settings WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}
```

- [ ] **Step 3: 创建 Agent-Settings 绑定 Repository**

```go
// 文件路径: isdp/internal/repo/agent_settings_binding.go
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

type AgentSettingsBindingRepository struct {
	db *sql.DB
}

func NewAgentSettingsBindingRepository(db *sql.DB) *AgentSettingsBindingRepository {
	return &AgentSettingsBindingRepository{db: db}
}

func (r *AgentSettingsBindingRepository) Create(ctx context.Context, binding *model.AgentSettingsBinding) error {
	query := `INSERT INTO agent_settings_bindings (id, agent_role_id, settings_id, created_at) VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		binding.ID.String(), binding.AgentRoleID.String(), binding.SettingsID.String(), binding.CreatedAt,
	)
	return err
}

func (r *AgentSettingsBindingRepository) FindByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT settings_id FROM agent_settings_bindings WHERE agent_role_id = ?`
	rows, err := r.db.QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	settingsIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var settingsIDStr string
		if err := rows.Scan(&settingsIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan settings_id: %w", err)
		}
		settingsID, _ := uuid.Parse(settingsIDStr)
		settingsIDs = append(settingsIDs, settingsID)
	}
	return settingsIDs, nil
}

func (r *AgentSettingsBindingRepository) FindBySettingsID(ctx context.Context, settingsID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT agent_role_id FROM agent_settings_bindings WHERE settings_id = ?`
	rows, err := r.db.QueryContext(ctx, query, settingsID.String())
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

func (r *AgentSettingsBindingRepository) DeleteByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) error {
	query := `DELETE FROM agent_settings_bindings WHERE agent_role_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String())
	return err
}

func (r *AgentSettingsBindingRepository) DeleteBinding(ctx context.Context, agentRoleID, settingsID uuid.UUID) error {
	query := `DELETE FROM agent_settings_bindings WHERE agent_role_id = ? AND settings_id = ?`
	_, err := r.db.ExecContext(ctx, query, agentRoleID.String(), settingsID.String())
	return err
}

func (r *AgentSettingsBindingRepository) ExistsBinding(ctx context.Context, agentRoleID, settingsID uuid.UUID) (bool, error) {
	query := `SELECT COUNT(*) FROM agent_settings_bindings WHERE agent_role_id = ? AND settings_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, agentRoleID.String(), settingsID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
```

- [ ] **Step 4: 提交代码**

```bash
git add isdp/internal/model/settings.go isdp/internal/repo/settings.go isdp/internal/repo/agent_settings_binding.go
git commit -m "feat(model): 添加 Settings 模型和 Repository"
```

---

## Task 3: AssetPackage 模型和 Repository

**Files:**
- Create: `isdp/internal/model/asset_package.go`
- Create: `isdp/internal/repo/asset_package.go`

- [ ] **Step 1: 创建 AssetPackage 模型**

```go
// 文件路径: isdp/internal/model/asset_package.go
package model

import (
	"time"
	"github.com/google/uuid"
)

// AssetPackage 资产包模型
type AssetPackage struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"` // v1.0.0-20240331-143052
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (a *AssetPackage) TableName() string {
	return "asset_packages"
}

// AssetPackageManifest 资产包 manifest.json 结构
type AssetPackageManifest struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	ExportedAt  string                 `json:"exportedAt"`
	Description string                 `json:"description"`
	Assets      AssetPackageAssetsList `json:"assets"`
}

// AssetPackageAssetsList 资产列表
type AssetPackageAssetsList struct {
	Skills     []AssetPackageSkillItem     `json:"skills,omitempty"`
	Commands   []AssetPackageCommandItem   `json:"commands,omitempty"`
	Subagents  []AssetPackageSubagentItem  `json:"subagents,omitempty"`
	Rules      []AssetPackageRuleItem      `json:"rules,omitempty"`
	Settings   []AssetPackageSettingsItem  `json:"settings,omitempty"`
}

// AssetPackageSkillItem 技能项
type AssetPackageSkillItem struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// AssetPackageCommandItem 命令项
type AssetPackageCommandItem struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	BoundSkills []string `json:"boundSkills,omitempty"`
}

// AssetPackageSubagentItem 子代理项
type AssetPackageSubagentItem struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	BoundSkills []string `json:"boundSkills,omitempty"`
}

// AssetPackageRuleItem 规则项
type AssetPackageRuleItem struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// AssetPackageSettingsItem 配置项
type AssetPackageSettingsItem struct {
	Name string `json:"name"`
}

// AssetPackageListQuery 资产包列表查询参数
type AssetPackageListQuery struct {
	Search   string `form:"search"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

// ExportAssetPackageRequest 导出资产包请求
type ExportAssetPackageRequest struct {
	Name        string    `json:"name" binding:"required"`
	Version     string    `json:"version" binding:"required"` // 语义化版本如 "1.0.0"
	Description string    `json:"description"`
	SkillIDs    []uuid.UUID `json:"skillIds"`
	CommandIDs  []uuid.UUID `json:"commandIds"`
	SubagentIDs []uuid.UUID `json:"subagentIds"`
	RuleIDs     []uuid.UUID `json:"ruleIds"`
	SettingsIDs []uuid.UUID `json:"settingsIds"`
}

// ImportResult 导入结果
type ImportResult struct {
	PackageName string          `json:"packageName"`
	PackageID   uuid.UUID       `json:"packageId"`
	Success     int             `json:"success"`
	Skipped     int             `json:"skipped"`
	Failed      int             `json:"failed"`
	Details     []ImportDetail  `json:"details"`
}

// ImportDetail 导入详情
type ImportDetail struct {
	AssetType string `json:"assetType"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Status    string `json:"status"` // success, skipped, failed
	Message   string `json:"message,omitempty"`
}
```

- [ ] **Step 2: 创建 AssetPackage Repository**

```go
// 文件路径: isdp/internal/repo/asset_package.go
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

type AssetPackageRepository struct {
	db *sql.DB
}

func NewAssetPackageRepository(db *sql.DB) *AssetPackageRepository {
	return &AssetPackageRepository{db: db}
}

func (r *AssetPackageRepository) Create(ctx context.Context, pkg *model.AssetPackage) error {
	query := `INSERT INTO asset_packages (id, name, version, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		pkg.ID.String(), pkg.Name, pkg.Version, pkg.Description, pkg.CreatedAt, pkg.UpdatedAt,
	)
	return err
}

func scanAssetPackage(scanner interface{ Scan(dest ...interface{}) error }) (*model.AssetPackage, error) {
	pkg := &model.AssetPackage{}
	var idStr string
	var description sql.NullString

	err := scanner.Scan(&idStr, &pkg.Name, &pkg.Version, &description, &pkg.CreatedAt, &pkg.UpdatedAt)
	if err != nil {
		return nil, err
	}

	pkg.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		pkg.Description = description.String
	}

	return pkg, nil
}

func (r *AssetPackageRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.AssetPackage, error) {
	query := `SELECT id, name, version, description, created_at, updated_at FROM asset_packages WHERE id = ?`
	pkg, err := scanAssetPackage(r.db.QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("asset package not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find asset package: %w", err)
	}
	return pkg, nil
}

func (r *AssetPackageRepository) List(ctx context.Context, query *model.AssetPackageListQuery) ([]*model.AssetPackage, int64, error) {
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

	countQuery := "SELECT COUNT(*) FROM asset_packages " + whereClause
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count asset packages: %w", err)
	}

	page := query.Page
	pageSize := query.PageSize
	if page < 1 { page = 1 }
	if pageSize < 1 { pageSize = 20 }
	if pageSize > 100 { pageSize = 100 }
	offset := (page - 1) * pageSize

	listQuery := `SELECT id, name, version, description, created_at, updated_at FROM asset_packages ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list asset packages: %w", err)
	}
	defer rows.Close()

	packages := make([]*model.AssetPackage, 0)
	for rows.Next() {
		pkg, err := scanAssetPackage(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan asset package: %w", err)
		}
		packages = append(packages, pkg)
	}

	return packages, total, nil
}

func (r *AssetPackageRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM asset_packages WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}
```

- [ ] **Step 3: 提交代码**

```bash
git add isdp/internal/model/asset_package.go isdp/internal/repo/asset_package.go
git commit -m "feat(model): 添加 AssetPackage 模型和 Repository"
```

---

## Task 4: 修改现有模型添加 Version 字段

> **注意**：Skill 模型已有 Version 字段（见 `isdp/internal/model/skill.go:45`），无需修改。本任务仅需为 Command、Subagent、Rule 添加 Version 字段。

**Files:**
- Modify: `isdp/internal/model/command.go`
- Modify: `isdp/internal/model/subagent.go`
- Modify: `isdp/internal/model/rule.go`

- [ ] **Step 1: 修改 Command 模型添加 Version 字段**

> **关于 Content 字段**：Command 的 Content 字段保留，用于读取时返回文件内容（从文件系统读取填充）。Create/Update 时不写入数据库，仅操作文件系统。

在 `isdp/internal/model/command.go` 的 Command 结构体中添加 Version 字段：

```go
// Command 命令模型
type Command struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Content     string    `json:"content,omitempty"`
	Version     string    `json:"version"` // 新增
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
```

同时更新 CreateCommandRequest 和 UpdateCommandRequest：

```go
// CreateCommandRequest 创建Command请求
type CreateCommandRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Version     string `json:"version"` // 新增
}

// UpdateCommandRequest 更新Command请求
type UpdateCommandRequest struct {
	Description string `json:"description"`
	Version     string `json:"version"` // 新增
}
```

- [ ] **Step 2: 修改 Subagent 模型添加 Version，移除 Content**

> **关于 SkillID 和 boundSkills**：
> - 现有 Subagent 模型的 `SkillID` 字段保留用于向后兼容（单个 Skill 关联）
> - 同时支持通过 `subagent_skill_bindings` 表的多对多绑定（现有 `SubagentSkillBindingRepository` 已支持）
> - manifest.json 中的 `boundSkills` 对应多对多绑定

在 `isdp/internal/model/subagent.go` 中修改：

```go
// Subagent 子代理模型
type Subagent struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	// Content 字段移除，改为文件系统存储
	SkillID     uuid.UUID `json:"skillId,omitempty"`
	Version     string    `json:"version"` // 新增
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// CreateSubagentRequest 创建Subagent请求
type CreateSubagentRequest struct {
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description"`
	// Content 移除，改为目录上传
	SkillID     uuid.UUID `json:"skillId"`
	Version     string    `json:"version"` // 新增
}

// UpdateSubagentRequest 更新Subagent请求
type UpdateSubagentRequest struct {
	Description string `json:"description"`
	Version     string `json:"version"` // 新增
}
```

- [ ] **Step 3: 修改 Rule 模型添加 Version 字段**

> **关于 Content 字段**：Rule 的 Content 字段保留，用于读取时返回文件内容（从文件系统读取填充）。Create/Update 时不写入数据库，仅操作文件系统。

在 `isdp/internal/model/rule.go` 中修改：

```go
// Rule 规约模型
type Rule struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Content     string         `json:"content,omitempty"`
	Visibility  RuleVisibility `json:"visibility"`
	Version     string         `json:"version"` // 新增
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// CreateRuleRequest 创建Rule请求
type CreateRuleRequest struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description"`
	Visibility  RuleVisibility `json:"visibility"`
	Content     string         `json:"content"`
	Version     string         `json:"version"` // 新增
}

// UpdateRuleRequest 更新Rule请求
type UpdateRuleRequest struct {
	Description string         `json:"description"`
	Visibility  RuleVisibility `json:"visibility"`
	Version     string         `json:"version"` // 新增
}
```

- [ ] **Step 4: 提交代码**

```bash
git add isdp/internal/model/command.go isdp/internal/model/subagent.go isdp/internal/model/rule.go
git commit -m "feat(model): 为 Command, Subagent, Rule 添加 Version 字段"
```

---

## Task 5: 修改现有 Repository 适配 Version 字段

**Files:**
- Modify: `isdp/internal/repo/command.go`
- Modify: `isdp/internal/repo/subagent.go`
- Modify: `isdp/internal/repo/rule.go`

- [ ] **Step 1: 修改 Command Repository**

在 `isdp/internal/repo/command.go` 中：
- Create 方法添加 version 字段
- scanCommand 方法添加 version 读取
- Update 方法添加 version 更新
- 添加 FindByNameAndVersion 方法用于导入时检测冲突

```go
// FindByNameAndVersion 根据名称和版本查找（用于导入时检测冲突）
func (r *CommandRepository) FindByNameAndVersion(ctx context.Context, name, version string) (*model.Command, error) {
	query := `SELECT id, name, description, version, created_at, updated_at FROM commands WHERE name = ? AND version = ?`
	// 使用现有的 scanCommand 逻辑
	row := r.db.QueryRowContext(ctx, query, name, version)
	// ... 扫描逻辑
}
```

- [ ] **Step 2: 修改 Subagent Repository**

在 `isdp/internal/repo/subagent.go` 中：
- Create 方法移除 content，添加 version
- scanSubagent 方法移除 content，添加 version
- Update 方法移除 content，添加 version
- List 方法相应修改
- 添加 FindByNameAndVersion 方法

- [ ] **Step 3: 修改 Rule Repository**

在 `isdp/internal/repo/rule.go` 中：
- Create 方法添加 version 字段
- scanRule 方法添加 version 读取
- Update 方法添加 version 更新
- 添加 FindByNameAndVersion 方法

- [ ] **Step 4: 提交代码**

```bash
git add isdp/internal/repo/command.go isdp/internal/repo/subagent.go isdp/internal/repo/rule.go
git commit -m "feat(repo): Repository 适配 Version 字段，Subagent 移除 Content"
```

---

## Task 6: Settings 业务服务

**Files:**
- Create: `isdp/internal/service/settings/service.go`

- [ ] **Step 1: 创建 Settings Service**

```go
// 文件路径: isdp/internal/service/settings/service.go
package settings

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	settingsRepo         *repo.SettingsRepository
	agentSettingsBinding *repo.AgentSettingsBindingRepository
	agentRepo            *repo.AgentConfigRepository
	storagePath          string
	logger               *zap.Logger
}

func NewService(
	settingsRepo *repo.SettingsRepository,
	agentSettingsBinding *repo.AgentSettingsBindingRepository,
	agentRepo *repo.AgentConfigRepository,
	storagePath string,
	logger *zap.Logger,
) *Service {
	return &Service{
		settingsRepo:         settingsRepo,
		agentSettingsBinding: agentSettingsBinding,
		agentRepo:            agentRepo,
		storagePath:          storagePath,
		logger:               logger,
	}
}

// Create 创建 Settings（从目录上传）
func (s *Service) Create(ctx context.Context, name, description string, files map[string][]byte) (*model.Settings, error) {
	// 检查名称是否重复
	existing, err := s.settingsRepo.FindByName(ctx, name)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, fmt.Errorf("检查名称失败: %w", err)
	}
	if existing != nil {
		return nil, errors.New("Settings 名称已存在")
	}

	settingsID := uuid.New()
	settingsDir := filepath.Join(s.storagePath, name)

	// 创建目录并写入文件
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	for relPath, content := range files {
		fullPath := filepath.Join(settingsDir, relPath)
		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return nil, fmt.Errorf("创建子目录失败: %w", err)
		}
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			return nil, fmt.Errorf("写入文件失败: %w", err)
		}
	}

	settings := &model.Settings{
		ID:            settingsID,
		Name:          name,
		Description:   description,
		DirectoryPath: settingsDir,
		Version:       "1.0.0",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.settingsRepo.Create(ctx, settings); err != nil {
		// 清理已创建的目录
		os.RemoveAll(settingsDir)
		return nil, fmt.Errorf("创建记录失败: %w", err)
	}

	return settings, nil
}

// GetByID 根据ID获取
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.Settings, error) {
	return s.settingsRepo.FindByID(ctx, id)
}

// List 列表
func (s *Service) List(ctx context.Context, query *model.SettingsListQuery) ([]*model.Settings, int64, error) {
	return s.settingsRepo.List(ctx, query)
}

// Update 更新元数据
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateSettingsRequest) (*model.Settings, error) {
	settings, err := s.settingsRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Settings 不存在: %w", err)
	}

	if req.Description != "" {
		settings.Description = req.Description
	}
	if req.Version != "" {
		settings.Version = req.Version
	}
	settings.UpdatedAt = time.Now()

	if err := s.settingsRepo.Update(ctx, settings); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}

	return settings, nil
}

// Delete 删除 Settings
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	settings, err := s.settingsRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("Settings 不存在: %w", err)
	}

	// 检查绑定
	agentRoleIDs, err := s.agentSettingsBinding.FindBySettingsID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查绑定失败: %w", err)
	}
	if len(agentRoleIDs) > 0 {
		agentNames := make([]string, 0)
		for _, agentID := range agentRoleIDs {
			agent, err := s.agentRepo.FindByID(ctx, agentID)
			if err == nil {
				agentNames = append(agentNames, agent.Name)
			}
		}
		return fmt.Errorf("无法删除：已被以下 Agent 绑定：%s", strings.Join(agentNames, "、"))
	}

	// 删除数据库记录
	if err := s.settingsRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除记录失败: %w", err)
	}

	// 删除目录
	if settings.DirectoryPath != "" {
		if err := os.RemoveAll(settings.DirectoryPath); err != nil {
			s.logger.Warn("删除目录失败", zap.String("path", settings.DirectoryPath), zap.Error(err))
		}
	}

	return nil
}

// BindSettings 绑定 Settings 到 Agent
func (s *Service) BindSettings(ctx context.Context, agentRoleID uuid.UUID, settingsIDs []uuid.UUID) error {
	// 验证 Agent 存在
	_, err := s.agentRepo.FindByID(ctx, agentRoleID)
	if err != nil {
		return fmt.Errorf("Agent 不存在: %w", err)
	}

	// 验证 Settings 存在
	for _, settingsID := range settingsIDs {
		_, err := s.settingsRepo.FindByID(ctx, settingsID)
		if err != nil {
			return fmt.Errorf("Settings %s 不存在: %w", settingsID.String(), err)
		}
	}

	// 清理旧绑定
	if err := s.agentSettingsBinding.DeleteByAgentRoleID(ctx, agentRoleID); err != nil {
		return fmt.Errorf("清理旧绑定失败: %w", err)
	}

	// 创建新绑定
	for _, settingsID := range settingsIDs {
		binding := &model.AgentSettingsBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			SettingsID:  settingsID,
			CreatedAt:   time.Now(),
		}
		if err := s.agentSettingsBinding.Create(ctx, binding); err != nil {
			return err
		}
	}

	return nil
}

// UnbindSettings 解绑
func (s *Service) UnbindSettings(ctx context.Context, agentRoleID, settingsID uuid.UUID) error {
	return s.agentSettingsBinding.DeleteBinding(ctx, agentRoleID, settingsID)
}

// GetBoundSettings 获取 Agent 绑定的 Settings
func (s *Service) GetBoundSettings(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Settings, error) {
	settingsIDs, err := s.agentSettingsBinding.FindByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		return nil, err
	}

	settings := make([]*model.Settings, 0, len(settingsIDs))
	for _, settingsID := range settingsIDs {
		s, err := s.settingsRepo.FindByID(ctx, settingsID)
		if err != nil {
			continue
		}
		settings = append(settings, s)
	}

	return settings, nil
}

// ReadDirectoryContent 读取 Settings 目录内容
func (s *Service) ReadDirectoryContent(settings *model.Settings) (map[string][]byte, error) {
	if settings.DirectoryPath == "" {
		return nil, errors.New("目录路径未配置")
	}

	files := make(map[string][]byte)
	err := filepath.Walk(settings.DirectoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(settings.DirectoryPath, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[relPath] = content
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	return files, nil
}

// WriteDirectoryContent 写入目录内容（用于导入）
func (s *Service) WriteDirectoryContent(targetPath string, files map[string][]byte) error {
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return err
	}

	for relPath, content := range files {
		fullPath := filepath.Join(targetPath, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			return err
		}
	}

	return nil
}
```

- [ ] **Step 2: 提交代码**

```bash
git add isdp/internal/service/settings/service.go
git commit -m "feat(service): 添加 Settings 业务服务"
```

---

## Task 7: 资产包导入导出服务

**Files:**
- Create: `isdp/internal/service/assetpackage/service.go`

- [ ] **Step 1: 创建资产包导入导出服务**

```go
// 文件路径: isdp/internal/service/assetpackage/service.go
package assetpackage

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/settings"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	packageRepo           *repo.AssetPackageRepository
	skillRepo             *repo.SkillRepository
	commandRepo           *repo.CommandRepository
	subagentRepo          *repo.SubagentRepository
	ruleRepo              *repo.RuleRepository
	settingsRepo          *repo.SettingsRepository
	settingsService       *settings.Service
	commandSkillBinding   *repo.CommandSkillBindingRepository
	subagentSkillBinding  *repo.SubagentSkillBindingRepository
	skillStoragePath      string
	subagentStoragePath   string
	commandStoragePath    string
	ruleStoragePath       string
	settingsStoragePath   string
	logger                *zap.Logger
}

func NewService(
	packageRepo *repo.AssetPackageRepository,
	skillRepo *repo.SkillRepository,
	commandRepo *repo.CommandRepository,
	subagentRepo *repo.SubagentRepository,
	ruleRepo *repo.RuleRepository,
	settingsRepo *repo.SettingsRepository,
	settingsService *settings.Service,
	commandSkillBinding *repo.CommandSkillBindingRepository,
	subagentSkillBinding *repo.SubagentSkillBindingRepository,
	skillStoragePath string,
	subagentStoragePath string,
	commandStoragePath string,
	ruleStoragePath string,
	settingsStoragePath string,
	logger *zap.Logger,
) *Service {
	return &Service{
		packageRepo:           packageRepo,
		skillRepo:             skillRepo,
		commandRepo:           commandRepo,
		subagentRepo:          subagentRepo,
		ruleRepo:              ruleRepo,
		settingsRepo:          settingsRepo,
		settingsService:       settingsService,
		commandSkillBinding:   commandSkillBinding,
		subagentSkillBinding:  subagentSkillBinding,
		skillStoragePath:      skillStoragePath,
		subagentStoragePath:   subagentStoragePath,
		commandStoragePath:    commandStoragePath,
		ruleStoragePath:       ruleStoragePath,
		settingsStoragePath:   settingsStoragePath,
		logger:                logger,
	}
}

// List 列出资产包
func (s *Service) List(ctx context.Context, query *model.AssetPackageListQuery) ([]*model.AssetPackage, int64, error) {
	return s.packageRepo.List(ctx, query)
}

// GetByID 获取资产包详情
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.AssetPackage, error) {
	return s.packageRepo.FindByID(ctx, id)
}

// Delete 删除资产包记录（不影响已导入的资产）
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.packageRepo.Delete(ctx, id)
}

// Export 导出资产包
func (s *Service) Export(ctx context.Context, req *model.ExportAssetPackageRequest) ([]byte, string, error) {
	// 生成完整版本号：v{version}-{YYYYMMDD}-{HHMMSS}
	now := time.Now()
	fullVersion := fmt.Sprintf("v%s-%s-%s", req.Version, now.Format("20060102"), now.Format("150405"))

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "asset-export-*")
	if err != nil {
		return nil, "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 构建 manifest
	manifest := &model.AssetPackageManifest{
		Name:        req.Name,
		Version:     fullVersion,
		ExportedAt:  now.Format(time.RFC3339),
		Description: req.Description,
		Assets:      model.AssetPackageAssetsList{},
	}

	// 导出 Skills
	if len(req.SkillIDs) > 0 {
		skillDir := filepath.Join(tempDir, "skills")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return nil, "", err
		}

		for _, skillID := range req.SkillIDs {
			skill, err := s.skillRepo.FindByID(ctx, skillID)
			if err != nil {
				s.logger.Warn("导出 Skill 失败", zap.String("id", skillID.String()), zap.Error(err))
				continue
			}

			// 复制 Skill 目录
			srcDir := filepath.Join(s.skillStoragePath, skill.Name)
			dstDir := filepath.Join(skillDir, skill.Name)
			if err := copyDir(srcDir, dstDir); err != nil {
				s.logger.Warn("复制 Skill 目录失败", zap.String("skill", skill.Name), zap.Error(err))
				continue
			}

			manifest.Assets.Skills = append(manifest.Assets.Skills, model.AssetPackageSkillItem{
				Name:    skill.Name,
				Version: skill.Version,
			})
		}
	}

	// 导出 Commands
	if len(req.CommandIDs) > 0 {
		cmdDir := filepath.Join(tempDir, "commands")
		if err := os.MkdirAll(cmdDir, 0755); err != nil {
			return nil, "", err
		}

		for _, cmdID := range req.CommandIDs {
			cmd, err := s.commandRepo.FindByID(ctx, cmdID)
			if err != nil {
				s.logger.Warn("导出 Command 失败", zap.String("id", cmdID.String()), zap.Error(err))
				continue
			}

			// 复制 Command 文件
			srcFile := filepath.Join(s.commandStoragePath, cmd.Name+".md")
			dstFile := filepath.Join(cmdDir, cmd.Name+".md")
			if err := copyFile(srcFile, dstFile); err != nil {
				s.logger.Warn("复制 Command 文件失败", zap.String("command", cmd.Name), zap.Error(err))
				continue
			}

			// 获取关联的 Skills
			boundSkills := []string{}
			skillIDs, err := s.commandSkillBinding.FindByCommandID(ctx, cmdID)
			if err == nil {
				for _, skillID := range skillIDs {
					skill, err := s.skillRepo.FindByID(ctx, skillID)
					if err == nil {
						boundSkills = append(boundSkills, skill.Name)
					}
				}
			}

			manifest.Assets.Commands = append(manifest.Assets.Commands, model.AssetPackageCommandItem{
				Name:        cmd.Name,
				Version:     cmd.Version,
				BoundSkills: boundSkills,
			})
		}
	}

	// 导出 Subagents
	if len(req.SubagentIDs) > 0 {
		subDir := filepath.Join(tempDir, "subagents")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			return nil, "", err
		}

		for _, subID := range req.SubagentIDs {
			sub, err := s.subagentRepo.FindByID(ctx, subID)
			if err != nil {
				s.logger.Warn("导出 Subagent 失败", zap.String("id", subID.String()), zap.Error(err))
				continue
			}

			// 复制 Subagent 文件
			srcFile := filepath.Join(s.subagentStoragePath, sub.Name+".md")
			dstFile := filepath.Join(subDir, sub.Name+".md")
			if err := copyFile(srcFile, dstFile); err != nil {
				s.logger.Warn("复制 Subagent 文件失败", zap.String("subagent", sub.Name), zap.Error(err))
				continue
			}

			// 获取关联的 Skills
			boundSkills := []string{}
			skillIDs, err := s.subagentSkillBinding.FindBySubagentID(ctx, subID)
			if err == nil {
				for _, skillID := range skillIDs {
					skill, err := s.skillRepo.FindByID(ctx, skillID)
					if err == nil {
						boundSkills = append(boundSkills, skill.Name)
					}
				}
			}

			manifest.Assets.Subagents = append(manifest.Assets.Subagents, model.AssetPackageSubagentItem{
				Name:        sub.Name,
				Version:     sub.Version,
				BoundSkills: boundSkills,
			})
		}
	}

	// 导出 Rules
	if len(req.RuleIDs) > 0 {
		ruleDir := filepath.Join(tempDir, "rules")
		if err := os.MkdirAll(ruleDir, 0755); err != nil {
			return nil, "", err
		}

		for _, ruleID := range req.RuleIDs {
			rule, err := s.ruleRepo.FindByID(ctx, ruleID)
			if err != nil {
				s.logger.Warn("导出 Rule 失败", zap.String("id", ruleID.String()), zap.Error(err))
				continue
			}

			// 复制 Rule 文件
			srcFile := filepath.Join(s.ruleStoragePath, rule.Name+".md")
			dstFile := filepath.Join(ruleDir, rule.Name+".md")
			if err := copyFile(srcFile, dstFile); err != nil {
				s.logger.Warn("复制 Rule 文件失败", zap.String("rule", rule.Name), zap.Error(err))
				continue
			}

			manifest.Assets.Rules = append(manifest.Assets.Rules, model.AssetPackageRuleItem{
				Name:    rule.Name,
				Version: rule.Version,
			})
		}
	}

	// 导出 Settings
	if len(req.SettingsIDs) > 0 {
		settingsDir := filepath.Join(tempDir, "settings")
		if err := os.MkdirAll(settingsDir, 0755); err != nil {
			return nil, "", err
		}

		for _, settingsID := range req.SettingsIDs {
			settings, err := s.settingsRepo.FindByID(ctx, settingsID)
			if err != nil {
				s.logger.Warn("导出 Settings 失败", zap.String("id", settingsID.String()), zap.Error(err))
				continue
			}

			// 复制 Settings 目录
			dstDir := filepath.Join(settingsDir, settings.Name)
			if settings.DirectoryPath != "" {
				if err := copyDir(settings.DirectoryPath, dstDir); err != nil {
					s.logger.Warn("复制 Settings 目录失败", zap.String("settings", settings.Name), zap.Error(err))
					continue
				}
			}

			manifest.Assets.Settings = append(manifest.Assets.Settings, model.AssetPackageSettingsItem{
				Name: settings.Name,
			})
		}
	}

	// 写入 manifest.json
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("序列化 manifest 失败: %w", err)
	}
	manifestPath := filepath.Join(tempDir, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return nil, "", fmt.Errorf("写入 manifest 失败: %w", err)
	}

	// 创建 ZIP 文件
	zipPath := filepath.Join(tempDir, "package.zip")
	if err := createZip(tempDir, zipPath); err != nil {
		return nil, "", fmt.Errorf("创建 ZIP 失败: %w", err)
	}

	// 读取 ZIP 内容
	zipData, err := os.ReadFile(zipPath)
	if err != nil {
		return nil, "", fmt.Errorf("读取 ZIP 失败: %w", err)
	}

	// 创建资产包记录
	pkg := &model.AssetPackage{
		ID:          uuid.New(),
		Name:        req.Name,
		Version:     fullVersion,
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.packageRepo.Create(ctx, pkg); err != nil {
		s.logger.Warn("创建资产包记录失败", zap.Error(err))
	}

	// 生成文件名
	fileName := fmt.Sprintf("%s_%s.zip", req.Name, fullVersion)

	return zipData, fileName, nil
}

// Import 导入资产包
func (s *Service) Import(ctx context.Context, zipData []byte) (*model.ImportResult, error) {
	// 创建临时目录解压
	tempDir, err := os.MkdirTemp("", "asset-import-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 解压 ZIP
	if err := extractZip(zipData, tempDir); err != nil {
		return nil, fmt.Errorf("解压 ZIP 失败: %w", err)
	}

	// 读取 manifest.json
	manifestPath := filepath.Join(tempDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("读取 manifest.json 失败: %w", err)
	}

	var manifest model.AssetPackageManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("解析 manifest.json 失败: %w", err)
	}

	// 创建资产包记录
	pkgID := uuid.New()
	pkg := &model.AssetPackage{
		ID:          pkgID,
		Name:        manifest.Name,
		Version:     manifest.Version,
		Description: manifest.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.packageRepo.Create(ctx, pkg); err != nil {
		return nil, fmt.Errorf("创建资产包记录失败: %w", err)
	}

	result := &model.ImportResult{
		PackageName: manifest.Name,
		PackageID:   pkgID,
		Details:     []model.ImportDetail{},
	}

	// 导入 Skills
	if len(manifest.Assets.Skills) > 0 {
		skillDir := filepath.Join(tempDir, "skills")
		for _, item := range manifest.Assets.Skills {
			detail := s.importSkill(ctx, item, skillDir)
			result.Details = append(result.Details, detail)
			if detail.Status == "success" {
				result.Success++
			} else if detail.Status == "skipped" {
				result.Skipped++
			} else {
				result.Failed++
			}
		}
	}

	// 导入 Commands
	if len(manifest.Assets.Commands) > 0 {
		cmdDir := filepath.Join(tempDir, "commands")
		for _, item := range manifest.Assets.Commands {
			detail := s.importCommand(ctx, item, cmdDir, manifest.Assets.Skills)
			result.Details = append(result.Details, detail)
			if detail.Status == "success" {
				result.Success++
			} else if detail.Status == "skipped" {
				result.Skipped++
			} else {
				result.Failed++
			}
		}
	}

	// 导入 Subagents
	if len(manifest.Assets.Subagents) > 0 {
		subDir := filepath.Join(tempDir, "subagents")
		for _, item := range manifest.Assets.Subagents {
			detail := s.importSubagent(ctx, item, subDir, manifest.Assets.Skills)
			result.Details = append(result.Details, detail)
			if detail.Status == "success" {
				result.Success++
			} else if detail.Status == "skipped" {
				result.Skipped++
			} else {
				result.Failed++
			}
		}
	}

	// 导入 Rules
	if len(manifest.Assets.Rules) > 0 {
		ruleDir := filepath.Join(tempDir, "rules")
		for _, item := range manifest.Assets.Rules {
			detail := s.importRule(ctx, item, ruleDir)
			result.Details = append(result.Details, detail)
			if detail.Status == "success" {
				result.Success++
			} else if detail.Status == "skipped" {
				result.Skipped++
			} else {
				result.Failed++
			}
		}
	}

	// 导入 Settings
	if len(manifest.Assets.Settings) > 0 {
		settingsDir := filepath.Join(tempDir, "settings")
		for _, item := range manifest.Assets.Settings {
			detail := s.importSettings(ctx, item, settingsDir)
			result.Details = append(result.Details, detail)
			if detail.Status == "success" {
				result.Success++
			} else if detail.Status == "skipped" {
				result.Skipped++
			} else {
				result.Failed++
			}
		}
	}

	return result, nil
}

// importSkill 导入单个 Skill
func (s *Service) importSkill(ctx context.Context, item model.AssetPackageSkillItem, skillDir string) model.ImportDetail {
	detail := model.ImportDetail{
		AssetType: "skill",
		Name:      item.Name,
		Version:   item.Version,
	}

	// 检查是否存在（名称+版本）
	existing, err := s.skillRepo.FindByName(ctx, item.Name)
	if err == nil && existing.Version == item.Version {
		detail.Status = "skipped"
		detail.Message = "已存在相同版本"
		return detail
	}

	// 复制 Skill 目录
	srcDir := filepath.Join(skillDir, item.Name)
	dstDir := filepath.Join(s.skillStoragePath, item.Name)

	// 如果存在不同版本，跳过（保留现有版本）
	if existing != nil {
		detail.Status = "skipped"
		detail.Message = fmt.Sprintf("已存在版本 %s，保留现有", existing.Version)
		return detail
	}

	if err := copyDir(srcDir, dstDir); err != nil {
		detail.Status = "failed"
		detail.Message = err.Error()
		return detail
	}

	// 创建数据库记录
	skill := &model.Skill{
		ID:              uuid.New(),
		Name:            item.Name,
		Version:         item.Version,
		SourceType:      model.SkillSourcePersonal,
		Status:          model.SkillStatusActive,
		SupportedAgents: []string{"claude_code", "open_code"},
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.skillRepo.Create(ctx, skill); err != nil {
		os.RemoveAll(dstDir)
		detail.Status = "failed"
		detail.Message = err.Error()
		return detail
	}

	detail.Status = "success"
	return detail
}

// 其他 import 方法类似实现...

// 辅助函数
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil { return err }
		relPath, err := filepath.Rel(src, path)
		if err != nil { return err }
		dstPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil { return err }
	defer srcFile.Close()
	dstFile, err := os.Create(dst)
	if err != nil { return err }
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}

func createZip(srcDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil { return err }
	defer zipFile.Close()
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil { return err }
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil { return err }
		if info.IsDir() {
			zipWriter.CreateHeader(&zip.FileHeader{Name: relPath + "/", Mode: uint32(info.Mode())})
			return nil
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil { return err }
		header.Name = relPath
		writer, err := zipWriter.CreateHeader(header)
		if err != nil { return err }
		file, err := os.Open(path)
		if err != nil { return err }
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
}

func extractZip(zipData []byte, dstDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil { return err }
	for _, file := range reader.File {
		dstPath := filepath.Join(dstDir, file.Name)
		if file.FileInfo().IsDir() {
			os.MkdirAll(dstPath, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(dstPath), 0755)
		dstFile, err := os.Create(dstPath)
		if err != nil { return err }
		srcFile, err := file.Open()
		if err != nil {
			dstFile.Close()
			return err
		}
		io.Copy(dstFile, srcFile)
		srcFile.Close()
		dstFile.Close()
	}
	return nil
}
```

- [ ] **Step 2: 补充完整的 import 方法**

```go
// importCommand 导入单个 Command
func (s *Service) importCommand(ctx context.Context, item model.AssetPackageCommandItem, cmdDir string, existingSkills []model.AssetPackageSkillItem) model.ImportDetail {
	detail := model.ImportDetail{
		AssetType: "command",
		Name:      item.Name,
		Version:   item.Version,
	}

	// 检查是否存在
	existing, err := s.commandRepo.FindByName(ctx, item.Name)
	if err == nil && existing.Version == item.Version {
		detail.Status = "skipped"
		detail.Message = "已存在相同版本"
		return detail
	}

	// 复制 Command 文件
	srcFile := filepath.Join(cmdDir, item.Name+".md")
	dstFile := filepath.Join(s.commandStoragePath, item.Name+".md")

	if existing != nil {
		detail.Status = "skipped"
		detail.Message = fmt.Sprintf("已存在版本 %s，保留现有", existing.Version)
		return detail
	}

	if err := copyFile(srcFile, dstFile); err != nil {
		detail.Status = "failed"
		detail.Message = err.Error()
		return detail
	}

	// 创建数据库记录
	cmd := &model.Command{
		ID:          uuid.New(),
		Name:        item.Name,
		Version:     item.Version,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.commandRepo.Create(ctx, cmd); err != nil {
		os.Remove(dstFile)
		detail.Status = "failed"
		detail.Message = err.Error()
		return detail
	}

	// 恢复 Skill 绑定
	for _, skillName := range item.BoundSkills {
		skill, err := s.skillRepo.FindByName(ctx, skillName)
		if err == nil {
			binding := &model.CommandSkillBinding{
				ID:        uuid.New(),
				CommandID: cmd.ID,
				SkillID:   skill.ID,
				CreatedAt: time.Now(),
			}
			s.commandSkillBinding.Create(ctx, binding)
		}
	}

	detail.Status = "success"
	return detail
}

// importSubagent 导入单个 Subagent
func (s *Service) importSubagent(ctx context.Context, item model.AssetPackageSubagentItem, subDir string, existingSkills []model.AssetPackageSkillItem) model.ImportDetail {
	detail := model.ImportDetail{
		AssetType: "subagent",
		Name:      item.Name,
		Version:   item.Version,
	}

	existing, err := s.subagentRepo.FindByName(ctx, item.Name)
	if err == nil && existing.Version == item.Version {
		detail.Status = "skipped"
		detail.Message = "已存在相同版本"
		return detail
	}

	srcFile := filepath.Join(subDir, item.Name+".md")
	dstFile := filepath.Join(s.subagentStoragePath, item.Name+".md")

	if existing != nil {
		detail.Status = "skipped"
		detail.Message = fmt.Sprintf("已存在版本 %s，保留现有", existing.Version)
		return detail
	}

	if err := copyFile(srcFile, dstFile); err != nil {
		detail.Status = "failed"
		detail.Message = err.Error()
		return detail
	}

	sub := &model.Subagent{
		ID:        uuid.New(),
		Name:      item.Name,
		Version:   item.Version,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.subagentRepo.Create(ctx, sub); err != nil {
		os.Remove(dstFile)
		detail.Status = "failed"
		detail.Message = err.Error()
		return detail
	}

	// 恢复 Skill 绑定
	for _, skillName := range item.BoundSkills {
		skill, err := s.skillRepo.FindByName(ctx, skillName)
		if err == nil {
			binding := &model.SubagentSkillBinding{
				ID:         uuid.New(),
				SubagentID: sub.ID,
				SkillID:    skill.ID,
				CreatedAt:  time.Now(),
			}
			s.subagentSkillBinding.Create(ctx, binding)
		}
	}

	detail.Status = "success"
	return detail
}

// importRule 导入单个 Rule
func (s *Service) importRule(ctx context.Context, item model.AssetPackageRuleItem, ruleDir string) model.ImportDetail {
	detail := model.ImportDetail{
		AssetType: "rule",
		Name:      item.Name,
		Version:   item.Version,
	}

	existing, err := s.ruleRepo.FindByName(ctx, item.Name)
	if err == nil && existing.Version == item.Version {
		detail.Status = "skipped"
		detail.Message = "已存在相同版本"
		return detail
	}

	srcFile := filepath.Join(ruleDir, item.Name+".md")
	dstFile := filepath.Join(s.ruleStoragePath, item.Name+".md")

	if existing != nil {
		detail.Status = "skipped"
		detail.Message = fmt.Sprintf("已存在版本 %s，保留现有", existing.Version)
		return detail
	}

	if err := copyFile(srcFile, dstFile); err != nil {
		detail.Status = "failed"
		detail.Message = err.Error()
		return detail
	}

	rule := &model.Rule{
		ID:          uuid.New(),
		Name:        item.Name,
		Version:     item.Version,
		Visibility:  model.RuleVisibilityPrivate,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.ruleRepo.Create(ctx, rule); err != nil {
		os.Remove(dstFile)
		detail.Status = "failed"
		detail.Message = err.Error()
		return detail
	}

	detail.Status = "success"
	return detail
}

// importSettings 导入单个 Settings
func (s *Service) importSettings(ctx context.Context, item model.AssetPackageSettingsItem, settingsDir string) model.ImportDetail {
	detail := model.ImportDetail{
		AssetType: "settings",
		Name:      item.Name,
	}

	existing, err := s.settingsRepo.FindByName(ctx, item.Name)
	if err == nil {
		detail.Status = "skipped"
		detail.Message = "已存在同名 Settings"
		return detail
	}

	srcDir := filepath.Join(settingsDir, item.Name)
	dstDir := filepath.Join(s.settingsStoragePath, item.Name)

	if err := copyDir(srcDir, dstDir); err != nil {
		detail.Status = "failed"
		detail.Message = err.Error()
		return detail
	}

	settings := &model.Settings{
		ID:            uuid.New(),
		Name:          item.Name,
		DirectoryPath: dstDir,
		Version:       "1.0.0",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.settingsRepo.Create(ctx, settings); err != nil {
		os.RemoveAll(dstDir)
		detail.Status = "failed"
		detail.Message = err.Error()
		return detail
	}

	detail.Status = "success"
	return detail
}
```

- [ ] **Step 3: 提交代码**

```bash
git add isdp/internal/service/assetpackage/service.go
git commit -m "feat(service): 添加资产包导入导出服务"
```

---

## Task 8: HTTP Handler

**Files:**
- Create: `isdp/internal/api/settings_handler.go`
- Create: `isdp/internal/api/asset_package_handler.go`
- Modify: `isdp/cmd/server/main.go`

- [ ] **Step 1: 创建 Settings Handler**

```go
// 文件路径: isdp/internal/api/settings_handler.go
package api

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/settings"
)

type SettingsHandler struct {
	service *settings.Service
}

func NewSettingsHandler(service *settings.Service) *SettingsHandler {
	return &SettingsHandler{service: service}
}

// List 列表
func (h *SettingsHandler) List(c *gin.Context) {
	var query model.SettingsListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	list, total, err := h.service.List(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": list, "total": total})
}

// Create 创建（接收目录上传）
func (h *SettingsHandler) Create(c *gin.Context) {
	// 接收 multipart form，包含目录文件
	name := c.PostForm("name")
	description := c.PostForm("description")

	// 解析上传的文件
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的表单数据"})
		return
	}

	files := make(map[string][]byte)
	for _, fileHeaders := range form.File {
		for _, fileHeader := range fileHeaders {
			file, err := fileHeader.Open()
			if err != nil { continue }
			content := make([]byte, fileHeader.Size)
			file.Read(content)
			file.Close()
			files[fileHeader.Filename] = content
		}
	}

	result, err := h.service.Create(c.Request.Context(), name, description, files)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// GetByID 获取详情
func (h *SettingsHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	result, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Delete 删除
func (h *SettingsHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// BindToAgent 绑定到 AgentRole
func (h *SettingsHandler) BindToAgent(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("agentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的Agent ID"})
		return
	}

	var req model.BindSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.BindSettings(c.Request.Context(), agentRoleID, req.SettingsIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "绑定成功"})
}
```

- [ ] **Step 2: 创建 Asset Package Handler**

```go
// 文件路径: isdp/internal/api/asset_package_handler.go
package api

import (
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/assetpackage"
)

type AssetPackageHandler struct {
	service *assetpackage.Service
}

func NewAssetPackageHandler(service *assetpackage.Service) *AssetPackageHandler {
	return &AssetPackageHandler{service: service}
}

// List 列表
func (h *AssetPackageHandler) List(c *gin.Context) {
	var query model.AssetPackageListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	list, total, err := h.service.List(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": list, "total": total})
}

// GetByID 获取详情
func (h *AssetPackageHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	result, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Import 导入资产包
func (h *AssetPackageHandler) Import(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请上传文件"})
		return
	}

	zipData := make([]byte, c.Request.ContentLength)
	file.Read(zipData)
	file.Close()

	result, err := h.service.Import(c.Request.Context(), zipData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Export 导出资产包
func (h *AssetPackageHandler) Export(c *gin.Context) {
	var req model.ExportAssetPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	zipData, fileName, err := h.service.Export(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Data(http.StatusOK, "application/zip", zipData)
}

// Delete 删除资产包记录
func (h *AssetPackageHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
```

- [ ] **Step 3: 在 main.go 注册路由**

```go
// 在 main.go 的路由注册部分添加：

// Settings 路由
settingsHandler := api.NewSettingsHandler(settingsService)
v1.GET("/settings", settingsHandler.List)
v1.GET("/settings/:id", settingsHandler.GetByID)
v1.POST("/settings", settingsHandler.Create)
v1.DELETE("/settings/:id", settingsHandler.Delete)
v1.POST("/agent-roles/:agentId/settings", settingsHandler.BindToAgent)

// Asset Package 路由
packageHandler := api.NewAssetPackageHandler(packageService)
v1.GET("/asset-packages", packageHandler.List)
v1.GET("/asset-packages/:id", packageHandler.GetByID)
v1.POST("/asset-packages/import", packageHandler.Import)
v1.POST("/asset-packages/export", packageHandler.Export)
v1.DELETE("/asset-packages/:id", packageHandler.Delete)
```

- [ ] **Step 4: 提交代码**

---

## Task 9: 修改配置生成服务集成 Settings

**Files:**
- Modify: `isdp/internal/service/configgen/service.go`

- [ ] **Step 1: 添加 Settings 相关依赖**

- [ ] **Step 2: 修改 GenerateAgentConfig 方法**

在配置生成时，将绑定的 Settings 目录内容拷贝到生成的配置目录。

- [ ] **Step 3: 提交代码**

---

## Task 10: 前端资产包管理页面

**Files:**
- Create: `isdp/web/src/pages/AssetPackage/index.tsx`
- Create: `isdp/web/src/api/assetPackage.ts`
- Modify: `isdp/web/src/api/client.ts`
- Modify: `isdp/web/src/App.tsx` (路由配置在此文件中)

> **路由路径建议**：使用 `/asset-packages` 作为资产包管理页面路由，避免与现有路由冲突。

- [ ] **Step 1: 创建前端 API**

- [ ] **Step 2: 创建资产包管理页面**

页面功能：
- 资产包列表（卡片或表格展示）
- 导入按钮（上传 ZIP）
- 导出按钮（选择资产，生成 ZIP）
- 详情查看
- 删除

- [ ] **Step 3: 添加路由**

- [ ] **Step 4: 提交代码**

---

## Task 11: 前端 Settings 管理

**Files:**
- Create: `isdp/web/src/pages/SettingsManagement/index.tsx`
- Create: `isdp/web/src/api/settingsApi.ts`
- Modify: `isdp/web/src/App.tsx` (路由配置在此文件中)

> **路由路径建议**：使用 `/settings-assets` 作为 Settings 资产管理页面路由，避免与现有的 `/agents/settings`（PlaceholderPage）冲突。

- [ ] **Step 1: 创建 Settings API**

- [ ] **Step 2: 创建 Settings 管理页面**

支持：
- 目录上传（类似 Skill 的目录上传）
- 列表展示
- 绑定到 AgentRole

- [ ] **Step 3: 在 AgentRole 绑定界面添加 Settings 绑定**

- [ ] **Step 4: 提交代码**

---

## Task 12: Subagent 文件存储迁移

**Files:**
- Modify: `isdp/internal/service/subagent/service.go`
- Create: `isdp/sql-change/migrations/202603310003_migrate_subagent_content.sql`
- Create: `isdp/scripts/migrate_subagent_content.go` (应用程序迁移脚本)

> **重要说明**：纯 SQL 无法完成文件写入，需要通过应用程序脚本执行迁移。

- [ ] **Step 1: 创建数据迁移应用程序脚本**

```go
// 文件路径: isdp/scripts/migrate_subagent_content.go
// 运行方式: go run ./scripts/migrate_subagent_content.go
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// 从环境变量或配置获取数据库连接
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "user:password@tcp(localhost:3306)/isdp?parseTime=true"
	}

	storagePath := os.Getenv("SUBAGENT_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "./data/agent-assets/subagents"
	}

	db, err := sql.Open("mysql", dbURL)
	if err != nil {
		fmt.Printf("连接数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// 查询所有 subagent
	rows, err := db.Query("SELECT id, name, content FROM subagents WHERE content IS NOT NULL AND content != ''")
	if err != nil {
		fmt.Printf("查询失败: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	// 确保存储目录存在
	os.MkdirAll(storagePath, 0755)

	count := 0
	for rows.Next() {
		var id, name, content string
		if err := rows.Scan(&id, &name, &content); err != nil {
			fmt.Printf("扫描失败: %v\n", err)
			continue
		}

		// 写入文件
		filePath := filepath.Join(storagePath, name+".md")
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			fmt.Printf("写入文件失败 [%s]: %v\n", name, err)
			continue
		}

		count++
		fmt.Printf("迁移成功: %s -> %s\n", name, filePath)
	}

	fmt.Printf("\n迁移完成，共处理 %d 条记录\n", count)
}
```

- [ ] **Step 2: 创建 SQL 迁移脚本（标记性文件）**

```sql
-- 文件路径: isdp/sql-change/migrations/202603310003_migrate_subagent_content.sql
-- 变更说明：Subagent Content 从数据库迁移到文件系统
-- 注意：此迁移需要先运行应用程序脚本 migrate_subagent_content.go
-- 作者：AI Assistant
-- 日期：2026-03-31

SET NAMES utf8mb4;

-- 注意：先运行 go run ./scripts/migrate_subagent_content.go 完成数据迁移
-- 验证迁移成功后，执行以下语句移除 content 字段

-- 移除 content 字段（迁移完成后执行）
-- ALTER TABLE subagents DROP COLUMN content;

-- 回滚语句（如果需要回滚，需要重新添加 content 字段并从文件读取内容）
-- ALTER TABLE subagents ADD COLUMN content TEXT AFTER description;
```

- [ ] **Step 3: 修改 Subagent Service**

- 上传改为目录上传
- Content 改为从文件系统读取
- 删除时删除对应文件

- [ ] **Step 3: 执行迁移**

- [ ] **Step 4: 提交代码**

---

## Task 13: 集成测试和验证

- [ ] **Step 1: 测试导入导出流程**

手动测试：
1. 导出多个资产到 ZIP
2. 导入 ZIP 包
3. 验证资产正确导入
4. 验证绑定关系恢复

- [ ] **Step 2: 测试 Settings 功能**

1. 上传 Settings 目录
2. 绑定到 AgentRole
3. 生成配置验证 Settings 拷贝正确

- [ ] **Step 3: 测试版本冲突检测**

导入相同名称+版本的资产，验证跳过逻辑。

---

## 需要清理的旧产物

执行数据迁移后，需要从数据库移除 subagents.content 字段：

```sql
ALTER TABLE subagents DROP COLUMN content;
```

---

## 验证方法

### 后端单元测试

```bash
cd isdp && go test ./internal/service/assetpackage/... -v -cover
cd isdp && go test ./internal/service/settings/... -v -cover
cd isdp && go test ./internal/repo/... -v -cover
```

### API 测试

```bash
# 导出资产包
curl -X POST http://localhost:8080/api/v1/asset-packages/export \
  -H "Content-Type: application/json" \
  -d '{"name":"测试包","version":"1.0.0","skillIds":["uuid-1","uuid-2"]}' \
  --output test_package.zip

# 导入资产包
curl -X POST http://localhost:8080/api/v1/asset-packages/import \
  -F "file=@test_package.zip"

# Settings CRUD
curl http://localhost:8080/api/v1/settings
curl -X POST http://localhost:8080/api/v1/settings -F "name=团队配置" -F "description=测试" -F "files=@./settings_dir/"
```

### 前端手动测试

1. 访问资产包管理页面 `/asset-packages`
2. 测试导出流程：选择多个资产 → 输入包名和版本 → 下载 ZIP
3. 测试导入流程：上传 ZIP → 查看导入结果 → 验证资产列表
4. 测试 Settings：上传配置目录 → 绑定到 AgentRole → 生成配置验证

### 版本冲突测试

1. 导出资产包 A（版本 1.0.0）
2. 修改资产 A 的内容
3. 再次导出资产包 A（版本 1.0.1）
4. 导入第一个包，验证成功
5. 再次导入第一个包，验证跳过（相同版本）
6. 导入第二个包，验证跳过（不同版本但保留现有）

---

## API 版本说明

所有 API 统一使用 `/api/v1/` 前缀：
- Settings API: `/api/v1/settings`
- Asset Package API: `/api/v1/asset-packages`

### manifest.json 版本格式

- 资产版本（如 Skill.Version）：语义化版本，如 `1.0.0`（无 v 前缀）
- 资产包版本：完整版本，如 `v1.0.0-20260331-143052`（有 v 前缀 + 时间戳）