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
	"go.uber.org/zap"
)

// ParsedMention @mention 解析结果
type ParsedMention struct {
	Role      model.AgentRole // 角色类型（可能为空）
	AgentName string          // Agent 实例名称（可能为空）
	Raw       string          // 原始 mention 文本
}

// Orchestrator Agent编排器
type Orchestrator struct {
	invocationRepo   *repo.AgentInvocationRepository
	threadRepo       *repo.ThreadRepository
	msgRepo          *repo.MessageRepository
	configSvc        *ConfigService
	baseAgentSvc     *BaseAgentService
	baseAgentRepo    *repo.BaseAgentRepository // 直接访问repo获取完整BaseAgent
	tracker          *InvocationTracker
	workflow         *WorkflowEngine
	workflowRepo     *repo.WorkflowTemplateRepository // 新增：工作流模板仓库
	projectRepo      *repo.ProjectRepository          // 新增：项目仓库，用于获取项目路径
	wsHub            *ws.Hub
	defaultAdapter   AgentAdapter     // 默认适配器，用于向后兼容
	executionService *ExecutionService // 统一执行服务
	debugThreadMgr   *DebugThreadManager // 调试线程管理器

	runningAgents      map[uuid.UUID]*RunningAgent
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
	baseAgentRepo *repo.BaseAgentRepository,
	tracker *InvocationTracker,
	workflow *WorkflowEngine,
	workflowRepo *repo.WorkflowTemplateRepository,
	projectRepo *repo.ProjectRepository,
	wsHub *ws.Hub,
	defaultAdapter AgentAdapter,
) *Orchestrator {
	o := &Orchestrator{
		invocationRepo: invocationRepo,
		threadRepo:     threadRepo,
		msgRepo:        msgRepo,
		configSvc:      configSvc,
		baseAgentSvc:   baseAgentSvc,
		baseAgentRepo:  baseAgentRepo,
		tracker:        tracker,
		workflow:       workflow,
		workflowRepo:   workflowRepo,
		projectRepo:    projectRepo,
		wsHub:          wsHub,
		defaultAdapter: defaultAdapter,
		runningAgents:  make(map[uuid.UUID]*RunningAgent),
	}

	// 创建统一的执行服务用于工作流场景
	o.executionService = NewExecutionService(
		invocationRepo,
		threadRepo,
		msgRepo,
		configSvc,
		baseAgentSvc,
		baseAgentRepo,
		tracker,
		workflow,
		workflowRepo,
		projectRepo,
		wsHub,
		defaultAdapter,
	)

	return o
}

// SpawnAgent 启动Agent
func (o *Orchestrator) SpawnAgent(ctx context.Context, req *SpawnRequest) (*model.AgentInvocation, error) {
	// 委托给执行服务
	return o.executionService.SpawnAgent(ctx, req)
}

// handleAgentError 处理Agent错误
func (o *Orchestrator) handleAgentError(ctx context.Context, invocation *model.AgentInvocation, err error) {
	invocation.Status = model.InvocationStatusFailed
	invocation.Output = err.Error()
	invocation.CompletedAt = timePtr(time.Now())
	o.invocationRepo.Update(ctx, invocation)

	o.broadcastStatus(invocation.ThreadID, invocation.ID, "failed", invocation.Role)
}

// mergeConfig 合并 AgentRoleConfig 和 BaseAgent 的配置
func (o *Orchestrator) mergeConfig(config *model.AgentRoleConfig, baseAgent *model.BaseAgent) *model.AgentRoleConfig {
	// 复制原始配置
	merged := *config

	if baseAgent == nil {
		return &merged
	}

	// 注意：模型名称现在从 BaseAgent.DefaultModel 获取，不再存储在 AgentRoleConfig 中

	// 如果没有指定 MaxTokens，使用 BaseAgent 的配置
	if merged.MaxTokens == 0 && baseAgent.MaxTokens > 0 {
		merged.MaxTokens = baseAgent.MaxTokens
	}

	return &merged
}

// getAdapter 获取适配器
func (o *Orchestrator) getAdapter(ctx context.Context, config *model.AgentRoleConfig, baseAgent *model.BaseAgent) (AgentAdapter, error) {
	// 委托给ExecutionService的实现
	return o.executionService.getAdapter(ctx, config, baseAgent)
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


// CancelAgent 取消Agent
func (o *Orchestrator) CancelAgent(ctx context.Context, invocationID uuid.UUID) error {
	return o.executionService.CancelAgent(ctx, invocationID)
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
func (o *Orchestrator) broadcastOutputChunk(threadID, invocationID uuid.UUID, chunk string, agentID, agentName string) {
	if o.wsHub != nil {
		o.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_output_chunk",
			ThreadID:  threadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload: map[string]interface{}{
				"invocationId": invocationID.String(),
				"chunk":        chunk,
				"agentId":      agentID,
				"agentName":    agentName,
			},
		})
	}
}

