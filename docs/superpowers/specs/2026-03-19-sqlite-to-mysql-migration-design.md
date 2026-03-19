# ISDP 数据库迁移设计文档 - SQLite 到 MySQL

## 元信息

- **创建日期**: 2026-03-19
- **作者**: ISDP Team
- **状态**: 设计中

## 1. 背景

### 1.1 当前问题

ISDP项目目前使用SQLite作为数据库，在团队协作中存在以下问题：

1. **DDL操作受限** - SQLite的ALTER TABLE功能有限，无法直接DROP COLUMN、修改列类型等
2. **协同不便** - 表结构变化时，本地数据库同步困难，需要手动执行DDL
3. **数据隔离困难** - 团队成员各自维护本地数据库，无法共享数据进行联调

### 1.2 目标

将数据库从SQLite迁移到华为云RDS MySQL，支持**渐进式过渡**：

- SQLite与MySQL并存运行，配置一键切换
- 多Schema隔离，每个开发者独立的开发数据空间
- 提供数据迁移工具，SQLite数据可导入MySQL
- 验证OK后，彻底移除SQLite支持

## 2. 设计决策

| 决策项 | 选择 | 理由 |
|--------|------|------|
| 数据库类型 | MySQL | 与大客户华为云环境保持一致 |
| 部署方式 | 华为云 RDS MySQL | 托管服务，零运维 |
| 隔离策略 | 多Schema | 一个实例服务多人，成本可控 |
| 过渡策略 | 并存期 | 配置切换，平滑过渡 |

## 3. 架构设计

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                    应用层 (Go API)                       │
├─────────────────────────────────────────────────────────┤
│                 数据库抽象层 (新增)                       │
│   ┌─────────────┐              ┌─────────────┐         │
│   │ SQLite Driver│              │ MySQL Driver │         │
│   └─────────────┘              └─────────────┘         │
├─────────────────────────────────────────────────────────┤
│              配置层 (config.yaml)                        │
│   database:                                              │
│     type: sqlite | mysql    ← 一键切换                   │
└─────────────────────────────────────────────────────────┘
                           │
           ┌───────────────┴───────────────┐
           ▼                               ▼
   ┌──────────────┐               ┌──────────────────┐
   │ SQLite 本地   │               │ 华为云 RDS MySQL │
   │ data/isdp.db │               │ 多Schema隔离     │
   └──────────────┘               └──────────────────┘
```

### 3.2 多Schema设计

华为云RDS MySQL实例结构：

```
isdp_dev (数据库)
├── shared          # 共享数据，用于团队联调
├── dev_{user1}     # 开发者1独立schema
├── dev_{user2}     # 开发者2独立schema
└── dev_{userN}     # 开发者N独立schema
```

命名规范：
- 共享schema: `shared`
- 个人schema: `dev_{用户名或缩写}`，如 `dev_zhangsan`、`dev_lisi`

## 4. 详细设计

### 4.1 配置文件改造

**config.yaml 扩展：**

```yaml
database:
  # 数据库类型: sqlite | mysql
  type: sqlite

  # SQLite配置 (type=sqlite时生效)
  path: ./data/isdp.db

  # MySQL配置 (type=mysql时生效)
  mysql:
    host: ""                  # 华为云RDS地址，如 rm-xxx.mysql.rds.huaweicloud.com
    port: 3306
    database: isdp_dev        # 数据库名
    username: ""
    password: ""
    schema: dev_default       # 当前开发者使用的schema
    charset: utf8mb4
    max_open_conns: 10        # 最大连接数
    max_idle_conns: 5         # 最大空闲连接数
    conn_max_lifetime: 300    # 连接最大生命周期(秒)
```

### 4.2 数据库抽象层

#### 4.2.1 文件结构

```
internal/repo/
├── db.go              # 数据库接口定义
├── db_sqlite.go       # SQLite实现
├── db_mysql.go        # MySQL实现
├── db_factory.go      # 工厂函数
└── ... (其他repo文件保持不变)
```

#### 4.2.2 接口设计

```go
// db.go

package repo

import (
    "context"
    "database/sql"
)

// DBType 数据库类型
type DBType string

const (
    DBTypeSQLite DBType = "sqlite"
    DBTypeMySQL  DBType = "mysql"
)

// DBConfig 数据库配置
type DBConfig struct {
    Type DBType

    // SQLite
    Path string

    // MySQL
    MySQL MySQLConfig
}

// MySQLConfig MySQL配置
type MySQLConfig struct {
    Host            string
    Port            int
    Database        string
    Username        string
    Password        string
    Schema          string
    Charset         string
    MaxOpenConns    int
    MaxIdleConns    int
    ConnMaxLifetime int
}

