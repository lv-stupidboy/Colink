package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// MarketRepository 市场配置数据访问
type MarketRepository struct {
	BaseRepository
}

// NewMarketRepository 创建市场 Repository
func NewMarketRepository(db *sql.DB, dbType DBType) *MarketRepository {
	return &MarketRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建市场配置
func (r *MarketRepository) Create(ctx context.Context, market *model.Market) error {
	query := `
		INSERT INTO markets (id, name, url, branch, enabled, auto_update, check_interval, last_synced_at, last_error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	id := uuid.New()

	var lastSyncedAt interface{}
	if market.LastSyncedAt != nil {
		lastSyncedAt = market.LastSyncedAt.Format(time.RFC3339)
	}

	_, err := r.DB().ExecContext(ctx, query,
		id.String(),
		market.Name,
		market.URL,
		market.Branch,
		market.Enabled,
		market.AutoUpdate,
		market.CheckInterval,
		lastSyncedAt,
		market.LastError,
		now.Format(time.RFC3339),
		now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to create market: %w", err)
	}

	market.ID = id
	market.CreatedAt = now
	market.UpdatedAt = now
	return nil
}

// List 列出所有市场配置
func (r *MarketRepository) List(ctx context.Context) ([]model.Market, error) {
	query := `
		SELECT id, name, url, branch, enabled, auto_update, check_interval, last_synced_at, last_error, created_at, updated_at
		FROM markets ORDER BY created_at DESC
	`
	rows, err := r.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list markets: %w", err)
	}
	defer rows.Close()

	var markets []model.Market
	for rows.Next() {
		var m model.Market
		var idStr string
		var lastSyncedAt sql.NullString
		var createdAt, updatedAt SQLiteTimeScanner

		if err := rows.Scan(
			&idStr,
			&m.Name,
			&m.URL,
			&m.Branch,
			&m.Enabled,
			&m.AutoUpdate,
			&m.CheckInterval,
			&lastSyncedAt,
			&m.LastError,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan market: %w", err)
		}

		m.ID, _ = uuid.Parse(idStr)
		if lastSyncedAt.Valid {
			t, _ := time.Parse(time.RFC3339, lastSyncedAt.String)
			m.LastSyncedAt = &t
		}
		m.CreatedAt = createdAt.Time
		m.UpdatedAt = updatedAt.Time
		markets = append(markets, m)
	}
	return markets, nil
}

// FindByID 根据ID查找市场配置
func (r *MarketRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Market, error) {
	query := `
		SELECT id, name, url, branch, enabled, auto_update, check_interval, last_synced_at, last_error, created_at, updated_at
		FROM markets WHERE id = ?
	`
	row := r.DB().QueryRowContext(ctx, query, id.String())

	var m model.Market
	var idStr string
	var lastSyncedAt sql.NullString
	var createdAt, updatedAt SQLiteTimeScanner

	err := row.Scan(
		&idStr,
		&m.Name,
		&m.URL,
		&m.Branch,
		&m.Enabled,
		&m.AutoUpdate,
		&m.CheckInterval,
		&lastSyncedAt,
		&m.LastError,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find market by id: %w", err)
	}

	m.ID, _ = uuid.Parse(idStr)
	if lastSyncedAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastSyncedAt.String)
		m.LastSyncedAt = &t
	}
	m.CreatedAt = createdAt.Time
	m.UpdatedAt = updatedAt.Time
	return &m, nil
}

// Update 更新市场配置
func (r *MarketRepository) Update(ctx context.Context, market *model.Market) error {
	query := `
		UPDATE markets
		SET name = ?, url = ?, branch = ?, enabled = ?, auto_update = ?, check_interval = ?, last_synced_at = ?, last_error = ?, updated_at = ?
		WHERE id = ?
	`
	now := time.Now()

	var lastSyncedAt interface{}
	if market.LastSyncedAt != nil {
		lastSyncedAt = market.LastSyncedAt.Format(time.RFC3339)
	}

	_, err := r.DB().ExecContext(ctx, query,
		market.Name,
		market.URL,
		market.Branch,
		market.Enabled,
		market.AutoUpdate,
		market.CheckInterval,
		lastSyncedAt,
		market.LastError,
		now.Format(time.RFC3339),
		market.ID.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update market: %w", err)
	}

	market.UpdatedAt = now
	return nil
}

// Delete 删除市场配置
func (r *MarketRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM markets WHERE id = ?`
	_, err := r.DB().ExecContext(ctx, query, id.String())
	if err != nil {
		return fmt.Errorf("failed to delete market: %w", err)
	}
	return nil
}

// UpdateSyncStatus 更新同步状态
func (r *MarketRepository) UpdateSyncStatus(ctx context.Context, id uuid.UUID, syncedAt *time.Time, errMsg string) error {
	query := `
		UPDATE markets
		SET last_synced_at = ?, last_error = ?, updated_at = ?
		WHERE id = ?
	`
	now := time.Now()

	var lastSyncedAt interface{}
	if syncedAt != nil {
		lastSyncedAt = syncedAt.Format(time.RFC3339)
	}

	_, err := r.DB().ExecContext(ctx, query,
		lastSyncedAt,
		errMsg,
		now.Format(time.RFC3339),
		id.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to update market sync status: %w", err)
	}
	return nil
}