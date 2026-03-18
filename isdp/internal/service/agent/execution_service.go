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

// ExecutionService 统一执行服务，整合Orchestrator和InteractiveSession的功能
type ExecutionService struct {
	invocationRepo   *repo.AgentInvocationRepository
	threadRepo       *repo.ThreadRepository
	msgRepo          *repo.MessageRepository
	configSvc        *ConfigService
	baseAgentSvc     *BaseAgentService
	tracker          *InvocationTracker
	workflow         *WorkflowEngine
	workflowRepo     *repo.WorkflowTemplateRepository
	projectRepo      *repo.ProjectRepository
	wsHub            *ws.Hub
	defaultAdapter   AgentAdapter

	runningAgents map[uuid.UUID]*RunningAgent
	mu            sync.RWMutex
}

// NewExecutionService 创建统一执行服务
func NewExecutionService(
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
) *ExecutionService {
	es := &ExecutionService{
		invocationRepo:   invocationRepo,
		threadRepo:       threadRepo,
		msgRepo:          msgRepo,
		configSvc:        configSvc,
		baseAgentSvc:     baseAgentSvc,
		tracker:          tracker,
		workflow:         workflow,
		workflowRepo:     workflowRepo,
		projectRepo:      projectRepo,
		wsHub:            wsHub,
		defaultAdapter:   defaultAdapter,
		runningAgents:    make(map[uuid.UUID]*RunningAgent),
	}

	return es
}

// SpawnAgent 启动Agent（统一执行入口）
func (es *ExecutionService) SpawnAgent(ctx context.Context, req *SpawnRequest) (*model.AgentInvocation, error) {
	// 解析配置和BaseAgent
	config, baseAgent, err := es.resolveConfigAndBaseAgent(ctx, req)
	if err != nil {
		return nil, err
	}

	// 创建调用记录
	invocation := &model.AgentInvocation{
		ID:            uuid.New(),
		ThreadID:      req.ThreadID,
		AgentConfigID: config.ID,
		Role:          config.Role,
		Status:        model.InvocationStatusPending,
		Input:         req.Input,
		CreatedAt:     time.Now(),
	}

	if err := es.invocationRepo.Create(ctx, invocation); err != nil {
		return nil, fmt.Errorf("failed to create invocation: %w", err)
	}

	// 创建上下文 - 使用独立的context，不受HTTP请求生命周期影响
	agentCtx, cancel := context.WithCancel(context.Background())

	// 记录运行中的Agent
	es.mu.Lock()
	es.runningAgents[invocation.ID] = &RunningAgent{
		InvocationID: invocation.ID,
		ThreadID:     req.ThreadID,
		AgentConfig:  config,
		BaseAgent:    baseAgent,
		StartedAt:    time.Now(),
		CancelFunc:   cancel,
	}
	es.mu.Unlock()

	// 广播状态更新
	es.broadcastStatus(req.ThreadID, invocation.ID, "started", config.Role)

	// 异步执行Agent
	go es.executeAgent(agentCtx, invocation, config, baseAgent, req)

	return invocation, nil
}

