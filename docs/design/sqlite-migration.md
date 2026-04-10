# ISDP SQLite 能力补齐完整方案

## Context

ISDP 项目定位为本地个人工具，需补齐 SQLite 迁移和备份能力：
1. 使用成熟迁移工具 goose（而非自研）
2. 过渡期 SQLite 和 MySQL 表结构保持一致
3. 版本升级时的数据库迁移能力
4. 跨机器数据迁移能力（导出导入）
5. 修复数据库结构不一致问题

## 一、迁移工具选型对比

### Go SQLite 迁移工具对比

| 特性 | goose | golang-migrate | sql-migrate | atlas |
|------|-------|----------------|-------------|-------|
| **GitHub Stars** | 7k+ | 10k+ | 3k+ | 3k+ |
| **SQLite 支持** | ✅ 原生 | ✅ 原生 | ✅ 原生 | ✅ 原生 |
| **Embed 支持** | ✅ Go embed | ✅ Go embed | ✅ embed/bindata | ✅ embed |
| **SQL 迁移** | ✅ | ✅ | ✅ | ✅ |
| **Go 迁移** | ✅ 支持 Go 函数 | ✅ | ✅ | ✅ |
| **CLI 工具** | ✅ | ✅ | ✅ | ✅ |
| **库模式** | ✅ | ✅ | ✅ | ✅ |
| **事务控制** | 每条语句 | 可配置 | 每迁移 | ✅ |
| **版本表名** | `goose_db_version` | `schema_migrations` | `gorp_migrations` | `atlas_schema_revisions` |
| **回滚支持** | ✅ | ✅ | ✅ | ✅ |
| **Pure Go SQLite** | ✅ modernc | ✅ modernc | ✅ mattn | ✅ |
| **适合场景** | Go 函数迁移、embed | 数据库无关、大型项目 | gorm 集成、简单场景 | 云原生、CI/CD |

### 推荐：goose

