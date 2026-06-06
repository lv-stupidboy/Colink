// internal/service/agent/long_running_session.go
// 长连接 Session 数据结构
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// LongRunningSession 长连接 Session
// 用于 OpenCode/CodeAgent 等不支持原生 resume 的 CLI
// 保持进程存活，避免每次都重新启动，维护对话上下文
type LongRunningSession struct {
	// 标识信息
	ID           string `json:"id"`           // 内部 session ID
	AcpSessionID string `json:"acpSessionId"` // ACP 协议的 session ID（CLI 内部）
	ThreadID     string `json:"threadId"`     // Thread ID
	AgentID      string `json:"agentId"`      // Agent Config ID
	InvocationID string `json:"invocationId"` // 当前执行的 Invocation ID

	// 进程管理
	Process   *exec.Cmd   `json:"-"` // OpenCode 进程
	StdinPipe io.WriteCloser `json:"-"` // stdin 管道
	StdoutPipe io.ReadCloser `json:"-"` // stdout 管道

	// Adapter 引用（用于调用长连接方法）
	Adapter    AgentAdapter    `json:"-"` // ACP adapter
	BaseAgent  *model.BaseAgent `json:"-"` // BaseAgent 配置

	// 状态管理
	Status       SessionStatus `json:"status"`       // 当前状态
	LastActiveAt time.Time     `json:"lastActiveAt"` // 最后活跃时间
	CreatedAt    time.Time     `json:"createdAt"`    // 创建时间
	TurnCount    int           `json:"turnCount"`    // 对话轮数
	SealReason   SealReason    `json:"sealReason"`   // 封存原因

	// 对话累积
	Conversation *ConversationBuffer `json:"conversation"`

	// 待处理的问题（AskUserQuestion）
	PendingQuestion *Chunk `json:"pendingQuestion,omitempty"`

	// 待处理的用户输入（用于恢复或发送新 prompt）
	PendingInput string `json:"pendingInput,omitempty"`

	// 恢复历史（从 Sealed 状态恢复时设置）
	recoveryHistory *RecoveryHistory `json:"-"` // 压缩后的历史内容

	// 进程信息（用于诊断）
	ProcessPID        int    `json:"processPid"`       // 进程 PID
	StderrBuffer      string `json:"stderrBuffer"`     // stderr 输出
	LastError         string `json:"lastError"`        // 最后错误信息
	NotificationCount int    `json:"notificationCount"` // 收到的通知总数

	// 上下文
	Ctx    context.Context    `json:"-"`
	Cancel context.CancelFunc `json:"-"`

	// 回调
	OnChunk func(Chunk) `json:"-"`

	// 并发控制
	mu sync.RWMutex `json:"-"`
}

// ConversationBuffer 对话缓冲区
// 累积对话历史，用于持久化恢复
type ConversationBuffer struct {
	Turns       []ConversationTurn `json:"turns"`       // 对话回合列表
	TotalTokens int                `json:"totalTokens"` // Token 总数
	KeyEntities []KeyEntity        `json:"keyEntities"` // 关键实体追踪

	mu sync.RWMutex `json:"-"`
}

// ConversationTurn 对话回合
type ConversationTurn struct {
	TurnID        string                 `json:"turnId"`        // 唯一 ID
	Role          string                 `json:"role"`          // "user" 或 "agent"
	Content       string                 `json:"content"`       // 消息内容
	Timestamp     time.Time              `json:"timestamp"`     // 时间戳
	TokenCount    int                    `json:"tokenCount"`    // Token 计数
	ContentBlocks []ContentBlockData     `json:"contentBlocks"` // 结构化内容块
	Metadata      map[string]string      `json:"metadata"`      // agentID, toolCalls 等
}

