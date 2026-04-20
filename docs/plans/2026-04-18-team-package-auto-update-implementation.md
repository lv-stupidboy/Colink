# 团队包自动更新功能实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 ISDP 平台增加团队包自动更新能力，从远程 Git 仓库获取团队包，通过版本对比判断是否需要更新。

**Architecture:** 
- 新增 `team_package_versions` 表存储版本信息
- 新增 `teampackagesync` 服务目录（零侵入复用现有导入逻辑）
- 前端扩展通用设置和团队包管理页面

**Tech Stack:** Go + Gin + SQLite, React + Ant Design, Git CLI

---

## Task 1: 数据库迁移 - SQLite

**Files:**
- Create: `sql-change/v1.2.2/sqlite/00003_team_package_versions.sql`

**Step 1: 创建 SQLite 迁移文件**

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE team_package_versions (
    id UUID PRIMARY KEY DEFAULT (lower(hex(random_blob(16)))),
    workflow_id UUID NOT NULL REFERENCES workflow_templates(id),
    name VARCHAR(255) NOT NULL,
    category VARCHAR(255),
    version VARCHAR(50) NOT NULL,
    description TEXT,
    last_synced_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workflow_id),
    UNIQUE(name)
);

CREATE INDEX idx_tpv_workflow ON team_package_versions(workflow_id);
CREATE INDEX idx_tpv_name ON team_package_versions(name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS team_package_versions;
-- +goose StatementEnd
```

**Step 2: Commit**

```bash
git add sql-change/v1.2.2/sqlite/00003_team_package_versions.sql
git commit -m "feat(db): add team_package_versions table (SQLite)"
```

---

## Task 2: 数据库迁移 - MySQL

**Files:**
- Create: `sql-change/v1.2.2/mysql/00003_team_package_versions.sql`

**Step 1: 创建 MySQL 迁移文件**

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE team_package_versions (
    id CHAR(36) PRIMARY KEY,
    workflow_id CHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(255),
    version VARCHAR(50) NOT NULL,
    description TEXT,
    last_synced_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_workflow (workflow_id),
    UNIQUE KEY uk_name (name),
    FOREIGN KEY (workflow_id) REFERENCES workflow_templates(id) ON DELETE CASCADE,
    INDEX idx_tpv_workflow (workflow_id),
    INDEX idx_tpv_name (name)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS team_package_versions;
-- +goose StatementEnd
```

**Step 2: Commit**

```bash
git add sql-change/v1.2.2/mysql/00003_team_package_versions.sql
git commit -m "feat(db): add team_package_versions table (MySQL)"
```

---

## Task 3: 扩展配置 - TeamPackageSyncConfig

**Files:**
- Modify: `pkg/config/config.go`
- Modify: `configs/config.yaml.example`

**Step 1: 在 Config 结构体追加字段（约第31行）**

```go
TeamPackageSync TeamPackageSyncConfig `mapstructure:"team_package_sync"`
```

**Step 2: 在文件末尾追加配置结构和方法**

```go
// TeamPackageSyncConfig 团队包同步配置
type TeamPackageSyncConfig struct {
    RemoteRepoURL    string `mapstructure:"remote_repo_url"`
    AutoUpdateEnabled bool  `mapstructure:"auto_update_enabled"`
    CheckInterval     string `mapstructure:"check_interval"`
    Branch            string `mapstructure:"branch"`
}

func (c *TeamPackageSyncConfig) ApplyDefaults() {
    if c.RemoteRepoURL == "" {
        c.RemoteRepoURL = "https://gitee.com/colink_1/isdp.git"
    }
    if c.CheckInterval == "" {
        c.CheckInterval = "24h"
    }
    if c.Branch == "" {
        c.Branch = "main"
    }
}

func (c *TeamPackageSyncConfig) GetCheckInterval() time.Duration {
    d, err := time.ParseDuration(c.CheckInterval)
    if err != nil {
        return 24 * time.Hour
    }
    return d
}

func (c *TeamPackageSyncConfig) IsEnabled() bool {
    return c.AutoUpdateEnabled && c.RemoteRepoURL != ""
}
```

**Step 3: 在 Load 函数应用默认值（约第472行后）**

```go
cfg.TeamPackageSync.ApplyDefaults()
```

**Step 4: 在 setDefaults 函数追加（约第566行后）**

```go
viper.SetDefault("team_package_sync.remote_repo_url", "https://gitee.com/colink_1/isdp.git")
viper.SetDefault("team_package_sync.auto_update_enabled", true)
viper.SetDefault("team_package_sync.check_interval", "24h")
viper.SetDefault("team_package_sync.branch", "main")
```

**Step 5: 在 config.yaml.example 末尾追加**

```yaml
# 团队包自动更新配置
team_package_sync:
  remote_repo_url: "https://gitee.com/colink_1/isdp.git"
  auto_update_enabled: true
  check_interval: "24h"
  branch: "main"
```

**Step 6: Commit**

```bash
git add pkg/config/config.go configs/config.yaml.example
git commit -m "feat(config): add TeamPackageSyncConfig"
```

---

## Task 4: Model - TeamPackageVersion

**Files:**
- Create: `internal/model/team_package_version.go`

**Step 1: 创建模型文件**

```go
package model

import (
    "time"
    "github.com/google/uuid"
)

type TeamPackageVersion struct {
    ID           uuid.UUID  `json:"id"`
    WorkflowID   uuid.UUID  `json:"workflowId"`
    Name         string     `json:"name"`
    Category     string     `json:"category"`
    Version      string     `json:"version"`
    Description  string     `json:"description"`
    LastSyncedAt *time.Time `json:"lastSyncedAt,omitempty"`
    CreatedAt    time.Time  `json:"createdAt"`
    UpdatedAt    time.Time  `json:"updatedAt"`
}

func (t *TeamPackageVersion) TableName() string {
    return "team_package_versions"
}
```

**Step 2: Commit**

```bash
git add internal/model/team_package_version.go
git commit -m "feat(model): add TeamPackageVersion model"
```

---

## Task 5: Repository - TeamPackageVersionRepository

**Files:**
- Create: `internal/repo/team_package_version_repo.go`

**Step 1: 创建 Repository 文件**

```go
package repo

import (
    "context"
    "database/sql"
    "fmt"
    "time"

    "github.com/anthropic/isdp/internal/model"
    "github.com/google/uuid"
)

type TeamPackageVersionRepository struct {
    BaseRepository
}

func NewTeamPackageVersionRepository(db *sql.DB, dbType DBType) *TeamPackageVersionRepository {
    return &TeamPackageVersionRepository{BaseRepository: NewBaseRepository(db, dbType)}
}

func (r *TeamPackageVersionRepository) Create(ctx context.Context, version *model.TeamPackageVersion) error {
    query := `INSERT INTO team_package_versions (id, workflow_id, name, category, version, description, last_synced_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
    now := time.Now()
    id := uuid.New()
    var lastSyncedAt interface{}
    if version.LastSyncedAt != nil {
        lastSyncedAt = version.LastSyncedAt.Format(time.RFC3339)
    }
    _, err := r.DB().ExecContext(ctx, query, id.String(), version.WorkflowID.String(), version.Name, version.Category, version.Version, version.Description, lastSyncedAt, now.Format(time.RFC3339), now.Format(time.RFC3339))
    if err != nil {
        return err
    }
    version.ID = id
    version.CreatedAt = now
    version.UpdatedAt = now
    return nil
}

func (r *TeamPackageVersionRepository) FindByName(ctx context.Context, name string) (*model.TeamPackageVersion, error) {
    query := `SELECT id, workflow_id, name, category, version, description, last_synced_at, created_at, updated_at FROM team_package_versions WHERE name = ?`
    row := r.DB().QueryRowContext(ctx, query, name)
    var idStr, workflowIDStr string
    var lastSyncedAt sql.NullString
    var createdAt, updatedAt SQLiteTimeScanner
    v := &model.TeamPackageVersion{}
    err := row.Scan(&idStr, &workflowIDStr, &v.Name, &v.Category, &v.Version, &v.Description, &lastSyncedAt, &createdAt, &updatedAt)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, nil
        }
        return nil, err
    }
    v.ID, _ = uuid.Parse(idStr)
    v.WorkflowID, _ = uuid.Parse(workflowIDStr)
    if lastSyncedAt.Valid {
        t, _ := time.Parse(time.RFC3339, lastSyncedAt.String)
        v.LastSyncedAt = &t
    }
    v.CreatedAt = createdAt.Time
    v.UpdatedAt = updatedAt.Time
    return v, nil
}

