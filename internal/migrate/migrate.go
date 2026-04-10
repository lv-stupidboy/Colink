// Package migrate 提供数据库迁移功能，基于 goose 实现
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

// DBType 数据库类型
type DBType string

const (
	DBTypeSQLite DBType = "sqlite3"
	DBTypeMySQL  DBType = "mysql"
)

// Migrator 迁移执行器
type Migrator struct {
	db            *sql.DB
	dbType        DBType
	migrationsFS  fs.FS
	migrationsDir string
}

// NewMigrator 创建迁移执行器
// migrationsFS: 嵌入的迁移文件系统，可为 nil（使用本地文件）
// migrationsDir: 迁移脚本目录路径
func NewMigrator(db *sql.DB, dbType DBType, migrationsFS fs.FS, migrationsDir string) *Migrator {
	return &Migrator{
		db:            db,
		dbType:        dbType,
		migrationsFS:  migrationsFS,
		migrationsDir: migrationsDir,
	}
}

// Setup 设置 goose 配置
func (m *Migrator) Setup() error {
	// 设置数据库方言
	if err := goose.SetDialect(string(m.dbType)); err != nil {
		return fmt.Errorf("设置方言失败: %w", err)
	}

	// 设置嵌入式文件系统（如果有）
	if m.migrationsFS != nil {
		goose.SetBaseFS(m.migrationsFS)
	}

	return nil
}

// Status 返回当前迁移状态（通过 goose.Status 打印）
func (m *Migrator) Status(ctx context.Context) error {
	if err := m.Setup(); err != nil {
		return err
	}
	return goose.StatusContext(ctx, m.db, m.migrationsDir)
}

// GetVersion 获取当前数据库版本
func (m *Migrator) GetVersion(ctx context.Context) (int64, error) {
	if err := m.Setup(); err != nil {
		return 0, err
	}
	return goose.GetDBVersionContext(ctx, m.db)
}

// Up 执行所有待执行的迁移
func (m *Migrator) Up(ctx context.Context) error {
	if err := m.Setup(); err != nil {
		return err
	}
	return goose.UpContext(ctx, m.db, m.migrationsDir)
}

// UpTo 执行迁移到指定版本
func (m *Migrator) UpTo(ctx context.Context, version int64) error {
	if err := m.Setup(); err != nil {
		return err
	}
	return goose.UpToContext(ctx, m.db, m.migrationsDir, version)
}

// Down 回滚一个迁移
func (m *Migrator) Down(ctx context.Context) error {
	if err := m.Setup(); err != nil {
		return err
	}
	return goose.DownContext(ctx, m.db, m.migrationsDir)
}

// DownTo 回滚到指定版本
func (m *Migrator) DownTo(ctx context.Context, version int64) error {
	if err := m.Setup(); err != nil {
		return err
	}
	return goose.DownToContext(ctx, m.db, m.migrationsDir, version)
}

// Reset 重置所有迁移（回滚到 0）
func (m *Migrator) Reset(ctx context.Context) error {
	if err := m.Setup(); err != nil {
		return err
	}
	return goose.ResetContext(ctx, m.db, m.migrationsDir)
}

// Run 执行 goose 命令（通用接口）
func (m *Migrator) Run(ctx context.Context, command string, args ...string) error {
	if err := m.Setup(); err != nil {
		return err
	}
	return goose.RunWithOptionsContext(ctx, command, m.db, m.migrationsDir, args)
}