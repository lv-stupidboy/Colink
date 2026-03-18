# SQLite 到 MySQL 数据库迁移实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现SQLite与MySQL并存，支持配置一键切换，为最终迁移到华为云RDS MySQL做准备。

**Architecture:** 数据库抽象层模式，通过配置选择驱动（SQLite或MySQL），工厂函数创建对应连接。保持现有业务代码不变，只替换底层实现。

**Tech Stack:** Go 1.25, modernc.org/sqlite (现有), github.com/go-sql-driver/mysql (新增), spf13/viper配置管理

---

## 文件结构

| 文件 | 操作 | 说明 |
|------|------|------|
| `isdp/pkg/config/config.go` | 修改 | 扩展DatabaseConfig，添加MySQL配置结构 |
| `isdp/configs/config.yaml` | 修改 | 添加MySQL配置项 |
| `isdp/internal/repo/db.go` | 重构 | 定义接口、工厂函数 |
| `isdp/internal/repo/db_sqlite.go` | 新建 | SQLite驱动实现 |
| `isdp/internal/repo/db_mysql.go` | 新建 | MySQL驱动实现 |
| `isdp/scripts/init_db_mysql.sql` | 新建 | MySQL建表脚本 |
| `isdp/scripts/schema.sh` | 新建 | MySQL Schema管理脚本 |
| `isdp/cmd/server/main.go` | 修改 | 使用新DB工厂，支持双数据库初始化 |
| `isdp/cmd/migrate_data/main.go` | 新建 | SQLite到MySQL数据迁移工具 |
| `isdp/go.mod` | 修改 | 添加MySQL驱动依赖 |
| `isdp/internal/repo/db_test.go` | 新建 | 数据库连接测试 |

---

## Task 1: 扩展配置结构

**Files:**
- Modify: `isdp/pkg/config/config.go`

- [ ] **Step 1: 扩展 DatabaseConfig 结构体**

在 `isdp/pkg/config/config.go` 中修改 `DatabaseConfig` 结构体：

```go
// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type string `mapstructure:"type"` // sqlite | mysql

	// SQLite配置
	Path string `mapstructure:"path"`

	// MySQL配置
	MySQL MySQLConfig `mapstructure:"mysql"`
}

// MySQLConfig MySQL配置
type MySQLConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Database        string `mapstructure:"database"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	Schema          string `mapstructure:"schema"`
	Charset         string `mapstructure:"charset"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}
```

- [ ] **Step 2: 添加默认值**

在 `setDefaults()` 函数中添加：

```go
// 数据库默认值
viper.SetDefault("database.type", "sqlite")
viper.SetDefault("database.path", "./data/isdp.db")
viper.SetDefault("database.mysql.host", "")
viper.SetDefault("database.mysql.port", 3306)
viper.SetDefault("database.mysql.database", "isdp_dev")
viper.SetDefault("database.mysql.schema", "dev_default")
viper.SetDefault("database.mysql.charset", "utf8mb4")
viper.SetDefault("database.mysql.max_open_conns", 10)
viper.SetDefault("database.mysql.max_idle_conns", 5)
viper.SetDefault("database.mysql.conn_max_lifetime", 300)
```

- [ ] **Step 3: 验证配置加载**

```bash
cd isdp && go build ./pkg/config/...
```

Expected: 编译成功，无错误

- [ ] **Step 4: Commit**

```bash
git add isdp/pkg/config/config.go
git commit -m "feat(config): add MySQL database configuration support"
```

---

## Task 2: 更新配置文件

**Files:**
- Modify: `isdp/configs/config.yaml`

- [ ] **Step 1: 扩展 config.yaml**

修改 `isdp/configs/config.yaml`：

```yaml
database:
  # 数据库类型: sqlite | mysql
  type: sqlite

  # SQLite配置 (type=sqlite时生效)
  path: ./data/isdp.db

  # MySQL配置 (type=mysql时生效)
  mysql:
    host: ""                  # 华为云RDS地址
    port: 3306
    database: isdp_dev
    username: ""
    password: ""
    schema: dev_default       # 当前开发者使用的schema
    charset: utf8mb4
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime: 300
```

- [ ] **Step 2: Commit**

