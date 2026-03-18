// Package main 提供数据迁移工具，用于将 SQLite 数据迁移到 MySQL
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/pkg/config"
)

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

		insertSQL := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)",
			table,
			joinColumnNames(columns),
			joinStrings(placeholders, ", "))

		if _, err := dst.ExecContext(ctx, insertSQL, values...); err != nil {
			// 忽略主键冲突错误
			if !isDuplicateKeyError(err) {
				return count, fmt.Errorf("insert row: %w", err)
			}
		}
		count++
	}

	return count, rows.Err()
}

func joinColumnNames(columns []string) string {
	result := ""
	for i, col := range columns {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf("`%s`", col)
	}
	return result
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// MySQL duplicate key error code is 1062
	// 错误信息通常包含 "Duplicate entry"
	errMsg := err.Error()
	return len(errMsg) > 0 && (contains(errMsg, "Duplicate entry") ||
		contains(errMsg, "1062"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}