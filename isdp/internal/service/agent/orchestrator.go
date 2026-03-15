package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
)

// Orchestrator Agent编排器
type Orchestrator struct {
	invocationRepo  *repo.AgentInvocationRepository
	threadRepo      *repo.ThreadRepository
	msgRepo         *repo.MessageRepository
	configSvc       *ConfigService
	baseAgentSvc    *BaseAgentService
	tracker         *InvocationTracker
	workflow        *WorkflowEngine
	wsHub           *ws.Hub
	defaultAdapter  AgentAdapter // 默认适配器，用于向后兼容

	runningAgents      map[uuid.UUID]*RunningAgent
	interactiveManager *InteractiveSessionManager
	mu                 sync.RWMutex
}

// RunningAgent 运行中的Agent
type RunningAgent struct {
	InvocationID uuid.UUID
	ThreadID     uuid.UUID
	AgentConfig  *model.AgentRoleConfig
	BaseAgent    *model.BaseAgent // 关联的基础Agent配置
	StartedAt    time.Time
	CancelFunc   context.CancelFunc
}

// NewOrchestrator 创建编排器
func NewOrchestrator(
	invocationRepo *repo.AgentInvocationRepository,
	threadRepo *repo.ThreadRepository,
	msgRepo *repo.MessageRepository,
	configSvc *ConfigService,
	baseAgentSvc *BaseAgentService,
	tracker *InvocationTracker,
	workflow *WorkflowEngine,
	wsHub *ws.Hub,
	defaultAdapter AgentAdapter,
) *Orchestrator {
	o := &Orchestrator{
		invocationRepo: invocationRepo,
		threadRepo:     threadRepo,
		msgRepo:        msgRepo,
		configSvc:      configSvc,
		baseAgentSvc:   baseAgentSvc,
		tracker:        tracker,
		workflow:       workflow,
		wsHub:          wsHub,
		defaultAdapter: defaultAdapter,
		runningAgents:  make(map[uuid.UUID]*RunningAgent),
	}
	o.interactiveManager = NewInteractiveSessionManager(wsHub)
	return o
}

// SpawnAgent 启动Agent
func (o *Orchestrator) SpawnAgent(ctx context.Context, req *SpawnRequest) (*model.AgentInvocation, error) {
	// 获取Agent配置
	config, err := o.configSvc.GetByID(ctx, req.ConfigID)
	if err != nil {
		config, err = o.configSvc.GetDefaultByRole(ctx, req.Role)
		if err != nil {
			return nil, fmt.Errorf("failed to get agent config: %w", err)
		}
	}

	// 获取关联的BaseAgent配置
	var baseAgent *model.BaseAgent
	if config.BaseAgentID != uuid.Nil && o.baseAgentSvc != nil {
		baseAgent, err = o.baseAgentSvc.GetByID(ctx, config.BaseAgentID)
		if err != nil {
			// 记录警告但不阻止执行
			baseAgent = nil
		}
	}

	// 创建调用记录
	invocation := &model.AgentInvocation{
		ID:           uuid.New(),
		ThreadID:     req.ThreadID,
		AgentConfigID: config.ID,
		Role:         config.Role,
		Status:       model.InvocationStatusPending,
		Input:        req.Input,
		CreatedAt:    time.Now(),
	}

	if err := o.invocationRepo.Create(ctx, invocation); err != nil {
		return nil, fmt.Errorf("failed to create invocation: %w", err)
	}

	// 创建上下文 - 使用独立的context，不受HTTP请求生命周期影响
	agentCtx, cancel := context.WithCancel(context.Background())

	// 记录运行中的Agent
	o.mu.Lock()
	o.runningAgents[invocation.ID] = &RunningAgent{
		InvocationID: invocation.ID,
		ThreadID:     req.ThreadID,
		AgentConfig:  config,
		BaseAgent:    baseAgent,
		StartedAt:    time.Now(),
		CancelFunc:   cancel,
	}
	o.mu.Unlock()

	// 广播状态更新
	o.broadcastStatus(req.ThreadID, invocation.ID, "started", config.Role)

	// 异步执行Agent
	go o.executeAgent(agentCtx, invocation, config, baseAgent, req)

	return invocation, nil
}

