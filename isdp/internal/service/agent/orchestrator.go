package agent

import (
	"context"
	"encoding/json"
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
	tracker          *InvocationTracker
	workflow         *WorkflowEngine
	workflowRepo     *repo.WorkflowTemplateRepository // 新增：工作流模板仓库
	projectRepo      *repo.ProjectRepository          // 新增：项目仓库，用于获取项目路径
	wsHub            *ws.Hub
	defaultAdapter   AgentAdapter     // 默认适配器，用于向后兼容
	executionService *ExecutionService // 统一执行服务

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
		tracker,
		workflow,
		workflowRepo,
		projectRepo,
		wsHub,
		defaultAdapter,
		ExecutionContextWorkflow, // 工作流上下文
	)

	o.interactiveManager = NewInteractiveSessionManager(wsHub)
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
	// 委托给ExecutionService的实现
	return o.executionService.getAdapter(ctx, config, baseAgent)
}

// checkRouting 检查路由
func (o *Orchestrator) checkRouting(ctx context.Context, threadID uuid.UUID, currentConfig *model.AgentRoleConfig, output string) {
	mentions := o.parseMentions(output)

	if len(mentions) == 0 {
		// 检查信号路由
		o.checkSignalRouting(ctx, threadID, currentConfig, output)
		return
	}

	// 获取工作流模板中的 Agent 列表
	allowedAgents := o.getAllowedAgentsFromWorkflow(ctx, threadID)

	// 获取项目路径
	var projectPath string
	if o.projectRepo != nil {
		project, err := o.projectRepo.GetByThreadID(ctx, threadID)
		if err == nil && project != nil {
			projectPath = project.LocalPath
		}
	}

	for _, mention := range mentions {
		var targetConfig *model.AgentRoleConfig

		if mention.Role != "" {
			// 按 role 查找
			targetConfig = o.findAgentByRole(allowedAgents, mention.Role)
		} else {
			// 按 name 查找
			targetConfig = o.findAgentByName(allowedAgents, mention.AgentName)
		}

		if targetConfig == nil {
			logInfo("路由被拒绝：目标不在工作流模板中",
				zap.String("mention", mention.Raw),
				zap.String("threadId", threadID.String()))
			continue
		}

		// 使用工作流模板中指定的 Agent 实例
		o.SpawnAgent(ctx, &SpawnRequest{
			ThreadID:    threadID,
			ConfigID:    targetConfig.ID,
			Role:        targetConfig.Role,
			Input:       output,
			ProjectPath: projectPath,
		})
	}
}

// parseMentions 解析@mention
// 支持: @developer (角色) 或 @前端开发 (实例名称)
func (o *Orchestrator) parseMentions(content string) []ParsedMention {
	var mentions []ParsedMention
	lines := strings.Split(content, "\n")
	count := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if count >= 2 {
			break
		}
		if strings.HasPrefix(line, "@") {
			mention := strings.Fields(line[1:])[0]
			if mention != "" {
				// 尝试解析为角色
				role := parseAgentRole(mention)

				mentions = append(mentions, ParsedMention{
					Role:      role,
					AgentName: mention,
					Raw:       mention,
				})
				count++
			}
		}
	}
	return mentions
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

// getAllowedAgentsFromWorkflow 从工作流模板获取允许路由的 Agent 列表
// 数据流: Thread → WorkflowTemplate → AgentIDs → AgentConfigs
func (o *Orchestrator) getAllowedAgentsFromWorkflow(ctx context.Context, threadID uuid.UUID) []*model.AgentRoleConfig {
	// 1. 获取 Thread
	thread, err := o.threadRepo.FindByID(ctx, threadID)
	if err != nil || thread.WorkflowTemplateID == nil {
		return nil
	}

	// 2. 获取工作流模板
	workflow, err := o.workflowRepo.FindByID(ctx, *thread.WorkflowTemplateID)
	if err != nil || workflow == nil {
		return nil
	}

	// 3. 解析 AgentIDs JSON
	var agentIDs []string
	if len(workflow.AgentIDs) == 0 {
		return nil
	}
	if err := json.Unmarshal(workflow.AgentIDs, &agentIDs); err != nil {
		return nil
	}

	// 4. 查询每个 Agent 的配置
	var agents []*model.AgentRoleConfig
	for _, idStr := range agentIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		agent, err := o.configSvc.GetByID(ctx, id)
		if err == nil {
			agents = append(agents, agent)
		}
	}

	return agents
}

// findAgentByRole 在 Agent 列表中按角色查找
func (o *Orchestrator) findAgentByRole(agents []*model.AgentRoleConfig, role model.AgentRole) *model.AgentRoleConfig {
	for _, agent := range agents {
		if agent.Role == role {
			return agent
		}
	}
	return nil
}

// findAgentByName 在 Agent 列表中按名称查找
func (o *Orchestrator) findAgentByName(agents []*model.AgentRoleConfig, name string) *model.AgentRoleConfig {
	for _, agent := range agents {
		if agent.Name == name {
			return agent
		}
	}
	return nil
}

// checkSignalRouting 检查信号路由（原有逻辑提取）
func (o *Orchestrator) checkSignalRouting(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, output string) {
	for _, signal := range config.RoutingConfig.RouteOnSignal {
		if strings.Contains(output, signal) {
			nextPhase := o.workflow.GetNextPhase(getPhaseFromSignal(signal))
			nextRole := o.workflow.GetPhaseAgent(nextPhase)

			// 获取项目路径
			var projectPath string
			if o.projectRepo != nil {
				project, err := o.projectRepo.GetByThreadID(ctx, threadID)
				if err == nil && project != nil {
					projectPath = project.LocalPath
				}
			}

			o.SpawnAgent(ctx, &SpawnRequest{
				ThreadID:    threadID,
				Role:        nextRole,
				Input:       output,
				ProjectPath: projectPath,
			})
			break
		}
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

// StartInteractiveSession 启动交互式会话
func (o *Orchestrator) StartInteractiveSession(ctx context.Context, req *SpawnRequest) (*InteractiveSession, error) {
	// 获取Agent配置
	var config *model.AgentRoleConfig
	var err error

	// 优先使用 ConfigID
	if req.ConfigID != uuid.Nil {
		config, err = o.configSvc.GetByID(ctx, req.ConfigID)
		if err != nil {
			return nil, fmt.Errorf("failed to get agent config by id: %w", err)
		}
	} else {
		// 如果没有 ConfigID，尝试通过 role 查找默认配置
		config, err = o.configSvc.GetDefaultByRole(ctx, req.Role)
		if err != nil {
			return nil, fmt.Errorf("failed to get agent config by role: %w", err)
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
// SpawnAgentForUserMessage 为用户消息触发Agent响应
// 实现message.AgentSpawner接口
// 使用工作流模板中指定的Agent，而不是根据Phase硬编码选择
func (o *Orchestrator) SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string) error {
	// 委托给执行服务
	return o.executionService.SpawnAgentForUserMessage(ctx, threadID, userMessage)
}