```bash
git add isdp/configs/config.yaml
git commit -m "feat(config): extend config.yaml with MySQL options"
```

---

## Task 3: 添加MySQL驱动依赖

**Files:**
- Modify: `isdp/go.mod`

- [ ] **Step 1: 添加MySQL驱动**

```bash
cd isdp && go get github.com/go-sql-driver/mysql
```

Expected: go.mod 中新增 `github.com/go-sql-driver/mysql`

- [ ] **Step 2: 整理依赖**

```bash
cd isdp && go mod tidy
```

- [ ] **Step 3: Commit**

```bash
git add isdp/go.mod isdp/go.sum
git commit -m "deps: add MySQL driver dependency"
```

---

## Task 4: 创建SQLite驱动实现

**Files:**
- Create: `isdp/internal/repo/db_sqlite.go`

- [ ] **Step 1: 创建 db_sqlite.go**

```go
// Package repo 提供数据库访问层
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteDialect SQLite方言
type SQLiteDialect struct{}

func (d *SQLiteDialect) Placeholder() string     { return "?" }
func (d *SQLiteDialect) QuoteIdentifier() string { return `"` }
func (d *SQLiteDialect) AutoIncrement() string   { return "AUTOINCREMENT" }

// newSQLiteDB 创建SQLite数据库连接
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
		db.Close()
		return nil, nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// 测试连接
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, &SQLiteDialect{}, nil
}
```

- [ ] **Step 2: 验证编译**

```bash
cd isdp && go build ./internal/repo/...
```

Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
git add isdp/internal/repo/db_sqlite.go
git commit -m "feat(repo): add SQLite driver implementation"
```

---

## Task 5: 创建MySQL驱动实现

**Files:**
- Create: `isdp/internal/repo/db_mysql.go`

- [ ] **Step 1: 创建 db_mysql.go**

```go
// Package repo 提供数据库访问层
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLDialect MySQL方言
type MySQLDialect struct{}

func (d *MySQLDialect) Placeholder() string     { return "?" }
func (d *MySQLDialect) QuoteIdentifier() string { return "`" }
func (d *MySQLDialect) AutoIncrement() string   { return "AUTO_INCREMENT" }

// newMySQLDB 创建MySQL数据库连接
func newMySQLDB(cfg DBConfig) (*sql.DB, Dialect, error) {
	mc := cfg.MySQL

	// 验证必要配置
	if mc.Host == "" {
		return nil, nil, fmt.Errorf("mysql host is required")
	}
	if mc.Username == "" {
		return nil, nil, fmt.Errorf("mysql username is required")
	}

	// DSN格式: username:password@tcp(host:port)/database?params
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

	// 设置连接池
	db.SetMaxOpenConns(mc.MaxOpenConns)
	db.SetMaxIdleConns(mc.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(mc.ConnMaxLifetime) * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 测试连接
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// 设置当前schema (在MySQL中schema即database)
	// 如果指定了schema且与database不同，切换到该schema
	if mc.Schema != "" && mc.Schema != mc.Database {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("USE `%s`", mc.Schema)); err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("failed to use schema %s: %w", mc.Schema, err)
		}
	}

	return db, &MySQLDialect{}, nil
}
```

- [ ] **Step 2: 验证编译**

```bash
cd isdp && go build ./internal/repo/...
```

Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
git add isdp/internal/repo/db_mysql.go
git commit -m "feat(repo): add MySQL driver implementation"
```

---

## Task 6: 重构db.go为接口和工厂

**Files:**
- Modify: `isdp/internal/repo/db.go`

- [ ] **Step 1: 重写 db.go**

将现有 `db.go` 内容替换为：

```go
// Package repo 提供数据库访问层
package repo

import (
	"database/sql"
	"fmt"
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
	Placeholder() string
	QuoteIdentifier() string
	AutoIncrement() string
}

// NewDB 创建数据库连接（工厂函数）
func NewDB(cfg DBConfig) (*sql.DB, Dialect, error) {
	switch cfg.Type {
	case DBTypeSQLite, "": // 默认SQLite
		return newSQLiteDB(cfg)
	case DBTypeMySQL:
		return newMySQLDB(cfg)
	default:
		return nil, nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}
}

// NewDBFromConfig 从配置创建数据库连接（兼容旧接口）
// Deprecated: Use NewDB instead
func NewDBFromConfig(cfg DBConfig) (*sql.DB, error) {
	db, _, err := NewDB(cfg)
	return db, err
}
```

