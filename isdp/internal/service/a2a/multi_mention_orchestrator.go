package a2a

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// MultiMentionOrchestrator 多讨论编排器
// 管理多 Agent 并行讨论的状态机和响应聚合
type MultiMentionOrchestrator struct {
	repo repo.MultiMentionRepository

	// activeTargets 记录当前活跃的被召唤 Agent
	// key: threadID, value: agentID 列表
	// 用于防止级联调用
	activeTargets sync.Map // map[uuid.UUID][]string

	// timeoutTimers 超时定时器
	timeoutTimers sync.Map // map[uuid.UUID]*time.Timer

	// callbacks 聚合完成回调
	callbacks sync.Map // map[uuid.UUID]func(*model.AggregatedMultiMentionResult)
}

// NewMultiMentionOrchestrator 创建编排器
func NewMultiMentionOrchestrator(repo repo.MultiMentionRepository) *MultiMentionOrchestrator {
	return &MultiMentionOrchestrator{
		repo: repo,
	}
}

// CreateParams 创建请求参数
type CreateParams struct {
	ThreadID       uuid.UUID
	Initiator      string
	CallbackTo     string
	Targets        []string // 1-3 个
	Question       string
	Context        string
	TimeoutMinutes int // 默认 8
	SearchEvidence []string
	OverrideReason string
}

// CreateResult 创建结果
type CreateResult struct {
	RequestID     uuid.UUID
	Status        model.MultiMentionStatus
	CallbackToken string
}

var (
	ErrTargetsEmpty             = errors.New("targets cannot be empty")
	ErrTargetsExceed            = errors.New("targets cannot exceed 3")
	ErrTargetsInvalid           = errors.New("target not in available agents")
	ErrCallbackToInvalid        = errors.New("callbackTo not in available agents")
	ErrMissingSearchEvidence    = errors.New("searchEvidenceRefs or overrideReason is required (先搜后问原则)")
	ErrCascadeBlocked           = errors.New("caller is an active multi-mention target, cascade blocked")
	ErrRequestNotFound           = errors.New("request not found")
	ErrInvalidStatusTransition  = errors.New("invalid status transition")
)