func (r *TeamPackageVersionRepository) ListAll(ctx context.Context) ([]model.TeamPackageVersion, error) {
    query := `SELECT id, workflow_id, name, category, version, description, last_synced_at, created_at, updated_at FROM team_package_versions ORDER BY created_at DESC`
    rows, err := r.DB().QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var versions []model.TeamPackageVersion
    for rows.Next() {
        var v model.TeamPackageVersion
        var idStr, workflowIDStr string
        var lastSyncedAt sql.NullString
        var createdAt, updatedAt SQLiteTimeScanner
        if err := rows.Scan(&idStr, &workflowIDStr, &v.Name, &v.Category, &v.Version, &v.Description, &lastSyncedAt, &createdAt, &updatedAt); err != nil {
            return nil, err
        }
        v.ID, _ = uuid.Parse(idStr)
        v.WorkflowID, _ = uuid.Parse(workflowIDStr)
        if lastSyncedAt.Valid {
            t, _ := time.Parse(time.RFC3339, lastSyncedAt.String)
            v.LastSyncedAt = &t
        }
        v.CreatedAt = createdAt.Time
        v.UpdatedAt = updatedAt.Time
        versions = append(versions, v)
    }
    return versions, nil
}

func (r *TeamPackageVersionRepository) Update(ctx context.Context, version *model.TeamPackageVersion) error {
    query := `UPDATE team_package_versions SET version = ?, description = ?, category = ?, last_synced_at = ?, updated_at = ? WHERE id = ?`
    now := time.Now()
    var lastSyncedAt interface{}
    if version.LastSyncedAt != nil {
        lastSyncedAt = version.LastSyncedAt.Format(time.RFC3339)
    }
    _, err := r.DB().ExecContext(ctx, query, version.Version, version.Description, version.Category, lastSyncedAt, now.Format(time.RFC3339), version.ID.String())
    if err != nil {
        return err
    }
    version.UpdatedAt = now
    return nil
}
```

**Step 2: Commit**

```bash
git add internal/repo/team_package_version_repo.go
git commit -m "feat(repo): add TeamPackageVersionRepository"
```

---

## Task 6: 同步服务 - types.go

**Files:**
- Create: `internal/service/teampackagesync/types.go`

**Step 1: 创建类型定义**

```go
package teampackagesync