- [ ] **Step 2: 验证编译**

```bash
cd isdp && go build ./internal/repo/...
```

Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
git add isdp/internal/repo/db.go
git commit -m "refactor(repo): convert db.go to interface and factory pattern"
```

---

## Task 7: 创建MySQL建表脚本

**Files:**
- Create: `isdp/scripts/init_db_mysql.sql`

- [ ] **Step 1: 创建 init_db_mysql.sql**

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
    local_path VARCHAR(512) DEFAULT '',
    git_repo VARCHAR(512),
    config TEXT,
    workflow_template_id VARCHAR(64),
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
    workflow_template_id VARCHAR(64),
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
    description TEXT,
    system_prompt TEXT,
    max_tokens INT DEFAULT 4096,
    temperature DECIMAL(3,2) DEFAULT 0.7,
    routing_config JSON,
    base_agent_id VARCHAR(64),
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

-- 工作流模板表
CREATE TABLE IF NOT EXISTS workflow_templates (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    agent_ids TEXT,
    checkpoints TEXT,
    estimated_time VARCHAR(64),
    is_system TINYINT DEFAULT 0,
    is_default TINYINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
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

- [ ] **Step 2: Commit**

```bash
git add isdp/scripts/init_db_mysql.sql
git commit -m "feat(scripts): add MySQL database initialization script"
```

---

## Task 7.5: 创建Schema管理脚本

**Files:**
- Create: `isdp/scripts/schema.sh`

- [ ] **Step 1: 创建 schema.sh**

```bash
#!/bin/bash
# Schema管理工具 - 用于管理MySQL多schema开发环境

set -e

# 从环境变量读取配置
MYSQL_HOST="${MYSQL_HOST:-localhost}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASS="${MYSQL_PASS:-}"
MYSQL_DB="${MYSQL_DB:-isdp_dev}"

COMMAND=$1
SCHEMA_NAME=$2

usage() {
    echo "Usage: $0 {create|drop|list|init} [schema_name]"
    echo ""
    echo "Commands:"
    echo "  create <name>  - 创建新schema并初始化表结构"
    echo "  drop <name>    - 删除schema"
    echo "  list           - 列出所有开发schema"
    echo "  init <name>    - 在已存在的schema中初始化表结构"
    echo ""
    echo "Environment Variables:"
    echo "  MYSQL_HOST     - MySQL主机地址 (default: localhost)"
    echo "  MYSQL_PORT     - MySQL端口 (default: 3306)"
    echo "  MYSQL_USER     - MySQL用户名 (default: root)"
    echo "  MYSQL_PASS     - MySQL密码"
    echo "  MYSQL_DB       - MySQL数据库名 (default: isdp_dev)"
    exit 1
}

mysql_cmd() {
    mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" "$MYSQL_DB" -e "$1"
}

case $COMMAND in
    create)
        if [ -z "$SCHEMA_NAME" ]; then
            echo "Error: schema name is required"
            usage
        fi

        echo "Creating schema: $SCHEMA_NAME"
        mysql_cmd "CREATE SCHEMA IF NOT EXISTS \`$SCHEMA_NAME\` DEFAULT CHARACTER SET utf8mb4;"

        echo "Initializing tables in schema: $SCHEMA_NAME"
        mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" "$SCHEMA_NAME" < "$(dirname "$0")/init_db_mysql.sql"

        echo "Schema '$SCHEMA_NAME' created and initialized successfully."
        ;;
    drop)
        if [ -z "$SCHEMA_NAME" ]; then
            echo "Error: schema name is required"
            usage
        fi

        echo "Dropping schema: $SCHEMA_NAME"
        mysql_cmd "DROP SCHEMA IF EXISTS \`$SCHEMA_NAME\`;"
        echo "Schema '$SCHEMA_NAME' dropped."
        ;;
    list)
        echo "Listing development schemas:"
        mysql_cmd "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME LIKE 'dev_%' OR SCHEMA_NAME = 'shared' ORDER BY SCHEMA_NAME;"
        ;;
    init)
        if [ -z "$SCHEMA_NAME" ]; then
            echo "Error: schema name is required"
            usage
        fi

        echo "Initializing tables in existing schema: $SCHEMA_NAME"
        mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASS" "$SCHEMA_NAME" < "$(dirname "$0")/init_db_mysql.sql"
        echo "Tables initialized in schema '$SCHEMA_NAME'."
        ;;
    *)
        usage
        ;;