// Create 创建多讨论请求
func (o *MultiMentionOrchestrator) Create(ctx context.Context, params CreateParams, availableAgents []string) (*CreateResult, error) {
	// 1. 参数校验：targets 数量
	if len(params.Targets) == 0 {
		return nil, ErrTargetsEmpty
	}
	if len(params.Targets) > 3 {
		return nil, ErrTargetsExceed
	}

	// 2. 参数校验：targets 必须在 availableAgents 范围内
	agentSet := make(map[string]bool)
	for _, a := range availableAgents {
		agentSet[a] = true
	}
	for _, t := range params.Targets {
		if !agentSet[t] {
			return nil, ErrTargetsInvalid
		}
	}

	// 3. 参数校验：callbackTo 必须在 availableAgents 范围内
	if !agentSet[params.CallbackTo] {
		return nil, ErrCallbackToInvalid
	}

	// 4. 参数校验：先搜后问原则
	if len(params.SearchEvidence) == 0 && params.OverrideReason == "" {
		return nil, ErrMissingSearchEvidence
	}

	// 5. 级联防护检查
	if o.IsActiveTarget(params.ThreadID, params.Initiator) {
		return nil, ErrCascadeBlocked
	}

	// 6. 去重 targets
	uniqueTargets := make([]string, 0, len(params.Targets))
	seen := make(map[string]bool)
	for _, t := range params.Targets {
		if !seen[t] {
			seen[t] = true
			uniqueTargets = append(uniqueTargets, t)
		}
	}

	// 7. 设置默认超时
	timeoutMinutes := params.TimeoutMinutes
	if timeoutMinutes <= 0 {
		timeoutMinutes = 8
	}

	// 8. 创建请求
	now := time.Now()
	req := &model.MultiMentionRequest{
		ID:             uuid.New(),
		ThreadID:       params.ThreadID,
		Initiator:      params.Initiator,
		CallbackTo:     params.CallbackTo,
		Targets:        uniqueTargets,
		Question:       params.Question,
		Context:        params.Context,
		Status:         model.MultiMentionStatusPending,
		TimeoutMinutes: timeoutMinutes,
		SearchEvidence: params.SearchEvidence,
		OverrideReason: params.OverrideReason,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := o.repo.CreateRequest(ctx, req); err != nil {
		return nil, err
	}

	// 9. 记录 activeTargets（待 start 后激活）
	// 注意：创建时还没启动，不记录到 activeTargets

	return &CreateResult{
		RequestID:     req.ID,
		Status:        req.Status,
		CallbackToken: uuid.New().String(), // 生成回调令牌
	}, nil
}

// Start 启动请求执行
func (o *MultiMentionOrchestrator) Start(ctx context.Context, requestID uuid.UUID) error {
	req, err := o.repo.GetRequestByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req == nil {
		return ErrRequestNotFound
	}

	// 状态检查：只能从 pending 启动
	if req.Status != model.MultiMentionStatusPending {
		return ErrInvalidStatusTransition
	}

	// 更新状态为 running
	if err := o.repo.UpdateRequestStatus(ctx, requestID, model.MultiMentionStatusRunning); err != nil {
		return err
	}

	// 记录 activeTargets（级联防护）
	o.SetActiveTargets(req.ThreadID, req.Targets)

	// 启动超时定时器
	go o.scheduleTimeout(context.Background(), requestID, req.TimeoutMinutes)

	return nil
}

// RecordResponse 记录 Agent 响应
// 返回新的状态
func (o *MultiMentionOrchestrator) RecordResponse(ctx context.Context, requestID uuid.UUID, agentID string, content string) (model.MultiMentionStatus, error) {
	req, err := o.repo.GetRequestByID(ctx, requestID)
	if err != nil {
		return "", err
	}
	if req == nil {
		return "", ErrRequestNotFound
	}

	// 状态检查：只能在 running 状态记录响应
	if req.Status != model.MultiMentionStatusRunning {
		return "", ErrInvalidStatusTransition
	}

	// 检查该 Agent 是否已响应（防止重复）
	hasResponded, err := o.repo.HasAgentResponded(ctx, requestID, agentID)
	if err != nil {
		return "", err
	}
	if hasResponded {
		// 已响应，忽略
		return req.Status, nil
	}

	// 创建响应记录
	resp := &model.MultiMentionResponse{
		ID:        uuid.New(),
		RequestID: requestID,
		AgentID:   agentID,
		Content:   content,
		CreatedAt: time.Now(),
	}
	if err := o.repo.CreateResponse(ctx, resp); err != nil {
		return "", err
	}

	// 检查是否所有目标都已响应
	responseCount, err := o.repo.CountResponsesByRequestID(ctx, requestID)
	if err != nil {
		return "", err
	}

	newStatus := req.Status
	if responseCount >= len(req.Targets) {
		// 全部响应完成
		newStatus = model.MultiMentionStatusDone
	} else {
		// 部分响应
		newStatus = model.MultiMentionStatusPartial
	}

	// 更新状态
	if newStatus != req.Status {
		if err := o.repo.UpdateRequestStatus(ctx, requestID, newStatus); err != nil {
			return "", err
		}
	}

	// 如果完成，触发回调
	if newStatus == model.MultiMentionStatusDone {
		o.onRequestComplete(ctx, requestID)
	}

	return newStatus, nil
}

// HandleTimeout 处理超时
func (o *MultiMentionOrchestrator) HandleTimeout(ctx context.Context, requestID uuid.UUID) error {
	req, err := o.repo.GetRequestByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req == nil {
		return ErrRequestNotFound
	}

	// 状态检查：只能在 running/partial 状态处理超时
	if req.Status != model.MultiMentionStatusRunning && req.Status != model.MultiMentionStatusPartial {
		return nil // 已经完成，忽略超时
	}

	// 更新状态为 timeout
	if err := o.repo.UpdateRequestStatus(ctx, requestID, model.MultiMentionStatusTimeout); err != nil {
		return err
	}

	// 触发回调（即使超时也要聚合已收到的响应）
	o.onRequestComplete(ctx, requestID)

	return nil
}

// GetResult 获取聚合结果
func (o *MultiMentionOrchestrator) GetResult(ctx context.Context, requestID uuid.UUID) (*model.AggregatedMultiMentionResult, error) {
	req, err := o.repo.GetRequestByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, ErrRequestNotFound
	}

	responses, err := o.repo.GetResponsesByRequestID(ctx, requestID)
	if err != nil {
		return nil, err
	}

	result := &model.AggregatedMultiMentionResult{
		RequestID: requestID,
		Status:    req.Status,
		Responses: responses,
		Timeout:   req.Status == model.MultiMentionStatusTimeout,
	}

	return result, nil
}