// broadcastAgentMessage 广播Agent消息（实时显示）
func (o *Orchestrator) broadcastAgentMessage(threadID uuid.UUID, msg *model.Message, agentName, agentRole string) {
	if o.wsHub != nil {
		o.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_message",
			ThreadID:  threadID.String(),
			Timestamp: msg.CreatedAt.UnixMilli(),
			Payload: map[string]interface{}{
				"messageId":   msg.ID.String(),
				"agentId":     msg.AgentID,
				"content":     msg.Content,
				"agentName":   agentName,
				"agentRole":   agentRole,
			},
		})
	}
}

// GetInvocationsByThread 获取 Thread 的所有 Agent 调用
func (o *Orchestrator) GetInvocationsByThread(ctx context.Context, threadID uuid.UUID) ([]model.AgentInvocation, error) {
	return o.executionService.GetInvocationsByThread(ctx, threadID)
}

// GetInvocationStatus 获取单个调用的状态
func (o *Orchestrator) GetInvocationStatus(ctx context.Context, invocationID uuid.UUID) (*model.AgentInvocation, error) {
	return o.executionService.GetInvocationStatus(ctx, invocationID)
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
// SpawnAgentForUserMessage 为用户消息触发Agent响应
// 实现message.AgentSpawner接口
// 使用工作流模板中指定的Agent，而不是根据Phase硬编码选择
func (o *Orchestrator) SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string) error {
	// 委托给执行服务
	return o.executionService.SpawnAgentForUserMessage(ctx, threadID, userMessage)
}

// SetDebugThreadManager 设置调试线程管理器
func (o *Orchestrator) SetDebugThreadManager(mgr *DebugThreadManager) {
	o.debugThreadMgr = mgr
}

// SpawnDebugAgent 调试模式启动Agent
func (o *Orchestrator) SpawnDebugAgent(ctx context.Context, req *SpawnRequest) (*model.AgentInvocation, error) {
	if o.debugThreadMgr == nil {
		return nil, fmt.Errorf("debug thread manager not initialized")
	}

	// 验证调试线程存在
	debugThread := o.debugThreadMgr.GetThread(req.ThreadID)
	if debugThread == nil {
		return nil, fmt.Errorf("debug thread not found: %s", req.ThreadID)
	}

	// 原子地将状态从 idle 或 completed 转换为 running
	if !o.debugThreadMgr.TryStartExecution(req.ThreadID) {
		return nil, fmt.Errorf("agent is busy, current status: %s", debugThread.Status)
	}

	// 获取Agent配置
	config, err := o.configSvc.GetByID(ctx, req.ConfigID)
	if err != nil {
		o.debugThreadMgr.SetStatus(req.ThreadID, DebugThreadStatusIdle)
		return nil, fmt.Errorf("agent config not found: %w", err)
	}

	// 获取基础Agent（直接从repo获取，包含ApiToken）
	baseAgent, err := o.baseAgentRepo.FindByID(ctx, config.BaseAgentID)
	if err != nil {
		o.debugThreadMgr.SetStatus(req.ThreadID, DebugThreadStatusIdle)
		return nil, fmt.Errorf("base agent not found: %w", err)
	}
	logInfo("SpawnDebugAgent: got baseAgent", zap.String("id", baseAgent.ID.String()), zap.String("name", baseAgent.Name), zap.Bool("hasApiToken", baseAgent.ApiToken != ""), zap.String("apiUrl", baseAgent.ApiURL))

	// 创建适配器
	adapter := NewAdapter(baseAgent)
	if adapter == nil {
		o.debugThreadMgr.SetStatus(req.ThreadID, DebugThreadStatusIdle)
		return nil, fmt.Errorf("unsupported agent type: %s", baseAgent.Type)
	}

	// 添加用户消息到内存
	userMsg := &model.Message{
		ID:        uuid.New(),
		ThreadID:  req.ThreadID,
		Role:      model.MessageRoleUser,
		Content:   req.Input,
		CreatedAt: time.Now(),
	}
	o.debugThreadMgr.AddMessage(req.ThreadID, userMsg)

	// 创建调用记录（内存中，不写数据库）
	invocation := &model.AgentInvocation{
		ID:            uuid.New(),
		ThreadID:      req.ThreadID,
		AgentConfigID: req.ConfigID,
		Role:          req.Role,
		Status:        model.InvocationStatusRunning,
		Input:         req.Input,
		StartedAt:     timePtr(time.Now()),
	}

	// 启动goroutine执行Agent
	go o.executeDebugAgent(req.ThreadID, invocation, adapter, config, baseAgent, req)

	return invocation, nil
}