// executeAgent 执行Agent（统一执行路径）
func (es *ExecutionService) executeAgent(ctx context.Context, invocation *model.AgentInvocation, config *model.AgentRoleConfig, baseAgent *model.BaseAgent, req *SpawnRequest) {
	defer func() {
		// 恢复可能的panic
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in executeAgent: %v", r)
			es.handleAgentError(ctx, invocation, err)
		}
		es.mu.Lock()
		delete(es.runningAgents, invocation.ID)
		es.mu.Unlock()
	}()

	logInfo("executeAgent started", zap.String("invocationID", invocation.ID.String()))

	// 构建上下文
	contextLayers, err := es.buildContextLayers(ctx, req.ThreadID, config)
	if err != nil {
		logError("buildContextLayers failed", zap.Error(err))
		es.handleAgentError(ctx, invocation, fmt.Errorf("failed to build context layers: %w", err))
		return
	}
	logDebug("Context layers built")

	// 获取适配器
	adapter, err := es.getAdapter(ctx, config, baseAgent)
	if err != nil {
		logError("getAdapter failed", zap.Error(err))
		es.handleAgentError(ctx, invocation, fmt.Errorf("failed to get adapter: %w", err))
		return
	}
	logDebug("Adapter obtained", zap.String("adapterType", fmt.Sprintf("%T", adapter)))

	// 构建ExecutionRequest
	execReq := &ExecutionRequest{
		Config:    config,
		BaseAgent: baseAgent,
		Context:   contextLayers,
		Input:     req.Input,
		WorkDir:   req.ProjectPath,
	}

	// 使用流式执行，实时广播输出
	var outputBuilder strings.Builder
	result, err := adapter.ExecuteWithStream(ctx, execReq, func(chunk Chunk) {
		outputBuilder.WriteString(chunk.Content)
		// 实时广播输出块
		es.broadcastChunk(req.ThreadID, invocation.ID, chunk, config.ID.String(), config.Name)
	})

	if err != nil {
		logError("Adapter.ExecuteWithStream failed", zap.Error(err))
		es.handleAgentError(ctx, invocation, fmt.Errorf("adapter execution failed: %w", err))
		return
	}

	output := outputBuilder.String()
	logInfo("Execution completed", zap.Int("outputLength", len(output)), zap.String("sessionKey", result.SessionKey))

	// 更新调用记录
	invocation.Status = model.InvocationStatusCompleted
	invocation.Output = output
	invocation.CompletedAt = timePtr(time.Now())
	es.invocationRepo.Update(ctx, invocation)

	// 保存输出消息到数据库
	es.saveAgentMessage(ctx, req.ThreadID, config, output)

	// 广播完成状态
	es.broadcastStatus(req.ThreadID, invocation.ID, "completed", config.Role)

	// 检查是否需要路由到下一个Agent
	es.checkRouting(ctx, req.ThreadID, config, output)
}

// resolveConfigAndBaseAgent 解析配置和BaseAgent
func (es *ExecutionService) resolveConfigAndBaseAgent(ctx context.Context, req *SpawnRequest) (*model.AgentRoleConfig, *model.BaseAgent, error) {
	var config *model.AgentRoleConfig
	var err error

	if req.ConfigID != uuid.Nil {
		config, err = es.configSvc.GetByID(ctx, req.ConfigID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get agent config: %w", err)
		}
	} else {
		config, err = es.configSvc.GetDefaultByRole(ctx, req.Role)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get default agent config: %w", err)
		}
	}

	var baseAgent *model.BaseAgent
	if config.BaseAgentID != uuid.Nil && es.baseAgentSvc != nil {
		baseAgent, err = es.baseAgentSvc.GetByID(ctx, config.BaseAgentID)
		if err != nil {
			baseAgent = nil // 不阻止执行
		}
	}

	return config, baseAgent, nil
}

// saveAgentMessage 保存Agent消息
func (es *ExecutionService) saveAgentMessage(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, output string) {
	metadata, _ := json.Marshal(map[string]string{
		"agentName": config.Name,
		"agentRole": string(config.Role),
	})
	msg := &model.Message{
		ThreadID:    threadID,
		Role:        model.MessageRoleAgent,
		AgentID:     config.ID.String(),
		Content:     output,
		MessageType: model.MessageTypeText,
		Metadata:    metadata,
		CreatedAt:   time.Now(),
	}
	if err := es.msgRepo.Create(ctx, msg); err != nil {
		logError("Failed to save agent message", zap.Error(err))
	}
}