// IsActiveTarget 检查 Agent 是否是某个活跃 multi_mention 的目标
// 用于级联防护
func (o *MultiMentionOrchestrator) IsActiveTarget(threadID uuid.UUID, agentID string) bool {
	if targets, ok := o.activeTargets.Load(threadID); ok {
		for _, t := range targets.([]string) {
			if t == agentID {
				return true
			}
		}
	}
	return false
}

// SetActiveTargets 设置活跃目标（内部方法）
func (o *MultiMentionOrchestrator) SetActiveTargets(threadID uuid.UUID, targets []string) {
	o.activeTargets.Store(threadID, targets)
}

// ClearActiveTargets 清除活跃目标（请求完成时调用）
func (o *MultiMentionOrchestrator) ClearActiveTargets(threadID uuid.UUID) {
	o.activeTargets.Delete(threadID)
}

// RegisterCallback 注册聚合完成回调
func (o *MultiMentionOrchestrator) RegisterCallback(requestID uuid.UUID, callback func(*model.AggregatedMultiMentionResult)) {
	o.callbacks.Store(requestID, callback)
}

// scheduleTimeout 调度超时
func (o *MultiMentionOrchestrator) scheduleTimeout(ctx context.Context, requestID uuid.UUID, timeoutMinutes int) {
	timer := time.NewTimer(time.Duration(timeoutMinutes) * time.Minute)
	o.timeoutTimers.Store(requestID, timer)

	select {
	case <-timer.C:
		// 超时触发
		o.HandleTimeout(ctx, requestID)
	case <-ctx.Done():
		// 上下文取消
		timer.Stop()
	}

	o.timeoutTimers.Delete(requestID)
}

// cancelTimeout 取消超时定时器
func (o *MultiMentionOrchestrator) cancelTimeout(requestID uuid.UUID) {
	if timer, ok := o.timeoutTimers.Load(requestID); ok {
		timer.(*time.Timer).Stop()
		o.timeoutTimers.Delete(requestID)
	}
}

// onRequestComplete 请求完成处理
func (o *MultiMentionOrchestrator) onRequestComplete(ctx context.Context, requestID uuid.UUID) {
	// 取消超时定时器
	o.cancelTimeout(requestID)

	// 获取请求信息
	req, err := o.repo.GetRequestByID(ctx, requestID)
	if err != nil || req == nil {
		return
	}

	// 清除 activeTargets
	o.ClearActiveTargets(req.ThreadID)

	// 触发回调
	if callback, ok := o.callbacks.Load(requestID); ok {
		result, err := o.GetResult(ctx, requestID)
		if err == nil {
			callback.(func(*model.AggregatedMultiMentionResult))(result)
		}
		o.callbacks.Delete(requestID)
	}
}

// MarkFailed 标记请求失败
func (o *MultiMentionOrchestrator) MarkFailed(ctx context.Context, requestID uuid.UUID) error {
	req, err := o.repo.GetRequestByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req == nil {
		return ErrRequestNotFound
	}

	// 更新状态为 failed
	if err := o.repo.UpdateRequestStatus(ctx, requestID, model.MultiMentionStatusFailed); err != nil {
		return err
	}

	// 清理
	o.cancelTimeout(requestID)
	o.ClearActiveTargets(req.ThreadID)

	return nil
}