// KeyEntity 关键实体追踪
// 用于跟踪对话中的关键信息（如问题单号、项目名等）
// 恢复时优先注入这些信息，确保 Agent 记住核心上下文
type KeyEntity struct {
	Type         string `json:"type"`         // 实体类型: "issue_id", "project_name", "requirement"
	Value        string `json:"value"`        // 实体值: "BUG-12345", "isdp"
	Source       string `json:"source"`       // 来源: "user_input", "agent_output"
	MentionedAt  []int  `json:"mentionedAt"`  // 出现在哪些 turnID（索引）
	LastUpdateAt int    `json:"lastUpdateAt"` // 最后提到的 turn 序号
	Confidence   float64 `json:"confidence"`   // 置信度（0-1）
}

// NewLongRunningSession 创建新的长连接 session
func NewLongRunningSession(threadID, agentID string) *LongRunningSession {
	return &LongRunningSession{
		ID:           uuid.New().String(),
		ThreadID:     threadID,
		AgentID:      agentID,
		Status:       SessionStatusActive,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
		Conversation: NewConversationBuffer(),
		TurnCount:    0,
	}
}

// NewConversationBuffer 创建新的对话缓冲区
func NewConversationBuffer() *ConversationBuffer {
	return &ConversationBuffer{
		Turns:       make([]ConversationTurn, 0),
		KeyEntities: make([]KeyEntity, 0),
		TotalTokens: 0,
	}
}

// AppendTurn 添加对话回合
func (cb *ConversationBuffer) AppendTurn(turn ConversationTurn) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// 计算 Token 数
	if turn.TokenCount == 0 {
		turn.TokenCount = EstimateTokens(turn.Content)
	}

	// 设置 TurnID
	if turn.TurnID == "" {
		turn.TurnID = fmt.Sprintf("turn-%d-%d", len(cb.Turns), time.Now().UnixMilli())
	}

	// 设置时间戳
	if turn.Timestamp.IsZero() {
		turn.Timestamp = time.Now()
	}

	cb.Turns = append(cb.Turns, turn)
	cb.TotalTokens += turn.TokenCount

	// 提取关键实体
	cb.extractKeyEntities(turn)
}

// extractKeyEntities 从对话中提取关键实体
func (cb *ConversationBuffer) extractKeyEntities(turn ConversationTurn) {
	// 使用预定义的模式提取关键实体
	patterns := map[string][]string{
		"issue_id": {
			`BUG-\d+`, `ISSUE-\d+`, `TASK-\d+`, `JIRA-\d+`,
			`#[\d]+`, `问题单[号]?[:：]\s*[\w\-]+`,
		},
		"project_name": {
			`项目[:：]\s*[\w\-]+`, `project[:：]\s*[\w\-]+`,
		},
		"requirement": {
			`需求[:：]\s*[^\n]+`, `requirement[:：]\s*[^\n]+`,
		},
		"decision": {
			`决定[:：]\s*[^\n]+`, `决策[:：]\s*[^\n]+`, `decision[:：]\s*[^\n]+`,
		},
	}

	turnIndex := len(cb.Turns) - 1

	for entityType, entityPatterns := range patterns {
		for _, pattern := range entityPatterns {
			// 简化的正则匹配（实际实现需要使用 regexp）
			matches := findPatternMatches(turn.Content, pattern)
			for _, match := range matches {
				// 检查是否已存在
				existing := cb.findKeyEntity(entityType, match)
				if existing != nil {
					// 更新现有实体
					existing.MentionedAt = append(existing.MentionedAt, turnIndex)
					existing.LastUpdateAt = turnIndex
					existing.Source = turn.Role
				} else {
					// 创建新实体
					cb.KeyEntities = append(cb.KeyEntities, KeyEntity{
						Type:         entityType,
						Value:        match,
						Source:       turn.Role,
						MentionedAt:  []int{turnIndex},
						LastUpdateAt: turnIndex,
						Confidence:   0.8, // 默认置信度
					})
				}
			}
		}
	}
}

// findKeyEntity 查找已存在的关键实体
func (cb *ConversationBuffer) findKeyEntity(entityType, value string) *KeyEntity {
	for i := range cb.KeyEntities {
		if cb.KeyEntities[i].Type == entityType && cb.KeyEntities[i].Value == value {
			return &cb.KeyEntities[i]
		}
	}
	return nil
}