// mergeConfig 合并 AgentRoleConfig 和 BaseAgent 的配置
func (es *ExecutionService) mergeConfig(config *model.AgentRoleConfig, baseAgent *model.BaseAgent) *model.AgentRoleConfig {
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
func (es *ExecutionService) getAdapter(ctx context.Context, config *model.AgentRoleConfig, baseAgent *model.BaseAgent) (AgentAdapter, error) {
	// 如果有 BaseAgent，使用它创建适配器
	if baseAgent != nil {
		adapter := NewAdapter(baseAgent)
		if adapter == nil {
			return nil, fmt.Errorf("unsupported base agent type: %s", baseAgent.Type)
		}
		return adapter, nil
	}

	// 如果配置了BaseAgentID但没有传入baseAgent，尝试获取
	if config.BaseAgentID != uuid.Nil && es.baseAgentSvc != nil {
		ba, err := es.baseAgentSvc.GetByID(ctx, config.BaseAgentID)
		if err == nil {
			adapter := NewAdapter(ba)
			if adapter != nil {
				return adapter, nil
			}
		}
		// 如果获取失败，继续尝试使用默认适配器
	}

	// 向后兼容：使用默认适配器
	if es.defaultAdapter != nil {
		return es.defaultAdapter, nil
	}

	return nil, errors.New("no adapter available")
}

// handleAgentError 处理Agent错误
func (es *ExecutionService) handleAgentError(ctx context.Context, invocation *model.AgentInvocation, err error) {
	invocation.Status = model.InvocationStatusFailed
	invocation.Output = err.Error()
	invocation.CompletedAt = timePtr(time.Now())
	es.invocationRepo.Update(ctx, invocation)

	es.broadcastStatus(invocation.ThreadID, invocation.ID, "failed", invocation.Role)
}

// buildContextLayers 构建上下文层
func (es *ExecutionService) buildContextLayers(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig) (*ContextLayers, error) {
	layers := &ContextLayers{}

	// Layer 0: 系统提示
	layers.Layer0 = config.SystemPrompt

	// Layer 1: Thread历史
	messages, err := es.msgRepo.FindByThreadID(ctx, threadID, 100)
	if err != nil {
		return nil, err
	}
	layers.Layer1 = es.formatMessages(messages)

	// Layer 2: 工作产物
	thread, err := es.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return nil, err
	}
	layers.Layer2 = es.getArtifacts(thread)

	// Layer 3: 环境信息
	layers.Layer3 = es.getEnvironmentInfo(thread)

	return layers, nil
}