esac
```

- [ ] **Step 2: 设置执行权限**

```bash
chmod +x isdp/scripts/schema.sh
```

- [ ] **Step 3: Commit**

```bash
git add isdp/scripts/schema.sh
git commit -m "feat(scripts): add MySQL schema management script"
```

---

## Task 8: 更新main.go使用新DB工厂

**Files:**
- Modify: `isdp/cmd/server/main.go`

- [ ] **Step 1: 修改数据库连接代码**

在 `main.go` 中，修改数据库连接部分（约54-60行）：

将：
```go
// 连接数据库
db, err := repo.NewDBFromConfig(repo.DBConfig{
	Path: cfg.Database.Path,
})
```

改为：
```go
// 连接数据库
dbType := repo.DBType(cfg.Database.Type)
if dbType == "" {
	dbType = repo.DBTypeSQLite // 默认SQLite
}

db, _, err := repo.NewDB(repo.DBConfig{
	Type: dbType,
	Path: cfg.Database.Path,
	MySQL: repo.MySQLConfig{
		Host:            cfg.Database.MySQL.Host,
		Port:            cfg.Database.MySQL.Port,
		Database:        cfg.Database.MySQL.Database,
		Username:        cfg.Database.MySQL.Username,
		Password:        cfg.Database.MySQL.Password,
		Schema:          cfg.Database.MySQL.Schema,
		Charset:         cfg.Database.MySQL.Charset,
		MaxOpenConns:    cfg.Database.MySQL.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MySQL.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.MySQL.ConnMaxLifetime,
	},
})
```

- [ ] **Step 2: 添加MySQL初始化逻辑**

在 `initDatabase` 函数后添加 `initDatabaseMySQL` 函数：

```go
// initDatabaseMySQL 初始化MySQL数据库表结构
func initDatabaseMySQL(db *sql.DB) error {
	schema := `-- 读取并执行 init_db_mysql.sql 的内容
-- 实际实现应从文件读取，这里简化处理
`
	// 对于MySQL，直接执行建表语句
	// 实际项目中应该从 init_db_mysql.sql 文件读取
	_, err := db.Exec(schema)
	return err
}
```

修改 `initDatabase` 函数，根据数据库类型选择初始化方式：

```go
// initDatabase 初始化数据库表结构
func initDatabase(db *sql.DB, dbType string) error {
	// SQLite使用内嵌schema
	if dbType == "sqlite" || dbType == "" {
		return initDatabaseSQLite(db)
	}
	// MySQL使用独立脚本
	return initDatabaseMySQL(db)
}
```

- [ ] **Step 3: 验证编译**

```bash
cd isdp && go build ./cmd/server/...
```

Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
git add isdp/cmd/server/main.go
git commit -m "feat(server): update main.go to use new DB factory with type selection"
```

---

## Task 9: 添加数据库连接测试

**Files:**
- Create: `isdp/internal/repo/db_test.go`

- [ ] **Step 1: 创建 db_test.go**

```go
package repo

import (
	"testing"
)

func TestNewDB_SQLite(t *testing.T) {
	cfg := DBConfig{
		Type: DBTypeSQLite,
		Path: ":memory:",
	}

	db, dialect, err := NewDB(cfg)
	if err != nil {
		t.Fatalf("Failed to create SQLite connection: %v", err)
	}
	defer db.Close()

	// 验证方言
	if dialect.Placeholder() != "?" {
		t.Errorf("Expected placeholder '?', got %s", dialect.Placeholder())
	}

	// 验证连接
	if err := db.Ping(); err != nil {
		t.Errorf("Failed to ping database: %v", err)
	}
}

func TestNewDB_MySQL_MissingConfig(t *testing.T) {
	cfg := DBConfig{
		Type: DBTypeMySQL,
		MySQL: MySQLConfig{
			// 缺少必要配置
		},
	}

	_, _, err := NewDB(cfg)
	if err == nil {
		t.Error("Expected error for missing MySQL config")
	}
}

func TestNewDB_InvalidType(t *testing.T) {
	cfg := DBConfig{
		Type: "invalid",
	}

	_, _, err := NewDB(cfg)
	if err == nil {
		t.Error("Expected error for invalid database type")
	}
}
```