// executeAgent 执行Agent
func (o *Orchestrator) executeAgent(ctx context.Context, invocation *model.AgentInvocation, config *model.AgentRoleConfig, baseAgent *model.BaseAgent, req *SpawnRequest) {
	defer func() {
		// 恢复可能的panic
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in executeAgent: %v", r)
			o.handleAgentError(ctx, invocation, err)
		}
		o.mu.Lock()
		delete(o.runningAgents, invocation.ID)
		o.mu.Unlock()
	}()

	fmt.Printf("[DEBUG] executeAgent started for invocation %s\n", invocation.ID)

	// 构建上下文
	contextLayers, err := o.buildContextLayers(ctx, req.ThreadID, config)
	if err != nil {
		fmt.Printf("[DEBUG] buildContextLayers failed: %v\n", err)
		o.handleAgentError(ctx, invocation, fmt.Errorf("failed to build context layers: %w", err))
		return
	}
	fmt.Printf("[DEBUG] Context layers built\n")

	// 合并配置：AgentRoleConfig 优先，BaseAgent 作为默认值
	mergedConfig := o.mergeConfig(config, baseAgent)
	fmt.Printf("[DEBUG] Config merged, model: %s\n", mergedConfig.ModelName)

	// 获取适配器
	adapter, err := o.getAdapter(ctx, config, baseAgent)
	if err != nil {
		fmt.Printf("[DEBUG] getAdapter failed: %v\n", err)
		o.handleAgentError(ctx, invocation, fmt.Errorf("failed to get adapter: %w", err))
		return
	}
	fmt.Printf("[DEBUG] Adapter obtained: %T\n", adapter)

	// 使用流式执行，实时广播输出
	var outputBuilder strings.Builder
	err = adapter.ExecuteWithStream(ctx, mergedConfig, contextLayers, req.Input, req.ProjectPath, func(chunk string) {
		outputBuilder.WriteString(chunk)
		// 实时广播输出块
		o.broadcastOutputChunk(req.ThreadID, invocation.ID, chunk)
	})

	if err != nil {
		fmt.Printf("[DEBUG] Adapter.ExecuteWithStream failed: %v\n", err)
		o.handleAgentError(ctx, invocation, fmt.Errorf("adapter execution failed: %w", err))
		return
	}

	output := outputBuilder.String()
	fmt.Printf("[DEBUG] Execution completed, output length: %d\n", len(output))

	// 更新调用记录
	invocation.Status = model.InvocationStatusCompleted
	invocation.Output = output
	invocation.CompletedAt = timePtr(time.Now())
	o.invocationRepo.Update(ctx, invocation)

	// 保存输出消息
	o.msgRepo.Create(ctx, &model.Message{
		ThreadID:    req.ThreadID,
		Role:        model.MessageRoleAgent,
		AgentID:     config.ID.String(),
		Content:     output,
		MessageType: model.MessageTypeText,
		CreatedAt:   time.Now(),
	})

	// 广播完成状态
	o.broadcastStatus(req.ThreadID, invocation.ID, "completed", config.Role)

	// 检查是否需要路由到下一个Agent
	o.checkRouting(ctx, req.ThreadID, config, output)
}

// mergeConfig 合并 AgentRoleConfig 和 BaseAgent 的配置
func (o *Orchestrator) mergeConfig(config *model.AgentRoleConfig, baseAgent *model.BaseAgent) *model.AgentRoleConfig {
	// 复制原始配置
	merged := *config

	if baseAgent == nil {
		return &merged
	}

	// 如果 AgentRoleConfig 没有指定模型，使用 BaseAgent 的默认模型
	if merged.ModelName == "" && baseAgent.DefaultModel != "" {
		merged.ModelName = baseAgent.DefaultModel
	}

	// 如果没有指定 MaxTokens，使用 BaseAgent 的配置
	if merged.MaxTokens == 0 && baseAgent.MaxTokens > 0 {
		merged.MaxTokens = baseAgent.MaxTokens
	}

	return &merged
}

// getAdapter 获取适配器
func (o *Orchestrator) getAdapter(ctx context.Context, config *model.AgentRoleConfig, baseAgent *model.BaseAgent) (AgentAdapter, error) {
	// 如果有 BaseAgent，使用它创建适配器
	if baseAgent != nil {
		adapter := NewAdapter(baseAgent)
		if adapter == nil {
			return nil, fmt.Errorf("unsupported base agent type: %s", baseAgent.Type)
		}
		return adapter, nil
	}

	// 如果配置了BaseAgentID但没有传入baseAgent，尝试获取
	if config.BaseAgentID != uuid.Nil && o.baseAgentSvc != nil {
		ba, err := o.baseAgentSvc.GetByID(ctx, config.BaseAgentID)
		if err == nil {
			adapter := NewAdapter(ba)
			if adapter != nil {
				return adapter, nil
			}
		}
		// 如果获取失败，继续尝试使用默认适配器
	}

	// 向后兼容：使用默认适配器
	if o.defaultAdapter != nil {
		return o.defaultAdapter, nil
	}

	return nil, errors.New("no adapter available")
}

