package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteDialect SQLite数据库方言
type SQLiteDialect struct{}

func (d *SQLiteDialect) Placeholder() string     { return "?" }
func (d *SQLiteDialect) QuoteIdentifier() string { return `"` }
func (d *SQLiteDialect) AutoIncrement() string   { return "AUTOINCREMENT" }

// NowExpr 返回空字符串，表示使用参数传入时间（兼容两种数据库）
func (d *SQLiteDialect) NowExpr() string { return "" }

// JSONContainsExpr 返回 LIKE 模糊匹配表达式（SQLite 不支持 JSON_CONTAINS）
func (d *SQLiteDialect) JSONContainsExpr(column string) string {
	return column + " LIKE ?"
}

// JSONContainsParam 格式化参数为 LIKE 匹配模式
func (d *SQLiteDialect) JSONContainsParam(value string) string {
	return `%"` + value + `"%`
}

// parseSQLiteTime 解析 SQLite 时间字符串为 time.Time
// SQLite 存储时间格式: "2006-01-02 15:04:05" 或 "2006-01-02T15:04:05Z"
// modernc.org/sqlite 驱动可能存储 Go time.String() 格式（含 m=+... monotonic 部分）
// 重要：对于没有时区信息的格式，使用本地时区解析（而非 UTC）
func parseSQLiteTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}

	// 处理 Go time.String() 格式，去掉 m=+... monotonic clock 部分
	// 例如: "2026-04-11 10:46:45.7677474 +0800 CST m=+836.804542801"
	if idx := strings.Index(s, " m="); idx > 0 {
		s = s[:idx]
	}

	localLocation := time.Local

	// 尝试多种格式解析
	// 注意：对于带时区的格式（如 RFC3339），使用 time.Parse（时区信息在字符串中）
	// 对于不带时区的格式，使用 time.ParseInLocation（本地时区）
	layoutsUTC := []struct {
		layout string
		useUTC bool
	}{
		{"2006-01-02 15:04:05.999999999 -0700 MST", false}, // 带时区信息
		{"2006-01-02 15:04:05.999999999 +0700 CST", false}, // 带时区信息
		{"2006-01-02 15:04:05.999999999", true},            // 不带时区，用本地时区
		{"2006-01-02 15:04:05", true},                      // 不带时区，用本地时区
		{"2006-01-02T15:04:05.999999999", true},            // 不带时区，用本地时区
		{"2006-01-02T15:04:05Z", false},                    // UTC 时间
		{"2006-01-02T15:04:05", true},                      // 不带时区，用本地时区
		{time.RFC3339, false},                              // 带时区
		{time.RFC3339Nano, false},                          // 带时区
	}

	for _, item := range layoutsUTC {
		var t time.Time
		var err error
		if item.useUTC {
			t, err = time.ParseInLocation(item.layout, s, localLocation)
		} else {
			t, err = time.Parse(item.layout, s)
		}
		if err == nil {
			return t
		}
	}
	// 解析失败返回零值
	return time.Time{}
}

// SQLiteTimeScanner 辅助扫描 SQLite 时间字段
type SQLiteTimeScanner struct {
	Time  time.Time
	Valid bool
}

// Scan 实现 sql.Scanner 接口
func (s *SQLiteTimeScanner) Scan(value interface{}) error {
	if value == nil {
		s.Valid = false
		s.Time = time.Time{}
		return nil
	}
	switch v := value.(type) {
	case string:
		s.Time = parseSQLiteTime(v)
		s.Valid = !s.Time.IsZero()
		return nil
	case []byte:
		s.Time = parseSQLiteTime(string(v))
		s.Valid = !s.Time.IsZero()
		return nil
	case time.Time:
		s.Time = v
		s.Valid = true
		return nil
	default:
		s.Valid = false
		return fmt.Errorf("cannot scan %T into SQLiteTimeScanner", value)
	}
}

// newSQLiteDB 创建SQLite数据库连接
func newSQLiteDB(cfg DBConfig) (*sql.DB, Dialect, error) {
	if cfg.Path == "" {
		return nil, nil, fmt.Errorf("sqlite database path cannot be empty")
	}

	// 添加 _loc=auto 参数，让驱动自动解析 TEXT 时间字段为 time.Time
	dsn := cfg.Path + "?_loc=auto"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

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