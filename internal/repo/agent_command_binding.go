// 文件路径: isdp/internal/repo/agent_command_binding.go
package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// AgentCommandBindingRepository Agent-Command绑定数据访问
type AgentCommandBindingRepository struct {
	BaseRepository
}

// NewAgentCommandBindingRepository 创建AgentCommandBinding Repository
func NewAgentCommandBindingRepository(db *sql.DB, dbType DBType) *AgentCommandBindingRepository {
	return &AgentCommandBindingRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建绑定
func (r *AgentCommandBindingRepository) Create(ctx context.Context, binding *model.AgentCommandBinding) error {
	query := `
		INSERT INTO agent_command_bindings (id, agent_role_id, command_id, created_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := r.DB().ExecContext(ctx, query,
		binding.ID.String(), binding.AgentRoleID.String(), binding.CommandID.String(), binding.CreatedAt,
	)
	return err
}

// FindByAgentRoleID 根据AgentRole ID查找绑定的Command ID列表
func (r *AgentCommandBindingRepository) FindByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT command_id FROM agent_command_bindings WHERE agent_role_id = ?`
	rows, err := r.DB().QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find bindings: %w", err)
	}
	defer rows.Close()

	commandIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var commandIDStr string
		if err := rows.Scan(&commandIDStr); err != nil {
			return nil, fmt.Errorf("failed to scan command_id: %w", err)
		}
		commandID, _ := uuid.Parse(commandIDStr)
		commandIDs = append(commandIDs, commandID)
	}
	return commandIDs, nil
}

// FindCommandsByAgentRoleID 根据AgentRole ID查找绑定的Command详情列表
func (r *AgentCommandBindingRepository) FindCommandsByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Command, error) {
	query := `
		SELECT c.id, c.name, c.description, c.supported_agents, c.created_at, c.updated_at
		FROM commands c
		INNER JOIN agent_command_bindings b ON c.id = b.command_id
		WHERE b.agent_role_id = ?
		ORDER BY c.created_at DESC
	`
	rows, err := r.DB().QueryContext(ctx, query, agentRoleID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find commands: %w", err)
	}
	defer rows.Close()

	commands := make([]*model.Command, 0)
	for rows.Next() {
		command, err := scanCommand(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan command: %w", err)
		}
		commands = append(commands, command)
	}
	return commands, nil
}

// DeleteByAgentRoleID 删除AgentRole的所有绑定
func (r *AgentCommandBindingRepository) DeleteByAgentRoleID(ctx context.Context, agentRoleID uuid.UUID) error {
	query := `DELETE FROM agent_command_bindings WHERE agent_role_id = ?`
	_, err := r.DB().ExecContext(ctx, query, agentRoleID.String())
	return err
}

// DeleteBinding 删除特定绑定
func (r *AgentCommandBindingRepository) DeleteBinding(ctx context.Context, agentRoleID, commandID uuid.UUID) error {
	query := `DELETE FROM agent_command_bindings WHERE agent_role_id = ? AND command_id = ?`
	_, err := r.DB().ExecContext(ctx, query, agentRoleID.String(), commandID.String())
	return err
}

// ExistsBinding 检查绑定是否存在
func (r *AgentCommandBindingRepository) ExistsBinding(ctx context.Context, agentRoleID, commandID uuid.UUID) (bool, error) {
	query := `SELECT COUNT(*) FROM agent_command_bindings WHERE agent_role_id = ? AND command_id = ?`
	var count int
	err := r.DB().QueryRowContext(ctx, query, agentRoleID.String(), commandID.String()).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FindByCommandID 根据Command ID查找绑定的AgentRole ID列表
func (r *AgentCommandBindingRepository) FindByCommandID(ctx context.Context, commandID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT agent_role_id FROM agent_command_bindings WHERE command_id = ?`
	rows, err := r.DB().QueryContext(ctx, query, commandID.String())
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