// handleAgentError 处理Agent错误
func (o *Orchestrator) handleAgentError(ctx context.Context, invocation *model.AgentInvocation, err error) {
	invocation.Status = model.InvocationStatusFailed
	invocation.Output = err.Error()
	invocation.CompletedAt = timePtr(time.Now())
	o.invocationRepo.Update(ctx, invocation)

	o.broadcastStatus(invocation.ThreadID, invocation.ID, "failed", invocation.Role)
}

// buildContextLayers 构建上下文层
func (o *Orchestrator) buildContextLayers(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig) (*ContextLayers, error) {
	layers := &ContextLayers{}

	// Layer 0: 系统提示
	layers.Layer0 = config.SystemPrompt

	// Layer 1: Thread历史
	messages, err := o.msgRepo.FindByThreadID(ctx, threadID, 100)
	if err != nil {
		return nil, err
	}
	layers.Layer1 = o.formatMessages(messages)

	// Layer 2: 工作产物
	thread, err := o.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return nil, err
	}
	layers.Layer2 = o.getArtifacts(thread)

	// Layer 3: 环境信息
	layers.Layer3 = o.getEnvironmentInfo(thread)

	return layers, nil
}

// checkRouting 检查路由
func (o *Orchestrator) checkRouting(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, output string) {
	// 解析@mention (简单的行首@检测)
	mentions := parseMentions(output)

	if len(mentions) > 0 {
		// 有显式路由
		for _, role := range mentions {
			if role != "" {
				o.SpawnAgent(ctx, &SpawnRequest{
					ThreadID: threadID,
					Role:     role,
					Input:    output,
				})
			}
		}
	} else {
		// 检查信号路由
		for _, signal := range config.RoutingConfig.RouteOnSignal {
			if strings.Contains(output, signal) {
				nextPhase := o.workflow.GetNextPhase(getPhaseFromSignal(signal))
				nextRole := o.workflow.GetPhaseAgent(nextPhase)
				o.SpawnAgent(ctx, &SpawnRequest{
					ThreadID: threadID,
					Role:     nextRole,
					Input:    output,
				})
				break
			}
		}
	}
}

// parseMentions 解析@mention (简化版)
func parseMentions(content string) []model.AgentRole {
	var roles []model.AgentRole
	lines := strings.Split(content, "\n")
	count := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if count >= 2 {
			break
		}
		if strings.HasPrefix(line, "@") {
			roleStr := strings.Fields(line[1:])[0]
			role := parseAgentRole(roleStr)
			if role != "" {
				roles = append(roles, role)
				count++
			}
		}
	}
	return roles
}

// parseAgentRole 解析Agent角色
func parseAgentRole(s string) model.AgentRole {
	switch strings.ToLower(s) {
	case "requirement", "req":
		return model.AgentRoleRequirement
	case "architect", "arch":
		return model.AgentRoleArchitect
	case "developer", "dev":
		return model.AgentRoleDeveloper
	case "reviewer", "review":
		return model.AgentRoleReviewer
	case "testengineer", "test":
		return model.AgentRoleTestEngineer
	case "devops", "ops":
		return model.AgentRoleDevOps
	default:
		return ""
	}
}

// getPhaseFromSignal 从信号获取阶段
func getPhaseFromSignal(signal string) model.Phase {
	switch signal {
	case "需求完成", "requirement_done":
		return model.PhaseRequirement
	case "设计完成", "design_done":
		return model.PhaseDesign
	case "开发完成", "development_done":
		return model.PhaseDevelopment
	case "评审完成", "review_done":
		return model.PhaseReview
	case "测试完成", "test_done":
		return model.PhaseTest
	default:
		return model.PhaseRequirement
	}
}

// formatMessages 格式化消息
func (o *Orchestrator) formatMessages(messages []*model.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		role := "用户"
		if msg.Role == model.MessageRoleAgent {
			role = msg.AgentID
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", role, msg.Content))
	}
	return sb.String()
}

// getArtifacts 获取工作产物
func (o *Orchestrator) getArtifacts(thread *model.Thread) string {
	// TODO: 实现获取工作产物
	return ""
}

