package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// TeamPackageVersionRepository 团队包版本数据访问
type TeamPackageVersionRepository struct {
	BaseRepository
}

// NewTeamPackageVersionRepository 创建团队包版本 Repository
func NewTeamPackageVersionRepository(db *sql.DB, dbType DBType) *TeamPackageVersionRepository {
	return &TeamPackageVersionRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建团队包版本
func (r *TeamPackageVersionRepository) Create(ctx context.Context, version *model.TeamPackageVersion) error {
	query := `
		INSERT INTO team_package_versions (id, workflow_id, name, category, version, description, last_synced_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	id := uuid.New()

	var lastSyncedAt interface{}
	if version.LastSyncedAt != nil {
		lastSyncedAt = version.LastSyncedAt.Format(time.RFC3339)
	}

	_, err := r.DB().ExecContext(ctx, query,
		id.String(),
		version.WorkflowID.String(),
		version.Name,
		version.Category,
		version.Version,
		version.Description,
		lastSyncedAt,
		now.Format(time.RFC3339),
		now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to create team package version: %w", err)
	}

	version.ID = id
	version.CreatedAt = now
	version.UpdatedAt = now
	return nil
}

// FindByName 根据名称查找团队包版本
func (r *TeamPackageVersionRepository) FindByName(ctx context.Context, name string) (*model.TeamPackageVersion, error) {
	query := `
		SELECT id, workflow_id, name, category, version, description, last_synced_at, created_at, updated_at
		FROM team_package_versions WHERE name = ?
	`
	row := r.DB().QueryRowContext(ctx, query, name)

	var idStr, workflowIDStr string
	var lastSyncedAt sql.NullString
	var createdAt, updatedAt SQLiteTimeScanner
	v := &model.TeamPackageVersion{}

	err := row.Scan(
		&idStr,
		&workflowIDStr,
		&v.Name,
		&v.Category,
		&v.Version,
		&v.Description,
		&lastSyncedAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find team package version by name: %w", err)
	}

	v.ID, _ = uuid.Parse(idStr)
	v.WorkflowID, _ = uuid.Parse(workflowIDStr)
	if lastSyncedAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastSyncedAt.String)
		v.LastSyncedAt = &t
	}
	v.CreatedAt = createdAt.Time
	v.UpdatedAt = updatedAt.Time
	return v, nil
}

// ListAll 列出所有团队包版本
func (r *TeamPackageVersionRepository) ListAll(ctx context.Context) ([]model.TeamPackageVersion, error) {
	query := `
		SELECT id, workflow_id, name, category, version, description, last_synced_at, created_at, updated_at
		FROM team_package_versions ORDER BY created_at DESC
	`
	rows, err := r.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list team package versions: %w", err)
	}
	defer rows.Close()

	var versions []model.TeamPackageVersion
	for rows.Next() {
		var v model.TeamPackageVersion
		var idStr, workflowIDStr string
		var lastSyncedAt sql.NullString
		var createdAt, updatedAt SQLiteTimeScanner

		if err := rows.Scan(
			&idStr,
			&workflowIDStr,
			&v.Name,
			&v.Category,
			&v.Version,
			&v.Description,
			&lastSyncedAt,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan team package version: %w", err)
		}

		v.ID, _ = uuid.Parse(idStr)
		v.WorkflowID, _ = uuid.Parse(workflowIDStr)
		if lastSyncedAt.Valid {
			t, _ := time.Parse(time.RFC3339, lastSyncedAt.String)
			v.LastSyncedAt = &t
		}
		v.CreatedAt = createdAt.Time
		v.UpdatedAt = updatedAt.Time
		versions = append(versions, v)
	}
	return versions, nil
}

// Update 更新团队包版本
func (r *TeamPackageVersionRepository) Update(ctx context.Context, version *model.TeamPackageVersion) error {
	query := `
		UPDATE team_package_versions
		SET version = ?, description = ?, category = ?, last_synced_at = ?, updated_at = ?
		WHERE id = ?
	`
	now := time.Now()

	var lastSyncedAt interface{}
	if version.LastSyncedAt != nil {
		lastSyncedAt = version.LastSyncedAt.Format(time.RFC3339)
	}

	_, err := r.DB().ExecContext(ctx, query,
		version.Version,
		version.Description,
		version.Category,
		lastSyncedAt,
		now.Format(time.RFC3339),
		version.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update team package version: %w", err)
	}

	version.UpdatedAt = now
	return nil
}