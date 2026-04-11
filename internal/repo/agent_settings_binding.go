package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// AgentSettingsBindingRepository Agent-Settings绑定数据访问
type AgentSettingsBindingRepository struct {
	BaseRepository
}

// NewAgentSettingsBindingRepository 创建AgentSettingsBinding Repository
func NewAgentSettingsBindingRepository(db *sql.DB, dbType DBType) *AgentSettingsBindingRepository {
	return &AgentSettingsBindingRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建绑定
func (r *AgentSettingsBindingRepository) Create(ctx context.Context, binding *model.AgentSettingsBinding) error {
	query := `
		INSERT INTO agent_settings_bindings (id, agent_role_id, settings_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.DB().ExecContext(ctx, query,
		binding.ID.String(), binding.AgentRoleID.String(), binding.SettingsID.String(), binding.CreatedAt,
	)
	return err
}

// FindByAgentRoleID 根据AgentRole ID查找绑定的Settings ID列表
func (r *AgentSettingsBindingRepository) FindByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT settings_id FROM agent_settings_bindings WHERE agent_role_id = ?`
	rows, err := r.DB().QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	settingsIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var settingsIDStr string
		if err := rows.Scan(&settingsIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan settings_id: %w", err)
		}
		settingsID, _ := uuid.Parse(settingsIDStr)
		settingsIDs = append(settingsIDs, settingsID)
	}
	return settingsIDs, nil
}

// FindBySettingsID 根据Settings ID查找绑定的AgentRole ID列表
func (r *AgentSettingsBindingRepository) FindBySettingsID(ctx context.Context, settingsID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT agent_role_id FROM agent_settings_bindings WHERE settings_id = ?`
	rows, err := r.DB().QueryContext(ctx, query, settingsID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	agentRoleIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var agentRoleIDStr string
		if err := rows.Scan(&agentRoleIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan agent_role_id: %w", err)
		}
		agentRoleID, _ := uuid.Parse(agentRoleIDStr)
		agentRoleIDs = append(agentRoleIDs, agentRoleID)
	}
	return agentRoleIDs, nil
}

// DeleteByAgentRoleID 删除AgentRole的所有绑定
func (r *AgentSettingsBindingRepository) DeleteByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) error {
	query := `DELETE FROM agent_settings_bindings WHERE agent_role_id = ?`
	_, err := r.DB().ExecContext(ctx, query, agentRoleID.String())
	return err
}

// DeleteBinding 删除特定绑定
func (r *AgentSettingsBindingRepository) DeleteBinding(ctx context.Context, agentRoleID, settingsID uuid.UUID) error {
	query := `DELETE FROM agent_settings_bindings WHERE agent_role_id = ? AND settings_id = ?`
	_, err := r.DB().ExecContext(ctx, query, agentRoleID.String(), settingsID.String())
	return err
}

// ExistsBinding 检查绑定是否存在
func (r *AgentSettingsBindingRepository) ExistsBinding(ctx context.Context, agentRoleID, settingsID uuid.UUID) (bool, error) {
	query := `SELECT COUNT(*) FROM agent_settings_bindings WHERE agent_role_id = ? AND settings_id = ?`
	var count int
	err := r.DB().QueryRowContext(ctx, query, agentRoleID.String(), settingsID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}