- [ ] **Step 2: 运行测试**

```bash
cd isdp && go test ./internal/repo/... -v
```

Expected: 所有测试通过

- [ ] **Step 3: Commit**

```bash
git add isdp/internal/repo/db_test.go
git commit -m "test(repo): add database connection tests"
```

---

## Task 10: 创建数据迁移工具

**Files:**
- Create: `isdp/cmd/migrate_data/main.go`

- [ ] **Step 1: 创建迁移命令目录**

```bash
mkdir -p isdp/cmd/migrate_data
```

- [ ] **Step 2: 创建 main.go**

```go
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

// 表迁移顺序（按外键依赖关系排列）
var tables = []string{
	"base_agents",      // 无外键依赖，先迁移
	"agent_configs",    // 依赖 base_agents
	"projects",         // 无外键依赖
	"workflow_templates", // 无外键依赖
	"threads",          // 依赖 projects, workflow_templates
	"messages",         // 依赖 threads
	"agent_invocations", // 依赖 threads
	"artifacts",        // 依赖 threads
	"sandboxes",        // 依赖 threads
}

func main() {
	// 命令行参数
	sqlitePath := flag.String("sqlite", "./data/isdp.db", "SQLite database path")
	mysqlHost := flag.String("host", "", "MySQL host")
	mysqlPort := flag.Int("port", 3306, "MySQL port")
	mysqlUser := flag.String("user", "", "MySQL username")
	mysqlPass := flag.String("pass", "", "MySQL password")
	mysqlDB := flag.String("database", "isdp_dev", "MySQL database name")
	mysqlSchema := flag.String("schema", "", "Target schema (defaults to database name)")
	flag.Parse()

	if *mysqlHost == "" || *mysqlUser == "" {
		fmt.Fprintln(os.Stderr, "Error: MySQL host and user are required")
		flag.Usage()
		os.Exit(1)
	}

	targetSchema := *mysqlSchema
	if targetSchema == "" {
		targetSchema = *mysqlDB
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 连接源数据库 (SQLite)
	fmt.Printf("Connecting to SQLite: %s\n", *sqlitePath)
	sqliteDB, err := sql.Open("sqlite", *sqlitePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to SQLite: %v\n", err)
		os.Exit(1)
	}
	defer sqliteDB.Close()

	// 连接目标数据库 (MySQL)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local&multiStatements=true",
		*mysqlUser, *mysqlPass, *mysqlHost, *mysqlPort, *mysqlDB)
	fmt.Printf("Connecting to MySQL: %s:%d/%s\n", *mysqlHost, *mysqlPort, *mysqlDB)
	mysqlDBConn, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to MySQL: %v\n", err)
		os.Exit(1)
	}
	defer mysqlDBConn.Close()

	// 切换到目标schema
	if _, err := mysqlDBConn.ExecContext(ctx, fmt.Sprintf("USE `%s`", targetSchema)); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to use schema %s: %v\n", targetSchema, err)
		os.Exit(1)
	}

	// 迁移各表数据
	fmt.Println("Starting data migration...")
	totalRows := 0
	for _, table := range tables {
		count, err := migrateTable(ctx, sqliteDB, mysqlDBConn, table)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to migrate table %s: %v\n", table, err)
			os.Exit(1)
		}
		totalRows += count
	}

	fmt.Printf("\nMigration completed! Total rows migrated: %d\n", totalRows)
}

func migrateTable(ctx context.Context, src, dst *sql.DB, table string) (int, error) {
	fmt.Printf("Migrating table: %s... ", table)

	// 读取源数据
	rows, err := src.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", table))
	if err != nil {
		return 0, fmt.Errorf("query source: %w", err)
	}
	defer rows.Close()

	// 获取列信息
	columns, err := rows.Columns()
	if err != nil {
		return 0, fmt.Errorf("get columns: %w", err)
	}

	// 构建INSERT语句
	placeholders := ""
	for i := range columns {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
	}
	insertSQL := fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)",
		table, joinColumns(columns), placeholders)

	// 准备插入语句
	stmt, err := dst.PrepareContext(ctx, insertSQL)
	if err != nil {
		return 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	// 逐行迁移
	count := 0
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return count, fmt.Errorf("scan row: %w", err)
		}
		if _, err := stmt.ExecContext(ctx, valuePtrs...); err != nil {
			return count, fmt.Errorf("insert row: %w", err)
		}
		count++
	}

	fmt.Printf("%d rows\n", count)
	return count, rows.Err()
}

func joinColumns(columns []string) string {
	result := ""
	for i, col := range columns {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf("`%s`", col)
	}
	return result
}
```

