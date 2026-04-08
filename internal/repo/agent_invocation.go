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
		INSERT INTO agent_invocations (id, thread_id, agent_config_id, role, agent_name, status, input, output, started_at, completed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		invocation.ID.String(), invocation.ThreadID.String(), invocation.AgentConfigID.String(), invocation.Role, invocation.AgentName, invocation.Status, invocation.Input, invocation.Output, invocation.StartedAt, invocation.CompletedAt, invocation.CreatedAt,
	)
	return err
}

// FindByID 根据ID查找
func (r *AgentInvocationRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.AgentInvocation, error) {
	query := `
		SELECT id, thread_id, agent_config_id, role, agent_name, status, input, output, started_at, completed_at, created_at
		FROM agent_invocations WHERE id = ?
	`
	invocation := &model.AgentInvocation{}
	var idStr, threadIDStr, agentConfigIDStr string
	var agentName sql.NullString
	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &threadIDStr, &agentConfigIDStr, &invocation.Role, &agentName, &invocation.Status, &invocation.Input, &invocation.Output, &invocation.StartedAt, &invocation.CompletedAt, &invocation.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find invocation: %w", err)
	}
	invocation.ID, _ = uuid.Parse(idStr)
	invocation.ThreadID, _ = uuid.Parse(threadIDStr)
	invocation.AgentConfigID, _ = uuid.Parse(agentConfigIDStr)
	if agentName.Valid {
		invocation.AgentName = agentName.String
	}
	return invocation, nil
}

// FindByThreadID 根据ThreadID查找
func (r *AgentInvocationRepository) FindByThreadID(ctx context.Context, threadID uuid.UUID) ([]*model.AgentInvocation, error) {
	query := `
		SELECT id, thread_id, agent_config_id, role, agent_name, status, input, output, started_at, completed_at, created_at
		FROM agent_invocations WHERE thread_id = ? ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, threadID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find invocations: %w", err)
	}
	defer rows.Close()

	var invocations = make([]*model.AgentInvocation, 0) // 初始化为空数组，避免 JSON null
	for rows.Next() {
		invocation := &model.AgentInvocation{}
		var idStr, threadIDStr, agentConfigIDStr string
		var agentName sql.NullString
		err := rows.Scan(
			&idStr, &threadIDStr, &agentConfigIDStr, &invocation.Role, &agentName, &invocation.Status, &invocation.Input, &invocation.Output, &invocation.StartedAt, &invocation.CompletedAt, &invocation.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan invocation: %w", err)
		}
		invocation.ID, _ = uuid.Parse(idStr)
		invocation.ThreadID, _ = uuid.Parse(threadIDStr)
		invocation.AgentConfigID, _ = uuid.Parse(agentConfigIDStr)
		if agentName.Valid {
			invocation.AgentName = agentName.String
		}
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

// FindByStatus 根据状态查找调用记录（用于启动恢复）
func (r *AgentInvocationRepository) FindByStatus(ctx context.Context, status model.InvocationStatus) ([]*model.AgentInvocation, error) {
	query := `
		SELECT id, thread_id, agent_config_id, role, agent_name, status, input, output, started_at, completed_at, created_at, process_id
		FROM agent_invocations WHERE status = ? ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, string(status))
	if err != nil {
		return nil, fmt.Errorf("failed to find invocations by status: %w", err)
	}
	defer rows.Close()

	var invocations []*model.AgentInvocation
	for rows.Next() {
		invocation := &model.AgentInvocation{}
		var idStr, threadIDStr, agentConfigIDStr string
		var agentName, processID sql.NullString
		err := rows.Scan(
			&idStr, &threadIDStr, &agentConfigIDStr, &invocation.Role, &agentName, &invocation.Status, &invocation.Input, &invocation.Output, &invocation.StartedAt, &invocation.CompletedAt, &invocation.CreatedAt, &processID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan invocation: %w", err)
		}
		invocation.ID, _ = uuid.Parse(idStr)
		invocation.ThreadID, _ = uuid.Parse(threadIDStr)
		invocation.AgentConfigID, _ = uuid.Parse(agentConfigIDStr)
		if agentName.Valid {
			invocation.AgentName = agentName.String
		}
		if processID.Valid {
			invocation.ProcessID = &processID.String
		}
		invocations = append(invocations, invocation)
	}
	return invocations, nil
}

// FindRecentlyCompletedByThread 查找最近完成的 invocation（用于 WebSocket 重连状态同步）
func (r *AgentInvocationRepository) FindRecentlyCompletedByThread(ctx context.Context, threadID uuid.UUID, sinceMinutes int) ([]*model.AgentInvocation, error) {
	query := `
		SELECT id, thread_id, agent_config_id, role, agent_name, status, input, output, started_at, completed_at, created_at
		FROM agent_invocations
		WHERE thread_id = ?
			AND status IN ('completed', 'failed', 'interrupted')
			AND completed_at >= DATE_SUB(NOW(), INTERVAL ? MINUTE)
		ORDER BY completed_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, threadID.String(), sinceMinutes)
	if err != nil {
		return nil, fmt.Errorf("failed to find recently completed invocations: %w", err)
	}
	defer rows.Close()

	var invocations []*model.AgentInvocation
	for rows.Next() {
		invocation := &model.AgentInvocation{}
		var idStr, threadIDStr, agentConfigIDStr string
		var agentName sql.NullString
		err := rows.Scan(
			&idStr, &threadIDStr, &agentConfigIDStr, &invocation.Role, &agentName, &invocation.Status, &invocation.Input, &invocation.Output, &invocation.StartedAt, &invocation.CompletedAt, &invocation.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan invocation: %w", err)
		}
		invocation.ID, _ = uuid.Parse(idStr)
		invocation.ThreadID, _ = uuid.Parse(threadIDStr)
		invocation.AgentConfigID, _ = uuid.Parse(agentConfigIDStr)
		if agentName.Valid {
			invocation.AgentName = agentName.String
		}
		invocations = append(invocations, invocation)
	}
	return invocations, nil
}