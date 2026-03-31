package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// SettingsRepository Settings数据访问
type SettingsRepository struct {
	db *sql.DB
}

// NewSettingsRepository 创建Settings Repository
func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// Create 创建Settings
func (r *SettingsRepository) Create(ctx context.Context, settings *model.Settings) error {
	query := `
		INSERT INTO settings (id, name, description, directory_path, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		settings.ID.String(), settings.Name, settings.Description, settings.DirectoryPath, settings.Version, settings.CreatedAt, settings.UpdatedAt,
	)
	return err
}

// scanSettings 辅助函数，扫描Settings行
func scanSettings(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.Settings, error) {
	settings := &model.Settings{}
	var idStr string
	var description, directoryPath sql.NullString

	err := scanner.Scan(
		&idStr, &settings.Name, &description, &directoryPath, &settings.Version, &settings.CreatedAt, &settings.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	settings.ID, _ = uuid.Parse(idStr)
	if description.Valid {
		settings.Description = description.String
	}
	if directoryPath.Valid {
		settings.DirectoryPath = directoryPath.String
	}

	return settings, nil
}

// FindByID 根据ID查找
func (r *SettingsRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Settings, error) {
	query := `
		SELECT id, name, description, directory_path, version, created_at, updated_at
		FROM settings WHERE id = ?
	`
	settings, err := scanSettings(r.db.QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("settings not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find settings: %w", err)
	}
	return settings, nil
}

// FindByName 根据名称查找
func (r *SettingsRepository) FindByName(ctx context.Context, name string) (*model.Settings, error) {
	query := `
		SELECT id, name, description, directory_path, version, created_at, updated_at
		FROM settings WHERE name = ?
	`
	settings, err := scanSettings(r.db.QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("settings not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find settings: %w", err)
	}
	return settings, nil
}

// List 列出Settings，支持分页和过滤
func (r *SettingsRepository) List(ctx context.Context, query *model.SettingsListQuery) ([]*model.Settings, int64, error) {
	// 构建查询条件
	var conditions []string
	var args []interface{}

	if query.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR description LIKE ?)")
		searchPattern := "%" + query.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 计算总数
	countQuery := "SELECT COUNT(*) FROM settings " + whereClause
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count settings: %w", err)
	}

	// 分页
	page := query.Page
	pageSize := query.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	// 查询列表
	listQuery := `
		SELECT id, name, description, directory_path, version, created_at, updated_at
		FROM settings ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list settings: %w", err)
	}
	defer rows.Close()

	settingsList := make([]*model.Settings, 0)
	for rows.Next() {
		settings, err := scanSettings(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan settings: %w", err)
		}
		settingsList = append(settingsList, settings)
	}

	return settingsList, total, nil
}

// Update 更新Settings
func (r *SettingsRepository) Update(ctx context.Context, settings *model.Settings) error {
	query := `
		UPDATE settings
		SET name = ?, description = ?, directory_path = ?, version = ?, updated_at = NOW()
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		settings.Name, settings.Description, settings.DirectoryPath, settings.Version, settings.ID.String(),
	)
	return err
}

// Delete 删除Settings
func (r *SettingsRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM settings WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}