// checkRouting 检查路由
func (es *ExecutionService) checkRouting(ctx context.Context, threadID uuid.UUID, currentConfig *model.AgentRoleConfig, output string) {
	mentions := es.parseMentions(output)

	if len(mentions) == 0 {
		// 检查信号路由
		es.checkSignalRouting(ctx, threadID, currentConfig, output)
		return
	}

	// 获取工作流模板中的 Agent 列表
	allowedAgents := es.getAllowedAgentsFromWorkflow(ctx, threadID)

	// 获取项目路径
	var projectPath string
	if es.projectRepo != nil {
		project, err := es.projectRepo.GetByThreadID(ctx, threadID)
		if err == nil && project != nil {
			projectPath = project.LocalPath
		}
	}

	for _, mention := range mentions {
		var targetConfig *model.AgentRoleConfig

		if mention.Role != "" {
			// 按 role 查找
			targetConfig = es.findAgentByRole(allowedAgents, mention.Role)
		} else {
			// 按 name 查找
			targetConfig = es.findAgentByName(allowedAgents, mention.AgentName)
		}

		if targetConfig == nil {
			logInfo("路由被拒绝：目标不在工作流模板中",
				zap.String("mention", mention.Raw),
				zap.String("threadId", threadID.String()))
			continue
		}

		// 使用工作流模板中指定的 Agent 实例
		es.SpawnAgent(ctx, &SpawnRequest{
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
func (es *ExecutionService) parseMentions(content string) []ParsedMention {
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

// getAllowedAgentsFromWorkflow 从工作流模板获取允许路由的 Agent 列表
// 数据流: Thread → WorkflowTemplate → AgentIDs → AgentConfigs
func (es *ExecutionService) getAllowedAgentsFromWorkflow(ctx context.Context, threadID uuid.UUID) []*model.AgentRoleConfig {
	// 1. 获取 Thread
	thread, err := es.threadRepo.FindByID(ctx, threadID)
	if err != nil || thread.WorkflowTemplateID == nil {
		return nil
	}

	// 2. 获取工作流模板
	workflow, err := es.workflowRepo.FindByID(ctx, *thread.WorkflowTemplateID)
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
		agent, err := es.configSvc.GetByID(ctx, id)
		if err == nil {
			agents = append(agents, agent)
		}
	}

	return agents
}

// findAgentByRole 在 Agent 列表中按角色查找
func (es *ExecutionService) findAgentByRole(agents []*model.AgentRoleConfig, role model.AgentRole) *model.AgentRoleConfig {
	for _, agent := range agents {
		if agent.Role == role {
			return agent
		}
	}
	return nil
}

// findAgentByName 在 Agent 列表中按名称查找
func (es *ExecutionService) findAgentByName(agents []*model.AgentRoleConfig, name string) *model.AgentRoleConfig {
	for _, agent := range agents {
		if agent.Name == name {
			return agent
		}
	}
	return nil
}

// checkSignalRouting 检查信号路由（原有逻辑提取）
func (es *ExecutionService) checkSignalRouting(ctx context.Context, threadID uuid.UUID, config *model.AgentRoleConfig, output string) {
	for _, signal := range config.RoutingConfig.RouteOnSignal {
		if strings.Contains(output, signal) {
			nextPhase := es.workflow.GetNextPhase(getPhaseFromSignal(signal))
			nextRole := es.workflow.GetPhaseAgent(nextPhase)

			// 获取项目路径
			var projectPath string
			if es.projectRepo != nil {
				project, err := es.projectRepo.GetByThreadID(ctx, threadID)
				if err == nil && project != nil {
					projectPath = project.LocalPath
				}
			}

			es.SpawnAgent(ctx, &SpawnRequest{
				ThreadID:    threadID,
				Role:        nextRole,
				Input:       output,
				ProjectPath: projectPath,
			})
			break
		}
	}
}

// formatMessages 格式化消息
func (es *ExecutionService) formatMessages(messages []*model.Message) string {
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
func (es *ExecutionService) getArtifacts(thread *model.Thread) string {
	// TODO: 实现获取工作产物
	return ""
}

// getEnvironmentInfo 获取环境信息
func (es *ExecutionService) getEnvironmentInfo(thread *model.Thread) string {
	return fmt.Sprintf("Thread ID: %s\n当前阶段: %s\n状态: %s",
		thread.ID, thread.CurrentPhase, thread.Status)
}

// CancelAgent 取消Agent
func (es *ExecutionService) CancelAgent(ctx context.Context, invocationID uuid.UUID) error {
	es.mu.Lock()
	agent, exists := es.runningAgents[invocationID]
	if exists {
		agent.CancelFunc()
		delete(es.runningAgents, invocationID)
	}
	es.mu.Unlock()

	if !exists {
		return ErrAgentNotFound
	}

	// 更新状态
	invocation, err := es.invocationRepo.FindByID(ctx, invocationID)
	if err != nil {
		return err
	}

	invocation.Status = model.InvocationStatusCancelled
	invocation.CompletedAt = timePtr(time.Now())
	return es.invocationRepo.Update(ctx, invocation)
}

// broadcastStatus 广播状态
func (es *ExecutionService) broadcastStatus(threadID, invocationID uuid.UUID, status string, role model.AgentRole) {
	if es.wsHub != nil {
		es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
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

// broadcastChunk 广播输出块（实时流式输出）
func (es *ExecutionService) broadcastChunk(threadID, invocationID uuid.UUID, chunk Chunk, agentID, agentName string) {
	if es.wsHub != nil {
		es.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_output_chunk",
			ThreadID:  threadID.String(),
			Timestamp: time.Now().UnixMilli(),
			Payload: map[string]interface{}{
				"invocationId": invocationID.String(),
				"chunk":        chunk.Content,
				"chunkType":    string(chunk.Type),
				"agentId":      agentID,
				"agentName":    agentName,
			},
		})
	}
}

// GetInvocationsByThread 获取 Thread 的所有 Agent 调用
func (es *ExecutionService) GetInvocationsByThread(ctx context.Context, threadID uuid.UUID) ([]model.AgentInvocation, error) {
	invocations, err := es.invocationRepo.FindByThreadID(ctx, threadID)
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
func (es *ExecutionService) GetInvocationStatus(ctx context.Context, invocationID uuid.UUID) (*model.AgentInvocation, error) {
	return es.invocationRepo.FindByID(ctx, invocationID)
}

// SpawnAgentForUserMessage 为用户消息触发Agent响应
// 实现message.AgentSpawner接口
// 使用工作流模板中指定的Agent，而不是根据Phase硬编码选择
func (es *ExecutionService) SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string) error {
	// 检查是否已有Agent在运行
	es.mu.RLock()
	for _, agent := range es.runningAgents {
		if agent.ThreadID == threadID {
			es.mu.RUnlock()
			// 已有Agent运行，不需要再触发
			return nil
		}
	}
	es.mu.RUnlock()

	// 获取Thread信息
	thread, err := es.threadRepo.FindByID(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to get thread: %w", err)
	}

	// 获取项目路径
	var projectPath string
	if es.projectRepo != nil {
		project, err := es.projectRepo.GetByThreadID(ctx, threadID)
		if err == nil && project != nil {
			projectPath = project.LocalPath
		}
	}

	// 获取工作流模板中的Agent列表
	var agentIDs []string
	if thread.WorkflowTemplateID != nil && es.workflowRepo != nil {
		workflow, err := es.workflowRepo.FindByID(ctx, *thread.WorkflowTemplateID)
		if err == nil && workflow != nil {
			// 解析 agent_ids JSON
			if len(workflow.AgentIDs) > 0 {
				if err := json.Unmarshal(workflow.AgentIDs, &agentIDs); err != nil {
					logError("Failed to parse agent_ids", zap.Error(err))
				}
			}
		}
	}

	// 如果工作流模板中有Agent，使用第一个Agent
	if len(agentIDs) > 0 {
		configID, err := uuid.Parse(agentIDs[0])
		if err != nil {
			return fmt.Errorf("invalid agent id in workflow template: %w", err)
		}

		// 验证Agent配置存在
		config, err := es.configSvc.GetByID(ctx, configID)
		if err != nil {
			logError("Agent config not found, falling back to default", zap.Error(err))
			// 继续使用回退逻辑
		} else {
			// 使用工作流模板中指定的Agent
			_, err = es.SpawnAgent(ctx, &SpawnRequest{
				ThreadID:    threadID,
				Role:        config.Role,
				ConfigID:    config.ID,
				Input:       userMessage,
				ProjectPath: projectPath,
			})
			return err
		}
	}

	// 回退到原来的实现，以便能看到命令参数报错
	// 如果工作流模板中有Agent，使用第一个Agent
	if len(agentIDs) > 0 {
		configID, err := uuid.Parse(agentIDs[0])
		if err != nil {
			return fmt.Errorf("invalid agent id in workflow template: %w", err)
		}

		// 验证Agent配置存在
		config, err := es.configSvc.GetByID(ctx, configID)
		if err != nil {
			logError("Agent config not found, falling back to default", zap.Error(err))
			// 继续使用回退逻辑
		} else {
			// 使用工作流模板中指定的Agent
			_, err = es.SpawnAgent(ctx, &SpawnRequest{
				ThreadID:    threadID,
				Role:        config.Role,
				ConfigID:    config.ID,
				Input:       userMessage,
				ProjectPath: projectPath,
			})
			return err
		}
	}

	// 回退逻辑：根据当前阶段决定触发哪个Agent
	logDebug("No workflow agent found, using phase-based selection")
	role := es.workflow.GetPhaseAgent(thread.CurrentPhase)
	if role == "" {
		role = model.AgentRoleRequirement
	}

	// 检查是否有该角色的默认配置
	config, err := es.configSvc.GetDefaultByRole(ctx, role)
	if err != nil {
		// 如果没有找到配置，尝试获取任意一个可用的配置
		configs, listErr := es.configSvc.List(ctx)
		if listErr != nil || len(configs) == 0 {
			return fmt.Errorf("no agent config available: %w", err)
		}
		config = configs[0]
	}

	// 触发Agent
	_, err = es.SpawnAgent(ctx, &SpawnRequest{
		ThreadID:    threadID,
		Role:        config.Role,
		ConfigID:    config.ID,
		Input:       userMessage,
		ProjectPath: projectPath,
	})
	return err
}

// timePtr 返回时间的指针
func timePtr(t time.Time) *time.Time {
	return &t
}