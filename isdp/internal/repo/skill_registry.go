package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// SkillRegistryRepository 联邦技能源数据访问
type SkillRegistryRepository struct {
	db *sql.DB
}

// NewSkillRegistryRepository 创建 SkillRegistry Repository
func NewSkillRegistryRepository(db *sql.DB) *SkillRegistryRepository {
	return &SkillRegistryRepository{db: db}
}

// Create 创建注册表
func (r *SkillRegistryRepository) Create(ctx context.Context, registry *model.SkillRegistry) error {
	query := `
		INSERT INTO skill_registries (id, name, display_name, type, url, auth_config, sync_interval, last_sync_at, sync_status, skill_count, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	authConfig, _ := json.Marshal(registry.AuthConfig)

	var lastSyncAt interface{}
	if registry.LastSyncAt != nil {
		lastSyncAt = registry.LastSyncAt
	}

	_, err := r.db.ExecContext(ctx, query,
		registry.ID.String(), registry.Name, registry.DisplayName, registry.Type, registry.URL, authConfig, registry.SyncInterval, lastSyncAt, registry.SyncStatus, registry.SkillCount, registry.Status, registry.CreatedAt,
	)
	return err
}

// scanRegistry 辅助函数，扫描 SkillRegistry 行
func scanRegistry(scanner interface {
	Scan(dest ...interface{}) error
}) (*model.SkillRegistry, error) {
	registry := &model.SkillRegistry{}
	var idStr string
	var displayName sql.NullString
	var authConfig []byte
	var lastSyncAt sql.NullTime

	err := scanner.Scan(
		&idStr, &registry.Name, &displayName, &registry.Type, &registry.URL, &authConfig, &registry.SyncInterval, &lastSyncAt, &registry.SyncStatus, &registry.SkillCount, &registry.Status, &registry.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	registry.ID, _ = uuid.Parse(idStr)
	if displayName.Valid {
		registry.DisplayName = displayName.String
	}
	if lastSyncAt.Valid {
		registry.LastSyncAt = &lastSyncAt.Time
	}
	json.Unmarshal(authConfig, &registry.AuthConfig)

	return registry, nil
}

// FindByID 根据 ID 查找
func (r *SkillRegistryRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.SkillRegistry, error) {
	query := `
		SELECT id, name, display_name, type, url, auth_config, sync_interval, last_sync_at, sync_status, skill_count, status, created_at
		FROM skill_registries WHERE id = ?
	`
	registry, err := scanRegistry(r.db.QueryRowContext(ctx, query, id.String()))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("registry not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find registry: %w", err)
	}
	return registry, nil
}

// FindByName 根据名称查找
func (r *SkillRegistryRepository) FindByName(ctx context.Context, name string) (*model.SkillRegistry, error) {
	query := `
		SELECT id, name, display_name, type, url, auth_config, sync_interval, last_sync_at, sync_status, skill_count, status, created_at
		FROM skill_registries WHERE name = ?
	`
	registry, err := scanRegistry(r.db.QueryRowContext(ctx, query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("registry not found: %w", err)
		}
		return nil, fmt.Errorf("failed to find registry: %w", err)
	}
	return registry, nil
}

// RegistryListQuery 注册表列表查询参数
type RegistryListQuery struct {
	Type   string `form:"type"`
	Status string `form:"status"`
	Search string `form:"search"`
	Page   int    `form:"page"`
	Size   int    `form:"size"`
}

// List 列出注册表
func (r *SkillRegistryRepository) List(ctx context.Context, query *RegistryListQuery) ([]*model.SkillRegistry, int64, error) {
	// 构建查询条件
	var conditions []string
	var args []interface{}

	if query.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, query.Type)
	}
	if query.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, query.Status)
	}
	if query.Search != "" {
		conditions = append(conditions, "(name LIKE ? OR display_name LIKE ?)")
		searchPattern := "%" + query.Search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 计算总数
	countQuery := "SELECT COUNT(*) FROM skill_registries " + whereClause
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count registries: %w", err)
	}

	// 分页
	page := query.Page
	pageSize := query.Size
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
		SELECT id, name, display_name, type, url, auth_config, sync_interval, last_sync_at, sync_status, skill_count, status, created_at
		FROM skill_registries ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list registries: %w", err)
	}
	defer rows.Close()

	registries := make([]*model.SkillRegistry, 0)
	for rows.Next() {
		registry, err := scanRegistry(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan registry: %w", err)
		}
		registries = append(registries, registry)
	}

	return registries, total, nil
}

// Update 更新注册表
func (r *SkillRegistryRepository) Update(ctx context.Context, registry *model.SkillRegistry) error {
	query := `
		UPDATE skill_registries
		SET name = ?, display_name = ?, type = ?, url = ?, auth_config = ?, sync_interval = ?, last_sync_at = ?, sync_status = ?, skill_count = ?, status = ?
		WHERE id = ?
	`
	authConfig, _ := json.Marshal(registry.AuthConfig)

	var lastSyncAt interface{}
	if registry.LastSyncAt != nil {
		lastSyncAt = registry.LastSyncAt
	}

	_, err := r.db.ExecContext(ctx, query,
		registry.Name, registry.DisplayName, registry.Type, registry.URL, authConfig, registry.SyncInterval, lastSyncAt, registry.SyncStatus, registry.SkillCount, registry.Status, registry.ID.String(),
	)
	return err
}

// Delete 删除注册表
func (r *SkillRegistryRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM skill_registries WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}

// UpdateSyncStatus 更新同步状态
func (r *SkillRegistryRepository) UpdateSyncStatus(ctx context.Context, id uuid.UUID, status model.RegistrySyncStatus, skillCount int) error {
	query := `UPDATE skill_registries SET sync_status = ?, skill_count = ?, last_sync_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, status, skillCount, time.Now(), id.String())
	return err
}

// FindByStatus 根据状态查找注册表
func (r *SkillRegistryRepository) FindByStatus(ctx context.Context, status model.RegistryStatus) ([]*model.SkillRegistry, error) {
	query := `
		SELECT id, name, display_name, type, url, auth_config, sync_interval, last_sync_at, sync_status, skill_count, status, created_at
		FROM skill_registries WHERE status = ?
	`
	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to find registries by status: %w", err)
	}
	defer rows.Close()

	registries := make([]*model.SkillRegistry, 0)
	for rows.Next() {
		registry, err := scanRegistry(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan registry: %w", err)
		}
		registries = append(registries, registry)
	}

	return registries, nil
}

// FindAll 获取所有活跃注册表
func (r *SkillRegistryRepository) FindAll(ctx context.Context) ([]*model.SkillRegistry, error) {
	query := `
		SELECT id, name, display_name, type, url, auth_config, sync_interval, last_sync_at, sync_status, skill_count, status, created_at
		FROM skill_registries ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find all registries: %w", err)
	}
	defer rows.Close()

	registries := make([]*model.SkillRegistry, 0)
	for rows.Next() {
		registry, err := scanRegistry(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan registry: %w", err)
		}
		registries = append(registries, registry)
	}

	return registries, nil
}