// findPatternMatches 简化的模式匹配
// 实际实现应使用 regexp 包
func findPatternMatches(content, pattern string) []string {
	// 简化实现：直接字符串搜索
	// 完整实现需要使用正则表达式
	return []string{} // TODO: 实现正则匹配
}

// GetRecentTurns 获取最近的 N 轮对话
func (cb *ConversationBuffer) GetRecentTurns(n int) []ConversationTurn {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if n <= 0 || n > len(cb.Turns) {
		return cb.Turns
	}

	// 返回最近的 N 轮
	start := len(cb.Turns) - n
	return cb.Turns[start:]
}

// GetOldTurns 获取早期的对话（排除最近 N 轮）
func (cb *ConversationBuffer) GetOldTurns(recentN int) []ConversationTurn {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if recentN >= len(cb.Turns) {
		return []ConversationTurn{}
	}

	return cb.Turns[:len(cb.Turns)-recentN]
}

// FormatFullHistory 格式化完整历史（用于恢复 prompt）
func (cb *ConversationBuffer) FormatFullHistory() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("## 对话历史\n\n")

	for _, turn := range cb.Turns {
		role := "用户"
		if turn.Role == "agent" {
			role = "Agent"
		}
		sb.WriteString(fmt.Sprintf("**%s**: %s\n\n", role, turn.Content))
	}

	return sb.String()
}

// FormatKeyEntities 格式化关键实体（用于恢复 prompt）
func (cb *ConversationBuffer) FormatKeyEntities() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if len(cb.KeyEntities) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 关键信息\n\n")

	for _, entity := range cb.KeyEntities {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", entity.Type, entity.Value))
	}

	sb.WriteString("\n")
	return sb.String()
}

// ToJSON 序列化为 JSON
func (cb *ConversationBuffer) ToJSON() ([]byte, error) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return json.Marshal(cb)
}

// FromJSON 从 JSON 反序列化
func (cb *ConversationBuffer) FromJSON(data []byte) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return json.Unmarshal(data, cb)
}

// DeepCopy 创建深拷贝
func (cb *ConversationBuffer) DeepCopy() *ConversationBuffer {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	newBuffer := NewConversationBuffer()
	newBuffer.Turns = make([]ConversationTurn, len(cb.Turns))
	copy(newBuffer.Turns, cb.Turns)

	newBuffer.KeyEntities = make([]KeyEntity, len(cb.KeyEntities))
	copy(newBuffer.KeyEntities, cb.KeyEntities)

	newBuffer.TotalTokens = cb.TotalTokens
	return newBuffer
}

// LoadKeyEntities 从 JSON 加载关键实体
func (cb *ConversationBuffer) LoadKeyEntities(data []byte) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return json.Unmarshal(data, &cb.KeyEntities)
}

// UpdateStatus 更新 session 状态
func (s *LongRunningSession) UpdateStatus(status SessionStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
	s.LastActiveAt = time.Now()
}

// GetStatus 获取当前状态
func (s *LongRunningSession) GetStatus() SessionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Status
}

// IsActive 判断是否活跃
func (s *LongRunningSession) IsActive() bool {
	return s.GetStatus() == SessionStatusActive
}

// IsIdle 判断是否空闲
func (s *LongRunningSession) IsIdle() bool {
	return s.GetStatus() == SessionStatusIdle
}

// IsSealed 判断是否已封存
func (s *LongRunningSession) IsSealed() bool {
	return s.GetStatus() == SessionStatusSealed
}

// IncrementTurnCount 增加对话轮数
func (s *LongRunningSession) IncrementTurnCount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TurnCount++
	s.LastActiveAt = time.Now()
}

