package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// DBType 数据库类型
type DBType string

const (
	DBTypeSQLite DBType = "sqlite"
	DBTypeMySQL  DBType = "mysql"
)

// Dialect 数据库方言接口
type Dialect interface {
	Placeholder() string     // 参数占位符 (SQLite: ?, MySQL: ?)
	QuoteIdentifier() string // 标识符引用符 (SQLite: ", MySQL: `)
	AutoIncrement() string   // 自增语法 (SQLite: AUTOINCREMENT, MySQL: AUTO_INCREMENT)
}

// MySQLConfig MySQL数据库配置
type MySQLConfig struct {
	Host            string `mapstructure:"host" json:"host"`
	Port            int    `mapstructure:"port" json:"port"`
	Database        string `mapstructure:"database" json:"database"`
	Username        string `mapstructure:"username" json:"username"`
	Password        string `mapstructure:"password" json:"password"`
	Charset         string `mapstructure:"charset" json:"charset"`
	Schema          string `mapstructure:"schema" json:"schema"`
	MaxOpenConns    int    `mapstructure:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime" json:"conn_max_lifetime"`
}

// MySQLDialect MySQL数据库方言
type MySQLDialect struct{}

func (d *MySQLDialect) Placeholder() string     { return "?" }
func (d *MySQLDialect) QuoteIdentifier() string { return "`" }
func (d *MySQLDialect) AutoIncrement() string   { return "AUTO_INCREMENT" }

// buildMySQLDSN 构建MySQL连接字符串
func buildMySQLDSN(cfg MySQLConfig) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Local&multiStatements=true",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.Charset,
	)
}

// newMySQLDB 创建MySQL数据库连接
func newMySQLDB(cfg DBConfig) (*sql.DB, Dialect, error) {
	dsn := buildMySQLDSN(cfg.MySQL)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open mysql database: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(cfg.MySQL.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MySQL.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.MySQL.ConnMaxLifetime) * time.Second)

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// 设置当前schema
	if cfg.MySQL.Schema != "" {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("USE `%s`", cfg.MySQL.Schema)); err != nil {
			return nil, nil, fmt.Errorf("failed to use schema %s: %w", cfg.MySQL.Schema, err)
		}
	}

	return db, &MySQLDialect{}, nil
}