import "github.com/anthropic/isdp/internal/model"

type RemotePackageList struct {
    Categories []RemotePackageCategory `json:"categories"`
}

type RemotePackageCategory struct {
    Name     string          `json:"name"`
    Packages []RemotePackage `json:"packages"`
}

type RemotePackage struct {
    Name        string `json:"name"`
    Version     string `json:"version"`
    Description string `json:"description"`
    Path        string `json:"path"`
}

type UpdateCheckResult struct {
    NeedUpdate  []PackageUpdateInfo `json:"needUpdate"`
    NewPackages []RemotePackage     `json:"newPackages"`
    Removed     []string            `json:"removed"`
}

type PackageUpdateInfo struct {
    Local  model.TeamPackageVersion `json:"local"`
    Remote RemotePackage            `json:"remote"`
}

type SyncPackageRequest struct {
    PackageName string                         `json:"packageName" binding:"required"`
    Confirm     *model.TeamPackageImportConfirm `json:"confirm"`
}

type PackageInfo struct {
    Name        string `json:"name"`
    Version     string `json:"version"`
    Description string `json:"description"`
}
```

**Step 2: Commit**

```bash
git add internal/service/teampackagesync/types.go
git commit -m "feat(sync): add sync types"
```

---

## Task 7: Git 客户端 - git_client.go

**Files:**
- Create: `internal/service/teampackagesync/git_client.go`

**完整代码见设计文档 Section 3.6**

**Step 1: Commit**

```bash
git add internal/service/teampackagesync/git_client.go
git commit -m "feat(sync): add GitClient"
```

---

## Task 8: Parser - parser.go

**Files:**
- Create: `internal/service/teampackagesync/parser.go`

**完整代码见设计文档 Section 3.7**

**Step 1: Commit**

```bash
git add internal/service/teampackagesync/parser.go
git commit -m "feat(sync): add package.json parser"
```

---

## Task 9: 同步服务主逻辑 - service.go

**Files:**
- Create: `internal/service/teampackagesync/service.go`

**完整代码见设计文档 Section 3.5**

**Step 1: Commit**

```bash
git add internal/service/teampackagesync/service.go
git commit -m "feat(sync): add SyncService"
```

---

## Task 10: 定时检查器 - checker.go

**Files:**
- Create: `internal/service/teampackagesync/checker.go`

**完整代码见设计文档 Section 5.2**

**Step 1: Commit**

```bash
git add internal/service/teampackagesync/checker.go
git commit -m "feat(sync): add SyncChecker"
```

---

## Task 11: API Handler

**Files:**
- Create: `internal/api/team_package_sync_handler.go`

**完整代码见设计文档 Section 3.9**

**Step 1: Commit**

```bash
git add internal/api/team_package_sync_handler.go
git commit -m "feat(api): add TeamPackageSyncHandler"
```

---

## Task 12: 集成到 main.go

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Repository 区域追加**

```go
teamPackageVersionRepo := repo.NewTeamPackageVersionRepository(db, dbType)
```

**Step 2: Service 区域追加**

```go
teamPackageSyncSvc := teampackagesync.NewSyncService(teamPackageVersionRepo, workflowRepo, teamPackageSvc, cfg.TeamPackageSync, logger)
```

**Step 3: Handler 区域追加**

```go
teamPackageSyncHandler := api.NewTeamPackageSyncHandler(teamPackageSyncSvc, logger)
teamPackageSyncHandler.RegisterRoutes(v1)
```

**Step 4: 后台任务区域追加**

```go
if cfg.TeamPackageSync.IsEnabled() {
    syncChecker := teampackagesync.NewSyncChecker(teamPackageSyncSvc, cfg.TeamPackageSync.GetCheckInterval(), logger)
    syncChecker.Start()
    defer syncChecker.Stop()
}
```

**Step 5: import 追加**

```go
"github.com/anthropic/isdp/internal/service/teampackagesync"
```

**Step 6: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(server): integrate TeamPackageSync"
```