// Dialect 数据库方言接口
type Dialect interface {
    // Placeholder 返回参数占位符 (? 或 $1)
    Placeholder() string
    // QuoteIdentifier 返回引用标识符 (" 或 `)
    QuoteIdentifier() string
    // AutoIncrement 返回自增主键语法
    AutoIncrement() string
}

// NewDB 创建数据库连接 (工厂函数)
func NewDB(cfg DBConfig) (*sql.DB, Dialect, error) {
    switch cfg.Type {
    case DBTypeSQLite:
        return newSQLiteDB(cfg)
    case DBTypeMySQL:
        return newMySQLDB(cfg)
    default:
        return nil, nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
    }
}
```

#### 4.2.3 SQLite实现

```go
// db_sqlite.go

package repo

import (
    "context"
    "database/sql"
    "fmt"
    "time"

    _ "modernc.org/sqlite"
)

type SQLiteDialect struct{}

func (d *SQLiteDialect) Placeholder() string        { return "?" }
func (d *SQLiteDialect) QuoteIdentifier() string    { return `"` }
func (d *SQLiteDialect) AutoIncrement() string      { return "AUTOINCREMENT" }

func newSQLiteDB(cfg DBConfig) (*sql.DB, Dialect, error) {
    db, err := sql.Open("sqlite", cfg.Path)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to open sqlite database: %w", err)
    }

    // SQLite只支持单个写连接
    db.SetMaxOpenConns(1)
    db.SetMaxIdleConns(1)
    db.SetConnMaxLifetime(0)

    // 启用外键约束
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
        return nil, nil, fmt.Errorf("failed to enable foreign keys: %w", err)
    }

    if err := db.PingContext(ctx); err != nil {
        return nil, nil, fmt.Errorf("failed to ping database: %w", err)
    }

    return db, &SQLiteDialect{}, nil
}
```

#### 4.2.4 MySQL实现

```go
// db_mysql.go

package repo

import (
    "context"
    "database/sql"
    "fmt"
    "time"

    _ "github.com/go-sql-driver/mysql"
)

type MySQLDialect struct{}

func (d *MySQLDialect) Placeholder() string       { return "?" }
func (d *MySQLDialect) QuoteIdentifier() string   { return "`" }
func (d *MySQLDialect) AutoIncrement() string     { return "AUTO_INCREMENT" }

func newMySQLDB(cfg DBConfig) (*sql.DB, Dialect, error) {
    mc := cfg.MySQL

    // DSN格式: username:password@tcp(host:port)/database?charset&parseTime=true
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Local&multiStatements=true",
        mc.Username,
        mc.Password,
        mc.Host,
        mc.Port,
        mc.Database,
        mc.Charset,
    )

    db, err := sql.Open("mysql", dsn)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to open mysql database: %w", err)
    }

    db.SetMaxOpenConns(mc.MaxOpenConns)
    db.SetMaxIdleConns(mc.MaxIdleConns)
    db.SetConnMaxLifetime(time.Duration(mc.ConnMaxLifetime) * time.Second)

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := db.PingContext(ctx); err != nil {
        return nil, nil, fmt.Errorf("failed to ping database: %w", err)
    }

    // 设置当前schema
    if mc.Schema != "" {
        if _, err := db.ExecContext(ctx, fmt.Sprintf("USE `%s`", mc.Schema)); err != nil {
            return nil, nil, fmt.Errorf("failed to use schema %s: %w", mc.Schema, err)
        }
    }

    return db, &MySQLDialect{}, nil
}
```

### 4.3 建表脚本

#### 4.3.1 MySQL建表脚本 (init_db_mysql.sql)

```sql
-- ISDP Database Initialization Script for MySQL
-- Version: 2.0

