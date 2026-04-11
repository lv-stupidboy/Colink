package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// AgentSubagentBindingRepository Agent-Subagent绑定数据访问
type AgentSubagentBindingRepository struct {
	BaseRepository
}

// NewAgentSubagentBindingRepository 创建AgentSubagentBinding Repository
func NewAgentSubagentBindingRepository(db *sql.DB, dbType DBType) *AgentSubagentBindingRepository {
	return &AgentSubagentBindingRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建绑定
func (r *AgentSubagentBindingRepository) Create(ctx context.Context, binding *model.AgentSubagentBinding) error {
	query := `
		INSERT INTO agent_subagent_bindings (id, agent_role_id, subagent_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.DB().ExecContext(ctx, query,
		binding.ID.String(), binding.AgentRoleID.String(), binding.SubagentID.String(), binding.CreatedAt,
	)
	return err
}

// FindByAgentRoleID 根据AgentRole ID查找绑定的Subagent ID列表
func (r *AgentSubagentBindingRepository) FindByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT subagent_id FROM agent_subagent_bindings WHERE agent_role_id = ?`
	rows, err := r.DB().QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	subagentIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var subagentIDStr string
		if err := rows.Scan(&subagentIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan subagent_id: %w", err)
		}
		subagentID, _ := uuid.Parse(subagentIDStr)
		subagentIDs = append(subagentIDs, subagentID)
	}
	return subagentIDs, nil
}

// FindSubagentsByAgentRoleID 根据AgentRole ID查找绑定的Subagent详情列表
func (r *AgentSubagentBindingRepository) FindSubagentsByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Subagent, error) {
	query := `
		SELECT s.id, s.name, s.description, s.created_at, s.updated_at
		FROM subagents s
		INNER JOIN agent_subagent_bindings b ON s.id = b.subagent_id
		WHERE b.agent_role_id = ?
		ORDER BY s.created_at DESC
	`
	rows, err := r.DB().QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find subagents: %w", err)
	}
	defer rows.Close()

	subagents := make([]*model.Subagent, 0)
	for rows.Next() {
		subagent, err := scanSubagent(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subagent: %w", err)
		}
		subagents = append(subagents, subagent)
	}
	return subagents, nil
}

// DeleteByAgentRoleID 删除AgentRole的所有绑定
func (r *AgentSubagentBindingRepository) DeleteByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) error {
	query := `DELETE FROM agent_subagent_bindings WHERE agent_role_id = ?`
	_, err := r.DB().ExecContext(ctx, query, agentRoleID.String())
	return err
}

// DeleteBinding 删除特定绑定
func (r *AgentSubagentBindingRepository) DeleteBinding(ctx context.Context, agentRoleID, subagentID uuid.UUID) error {
	query := `DELETE FROM agent_subagent_bindings WHERE agent_role_id = ? AND subagent_id = ?`
	_, err := r.DB().ExecContext(ctx, query, agentRoleID.String(), subagentID.String())
	return err
}

// ExistsBinding 检查绑定是否存在
func (r *AgentSubagentBindingRepository) ExistsBinding(ctx context.Context, agentRoleID, subagentID uuid.UUID) (bool, error) {
	query := `SELECT COUNT(*) FROM agent_subagent_bindings WHERE agent_role_id = ? AND subagent_id = ?`
	var count int
	err := r.DB().QueryRowContext(ctx, query, agentRoleID.String(), subagentID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindBySubagentID 根据Subagent ID查找绑定的AgentRole ID列表
func (r *AgentSubagentBindingRepository) FindBySubagentID(ctx context.Context, subagentID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT agent_role_id FROM agent_subagent_bindings WHERE subagent_id = ?`
	rows, err := r.DB().QueryContext(ctx, query, subagentID.String())
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