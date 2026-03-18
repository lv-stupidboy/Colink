// Package main 提供数据迁移工具，用于将 SQLite 数据迁移到 MySQL
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/pkg/config"
)

// validTables 定义允许迁移的表名白名单，防止 SQL 注入
var validTables = map[string]bool{
	"projects":            true,
	"base_agents":         true,
	"agent_configs":       true,
	"threads":             true,
	"messages":            true,
	"agent_invocations":   true,
	"artifacts":           true,
	"sandboxes":           true,
	"workflow_templates":  true,
}

func main() {
	// 设置 Windows 控制台 UTF-8 编码
	os.Stdout.WriteString("\x1b[?65001h")
	os.Stderr.WriteString("\x1b[?65001h")

	if len(os.Args) < 3 {
		fmt.Println("Usage: migrate <sqlite-path> <mysql-schema>")
		fmt.Println("")
		fmt.Println("Arguments:")
		fmt.Println("  sqlite-path   Path to the SQLite database file")
		fmt.Println("  mysql-schema  Target MySQL schema name")
		fmt.Println("")
		fmt.Println("Example:")
		fmt.Println("  migrate ./data/isdp.db isdp_db")
		os.Exit(1)
	}

	sourcePath := os.Args[1]
	schema := os.Args[2]

	if err := MigrateDataCmd(sourcePath, schema); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// MigrateDataCmd 数据迁移命令
func MigrateDataCmd(sourcePath, schema string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fmt.Println("Starting data migration from SQLite to MySQL...")
	fmt.Printf("Source: %s\n", sourcePath)
	fmt.Printf("Target Schema: %s\n", schema)

	// 1. 检查源文件是否存在
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source SQLite file not found: %s", sourcePath)
	}

	// 2. 连接源数据库 (SQLite)
	sqliteCfg := repo.DBConfig{
		Type: repo.DBTypeSQLite,
		Path: sourcePath,
	}
	sqliteDB, _, err := repo.NewDB(sqliteCfg)
	if err != nil {
		return fmt.Errorf("connect sqlite: %w", err)
	}
	defer sqliteDB.Close()

	// 3. 加载 MySQL 配置
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 4. 连接目标数据库 (MySQL)
	mysqlCfg := cfg.Database
	mysqlCfg.Type = repo.DBTypeMySQL
	mysqlCfg.MySQL.Schema = schema
	mysqlDB, _, err := repo.NewDB(mysqlCfg)
	if err != nil {
		return fmt.Errorf("connect mysql: %w", err)
	}
	defer mysqlDB.Close()

	// 5. 测试 MySQL 连接
	if err := mysqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping mysql: %w", err)
	}
	fmt.Println("Successfully connected to MySQL")

	// 6. 迁移各表数据
	tables := []string{
		"projects",
		"base_agents",
		"agent_configs",
		"threads",
		"messages",
		"agent_invocations",
		"artifacts",
		"sandboxes",
		"workflow_templates",
	}

	totalRows := 0
	for _, table := range tables {
		count, err := migrateTable(ctx, sqliteDB, mysqlDB, table)
		if err != nil {
			return fmt.Errorf("migrate table %s: %w", table, err)
		}
		totalRows += count
		fmt.Printf("  %s: %d rows migrated\n", table, count)
	}

	fmt.Printf("\nData migration completed successfully! Total rows: %d\n", totalRows)
	return nil
}

func migrateTable(ctx context.Context, src, dst *sql.DB, table string) (int, error) {
	// 白名单验证表名，防止 SQL 注入
	if !validTables[table] {
		return 0, fmt.Errorf("invalid table name: %s (not in whitelist)", table)
	}

	// 检查源表是否存在
	var tableExists bool
	err := src.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)",
		table).Scan(&tableExists)
	if err != nil {
		return 0, fmt.Errorf("check table existence: %w", err)
	}
	if !tableExists {
		fmt.Printf("  %s: skipped (table not found)\n", table)
		return 0, nil
	}

	// 开始事务
	tx, err := dst.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

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

	// 逐行迁移
	count := 0
	for rows.Next() {
		// 动态分配扫描目标
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return count, fmt.Errorf("scan row: %w", err)
		}

		// 构建 INSERT 语句
		placeholders := make([]string, len(columns))
		for i := range placeholders {
			placeholders[i] = "?"
		}

		// 使用 strings.Builder 构建列名列表
		var colBuilder strings.Builder
		for i, col := range columns {
			if i > 0 {
				colBuilder.WriteString(", ")
			}
			colBuilder.WriteString("`")
			colBuilder.WriteString(col)
			colBuilder.WriteString("`")
		}

		insertSQL := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)",
			table,
			colBuilder.String(),
			strings.Join(placeholders, ", "))

		if _, err := tx.ExecContext(ctx, insertSQL, values...); err != nil {
			// 忽略主键冲突错误
			if !isDuplicateKeyError(err) {
				return count, fmt.Errorf("insert row: %w", err)
			}
		}
		count++
	}

	if err = rows.Err(); err != nil {
		return count, fmt.Errorf("rows error: %w", err)
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		return count, fmt.Errorf("commit transaction: %w", err)
	}

	return count, nil
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// MySQL duplicate key error code is 1062
	// 错误信息通常包含 "Duplicate entry"
	errMsg := err.Error()
	return strings.Contains(errMsg, "Duplicate entry") ||
		strings.Contains(errMsg, "1062")
}