-- 设置字符集
SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- 项目表
CREATE TABLE IF NOT EXISTS projects (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(64) NOT NULL,
    mode VARCHAR(64) NOT NULL,
    status VARCHAR(32) DEFAULT 'draft',
    git_repo VARCHAR(512),
    config TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 开发会话表
CREATE TABLE IF NOT EXISTS threads (
    id VARCHAR(64) PRIMARY KEY,
    project_id VARCHAR(64),
    status VARCHAR(32) DEFAULT 'idle',
    current_phase VARCHAR(64),
    current_agent VARCHAR(64),
    depth INT DEFAULT 0,
    abort_token TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 消息表
CREATE TABLE IF NOT EXISTS messages (
    id VARCHAR(64) PRIMARY KEY,
    thread_id VARCHAR(64),
    role VARCHAR(32) NOT NULL,
    agent_id VARCHAR(64),
    content LONGTEXT,
    message_type VARCHAR(32) DEFAULT 'text',
    metadata JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 基础Agent配置表
CREATE TABLE IF NOT EXISTS base_agents (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(64) NOT NULL,
    api_url VARCHAR(512),
    api_token TEXT,
    default_model VARCHAR(128),
    cli_path VARCHAR(512) DEFAULT 'claude',
    git_bash_path VARCHAR(512),
    max_tokens INT DEFAULT 4096,
    timeout_minutes INT DEFAULT 30,
    is_active TINYINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Agent配置表（Agent角色）
CREATE TABLE IF NOT EXISTS agent_configs (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(64) NOT NULL,
    base_agent_id VARCHAR(64),
    description TEXT,
    system_prompt TEXT,
    model_name VARCHAR(128) DEFAULT 'claude-sonnet-4-6',
    max_tokens INT DEFAULT 4096,
    temperature DECIMAL(3,2) DEFAULT 0.7,
    routing_config JSON,
    is_default TINYINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (base_agent_id) REFERENCES base_agents(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Agent调用记录表
CREATE TABLE IF NOT EXISTS agent_invocations (
    id VARCHAR(64) PRIMARY KEY,
    thread_id VARCHAR(64),
    agent_config_id VARCHAR(64),
    role VARCHAR(64) NOT NULL,
    status VARCHAR(32) DEFAULT 'running',
    input LONGTEXT,
    output LONGTEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 产物表
CREATE TABLE IF NOT EXISTS artifacts (
    id VARCHAR(64) PRIMARY KEY,
    thread_id VARCHAR(64),
    type VARCHAR(64) NOT NULL,
    name VARCHAR(255),
    path VARCHAR(512),
    content LONGTEXT,
    metadata JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 沙箱容器表
CREATE TABLE IF NOT EXISTS sandboxes (
    id VARCHAR(64) PRIMARY KEY,
    thread_id VARCHAR(64),
    name VARCHAR(255),
    image VARCHAR(255),
    status VARCHAR(32) DEFAULT 'created',
    container_id VARCHAR(128),
    port INT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP NULL,
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 创建索引
CREATE INDEX idx_threads_project_id ON threads(project_id);
CREATE INDEX idx_messages_thread_id ON messages(thread_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);
CREATE INDEX idx_agent_configs_base_agent_id ON agent_configs(base_agent_id);
CREATE INDEX idx_agent_invocations_thread_id ON agent_invocations(thread_id);
CREATE INDEX idx_artifacts_thread_id ON artifacts(thread_id);
CREATE INDEX idx_sandboxes_thread_id ON sandboxes(thread_id);
CREATE INDEX idx_base_agents_type ON base_agents(type);

SET FOREIGN_KEY_CHECKS = 1;
```

### 4.4 迁移工具

#### 4.4.1 Schema管理脚本 (scripts/schema.sh)

```bash
#!/bin/bash
# Schema管理工具

COMMAND=$1
SCHEMA_NAME=$2

case $COMMAND in
    create)
        mysql -h $MYSQL_HOST -u $MYSQL_USER -p$MYSQL_PASS $MYSQL_DB \
            -e "CREATE SCHEMA IF NOT EXISTS \`$SCHEMA_NAME\` DEFAULT CHARACTER SET utf8mb4;"
        mysql -h $MYSQL_HOST -u $MYSQL_USER -p$MYSQL_PASS $MYSQL_DB \
            -e "USE \`$SCHEMA_NAME\`; SOURCE scripts/init_db_mysql.sql;"
        echo "Schema '$SCHEMA_NAME' created and initialized."
        ;;
    drop)
        mysql -h $MYSQL_HOST -u $MYSQL_USER -p$MYSQL_PASS $MYSQL_DB \
            -e "DROP SCHEMA IF EXISTS \`$SCHEMA_NAME\`;"
        echo "Schema '$SCHEMA_NAME' dropped."
        ;;
    list)
        mysql -h $MYSQL_HOST -u $MYSQL_USER -p$MYSQL_PASS $MYSQL_DB \
            -e "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME LIKE 'dev_%' OR SCHEMA_NAME = 'shared';"
        ;;
    *)
        echo "Usage: $0 {create|drop|list} [schema_name]"
        exit 1
        ;;
esac
```

#### 4.4.2 数据迁移命令

新增CLI子命令 `migrate-data`，实现SQLite到MySQL的数据迁移：

```go
// cmd/migrate_data.go

package cmd

import (
    "context"
    "encoding/json"
    "fmt"
    "os"

    "isdp/internal/repo"
)

// MigrateDataCmd 数据迁移命令
func MigrateDataCmd(source, target, schema string) error {
    ctx := context.Background()

    // 1. 连接源数据库 (SQLite)
    sqliteDB, _, err := repo.NewDB(repo.DBConfig{
        Type: repo.DBTypeSQLite,
        Path: "./data/isdp.db",
    })
    if err != nil {
        return fmt.Errorf("connect sqlite: %w", err)
    }
    defer sqliteDB.Close()

    // 2. 连接目标数据库 (MySQL)
    mysqlDB, _, err := repo.NewDB(repo.DBConfig{
        Type:   repo.DBTypeMySQL,
        MySQL:  loadMySQLConfig(),
    })
    if err != nil {
        return fmt.Errorf("connect mysql: %w", err)
    }
    defer mysqlDB.Close()

    // 3. 切换到目标schema
    if _, err := mysqlDB.ExecContext(ctx, fmt.Sprintf("USE `%s`", schema)); err != nil {
        return fmt.Errorf("use schema: %w", err)
    }

    // 4. 迁移各表数据
    tables := []string{
        "projects", "base_agents", "agent_configs",
        "threads", "messages", "agent_invocations",
        "artifacts", "sandboxes",
    }

    for _, table := range tables {
        if err := migrateTable(ctx, sqliteDB, mysqlDB, table); err != nil {
            return fmt.Errorf("migrate table %s: %w", table, err)
        }
    }

    fmt.Println("Data migration completed successfully!")
    return nil
}

func migrateTable(ctx context.Context, src, dst *sql.DB, table string) error {
    // 读取源数据
    rows, err := src.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", table))
    if err != nil {
        return err
    }
    defer rows.Close()

    // 获取列信息
    columns, err := rows.Columns()
    if err != nil {
        return err
    }

    // 逐行迁移
    count := 0
    for rows.Next() {
        // ... 实现数据读取和插入
        count++
    }

    fmt.Printf("Migrated %d rows from %s\n", count, table)
    return nil
}
```

## 5. 实施计划

### 5.1 阶段划分

| 阶段 | 内容 | 预估时间 |
|------|------|----------|
| 一 | 并存基础能力 | 1-2天 |
| 二 | 建表脚本适配 | 0.5天 |
| 三 | 华为云RDS搭建 | 0.5天 |
| 四 | 数据迁移工具 | 1天 |
| 五 | 验证与切换 | 0.5天 |
| 六 | 清理SQLite | 验证OK后 |

### 5.2 详细任务

#### 阶段一：并存基础能力

| 序号 | 任务 | 说明 |
|------|------|------|
| 1.1 | 扩展配置文件 | database.type + mysql配置项 |
| 1.2 | 数据库抽象层 | db_factory.go，根据type选择驱动 |
| 1.3 | MySQL驱动实现 | db_mysql.go，连接池、参数配置 |
| 1.4 | SQLite驱动实现 | 重构现有db.go为db_sqlite.go |
| 1.5 | 单元测试 | 验证两种数据库都能正常连接 |

#### 阶段二：建表脚本适配

| 序号 | 任务 | 说明 |
|------|------|------|
| 2.1 | MySQL建表脚本 | init_db_mysql.sql，语法适配 |
| 2.2 | 迁移脚本适配 | 支持MySQL DDL语法 |

#### 阶段三：华为云RDS搭建

| 序号 | 任务 | 说明 |
|------|------|------|
| 3.1 | 创建RDS实例 | 最小规格即可（如2核4G） |
| 3.2 | 创建数据库和schema | isdp_dev + 各开发者schema |
| 3.3 | 配置白名单 | 开发者IP访问权限 |

#### 阶段四：数据迁移工具

| 序号 | 任务 | 说明 |
|------|------|------|
| 4.1 | 数据导出 | SQLite → 中间格式 |
| 4.2 | 数据导入 | 导入到指定MySQL schema |
| 4.3 | CLI命令 | isdp-server migrate-data |

#### 阶段五：验证与切换

| 序号 | 任务 | 说明 |
|------|------|------|
| 5.1 | 功能测试 | 切换到MySQL验证所有功能 |
| 5.2 | 性能测试 | 确认响应时间可接受 |
| 5.3 | 团队同步 | 其他开发者配置切换 |

#### 阶段六：清理SQLite

| 序号 | 任务 | 说明 |
|------|------|------|
| 6.1 | 移除SQLite代码 | 删除db_sqlite.go |
| 6.2 | 简化配置 | 移除database.type选项 |
| 6.3 | 清理迁移工具 | 移除数据迁移命令 |

## 6. 风险与应对

| 风险 | 影响 | 应对措施 |
|------|------|----------|
| SQL语法差异 | 部分查询可能需要调整 | 使用数据库抽象层封装差异 |
| 网络延迟 | 本地到云端有延迟 | 小数据量影响小，可接受 |
| 数据迁移丢失 | 迁移过程中数据丢失 | 迁移前备份，迁移后校验 |
| 云服务可用性 | RDS可能短暂不可用 | 华为云SLA保障，影响小 |

## 7. 后续优化

完成MySQL迁移后，可考虑：

1. **读写分离** - 如有需要，可配置只读副本
2. **自动化备份** - 配置RDS自动备份策略
3. **监控告警** - 配置数据库性能监控
4. **CI/CD集成** - 数据库迁移纳入发布流程