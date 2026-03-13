package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// AgentInvocationRepository Agent调用数据访问
type AgentInvocationRepository struct {
	db *sql.DB
}

// NewAgentInvocationRepository 创建Agent调用Repository
func NewAgentInvocationRepository(db *sql.DB) *AgentInvocationRepository {
	return &AgentInvocationRepository{db: db}
}

// Create 创建调用记录
func (r *AgentInvocationRepository) Create(ctx context.Context, invocation *model.AgentInvocation) error {
	query := `
		INSERT INTO agent_invocations (id, thread_id, agent_config_id, role, status, input, output, started_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		invocation.ID.String(), invocation.ThreadID.String(), invocation.AgentConfigID.String(), invocation.Role, invocation.Status, invocation.Input, invocation.Output, invocation.StartedAt, invocation.CompletedAt, invocation.CreatedAt,
	)
	return err
}

// FindByID 根据ID查找
func (r *AgentInvocationRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.AgentInvocation, error) {
	query := `
		SELECT id, thread_id, agent_config_id, role, status, input, output, started_at, completed_at, created_at
		FROM agent_invocations WHERE id = ?
	`
	invocation := &model.AgentInvocation{}
	var idStr, threadIDStr, agentConfigIDStr string
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &threadIDStr, &agentConfigIDStr, &invocation.Role, &invocation.Status, &invocation.Input, &invocation.Output, &invocation.StartedAt, &invocation.CompletedAt, &invocation.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find invocation: %w", err)
	}
	invocation.ID, _ = uuid.Parse(idStr)
	invocation.ThreadID, _ = uuid.Parse(threadIDStr)
	invocation.AgentConfigID, _ = uuid.Parse(agentConfigIDStr)
	return invocation, nil
}

// FindByThreadID 根据ThreadID查找
func (r *AgentInvocationRepository) FindByThreadID(ctx context.Context, threadID uuid.UUID) ([]*model.AgentInvocation, error) {
	query := `
		SELECT id, thread_id, agent_config_id, role, status, input, output, started_at, completed_at, created_at
		FROM agent_invocations WHERE thread_id = ? ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, threadID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find invocations: %w", err)
	}
	defer rows.Close()

	var invocations []*model.AgentInvocation
	for rows.Next() {
		invocation := &model.AgentInvocation{}
		var idStr, threadIDStr, agentConfigIDStr string
		err := rows.Scan(
			&idStr, &threadIDStr, &agentConfigIDStr, &invocation.Role, &invocation.Status, &invocation.Input, &invocation.Output, &invocation.StartedAt, &invocation.CompletedAt, &invocation.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan invocation: %w", err)
		}
		invocation.ID, _ = uuid.Parse(idStr)
		invocation.ThreadID, _ = uuid.Parse(threadIDStr)
		invocation.AgentConfigID, _ = uuid.Parse(agentConfigIDStr)
		invocations = append(invocations, invocation)
	}
	return invocations, nil
}

// Update 更新调用记录
func (r *AgentInvocationRepository) Update(ctx context.Context, invocation *model.AgentInvocation) error {
	query := `
		UPDATE agent_invocations
		SET status = ?, output = ?, started_at = ?, completed_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		invocation.Status, invocation.Output, invocation.StartedAt, invocation.CompletedAt, invocation.ID.String(),
	)
	return err
}

// Delete 删除调用记录
func (r *AgentInvocationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM agent_invocations WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id.String())
	return err
}