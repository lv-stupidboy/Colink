package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteDialect SQLite数据库方言
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
		return nil, nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, &SQLiteDialect{}, nil
}