// AppendUserTurn 添加用户对话回合
func (s *LongRunningSession) AppendUserTurn(content string) {
	s.Conversation.AppendTurn(ConversationTurn{
		Role:    "user",
		Content: content,
		Metadata: map[string]string{
			"threadId": s.ThreadID,
			"agentId":  s.AgentID,
		},
	})
	s.IncrementTurnCount()
}

// AppendAgentTurn 添加 Agent 对话回合
func (s *LongRunningSession) AppendAgentTurn(content string, contentBlocks []ContentBlockData) {
	s.Conversation.AppendTurn(ConversationTurn{
		Role:          "agent",
		Content:       content,
		ContentBlocks: contentBlocks,
		Metadata: map[string]string{
			"threadId":     s.ThreadID,
			"agentId":      s.AgentID,
			"invocationId": s.InvocationID,
		},
	})
	s.IncrementTurnCount()
}

// Persist 持久化对话内容
func (s *LongRunningSession) Persist() ([]byte, error) {
	return s.Conversation.ToJSON()
}

// Load 从持久化数据恢复
func (s *LongRunningSession) Load(data []byte) error {
	return s.Conversation.FromJSON(data)
}

// Seal 封存 session
func (s *LongRunningSession) Seal(reason SealReason, detail string) {
	s.mu.Lock()
	s.Status = SessionStatusSealing
	s.SealReason = reason
	s.mu.Unlock()

	// 记录日志
	logInfo("Session sealing started",
		zap.String("sessionId", s.ID),
		zap.String("reason", string(reason)),
		zap.String("detail", detail),
		zap.Int("turnCount", s.TurnCount),
		zap.Int("totalTokens", s.Conversation.TotalTokens))

	// 异步封存
	// 实际实现需要调用 session_pool.go 的 sealSession 方法
}

// IsProcessAlive 检查进程是否存活
// 对于 ACP adapter，需要调用 adapter 的 IsSessionAlive 方法
// 因为进程由 ACP adapter 内部管理（acpSession.cmd），而不是 LongRunningSession.Process
func (s *LongRunningSession) IsProcessAlive() bool {
	s.mu.RLock()
	process := s.Process
	adapter := s.Adapter
	acpSessionID := s.AcpSessionID
	status := s.Status
	s.mu.RUnlock()

	// 如果 AcpSessionID 为空，表示进程还没有启动
	// 此时不应判断为"进程死亡"，而是"进程未启动"
	// 进程健康检查应跳过这种情况
	if acpSessionID == "" {
		// 只有 Active 状态且无 AcpSessionID 时，才认为"等待启动"
		// 这不是进程死亡，而是正常的初始化阶段
		return status == SessionStatusActive
	}

	// 如果有 ACP adapter 和 acpSessionID，使用 adapter 的检查方法
	if adapter != nil && acpSessionID != "" {
		longRunning, ok := adapter.(LongRunningSessionCapable)
		if ok {
			return longRunning.IsSessionAlive(acpSessionID)
		}
	}

	// 回退：检查 Process 字段（用于非 ACP adapter 或旧代码兼容）
	if process == nil || process.Process == nil {
		return false
	}

	return process.Process != nil
}

// GetSessionKey 获取 session key（用于 SessionPool）
func (s *LongRunningSession) GetSessionKey() string {
	return fmt.Sprintf("%s:%s", s.ThreadID, s.AgentID)
}

// StartLongRunningSession 启动长连接 session
// 使用接口断言方式调用 adapter 的长连接方法（非侵入式）
func (s *LongRunningSession) StartLongRunningSession(ctx context.Context, req *ExecutionRequest) error {
	if s.Adapter == nil {
		return fmt.Errorf("adapter not set for session %s", s.ID)
	}

	// 接口断言：检查 adapter 是否支持长连接
	longRunning, ok := s.Adapter.(LongRunningSessionCapable)
	if !ok {
		return fmt.Errorf("adapter does not support long running session (type: %T)", s.Adapter)
	}

	// 调用 ACP adapter 的长连接启动方法
	acpSessionID, err := longRunning.StartLongRunningSession(ctx, req)
	if err != nil {
		s.mu.Lock()
		s.Status = SessionStatusError
		s.LastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("failed to start long running session: %w", err)
	}

	// 保存 ACP session ID
	s.mu.Lock()
	s.AcpSessionID = acpSessionID
	s.Status = SessionStatusActive
	s.LastActiveAt = time.Now()
	s.mu.Unlock()

	logInfo("LongRunningSession: started",
		zap.String("sessionId", s.ID),
		zap.String("acpSessionId", acpSessionID),
		zap.String("threadId", s.ThreadID),
		zap.String("agentId", s.AgentID))

	return nil
}

