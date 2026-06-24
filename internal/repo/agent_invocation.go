package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// AgentInvocationRepository Agent调用数据访问
type AgentInvocationRepository struct {
	BaseRepository
}

// NewAgentInvocationRepository 创建Agent调用Repository
func NewAgentInvocationRepository(db *sql.DB, dbType DBType) *AgentInvocationRepository {
	return &AgentInvocationRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建调用记录
func (r *AgentInvocationRepository) Create(ctx context.Context, invocation *model.AgentInvocation) error {
	query := `
		INSERT INTO agent_invocations (id, thread_id, agent_config_id, role, agent_name, status, input, full_prompt, output, started_at, completed_at, created_at, session_id, callback_token, triggered_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	var triggeredBy interface{}
	if invocation.TriggeredBy != uuid.Nil {
		triggeredBy = invocation.TriggeredBy.String()
	}
	_, err := r.DB().ExecContext(ctx, query,
		invocation.ID.String(), invocation.ThreadID.String(), invocation.AgentConfigID.String(), invocation.Role, invocation.AgentName, invocation.Status, invocation.Input, invocation.FullPrompt, invocation.Output, invocation.StartedAt, invocation.CompletedAt, invocation.CreatedAt, invocation.SessionID, invocation.CallbackToken, triggeredBy,
	)
	return err
}

// FindByID 根据ID查找
func (r *AgentInvocationRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.AgentInvocation, error) {
	query := `
		SELECT id, thread_id, agent_config_id, role, agent_name, status, input, full_prompt, output, started_at, completed_at, created_at, session_id,
		       input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, cost_usd, duration_ms, duration_api_ms, callback_token, triggered_by
		FROM agent_invocations WHERE id = ?
	`
	invocation := &model.AgentInvocation{}
	var idStr, threadIDStr, agentConfigIDStr string
	var agentName, sessionID, fullPrompt, callbackToken, triggeredBy sql.NullString
	var startedAt, completedAt, createdAt SQLiteTimeScanner
	err := r.DB().QueryRowContext(ctx, query, id.String()).Scan(
		&idStr, &threadIDStr, &agentConfigIDStr, &invocation.Role, &agentName, &invocation.Status, &invocation.Input, &fullPrompt, &invocation.Output, &startedAt, &completedAt, &createdAt, &sessionID,
		&invocation.InputTokens, &invocation.OutputTokens, &invocation.CacheReadTokens, &invocation.CacheCreationTokens, &invocation.CostUsd, &invocation.DurationMs, &invocation.DurationApiMs,
		&callbackToken, &triggeredBy,
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
	if fullPrompt.Valid {
		invocation.FullPrompt = fullPrompt.String
	}
	if sessionID.Valid {
		invocation.SessionID = sessionID.String
	}
	if callbackToken.Valid {
		invocation.CallbackToken = callbackToken.String
	}
	if triggeredBy.Valid {
		invocation.TriggeredBy, _ = uuid.Parse(triggeredBy.String)
	}
	invocation.CreatedAt = createdAt.Time
	if startedAt.Valid {
		invocation.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		invocation.CompletedAt = &completedAt.Time
	}
	return invocation, nil
}

// FindByThreadID 根据ThreadID查找
func (r *AgentInvocationRepository) FindByThreadID(ctx context.Context, threadID uuid.UUID) ([]*model.AgentInvocation, error) {
	query := `
		SELECT id, thread_id, agent_config_id, role, agent_name, status, input, full_prompt, output, started_at, completed_at, created_at, session_id,
		       input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, cost_usd, duration_ms, duration_api_ms, callback_token, triggered_by
		FROM agent_invocations WHERE thread_id = ? ORDER BY created_at DESC
	`
	rows, err := r.DB().QueryContext(ctx, query, threadID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to find invocations: %w", err)
	}
	defer rows.Close()

	var invocations = make([]*model.AgentInvocation, 0) // 初始化为空数组，避免 JSON null
	for rows.Next() {
		invocation := &model.AgentInvocation{}
		var idStr, threadIDStr, agentConfigIDStr string
		var agentName, sessionID, fullPrompt, callbackToken, triggeredBy sql.NullString
		var startedAt, completedAt, createdAt SQLiteTimeScanner
		err := rows.Scan(
			&idStr, &threadIDStr, &agentConfigIDStr, &invocation.Role, &agentName, &invocation.Status, &invocation.Input, &fullPrompt, &invocation.Output, &startedAt, &completedAt, &createdAt, &sessionID,
			&invocation.InputTokens, &invocation.OutputTokens, &invocation.CacheReadTokens, &invocation.CacheCreationTokens, &invocation.CostUsd, &invocation.DurationMs, &invocation.DurationApiMs,
			&callbackToken, &triggeredBy,
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
		if fullPrompt.Valid {
			invocation.FullPrompt = fullPrompt.String
		}
		if sessionID.Valid {
			invocation.SessionID = sessionID.String
		}
		if callbackToken.Valid {
			invocation.CallbackToken = callbackToken.String
		}
		if triggeredBy.Valid {
			invocation.TriggeredBy, _ = uuid.Parse(triggeredBy.String)
		}
		invocation.CreatedAt = createdAt.Time
		if startedAt.Valid {
			invocation.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			invocation.CompletedAt = &completedAt.Time
		}
		invocations = append(invocations, invocation)
	}
	return invocations, nil
}

// Update 更新调用记录
func (r *AgentInvocationRepository) Update(ctx context.Context, invocation *model.AgentInvocation) error {
	query := `
		UPDATE agent_invocations
		SET status = ?, full_prompt = ?, output = ?, started_at = ?, completed_at = ?, session_id = ?,
		    input_tokens = ?, output_tokens = ?, cache_read_tokens = ?, cache_creation_tokens = ?, cost_usd = ?, duration_ms = ?, duration_api_ms = ?,
		    callback_token = ?, triggered_by = ?
		WHERE id = ?
	`
	var triggeredBy interface{}
	if invocation.TriggeredBy != uuid.Nil {
		triggeredBy = invocation.TriggeredBy.String()
	}
	_, err := r.DB().ExecContext(ctx, query,
		invocation.Status, invocation.FullPrompt, invocation.Output, invocation.StartedAt, invocation.CompletedAt, invocation.SessionID,
		invocation.InputTokens, invocation.OutputTokens, invocation.CacheReadTokens, invocation.CacheCreationTokens, invocation.CostUsd, invocation.DurationMs, invocation.DurationApiMs,
		invocation.CallbackToken, triggeredBy,
		invocation.ID.String(),
	)
	return err
}

// Delete 删除调用记录
func (r *AgentInvocationRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM agent_invocations WHERE id = ?`
	_, err := r.DB().ExecContext(ctx, query, id.String())
	return err
}

// FindByStatus 根据状态查找调用记录（用于启动恢复）
func (r *AgentInvocationRepository) FindByStatus(ctx context.Context, status model.InvocationStatus) ([]*model.AgentInvocation, error) {
	query := `
		SELECT id, thread_id, agent_config_id, role, agent_name, status, input, full_prompt, output, started_at, completed_at, created_at, process_id, session_id,
		       input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, cost_usd, duration_ms, duration_api_ms, callback_token, triggered_by
		FROM agent_invocations WHERE status = ? ORDER BY created_at DESC
	`
	rows, err := r.DB().QueryContext(ctx, query, string(status))
	if err != nil {
		return nil, fmt.Errorf("failed to find invocations by status: %w", err)
	}
	defer rows.Close()

	var invocations []*model.AgentInvocation
	for rows.Next() {
		invocation := &model.AgentInvocation{}
		var idStr, threadIDStr, agentConfigIDStr string
		var agentName, processID, sessionID, fullPrompt, callbackToken, triggeredBy sql.NullString
		var startedAt, completedAt, createdAt SQLiteTimeScanner
		err := rows.Scan(
			&idStr, &threadIDStr, &agentConfigIDStr, &invocation.Role, &agentName, &invocation.Status, &invocation.Input, &fullPrompt, &invocation.Output, &startedAt, &completedAt, &createdAt, &processID, &sessionID,
			&invocation.InputTokens, &invocation.OutputTokens, &invocation.CacheReadTokens, &invocation.CacheCreationTokens, &invocation.CostUsd, &invocation.DurationMs, &invocation.DurationApiMs,
			&callbackToken, &triggeredBy,
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
		if fullPrompt.Valid {
			invocation.FullPrompt = fullPrompt.String
		}
		if sessionID.Valid {
			invocation.SessionID = sessionID.String
		}
		if callbackToken.Valid {
			invocation.CallbackToken = callbackToken.String
		}
		if triggeredBy.Valid {
			invocation.TriggeredBy, _ = uuid.Parse(triggeredBy.String)
		}
		invocation.CreatedAt = createdAt.Time
		if startedAt.Valid {
			invocation.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			invocation.CompletedAt = &completedAt.Time
		}
		invocations = append(invocations, invocation)
	}
	return invocations, nil
}

// FindRecentlyCompletedByThread 查找最近完成的 invocation（用于 WebSocket 重连状态同步）
func (r *AgentInvocationRepository) FindRecentlyCompletedByThread(ctx context.Context, threadID uuid.UUID, sinceMinutes int) ([]*model.AgentInvocation, error) {
	// 使用 Go 计算截止时间，避免数据库特定函数
	cutoffTime := time.Now().Add(-time.Duration(sinceMinutes) * time.Minute)
	query := `
		SELECT id, thread_id, agent_config_id, role, agent_name, status, input, full_prompt, output, started_at, completed_at, created_at, session_id,
		       input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, cost_usd, duration_ms, duration_api_ms, callback_token, triggered_by
		FROM agent_invocations
		WHERE thread_id = ?
			AND status IN ('completed', 'failed', 'interrupted', 'cancelled')
			AND completed_at >= ?
		ORDER BY completed_at DESC
	`
	rows, err := r.DB().QueryContext(ctx, query, threadID.String(), cutoffTime)
	if err != nil {
		return nil, fmt.Errorf("failed to find recently completed invocations: %w", err)
	}
	defer rows.Close()

	var invocations []*model.AgentInvocation
	for rows.Next() {
		invocation := &model.AgentInvocation{}
		var idStr, threadIDStr, agentConfigIDStr string
		var agentName, sessionID, fullPrompt, callbackToken, triggeredBy sql.NullString
		var startedAt, completedAt, createdAt SQLiteTimeScanner
		err := rows.Scan(
			&idStr, &threadIDStr, &agentConfigIDStr, &invocation.Role, &agentName, &invocation.Status, &invocation.Input, &fullPrompt, &invocation.Output, &startedAt, &completedAt, &createdAt, &sessionID,
			&invocation.InputTokens, &invocation.OutputTokens, &invocation.CacheReadTokens, &invocation.CacheCreationTokens, &invocation.CostUsd, &invocation.DurationMs, &invocation.DurationApiMs,
			&callbackToken, &triggeredBy,
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
		if fullPrompt.Valid {
			invocation.FullPrompt = fullPrompt.String
		}
		if sessionID.Valid {
			invocation.SessionID = sessionID.String
		}
		if callbackToken.Valid {
			invocation.CallbackToken = callbackToken.String
		}
		if triggeredBy.Valid {
			invocation.TriggeredBy, _ = uuid.Parse(triggeredBy.String)
		}
		invocation.CreatedAt = createdAt.Time
		if startedAt.Valid {
			invocation.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			invocation.CompletedAt = &completedAt.Time
		}
		invocations = append(invocations, invocation)
	}
	return invocations, nil
}