// getEnvironmentInfo 获取环境信息
func (o *Orchestrator) getEnvironmentInfo(thread *model.Thread) string {
	return fmt.Sprintf("Thread ID: %s\n当前阶段: %s\n状态: %s",
		thread.ID, thread.CurrentPhase, thread.Status)
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// CancelAgent 取消Agent
func (o *Orchestrator) CancelAgent(ctx context.Context, invocationID uuid.UUID) error {
	o.mu.Lock()
	agent, exists := o.runningAgents[invocationID]
	if exists {
		agent.CancelFunc()
		delete(o.runningAgents, invocationID)
	}
	o.mu.Unlock()

	if !exists {
		return ErrAgentNotFound
	}

	// 更新状态
	invocation, err := o.invocationRepo.FindByID(ctx, invocationID)
	if err != nil {
		return err
	}

	invocation.Status = model.InvocationStatusCancelled
	invocation.CompletedAt = timePtr(time.Now())
	return o.invocationRepo.Update(ctx, invocation)
}

// broadcastStatus 广播状态
func (o *Orchestrator) broadcastStatus(threadID, invocationID uuid.UUID, status string, role model.AgentRole) {
	if o.wsHub != nil {
		o.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_status",
			ThreadID:  threadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload: map[string]interface{}{
				"invocationId": invocationID.String(),
				"status":       status,
				"role":         string(role),
			},
		})
	}
}

// broadcastOutputChunk 广播输出块（实时流式输出）
func (o *Orchestrator) broadcastOutputChunk(threadID, invocationID uuid.UUID, chunk string) {
	if o.wsHub != nil {
		o.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_output_chunk",
			ThreadID:  threadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload: map[string]interface{}{
				"invocationId": invocationID.String(),
				"chunk":        chunk,
			},
		})
	}
}

// GetInvocationsByThread 获取 Thread 的所有 Agent 调用
func (o *Orchestrator) GetInvocationsByThread(ctx context.Context, threadID uuid.UUID) ([]model.AgentInvocation, error) {
	invocations, err := o.invocationRepo.FindByThreadID(ctx, threadID)
	if err != nil {
		return nil, err
	}

	// 转换为 slice 返回
	result := make([]model.AgentInvocation, 0, len(invocations))
	for _, inv := range invocations {
		result = append(result, *inv)
	}
	return result, nil
}

// GetInvocationStatus 获取单个调用的状态
func (o *Orchestrator) GetInvocationStatus(ctx context.Context, invocationID uuid.UUID) (*model.AgentInvocation, error) {
	return o.invocationRepo.FindByID(ctx, invocationID)
}

// StartInteractiveSession 启动交互式会话
func (o *Orchestrator) StartInteractiveSession(ctx context.Context, req *SpawnRequest) (*InteractiveSession, error) {
	// 获取Agent配置
	config, err := o.configSvc.GetByID(ctx, req.ConfigID)
	if err != nil {
		config, err = o.configSvc.GetDefaultByRole(ctx, req.Role)
		if err != nil {
			return nil, fmt.Errorf("failed to get agent config: %w", err)
		}
	}

	// 获取关联的BaseAgent配置
	var baseAgent *model.BaseAgent
	if config.BaseAgentID != uuid.Nil && o.baseAgentSvc != nil {
		baseAgent, err = o.baseAgentSvc.GetByID(ctx, config.BaseAgentID)
		if err != nil {
			baseAgent = nil
		}
	}

	// 启动交互式会话
	session, err := o.interactiveManager.StartSession(ctx, req.ThreadID, config, baseAgent, req.ProjectPath, req.Input)
	if err != nil {
		return nil, err
	}

	// 广播会话启动状态
	o.broadcastStatus(req.ThreadID, session.ID, "started", config.Role)

	return session, nil
}

// SendMessageToSession 向交互式会话发送消息
func (o *Orchestrator) SendMessageToSession(threadID uuid.UUID, message string) error {
	return o.interactiveManager.SendMessageToSession(threadID, message)
}

// StopInteractiveSession 停止交互式会话
func (o *Orchestrator) StopInteractiveSession(threadID uuid.UUID) error {
	return o.interactiveManager.StopSession(threadID)
}

// GetInteractiveSession 获取交互式会话
func (o *Orchestrator) GetInteractiveSession(threadID uuid.UUID) *InteractiveSession {
	return o.interactiveManager.GetSession(threadID)
}

// SpawnRequest 启动请求
type SpawnRequest struct {
	ThreadID    uuid.UUID
	ConfigID    uuid.UUID
	Role        model.AgentRole
	Input       string
	ProjectPath string // 工作目录
}

// ContextLayers 上下文层
type ContextLayers struct {
	Layer0 string // 系统提示
	Layer1 string // Thread历史
	Layer2 string // 工作产物
	Layer3 string // 环境信息
}

var (
	ErrAgentNotFound = errors.New("agent not found")
)