// SendPromptToSession 向已有 session 发送新 prompt
// 使用接口断言方式调用 adapter 的长连接方法（非侵入式）
func (s *LongRunningSession) SendPromptToSession(ctx context.Context, prompt string, onChunk func(Chunk)) error {
	if s.Adapter == nil {
		return fmt.Errorf("adapter not set for session %s", s.ID)
	}

	// 接口断言：检查 adapter 是否支持长连接
	longRunning, ok := s.Adapter.(LongRunningSessionCapable)
	if !ok {
		return fmt.Errorf("adapter does not support long running session (type: %T)", s.Adapter)
	}

	// 检查 session 是否存活 - 使用 AcpSessionID（ACP adapter 返回的 ID）
	if !longRunning.IsSessionAlive(s.AcpSessionID) {
		logInfo("LongRunningSession: process not alive, need recovery",
			zap.String("sessionId", s.ID),
			zap.String("acpSessionId", s.AcpSessionID))
		return fmt.Errorf("process not alive for session %s", s.AcpSessionID)
	}

	// 更新状态
	s.mu.Lock()
	s.Status = SessionStatusActive
	s.LastActiveAt = time.Now()
	s.PendingInput = prompt
	s.OnChunk = onChunk
	s.mu.Unlock()

	// 调用 ACP adapter 发送 prompt - 使用 AcpSessionID
	err := longRunning.SendPromptToSession(ctx, s.AcpSessionID, prompt, onChunk)
	if err != nil {
		s.mu.Lock()
		s.LastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("failed to send prompt to session: %w", err)
	}

	logInfo("LongRunningSession: prompt sent",
		zap.String("sessionId", s.ID),
		zap.String("acpSessionId", s.AcpSessionID),
		zap.Int("promptLen", len(prompt)))

	return nil
}

// StopLongRunningSession 停止长连接 session
// 使用接口断言方式调用 adapter 的长连接方法（非侵入式）
func (s *LongRunningSession) StopLongRunningSession() error {
	if s.Adapter == nil {
		return nil // 无 adapter，无需停止
	}

	// 接口断言：检查 adapter 是否支持长连接
	longRunning, ok := s.Adapter.(LongRunningSessionCapable)
	if !ok {
		return nil // adapter 不支持长连接，无需停止
	}

	// 使用 AcpSessionID 停止 session
	err := longRunning.StopLongRunningSession(s.AcpSessionID)
	if err != nil {
		logError("LongRunningSession: failed to stop",
			zap.String("sessionId", s.ID),
			zap.Error(err))
		return err
	}

	s.mu.Lock()
	s.Status = SessionStatusSealed
	s.mu.Unlock()

	logInfo("LongRunningSession: stopped",
		zap.String("sessionId", s.ID))

	return nil
}

// SetAdapter 设置 adapter 引用
func (s *LongRunningSession) SetAdapter(adapter AgentAdapter, baseAgent *model.BaseAgent) {
	s.mu.Lock()
	s.Adapter = adapter
	s.BaseAgent = baseAgent
	s.mu.Unlock()
}

// SetInvocationID 设置当前 invocation ID
func (s *LongRunningSession) SetInvocationID(invocationID string) {
	s.mu.Lock()
	s.InvocationID = invocationID
	s.mu.Unlock()
}

