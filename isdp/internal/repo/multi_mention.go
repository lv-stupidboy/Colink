package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// MultiMentionRepository 多讨论请求数据访问
type MultiMentionRepository struct {
	db *sql.DB
}

// NewMultiMentionRepository 创建 MultiMention Repository
func NewMultiMentionRepository(db *sql.DB) *MultiMentionRepository {
	return &MultiMentionRepository{db: db}
}

// CreateRequest 创建多讨论请求
func (r *MultiMentionRepository) CreateRequest(ctx context.Context, req *model.MultiMentionRequest) error {
	query := `
		INSERT INTO multi_mention_requests
		(id, thread_id, initiator, callback_to, targets, question, context, status, timeout_minutes, search_evidence, override_reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()

	// 序列化 JSON 字段
	targetsJSON, _ := json.Marshal(req.Targets)
	searchEvidenceJSON, _ := json.Marshal(req.SearchEvidence)

	_, err := r.db.ExecContext(ctx, query,
		req.ID.String(),
		req.ThreadID.String(),
		req.Initiator,
		req.CallbackTo,
		targetsJSON,
		req.Question,
		req.Context,
		req.Status,
		req.TimeoutMinutes,
		searchEvidenceJSON,
		req.OverrideReason,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create multi_mention_request: %w", err)
	}
	req.CreatedAt = now
	req.UpdatedAt = now
	return nil
}

// GetRequestByID 根据ID获取请求
func (r *MultiMentionRepository) GetRequestByID(ctx context.Context, id uuid.UUID) (*model.MultiMentionRequest, error) {
	query := `
		SELECT id, thread_id, initiator, callback_to, targets, question, context, status, timeout_minutes,
		       search_evidence, override_reason, created_at, updated_at
		FROM multi_mention_requests WHERE id = ?
	`
	req := &model.MultiMentionRequest{}
	var idStr, threadIDStr string
	var targetsJSON, searchEvidenceJSON []byte

	err := r.db.QueryRowContext(ctx, query, id.String()).Scan(
		&idStr,
		&threadIDStr,
		&req.Initiator,
		&req.CallbackTo,
		&targetsJSON,
		&req.Question,
		&req.Context,
		&req.Status,
		&req.TimeoutMinutes,
		&searchEvidenceJSON,
		&req.OverrideReason,
		&req.CreatedAt,
		&req.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get multi_mention_request: %w", err)
	}

	req.ID, _ = uuid.Parse(idStr)
	req.ThreadID, _ = uuid.Parse(threadIDStr)

	// 反序列化 JSON 字段
	if len(targetsJSON) > 0 {
		json.Unmarshal(targetsJSON, &req.Targets)
	}
	if len(searchEvidenceJSON) > 0 {
		json.Unmarshal(searchEvidenceJSON, &req.SearchEvidence)
	}

	return req, nil
}

// GetRequestsByThreadID 获取会话的所有请求
func (r *MultiMentionRepository) GetRequestsByThreadID(ctx context.Context, threadID uuid.UUID) ([]*model.MultiMentionRequest, error) {
	query := `
		SELECT id, thread_id, initiator, callback_to, targets, question, context, status, timeout_minutes,
		       search_evidence, override_reason, created_at, updated_at
		FROM multi_mention_requests WHERE thread_id = ? ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, threadID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get multi_mention_requests: %w", err)
	}
	defer rows.Close()

	var requests []*model.MultiMentionRequest
	for rows.Next() {
		req := &model.MultiMentionRequest{}
		var idStr, threadIDStr string
		var targetsJSON, searchEvidenceJSON []byte

		err := rows.Scan(
			&idStr,
			&threadIDStr,
			&req.Initiator,
			&req.CallbackTo,
			&targetsJSON,
			&req.Question,
			&req.Context,
			&req.Status,
			&req.TimeoutMinutes,
			&searchEvidenceJSON,
			&req.OverrideReason,
			&req.CreatedAt,
			&req.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan multi_mention_request: %w", err)
		}

		req.ID, _ = uuid.Parse(idStr)
		req.ThreadID, _ = uuid.Parse(threadIDStr)

		// 反序列化 JSON 字段
		if len(targetsJSON) > 0 {
			json.Unmarshal(targetsJSON, &req.Targets)
		}
		if len(searchEvidenceJSON) > 0 {
			json.Unmarshal(searchEvidenceJSON, &req.SearchEvidence)
		}

		requests = append(requests, req)
	}

	return requests, nil
}

