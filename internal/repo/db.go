// Package repo 提供数据库访问层
package repo

import (
	"database/sql"
	"fmt"
	"time"

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
	// 时间表达式：返回空字符串表示使用参数传入，返回表达式表示使用数据库函数
	NowExpr() string
	// JSON 包含查询：返回表达式模板，如 "JSON_CONTAINS(col, ?)" 或 "col LIKE ?"
	JSONContainsExpr(column string) string
	// JSON 包含查询参数格式化：MySQL 用 '"value"'，SQLite 用 '%"value"%'
	JSONContainsParam(value string) string
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

// BaseRepository 基础 Repository，包含数据库连接和类型信息
type BaseRepository struct {
	db     *sql.DB
	dbType DBType
}

// DB 返回数据库连接
func (r *BaseRepository) DB() *sql.DB {
	return r.db
}

// DBType 返回数据库类型
func (r *BaseRepository) DBType() DBType {
	return r.dbType
}

// NewBaseRepository 创建基础 Repository
func NewBaseRepository(db *sql.DB, dbType DBType) BaseRepository {
	return BaseRepository{db: db, dbType: dbType}
}

// ScanTime 通用时间扫描函数，根据 dbType 使用不同的扫描器
func ScanTime(dbType DBType, scanner interface{ Scan(...interface{}) error }, dest *time.Time) error {
	if dbType == DBTypeSQLite {
		var ts SQLiteTimeScanner
		if err := scanner.Scan(&ts); err != nil {
			return err
		}
		*dest = ts.Time
		return nil
	}
	// MySQL 直接扫描 time.Time（parseTime=true 已配置）
	return scanner.Scan(dest)
}

// ScanTimeNull 通用可空时间扫描函数
func ScanTimeNull(dbType DBType, scanner interface{ Scan(...interface{}) error }, dest **time.Time) error {
	if dbType == DBTypeSQLite {
		var ts SQLiteTimeScanner
		if err := scanner.Scan(&ts); err != nil {
			return err
		}
		if ts.Valid {
			*dest = &ts.Time
		} else {
			*dest = nil
		}
		return nil
	}
	// MySQL 使用 NullTime
	var nt sql.NullTime
	if err := scanner.Scan(&nt); err != nil {
		return err
	}
	if nt.Valid {
		*dest = &nt.Time
	} else {
		*dest = nil
	}
	return nil
}