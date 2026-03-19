package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLDialect MySQL数据库方言
type MySQLDialect struct{}

func (d *MySQLDialect) Placeholder() string     { return "?" }
func (d *MySQLDialect) QuoteIdentifier() string { return "`" }
func (d *MySQLDialect) AutoIncrement() string   { return "AUTO_INCREMENT" }

// newMySQLDB 创建MySQL数据库连接
func newMySQLDB(cfg DBConfig) (*sql.DB, Dialect, error) {
	mc := cfg.MySQL

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

	// 设置字符集，确保中文正确存储和读取
	if _, err := db.ExecContext(ctx, "SET NAMES utf8mb4"); err != nil {
		return nil, nil, fmt.Errorf("failed to set charset: %w", err)
	}

	if mc.Schema != "" {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("USE `%s`", mc.Schema)); err != nil {
			return nil, nil, fmt.Errorf("failed to use schema %s: %w", mc.Schema, err)
		}
	}

	return db, &MySQLDialect{}, nil
}