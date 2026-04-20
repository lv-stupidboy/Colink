package agent

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/humantask"
	"github.com/anthropic/isdp/internal/service/mention"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

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
	defaultAdapter   AgentAdapter        // 默认适配器，用于向后兼容
	executionService *ExecutionService   // 统一执行服务
	debugThreadMgr   *DebugThreadManager // 调试线程管理器

	// Mention 解析器（支持动态 patterns）
	mentionParser *mention.Parser

	// 后台执行支持：内容块持久化
	contentBlockRepo *repo.ContentBlockRepository

	runningAgents map[uuid.UUID]*RunningAgent
	mu            sync.RWMutex
}

// RunningAgent 运行中的Agent
type RunningAgent struct {
	InvocationID uuid.UUID
	ThreadID     uuid.UUID
	AgentConfig  *model.AgentRoleConfig
	BaseAgent    *model.BaseAgent // 关联的基础Agent配置
	StartedAt    time.Time
	LastActiveAt time.Time // 最后一次输出活动时间
	CancelFunc   context.CancelFunc

	// 工具执行状态跟踪（区分"无输出"和"真正卡死"）
	ActiveToolCount int                // 当前活跃的工具调用数量
	HeartbeatCancel context.CancelFunc // 心跳取消函数（工具执行期间定期更新 LastActiveAt）
	HeartbeatMu     sync.Mutex         // 保护心跳相关字段

	// 流式输出累积（用于 WebSocket 重连恢复）
	AccumulatedOutput string     // 累积的输出内容
	OutputMu          sync.Mutex // 保护输出累积字段

	// 结构化内容块累积（用于持久化）
	AccumulatedContentBlocks []ContentBlockData // 累积的内容块
	ContentBlocksMu          sync.Mutex         // 保护内容块累积字段

	// AskUserQuestion 相关状态
	WaitingForUserInput bool   // 是否正在等待用户输入（AskUserQuestion）
	PendingQuestionID   string // 待处理的 AskUserQuestion 工具ID
	LastQuestionToolID  string // 最后一个 AskUserQuestion 工具ID（用于判断 tool_result 是否是该工具的拒绝响应）

	// CLI 进程管理（用于取消执行）
	Adapter AgentAdapter // Adapter 引用（用于获取当前进程）
	Cmd     *exec.Cmd    // CLI 进程引用（由 adapter 在执行时设置）
	cmdMu   sync.Mutex   // 保护 Cmd 并发访问
}

// ContentBlockData 结构化内容块数据（用于序列化）
type ContentBlockData struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Content   string `json:"content,omitempty"`
	Timestamp int64  `json:"timestamp"`
	Status    string `json:"status,omitempty"`
	Done      bool   `json:"done,omitempty"`
	// 工具调用相关（字段名与前端 ToolUseBlock 对齐）
	ToolName string                 `json:"toolName,omitempty"`
	ToolID   string                 `json:"toolId,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`
	Output   string                 `json:"output,omitempty"`
	IsError  bool                   `json:"isError,omitempty"`
	// AskUserQuestion 相关（字段名与前端 QuestionBlock 对齐）
	Questions    []QuestionItem    `json:"questions,omitempty"`    // 问题列表
	InvocationID string            `json:"invocationId,omitempty"` // 关联的 invocation ID
	AgentID      string            `json:"agentId,omitempty"`      // 提出问题的 Agent ID（用于前端 resume）
	AgentName    string            `json:"agentName,omitempty"`    // 提出问题的 Agent 名称（用于前端 @mention resume）
	// 时间追踪
	StartedAt   int64 `json:"startedAt,omitempty"`
	CompletedAt int64 `json:"completedAt,omitempty"`
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
	mentionParser *mention.Parser,
	contentBlockRepo *repo.ContentBlockRepository,
	humanTaskSvc *humantask.Service,
	humanTaskEnabled bool,
) *Orchestrator {
	o := &Orchestrator{
		invocationRepo:   invocationRepo,
		threadRepo:       threadRepo,
		msgRepo:          msgRepo,
		configSvc:        configSvc,
		baseAgentSvc:     baseAgentSvc,
		baseAgentRepo:    baseAgentRepo,
		tracker:          tracker,
		workflow:         workflow,
		workflowRepo:     workflowRepo,
		projectRepo:      projectRepo,
		wsHub:            wsHub,
		defaultAdapter:   defaultAdapter,
		mentionParser:    mentionParser,
		contentBlockRepo: contentBlockRepo,
		runningAgents:    make(map[uuid.UUID]*RunningAgent),
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
		mentionParser,
		contentBlockRepo,
		humanTaskSvc,
		humanTaskEnabled,
	)

	return o
}

// SpawnAgent 启动Agent
func (o *Orchestrator) SpawnAgent(ctx context.Context, req *SpawnRequest) (*model.AgentInvocation, error) {
	// 委托给执行服务
	return o.executionService.SpawnAgent(ctx, req)
}

