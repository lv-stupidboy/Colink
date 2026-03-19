// Package repo 提供数据库访问层
package repo

import (
	"database/sql"
	"fmt"

	"github.com/anthropic/isdp/pkg/config"
)

// DBType 数据库类型（类型别名，使用 config 包的定义）
type DBType = config.DBType

const (
	DBTypeSQLite = config.DBTypeSQLite
	DBTypeMySQL  = config.DBTypeMySQL
)

// DBConfig 数据库配置（类型别名，使用 config 包的定义）
type DBConfig = config.DatabaseConfig

// MySQLConfig MySQL配置（类型别名）
type MySQLConfig = config.MySQLConfig

// Dialect 数据库方言接口
type Dialect interface {
	Placeholder() string
	QuoteIdentifier() string
	AutoIncrement() string
}

// NewDB 创建数据库连接（工厂函数）
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

// NewDBFromConfig 从配置创建数据库连接（向后兼容）
func NewDBFromConfig(cfg DBConfig) (*sql.DB, error) {
	db, _, err := NewDB(cfg)
	return db, err
}

// NewSQLiteDB 创建SQLite数据库连接（向后兼容旧API）
// Deprecated: 使用 NewDB 代替
func NewSQLiteDB(dbPath string) (*sql.DB, error) {
	cfg := DBConfig{
		Type: DBTypeSQLite,
		Path: dbPath,
	}
	db, _, err := NewDB(cfg)
	return db, err
}