---

## Task 13: 前端 API Client

**Files:**
- Modify: `web/src/api/client.ts`

**Step 1: 在 teamPackages 追加方法**

```typescript
getRemotePackages: (): Promise<RemotePackageList> =>
  request.get('/team-package-sync/remote'),

checkUpdates: (): Promise<UpdateCheckResult> =>
  request.get('/team-package-sync/check-update'),

syncPackage: (packageName: string, confirm?: ImportConfirm): Promise<ImportResult> =>
  request.post('/team-package-sync/sync', { packageName, confirm }),

listLocalVersions: (): Promise<{ data: TeamPackageVersion[]; total: number }> =>
  request.get('/team-package-sync/local-versions'),
```

**Step 2: Commit**

```bash
git add web/src/api/client.ts
git commit -m "feat(api): add sync methods"
```

---

## Task 14: 前端类型定义

**Files:**
- Modify: `web/src/types/index.ts`

**完整类型见设计文档 Section 4.4**

**Step 1: Commit**

```bash
git add web/src/types/index.ts
git commit -m "feat(types): add sync types"
```

---

## Task 15: 通用设置页面

**Files:**
- Modify: `web/src/pages/Settings/GeneralSettings.tsx`

**完整代码见设计文档 Section 4.1**

**Step 1: Commit**

```bash
git add web/src/pages/Settings/GeneralSettings.tsx
git commit -m "feat(settings): add sync card"
```

---

## Task 16: 团队包管理页面

**Files:**
- Modify: `web/src/pages/TeamPackage/index.tsx`

**完整代码见设计文档 Section 4.2**

**Step 1: Commit**

```bash
git add web/src/pages/TeamPackage/index.tsx
git commit -m "feat(team-package): add remote packages section"
```

---

## 执行完成验证

| 验证项 | 命令 |
|--------|------|
| 后端编译 | `go build ./cmd/server` |
| 前端编译 | `cd web && npm run build` |
| 迁移执行 | `bin/migrate.exe up --db ./data/sqlite/colink.db --version 1.2.2` |
| API 测试 | `curl http://localhost:26305/api/v1/team-package-sync/check-update` |
| 前端访问 | 打开 http://localhost:26306/settings/general |

---

## 侵入性总结

| 文件 | 改动 |
|------|------|
| `pkg/config/config.go` | 追加字段 + 结构体 + 方法 |
| `cmd/server/main.go` | 追加初始化代码 |
| `config.yaml.example` | 追加配置项 |
| `web/src/api/client.ts` | 追加方法 |
| `web/src/types/index.ts` | 追加类型 |
| `GeneralSettings.tsx` | 追加 Card |
| `TeamPackage/index.tsx` | 追加 Card |
| 其他 | 全部新增文件 |