// GetActiveRequestsByThread 获取会话中正在执行的请求
func (r *MultiMentionRepository) GetActiveRequestsByThread(ctx context.Context, threadID uuid.UUID) ([]*model.MultiMentionRequest, error) {
	query := `
		SELECT id, thread_id, initiator, callback_to, targets, question, context, status, timeout_minutes,
		       search_evidence, override_reason, created_at, updated_at
		FROM multi_mention_requests
		WHERE thread_id = ? AND status IN ('pending', 'running')
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, threadID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get active multi_mention_requests: %w", err)
	}
	defer rows.Close()

	var requests []*model.MultiMentionRequest
	for rows.Next() {
		req := &model.MultiMentionRequest{}
		var idStr, threadIDStr string
		var targetsJSON, searchEvidenceJSON []byte

		err := rows.Scan(
			&idStr,
			&threadIDStr,
			&req.Initiator,
			&req.CallbackTo,
			&targetsJSON,
			&req.Question,
			&req.Context,
			&req.Status,
			&req.TimeoutMinutes,
			&searchEvidenceJSON,
			&req.OverrideReason,
			&req.CreatedAt,
			&req.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan multi_mention_request: %w", err)
		}

		req.ID, _ = uuid.Parse(idStr)
		req.ThreadID, _ = uuid.Parse(threadIDStr)

		// 反序列化 JSON 字段
		if len(targetsJSON) > 0 {
			json.Unmarshal(targetsJSON, &req.Targets)
		}
		if len(searchEvidenceJSON) > 0 {
			json.Unmarshal(searchEvidenceJSON, &req.SearchEvidence)
		}

		requests = append(requests, req)
	}

	return requests, nil
}

// UpdateRequestStatus 更新请求状态
func (r *MultiMentionRepository) UpdateRequestStatus(ctx context.Context, id uuid.UUID, status model.MultiMentionStatus) error {
	query := `UPDATE multi_mention_requests SET status = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), id.String())
	if err != nil {
		return fmt.Errorf("failed to update multi_mention_request status: %w", err)
	}
	return nil
}

// CreateResponse 创建响应
func (r *MultiMentionRepository) CreateResponse(ctx context.Context, resp *model.MultiMentionResponse) error {
	query := `
		INSERT INTO multi_mention_responses (id, request_id, agent_id, content, created_at)
		VALUES (?, ?, ?, ?, ?)
	`
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query,
		resp.ID.String(),
		resp.RequestID.String(),
		resp.AgentID,
		resp.Content,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create multi_mention_response: %w", err)
	}
	resp.CreatedAt = now
	return nil
}

// GetResponsesByRequestID 获取请求的所有响应
func (r *MultiMentionRepository) GetResponsesByRequestID(ctx context.Context, requestID uuid.UUID) ([]*model.MultiMentionResponse, error) {
	query := `
		SELECT id, request_id, agent_id, content, created_at
		FROM multi_mention_responses WHERE request_id = ? ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, requestID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get multi_mention_responses: %w", err)
	}
	defer rows.Close()

	var responses []*model.MultiMentionResponse
	for rows.Next() {
		resp := &model.MultiMentionResponse{}
		var idStr, requestIDStr string

		err := rows.Scan(
			&idStr,
			&requestIDStr,
			&resp.AgentID,
			&resp.Content,
			&resp.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan multi_mention_response: %w", err)
		}

		resp.ID, _ = uuid.Parse(idStr)
		resp.RequestID, _ = uuid.Parse(requestIDStr)

		responses = append(responses, resp)
	}

	return responses, nil
}

// CountResponsesByRequestID 统计请求的响应数量
func (r *MultiMentionRepository) CountResponsesByRequestID(ctx context.Context, requestID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM multi_mention_responses WHERE request_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, requestID.String()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count multi_mention_responses: %w", err)
	}
	return count, nil
}

// HasAgentResponded 检查 Agent 是否已响应
func (r *MultiMentionRepository) HasAgentResponded(ctx context.Context, requestID uuid.UUID, agentID string) (bool, error) {
	query := `SELECT COUNT(*) FROM multi_mention_responses WHERE request_id = ? AND agent_id = ?`
	var count int
	err := r.db.QueryRowContext(ctx, query, requestID.String(), agentID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check agent response: %w", err)
	}
	return count > 0, nil
}