- [ ] **Step 3: 验证编译**

```bash
cd isdp && go build ./cmd/migrate_data/...
```

Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
git add isdp/cmd/migrate_data/
git commit -m "feat(cmd): add data migration tool for SQLite to MySQL"
```

---

## Task 11: 集成测试与验证

**Files:**
- None (测试验证)

- [ ] **Step 1: 验证SQLite模式正常工作**

```bash
cd isdp && go build -o server.exe ./cmd/server/... && ./server.exe &
```

测试API：
```bash
curl http://localhost:8080/health
```

Expected: 返回 `{"status":"ok",...}`

停止服务器后继续。

- [ ] **Step 2: 验证配置切换**

修改 `config.yaml` 中 `database.type` 为 `mysql`（保持MySQL配置为空，预期报错）：

```bash
./server.exe
```

Expected: 启动失败，提示 MySQL 配置缺失（符合预期）

- [ ] **Step 3: 恢复SQLite配置**

将 `database.type` 改回 `sqlite`。

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "test: verify database switching works correctly"
```

---

## Task 12: 更新DB_MIGRATION_GUIDE.md

**Files:**
- Modify: `isdp/DB_MIGRATION_GUIDE.md`

- [ ] **Step 1: 更新迁移指南**

在文件末尾添加MySQL迁移说明：

```markdown
## MySQL 迁移指南

### 配置切换

1. 编辑 `configs/config.yaml`
2. 将 `database.type` 改为 `mysql`
3. 填写 MySQL 连接信息

### Schema 管理

使用 `scripts/schema.sh` 管理 MySQL schema：

\`\`\`bash
# 创建schema
MYSQL_HOST=xxx MYSQL_USER=xxx MYSQL_PASS=xxx MYSQL_DB=isdp_dev \
    ./scripts/schema.sh create dev_zhangsan

# 列出所有schema
./scripts/schema.sh list

# 删除schema
./scripts/schema.sh drop dev_zhangsan
\`\`\`

### 数据迁移

使用 `migrate_data` 命令迁移数据：

\`\`\`bash
./migrate_data \
  -sqlite ./data/isdp.db \
  -host rm-xxx.mysql.rds.huaweicloud.com \
  -user your_user \
  -pass your_password \
  -database isdp_dev \
  -schema dev_zhangsan
\`\`\`
```

- [ ] **Step 2: Commit**

```bash
git add isdp/DB_MIGRATION_GUIDE.md
git commit -m "docs: update migration guide with MySQL instructions"
```

---

## 验收清单

- [ ] SQLite模式正常运行
- [ ] MySQL配置结构完整
- [ ] 配置切换机制工作正常
- [ ] 所有单元测试通过
- [ ] 代码编译无警告
- [ ] 文档已更新

---

## 后续工作（不在本计划范围）

1. 华为云RDS实例创建（需要云控制台操作）
2. 团队Schema初始化（使用schema.sh脚本）
3. 实际数据迁移执行（使用migrate_data工具）
4. 功能验证测试（切换到MySQL后全量测试）
5. SQLite代码清理（验证OK后移除db_sqlite.go）