// GetRunningAgentsForThread 获取运行中的 Agent 状态（实现 ws.RunningAgentsGetter）
func (o *Orchestrator) GetRunningAgentsForThread(threadID uuid.UUID) any {
	return o.executionService.GetRunningAgentsForThread(threadID)
}

// GetRunningInvocationsWithContentBlocks 获取运行中的 invocation 及其内容块（实现 ws.InvocationRecoverer）
func (o *Orchestrator) GetRunningInvocationsWithContentBlocks(ctx context.Context, threadID uuid.UUID) []ws.InvocationRecoveryData {
	return o.executionService.GetRunningInvocationsWithContentBlocks(ctx, threadID)
}

// GetRecentlyCompletedInvocations 获取最近完成的 invocation（实现 ws.InvocationRecoverer）
func (o *Orchestrator) GetRecentlyCompletedInvocations(ctx context.Context, threadID uuid.UUID, sinceMinutes int) []ws.InvocationRecoveryData {
	return o.executionService.GetRecentlyCompletedInvocations(ctx, threadID, sinceMinutes)
}

// SubmitQuestionAnswer 提交 AskUserQuestion 的用户答案
// 找到运行中的 Agent，并通过 stdin 发送答案给 CLI
func (o *Orchestrator) SubmitQuestionAnswer(threadID uuid.UUID, toolCallID string, answer string) error {
	return o.executionService.SubmitQuestionAnswer(threadID, toolCallID, answer)
}

// handleAgentError 处理Agent错误
func (o *Orchestrator) handleAgentError(ctx context.Context, invocation *model.AgentInvocation, err error) {
	invocation.Status = model.InvocationStatusFailed
	invocation.Output = err.Error()
	invocation.CompletedAt = timePtr(time.Now())
	o.invocationRepo.Update(ctx, invocation)

	o.broadcastStatus(invocation.ThreadID, invocation.ID, "failed", invocation.Role, invocation.AgentConfigID.String())
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
func (o *Orchestrator) broadcastStatus(threadID, invocationID uuid.UUID, status string, role model.AgentRole, agentID string) {
	// 委托给 ExecutionService 的实现（包含 input 支持）
	o.executionService.broadcastStatus(threadID, invocationID, status, role, "", agentID, "")

	// 通知外部 chunk 监听器（status 事件）
	o.executionService.NotifyChunkListeners(threadID, invocationID, Chunk{
		Type:    ChunkTypeStatus,
		Content: status,
	}, string(role), string(role))
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
				"messageId": msg.ID.String(),
				"agentId":   msg.AgentID,
				"content":   msg.Content,
				"agentName": agentName,
				"agentRole": agentRole,
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
	ThreadID        uuid.UUID
	ConfigID        uuid.UUID
	Role            model.AgentRole
	Input           string
	ProjectPath     string           // 工作目录
	SessionID       string           // 会话ID（用于 --resume 复用已有会话）
	SessionStrategy SessionStrategy  // 会话策略：new 或 resume
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

// GetExecutionService 获取执行服务实例（供外部注册 ChunkListener）
func (o *Orchestrator) GetExecutionService() *ExecutionService {
	return o.executionService
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
	var baseAgent *model.BaseAgent
	if config.BaseAgentID != uuid.Nil {
		baseAgent, err = o.baseAgentRepo.FindByID(ctx, config.BaseAgentID)
		if err != nil {
			logInfo("SpawnDebugAgent: baseAgent not found for config, trying default", zap.String("configId", config.ID.String()), zap.Error(err))
			baseAgent = nil
		}
	}

	// 如果角色未指定基础Agent或获取失败，使用默认基础Agent
	// 注意：直接使用 repo.FindDefault 获取完整信息（含 ApiToken）
	// 不能使用 baseAgentSvc.GetDefault，因为它会 sanitize 清除 ApiToken
	if baseAgent == nil {
		baseAgent, err = o.baseAgentRepo.FindDefault(ctx)
		if err != nil || baseAgent == nil {
			o.debugThreadMgr.SetStatus(req.ThreadID, DebugThreadStatusIdle)
			return nil, fmt.Errorf("未找到可用的基础Agent，请先设置一个默认的基础Agent")
		}
		logInfo("SpawnDebugAgent: using default baseAgent",
			zap.String("id", baseAgent.ID.String()),
			zap.String("name", baseAgent.Name))
	}

	logInfo("SpawnDebugAgent: got baseAgent",
		zap.String("id", baseAgent.ID.String()),
		zap.String("name", baseAgent.Name),
		zap.String("type", string(baseAgent.Type)),
		zap.String("defaultModel", baseAgent.DefaultModel),
		zap.Bool("hasApiToken", baseAgent.ApiToken != ""),
		zap.String("apiUrl", baseAgent.ApiURL),
		zap.String("cliPath", baseAgent.CliPath),
		zap.String("configPath", config.ConfigPath),
	)

	// 创建适配器
	adapter := GetAdapter(baseAgent)
	if adapter == nil {
		o.debugThreadMgr.SetStatus(req.ThreadID, DebugThreadStatusIdle)
		return nil, fmt.Errorf("不支持的基础Agent类型: %s", baseAgent.Type)
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