**决策：采用 [pressly/goose](https://github.com/pressly/goose) 作为迁移工具。**

推荐理由：
1. **支持 Go 函数迁移**：适合数据迁移场景（如删除废弃字段后数据调整）
2. **Embed 原生支持**：迁移文件可嵌入二进制，无需外部文件
3. **社区活跃**：7k+ stars，持续维护
4. **API 清晰**：库模式调用简单，适合安装器集成

**安装**：
```bash
go get github.com/pressly/goose/v3
```

**备选方案**：
- [golang-migrate/migrate](https://github.com/golang-migrate/migrate) - 数据库无关，适合多数据库项目
- [rubenv/sql-migrate](https://github.com/rubenv/sql-migrate) - 支持 gorm 集成

**参考资源**：
- [SQLite PRAGMA user_version](https://sqlite.org/pragma.html) - SQLite 官方版本追踪机制
- [Goose 文档](https://pressly.github.io/goose/)

## 二、数据库字段分析

### 本次版本需处理的问题

| 表 | 字段/问题 | 分析 | 操作 |
|------|------|------|------|
| `messages` | `MentionsUser` | **预留字段**（模型定义但 repo/db 未使用） | 保留，后续 A2A 功能可能使用 |
| `sandboxes` | `config` | **废弃字段**（SQL 定义但代码不使用） | 删除，重构表结构 |
| `skills` | `source_type` 注释 | 注释值与代码不符 | 修正注释 |

### sandboxes 表重构（必须修复）

**当前 SQL 定义（错误）**：
```sql
CREATE TABLE `sandboxes` (
  `id` varchar(64) NOT NULL,
  `thread_id` varchar(64) NOT NULL,
  `config` json DEFAULT NULL,  -- 废弃
  `status` varchar(32) DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
);
```

**正确定义（按实际代码）**：
```sql
CREATE TABLE `sandboxes` (
  `id` varchar(64) NOT NULL,
  `thread_id` varchar(64) NOT NULL,
  `name` varchar(255) DEFAULT NULL,       -- 新增
  `image` varchar(255) DEFAULT NULL,      -- 新增
  `status` varchar(32) DEFAULT NULL,
  `container_id` varchar(128) DEFAULT NULL,  -- 新增
  `port` int DEFAULT NULL,                -- 新增
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `ended_at` timestamp NULL DEFAULT NULL, -- 新增
  PRIMARY KEY (`id`)
);
```

### MentionsUser 字段分析

**字段定义**：`internal/model/message.go:42`
```go
// A2A 相关字段
Mentions     []string    `json:"mentions,omitempty"`      // 被 @mention 的 Agent IDs
MentionsUser bool        `json:"mentionsUser,omitempty"` // 是否 @用户（预留）
Origin       string      `json:"origin,omitempty"`        // "user", "callback", "stream"
ReplyTo      *uuid.UUID  `json:"replyTo,omitempty"`       // 回复的消息 ID
```

**分析结论**：
- 属于 A2A 相关字段组（与 Mentions, Origin, ReplyTo 同组）
- 数据库表和 repo/message.go 均未包含此字段
- 可能是预留用于标记"消息是否 @用户"（区别于 Mentions 字段的 @Agent）
- **建议保留**，待 A2A 功能完善后再决定是否启用

## 三、架构决策

### 3.1 数据库策略：过渡期保持一致

**决策：过渡期 SQLite 和 MySQL 表结构保持一致，后续移除 MySQL。**

过渡期策略：
- SQLite 和 MySQL 使用相同的 init.sql（自动转换方言）
- 迁移脚本同时提供 MySQL 和 SQLite 版本
- 配置支持 `database.type` 切换 sqlite/mysql
- 测试同时覆盖两种数据库

最终目标：
- 过渡完成后移除 MySQL 代码和迁移脚本
- 统一使用 SQLite

**MySQL 保留**：通过配置切换，开发人员可选择使用。

```yaml
database:
  type: sqlite  # 默认 sqlite，可选 mysql（过渡期保留）
  sqlite:
    path: ./data/isdp.db
  mysql:        # 过渡期保留，后续移除
    host: ...
```

### 3.2 迁移机制

**决策：使用 goose 迁移工具，在安装器升级过程中执行。**

| 场景 | 方案 | 执行时机 |
|------|------|----------|
| 首次安装 | goose 初始化脚本 | 安装器执行 |
| 版本升级 | goose 增量迁移 | 安装器升级过程中 |
| 数据迁移 | goose Go 函数迁移 | 需要时使用 |

### 3.3 自动备份

**决策：备份频率可配置，默认每日自动备份。**

```yaml
backup:
  enabled: true
  interval: "daily"  # daily | weekly | manual
  retention: 7       # 保留最近 7 个备份
  path: "./data/backups"
```

## 四、目录结构

```
sql-change/
├── init.sql                    # MySQL 初始化脚本
├── init_sqlite.sql             # SQLite 初始化脚本（新增，与 MySQL 结构一致）
├── migrations/                 # goose 迁移脚本目录（新增）
│   ├── 202604110001_init.sql   # 初始化迁移
│   ├── 202604110002_fix_sandboxes.go  # sandboxes 表修复迁移
│   └── ...
├── VERSION                     # 当前 Schema 版本号（新增）

internal/
├── migrate/                    # goose 集成（新增）
│   └── migrate.go              # 迁移初始化和执行
│
├── backup/                     # 备份导出（新增）
│   ├── backup.go               # 备份服务
│   ├── restore.go              # 恢复服务
│   ├── export.go               # 完整导出
│   └── import.go               # 完整导入

cmd/
├── migrate/                    # goose CLI 包装（新增）
│   └── main.go                 # 供安装器调用

installer/src/main/
├── installer.ts                # 集成迁移步骤（修改）
```

## 五、goose 迁移脚本格式

### 5.1 SQL 迁移脚本

**文件**: `sql-change/migrations/202604110001_init.sql`

```sql
-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS agent_configs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    role TEXT NOT NULL,
    -- ... 其他字段
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS agent_configs;
-- +goose StatementEnd
```

### 5.2 Go 迁移脚本（数据迁移场景）

**文件**: `sql-change/migrations/202604110002_fix_sandboxes.go`

```go
package migrations

import (
    "context"
    "database/sql"
    "github.com/pressly/goose/v3"
)

func init() {
    goose.AddMigrationContext(upFixSandboxes, downFixSandboxes)
}

func upFixSandboxes(ctx context.Context, tx *sql.Tx) error {
    // SQLite 需要重建表来修改结构
    // 1. 创建新表结构
    _, err := tx.ExecContext(ctx, `
        CREATE TABLE sandboxes_new (
            id TEXT PRIMARY KEY,
            thread_id TEXT NOT NULL,
            name TEXT DEFAULT NULL,
            image TEXT DEFAULT NULL,
            status TEXT DEFAULT NULL,
            container_id TEXT DEFAULT NULL,
            port INTEGER DEFAULT NULL,
            created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            ended_at TEXT DEFAULT NULL
        )
    `)
    if err != nil {
        return err
    }

    // 2. 复制数据
    _, err = tx.ExecContext(ctx, `
        INSERT INTO sandboxes_new (id, thread_id, status, created_at)
        SELECT id, thread_id, status, created_at FROM sandboxes
    `)
    if err != nil {
        return err
    }

    // 3. 删除旧表
    _, err = tx.ExecContext(ctx, "DROP TABLE sandboxes")
    if err != nil {
        return err
    }

    // 4. 重命名新表
    _, err = tx.ExecContext(ctx, "ALTER TABLE sandboxes_new RENAME TO sandboxes")
    if err != nil {
        return err
    }

    return nil
}

func downFixSandboxes(ctx context.Context, tx *sql.Tx) error {
    // 回滚逻辑（可选）
    return nil
}
```

## 六、安装器迁移流程

### 6.1 升级流程

```
安装器启动升级
    │
    ▼
检测已有数据库
    │
    ├── 无数据库 → goose 初始化迁移
    │
    └── 有数据库
        │
        ▼
    备份当前数据库
        │
        ▼
    调用 goose up 增量迁移
        │
        ▼
    完成安装
```

### 6.2 安装器集成

```typescript
// installer/src/main/installer.ts

async function runMigration(installDir: string): Promise<MigrationResult> {
  const dbPath = join(installDir, 'data', 'isdp.db');
  const migrationsDir = join(resourcePath, 'sql-change', 'migrations');

  // 调用 goose CLI
  const migrateTool = join(installDir, 'tools', 'migrate.exe');
  const result = await execFile(migrateTool, [
    'up',
    '--dir', migrationsDir,
    '--db-string', `sqlite:${dbPath}`
  ]);

  return { success: result.exitCode === 0 };
}
```

## 七、导出导入格式

**导出包结构**：
```
isdp-export-{timestamp}.zip
├── manifest.json              # Schema 版本 + 表统计
├── data/
│   ├── agent_configs.json     # 各表数据
│   ├── threads.json
│   └── ...
├── assets/                    # 文件资产
```

**manifest.json**：
```json
{
    "schemaVersion": 202604110001,
    "exportedAt": "2026-04-11T12:00:00Z",
    "databaseType": "sqlite",
    "appVersion": "1.0.0",
    "tables": {
        "agent_configs": { "rowCount": 5 },
        "threads": { "rowCount": 12 }
    }
}
```

## 八、实施步骤

### Phase 1: 数据库结构修复（P0，1天）

1. 更新 `sql-change/init.sql` 中 `sandboxes` 表定义（删除 config，添加缺失字段）
2. 修正 `skills.source_type` 注释
3. 生成 SQLite 版本 init_sqlite.sql（保持结构一致）
4. 验证两种数据库结构一致

### Phase 2: goose 迁移框架（P0，3天）

1. 添加 goose 依赖：`go get github.com/pressly/goose/v3`
2. 创建 `sql-change/migrations/` 目录
3. 编写初始化迁移脚本
4. 创建 `cmd/migrate/main.go` CLI 包装
5. 集成到 `internal/migrate/migrate.go`

### Phase 3: 安装器集成（P1，2天）

1. 修改 `installer/src/main/installer.ts`
2. 添加迁移执行步骤
3. 打包 goose CLI 工具
4. 测试升级迁移流程

### Phase 4: 导出导入与备份（P2，4天）

1. 创建 `internal/backup/export.go`
2. 创建 `internal/backup/import.go`
3. 创建 `internal/backup/backup.go`
4. API Handler 实现
5. 可配置自动备份定时任务

### Phase 5: 测试与完善（P3，2天）

1. 端到端测试：安装 -> 使用 -> 升级 -> 备份 -> 恢复
2. 跨机器迁移测试
3. 文档完善

## 九、关键文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `sql-change/init.sql` | 修改 | 重构 sandboxes 表、修正注释 |
| `sql-change/init_sqlite.sql` | 新建 | SQLite 初始化脚本（与 MySQL 结构一致） |
| `sql-change/migrations/` | 新建 | goose 迁移脚本目录 |
| `internal/migrate/migrate.go` | 新建 | goose 集成 |
| `cmd/migrate/main.go` | 新建 | goose CLI 包装 |
| `internal/backup/export.go` | 新建 | 完整导出服务 |
| `internal/backup/import.go` | 新建 | 完整导入服务 |
| `internal/backup/backup.go` | 新建 | 备份恢复服务 |
| `installer/src/main/installer.ts` | 修改 | 集成迁移步骤 |
| `pkg/config/config.go` | 修改 | 添加备份配置项 |
| `go.mod` | 修改 | 添加 goose 依赖 |

## 十、规范写入 CLAUDE.md

### 数据库迁移规范

1. **统一使用 SQLite**：开发和用户环境均使用 SQLite
2. **使用 goose 迁移工具**：迁移脚本存放在 `sql-change/migrations/`
3. **迁移脚本命名**：`YYYYMMDDNNNN_description.sql` 或 `.go`
4. **迁移执行时机**：安装器升级过程中自动执行
5. **MySQL 保留**：通过 `database.type` 配置切换

### 迁移脚本编写规范

```sql
-- +goose Up
-- +goose StatementBegin
-- DDL 语句
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 回滚语句
-- +goose StatementEnd
```

## 十一、验证方式

1. 新安装测试：goose 初始化迁移成功，服务启动正常
2. 升级迁移测试：安装器执行增量迁移成功
3. 数据库结构一致：SQLite 和 MySQL init.sql 结构同步
4. 导出导入测试：跨机器数据迁移正常
5. 备份恢复测试：自动备份生成，手动恢复成功

## Sources

- [pressly/goose](https://github.com/pressly/goose) - Go 迁移工具
- [SQLite PRAGMA user_version](https://sqlite.org/pragma.html) - SQLite 官方文档
- [golang-migrate/migrate](https://github.com/golang-migrate/migrate) - 备选方案
- [rubenv/sql-migrate](https://github.com/rubenv/sql-migrate) - 备选方案