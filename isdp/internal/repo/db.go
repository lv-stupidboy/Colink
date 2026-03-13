// Package repo 提供数据库访问层
package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DBConfig 数据库配置
type DBConfig struct {
	Path string // SQLite 数据库文件路径
}

// NewDB 创建数据库连接
func NewDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池
	db.SetMaxOpenConns(1) // SQLite 只支持单个写连接
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// 启用外键约束
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// 测试连接
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// NewDBFromConfig 从配置创建数据库连接
func NewDBFromConfig(cfg DBConfig) (*sql.DB, error) {
	return NewDB(cfg.Path)
}