// RecoveryHistory 存储压缩后的历史（用于恢复 prompt 构建）
type RecoveryHistory struct {
	Content     string    `json:"content"`     // 压缩后的历史内容
	Tokens      int       `json:"tokens"`      // Token 数
	CreatedAt   time.Time `json:"createdAt"`   // 创建时间
	KeyEntities []KeyEntity `json:"keyEntities"` // 关键实体
}

// SetRecoveryHistory 设置恢复历史（从 Sealed 状态恢复时设置）
func (s *LongRunningSession) SetRecoveryHistory(history string) {
	s.mu.Lock()
	s.recoveryHistory = &RecoveryHistory{
		Content:   history,
		Tokens:    EstimateTokens(history),
		CreatedAt: time.Now(),
		KeyEntities: s.Conversation.KeyEntities,
	}
	s.mu.Unlock()
}

// GetRecoveryHistory 获取恢复历史
func (s *LongRunningSession) GetRecoveryHistory() *RecoveryHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recoveryHistory
}

// BuildRecoveryPrompt 构建恢复 prompt
// 用于在长连接 session 启动后注入历史上下文
func (s *LongRunningSession) BuildRecoveryPrompt(newInput string) string {
	s.mu.RLock()
	history := s.recoveryHistory
	s.mu.RUnlock()

	if history == nil || history.Content == "" {
		return newInput // 无历史，直接返回新输入
	}

	var sb strings.Builder
	sb.WriteString("## 会话恢复\n\n")
	sb.WriteString("你正在继续一个之前中断的对话。以下是历史上下文，请在此基础上继续回答。\n\n")
	sb.WriteString(history.Content)
	sb.WriteString("\n---\n\n")
	sb.WriteString("## 当前请求\n\n")
	sb.WriteString(newInput)
	sb.WriteString("\n\n**注意**：\n")
	sb.WriteString("1. 请基于历史上下文理解当前请求\n")
	sb.WriteString("2. 如果历史中提到的关键信息（如问题单号）与当前请求相关，请继续使用\n")
	sb.WriteString("3. 如果当前请求是新的话题，可以独立处理\n")

	return sb.String()
}

// AppendChunk 累积 chunk 到 ConversationBuffer
// 用于在流式输出时实时记录对话内容
func (s *LongRunningSession) AppendChunk(chunk Chunk) {
	if s.Conversation == nil {
		return
	}

	// 根据 chunk 类型处理
	switch chunk.Type {
	case ChunkTypeText:
		// 累积 agent 输出文本
		s.Conversation.AppendTurn(ConversationTurn{
			Role:      "agent",
			Content:   chunk.Content,
			Timestamp: time.Now(),
		})
	case ChunkTypeStatus:
		// 状态更新，忽略（不需要记录到对话历史）
	case ChunkTypeThinking:
		// 思考过程，可选择记录
		s.Conversation.AppendTurn(ConversationTurn{
			Role:      "agent",
			Content:   "[thinking] " + chunk.Content,
			Timestamp: time.Now(),
			Metadata:  map[string]string{"type": "thinking"},
		})
	case ChunkTypeToolUse:
		// 工具调用，记录工具名和参数
		s.Conversation.AppendTurn(ConversationTurn{
			Role:      "agent",
			Content:   "[tool_use] " + chunk.Content,
			Timestamp: time.Now(),
			Metadata:  map[string]string{"type": "tool_use", "tool_id": chunk.ToolID, "tool_name": chunk.ToolName},
		})
	case ChunkTypeToolResult:
		// 工具结果，记录
		s.Conversation.AppendTurn(ConversationTurn{
			Role:      "agent",
			Content:   "[tool_result] " + chunk.Content,
			Timestamp: time.Now(),
			Metadata:  map[string]string{"type": "tool_result", "tool_id": chunk.ToolID},
		})
	default:
		// 其他类型记录原始内容
		if chunk.Content != "" {
			s.Conversation.AppendTurn(ConversationTurn{
				Role:      "agent",
				Content:   chunk.Content,
				Timestamp: time.Now(),
			})
		}
	}
}