// executeDebugAgent 执行调试Agent（异步）
func (o *Orchestrator) executeDebugAgent(
	threadID uuid.UUID,
	invocation *model.AgentInvocation,
	adapter AgentAdapter,
	config *model.AgentRoleConfig,
	baseAgent *model.BaseAgent,
	req *SpawnRequest,
) {
	ctx := context.Background()
	invocationID := invocation.ID.String()

	// 构建执行上下文
	execReq := &ExecutionRequest{
		Input:     req.Input,
		WorkDir:   req.ProjectPath,
		Config:    config,
		BaseAgent: baseAgent,
		ConfigDir: config.ConfigPath, // 使用生成的配置目录
		Context: &ContextLayers{
			Layer0: config.SystemPrompt,
		},
	}

	// 创建输出收集器
	var outputBuilder strings.Builder
	agentID := config.ID.String()
	agentName := config.Name
	agentRole := string(config.Role)

	// 执行Agent并收集输出
	chunkCount := 0
	result, err := adapter.ExecuteWithStream(ctx, execReq, func(chunk Chunk) {
		outputBuilder.WriteString(chunk.Content)
		chunkCount++
		// 广播流式输出
		logInfo("executeDebugAgent: calling BroadcastChunk", zap.Int("chunkNum", chunkCount), zap.String("type", string(chunk.Type)), zap.Int("contentLen", len(chunk.Content)))
		o.debugThreadMgr.BroadcastChunk(threadID.String(), invocationID, agentID, agentName, chunk.Content)
	})

	logInfo("executeDebugAgent: stream complete", zap.Int("chunkCount", chunkCount), zap.Error(err))

	if err != nil {
		o.debugThreadMgr.SetStatus(threadID, DebugThreadStatusError)
		o.debugThreadMgr.BroadcastError(threadID.String(), fmt.Sprintf("Agent执行失败: %v", err))
		return
	}

	// 使用 result.Output 如果有内容
	output := result.Output
	if output == "" {
		output = outputBuilder.String()
	}

	// 添加Agent消息到内存
	agentMsg := &model.Message{
		ID:        uuid.New(),
		ThreadID:  threadID,
		Role:      model.MessageRoleAgent,
		AgentID:   config.ID.String(),
		Content:   output,
		CreatedAt: time.Now(),
	}
	o.debugThreadMgr.AddMessage(threadID, agentMsg)

	// 广播完整消息
	o.debugThreadMgr.BroadcastMessage(threadID.String(), agentMsg.ID.String(), agentID, agentName, agentRole, agentMsg.Content)

	// 更新线程状态为完成
	o.debugThreadMgr.SetStatus(threadID, DebugThreadStatusCompleted)
}

// ContinueDebugAgent 继续调试会话
func (o *Orchestrator) ContinueDebugAgent(ctx context.Context, threadID uuid.UUID, message string) error {
	if o.debugThreadMgr == nil {
		return fmt.Errorf("debug thread manager not initialized")
	}

	// 验证调试线程存在
	debugThread := o.debugThreadMgr.GetThread(threadID)
	if debugThread == nil {
		return fmt.Errorf("debug thread not found: %s", threadID)
	}

	// 获取最后一条Agent消息确定配置
	var lastConfigID uuid.UUID
	for i := len(debugThread.Messages) - 1; i >= 0; i-- {
		if debugThread.Messages[i].Role == model.MessageRoleAgent && debugThread.Messages[i].AgentID != "" {
			lastConfigID, _ = uuid.Parse(debugThread.Messages[i].AgentID)
			break
		}
	}

	if lastConfigID == uuid.Nil {
		return fmt.Errorf("no previous agent context found")
	}

	// 获取Agent配置以获取Role
	config, err := o.configSvc.GetByID(ctx, lastConfigID)
	if err != nil {
		return fmt.Errorf("agent config not found: %w", err)
	}

	// 获取存储的项目路径
	projectPath := o.debugThreadMgr.GetProjectPath(threadID)

	// 使用相同的配置继续执行
	req := &SpawnRequest{
		ThreadID:    threadID,
		ConfigID:    lastConfigID,
		Role:        config.Role,
		Input:       message,
		ProjectPath: projectPath,
	}

	_, err = o.SpawnDebugAgent(ctx, req)
	return err
}
