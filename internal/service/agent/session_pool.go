// internal/service/agent/session_pool.go
// 长连接 Session Pool 管理
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SessionPool 长连接 Session 池
// 管理 OpenCode/CodeAgent 等不支持原生 resume 的 CLI 进程
type SessionPool struct {
	// sessions 活跃的 session 列表
	// key: "threadID:agentID"
	sessions map[string]*LongRunningSession

	// 配置
	idleTimeout      time.Duration // 空闲超时
	maxSessions      int           // 最大并发数
	persistInterval  int           // 每几轮对话后持久化
	maxHistoryTokens int           // 恢复历史最大 Token

	// 依赖注入
	repo       repo.SessionRecordRepository // Session 持久化
	wsHub      SessionBroadcaster           // WebSocket 广播器
	compressor *HistoryCompressor           // 历史压缩器

	// 并发控制
	mu    sync.RWMutex
	stopCh chan struct{} // 停止信号

	// 后台任务
	idleMonitorCancel context.CancelFunc
}

// SessionBroadcaster Session 状态广播接口
// 用于向前端通知 session 状态变化（sealed、recovered 等）
type SessionBroadcaster interface {
	BroadcastToThread(threadID string, eventType string, payload map[string]interface{})
}

// broadcastSessionEvent 广播 session 事件到前端
func (p *SessionPool) broadcastSessionEvent(threadID string, eventType string, payload map[string]interface{}) {
	p.mu.RLock()
	wsHub := p.wsHub
	p.mu.RUnlock()

	if wsHub == nil {
		return // 无 WebSocket Hub，不广播
	}

	wsHub.BroadcastToThread(threadID, eventType, payload)
}

// SessionPoolConfig SessionPool 配置
type SessionPoolConfig struct {
	IdleTimeout      time.Duration
	MaxSessions      int
	PersistInterval  int
	MaxHistoryTokens int
}

// NewSessionPool 创建 SessionPool
func NewSessionPool(config SessionPoolConfig, sessionRepo repo.SessionRecordRepository) *SessionPool {
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 10 * time.Minute
	}
	if config.MaxSessions == 0 {
		config.MaxSessions = 20
	}
	if config.PersistInterval == 0 {
		config.PersistInterval = 3
	}
	if config.MaxHistoryTokens == 0 {
		config.MaxHistoryTokens = 4000
	}

	pool := &SessionPool{
		sessions:         make(map[string]*LongRunningSession),
		idleTimeout:      config.IdleTimeout,
		maxSessions:      config.MaxSessions,
		persistInterval:  config.PersistInterval,
		maxHistoryTokens: config.MaxHistoryTokens,
		repo:             sessionRepo,
		compressor:       NewHistoryCompressor(config.MaxHistoryTokens),
		stopCh:           make(chan struct{}),
	}

	// 启动空闲监控
	pool.startIdleMonitor()

	return pool
}

// GetSessionKey 生成 session key
func GetSessionKey(threadID, agentID string) string {
	return fmt.Sprintf("%s:%s", threadID, agentID)
}

// Get 获取 session
func (p *SessionPool) Get(sessionKey string) *LongRunningSession {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sessions[sessionKey]
}

// GetByThread 根据 Thread 获取所有相关 session
func (p *SessionPool) GetByThread(threadID string) []*LongRunningSession {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []*LongRunningSession
	for _, session := range p.sessions {
		if session.ThreadID == threadID {
			result = append(result, session)
		}
	}
	return result
}

// GetOrCreate 获取或创建 session
// 这是长连接模式的核心入口
func (p *SessionPool) GetOrCreate(ctx context.Context, threadID, agentID string, baseAgentType model.BaseAgentType) (*LongRunningSession, error) {
	sessionKey := GetSessionKey(threadID, agentID)

	// 1. 检查是否有活跃 session
	p.mu.RLock()
	session := p.sessions[sessionKey]
	p.mu.RUnlock()

	if session != nil && session.IsActive() {
		// 直接复用活跃 session
		sessionPoolLogInfo("SessionPool: reuse active session",
			zap.String("sessionKey", sessionKey),
			zap.String("sessionId", session.ID),
			zap.Int("turnCount", session.TurnCount))
		return session, nil
	}

	// 2. 检查是否有空闲 session（可唤醒）
	if session != nil && session.IsIdle() {
		session.UpdateStatus(SessionStatusActive)
		sessionPoolLogInfo("SessionPool: wake up idle session",
			zap.String("sessionKey", sessionKey),
			zap.String("sessionId", session.ID))
		return session, nil
	}

	// 3. 检查是否有 Sealed session（可恢复）
	sealedRecord, err := p.repo.FindSealedByThreadAndAgent(ctx, threadID, agentID)
	if err == nil && sealedRecord != nil {
		// 有历史，尝试恢复
		sessionPoolLogInfo("SessionPool: found sealed session, attempting recovery",
			zap.String("sessionKey", sessionKey),
			zap.String("sealedId", sealedRecord.ID.String()))
		return p.RecoverFromSealed(ctx, sealedRecord, threadID, agentID, baseAgentType)
	}

	// 4. 创建新 session
	return p.CreateNew(ctx, threadID, agentID, baseAgentType)
}

// CreateNew 创建新 session
func (p *SessionPool) CreateNew(ctx context.Context, threadID, agentID string, baseAgentType model.BaseAgentType) (*LongRunningSession, error) {
	sessionKey := GetSessionKey(threadID, agentID)

	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查容量
	if len(p.sessions) >= p.maxSessions {
		// 尝试驱逐空闲 session
		evicted := p.evictOneSession()
		if !evicted {
			return nil, fmt.Errorf("SessionPool: max sessions limit reached (%d)", p.maxSessions)
		}
	}

	// 创建新 session
	session := NewLongRunningSession(threadID, agentID)

	p.sessions[sessionKey] = session

	sessionPoolLogInfo("SessionPool: created new session",
		zap.String("sessionKey", sessionKey),
		zap.String("sessionId", session.ID),
		zap.String("baseAgentType", string(baseAgentType)))

	return session, nil
}

// RecoverFromSealed 从 Sealed 状态恢复 session
func (p *SessionPool) RecoverFromSealed(ctx context.Context, sealedRecord *model.SessionRecord, threadID, agentID string, baseAgentType model.BaseAgentType) (*LongRunningSession, error) {
	sessionKey := GetSessionKey(threadID, agentID)

	// 1. 创建新 session 结构
	session := NewLongRunningSession(threadID, agentID)
	session.Status = SessionStatusRecovering

	// 2. 加载历史对话
	if len(sealedRecord.Conversation) > 0 {
		if err := session.Conversation.FromJSON(sealedRecord.Conversation); err != nil {
			sessionPoolLogError("SessionPool: failed to load conversation from sealed record",
				zap.Error(err),
				zap.String("sealedId", sealedRecord.ID.String()))
			// 恢复失败，创建新 session
			return p.CreateNew(ctx, threadID, agentID, baseAgentType)
		}
	}

	// 3. 压缩历史（Token 预算控制）
	var compressedHistory string
	if session.Conversation.TotalTokens > p.maxHistoryTokens {
		sessionPoolLogInfo("SessionPool: compressing conversation history",
			zap.Int("originalTokens", session.Conversation.TotalTokens),
			zap.Int("maxTokens", p.maxHistoryTokens))
		compressedHistory = p.compressor.Compress(session.Conversation)
	} else {
		compressedHistory = session.Conversation.FormatFullHistory()
	}

	// 4. 保存压缩后的历史到 session（用于构建恢复 prompt）
	session.SetRecoveryHistory(compressedHistory)

	// 5. 加载关键实体
	if len(sealedRecord.KeyEntities) > 0 {
		if err := session.Conversation.LoadKeyEntities(sealedRecord.KeyEntities); err != nil {
			sessionPoolLogError("SessionPool: failed to load key entities",
				zap.Error(err),
				zap.String("sealedId", sealedRecord.ID.String()))
		}
	}

	// 6. 注册到 pool
	p.mu.Lock()
	p.sessions[sessionKey] = session
	p.mu.Unlock()

	// 7. 更新数据库记录状态
	sealedRecord.Status = string(SessionStatusRecovering)
	if err := p.repo.Update(ctx, sealedRecord); err != nil {
		sessionPoolLogError("SessionPool: failed to update sealed record status",
			zap.Error(err))
	}

	sessionPoolLogInfo("SessionPool: recovered from sealed",
		zap.String("sessionKey", sessionKey),
		zap.String("sessionId", session.ID),
		zap.Int("turnCount", session.TurnCount),
		zap.Int("totalTokens", session.Conversation.TotalTokens))

	return session, nil
}

// MarkIdle 标记 session 为空闲状态
func (p *SessionPool) MarkIdle(sessionKey string) {
	p.mu.RLock()
	session := p.sessions[sessionKey]
	p.mu.RUnlock()

	if session == nil {
		return
	}

	session.UpdateStatus(SessionStatusIdle)
	sessionPoolLogInfo("SessionPool: session marked idle",
		zap.String("sessionKey", sessionKey),
		zap.String("sessionId", session.ID))
}

// SealIdleSession 封存空闲超时的 session
func (p *SessionPool) SealIdleSession(sessionKey string, reason SealReason) {
	p.mu.Lock()
	session := p.sessions[sessionKey]
	if session == nil {
		p.mu.Unlock()
		return
	}

	// 从 pool 移除
	delete(p.sessions, sessionKey)
	p.mu.Unlock()

	// 执行封存流程
	go p.sealSession(session, reason)
}

// sealSession 执行封存流程（异步）
func (p *SessionPool) sealSession(session *LongRunningSession, reason SealReason) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sessionPoolLogInfo("SessionPool: sealing session",
		zap.String("sessionId", session.ID),
		zap.String("reason", string(reason)),
		zap.Int("turnCount", session.TurnCount))

	// 1. 持久化对话内容
	conversationJSON, err := session.Persist()
	if err != nil {
		sessionPoolLogError("SessionPool: failed to persist conversation",
			zap.Error(err),
			zap.String("sessionId", session.ID))
	}

	// 2. 持久化关键实体
	keyEntitiesJSON, _ := json.Marshal(session.Conversation.KeyEntities)

	// 3. 保存到数据库
	record := &model.SessionRecord{
		ID:           uuid.New(),
		ThreadID:     uuid.MustParse(session.ThreadID),
		AgentID:      uuid.MustParse(session.AgentID),
		AgentType:    model.BaseAgentType("open_code"), // 默认 OpenCode 类型
		Status:       string(SessionStatusSealed),
		TurnCount:    session.TurnCount,
		TotalTokens:  session.Conversation.TotalTokens,
		Conversation: conversationJSON,
		KeyEntities:  keyEntitiesJSON,
		ProcessPID:   session.ProcessPID,
		LastActiveAt: session.LastActiveAt.Unix(),
		SealedAt:     time.Now().Unix(),
		CreatedAt:    session.CreatedAt.Unix(),
		UpdatedAt:    time.Now().Unix(),
	}

	if err := p.repo.Create(ctx, record); err != nil {
		sessionPoolLogError("SessionPool: failed to save sealed record",
			zap.Error(err),
			zap.String("sessionId", session.ID))
	}

	// 4. 更新状态
	session.UpdateStatus(SessionStatusSealed)

	// 5. 终止进程（如果还在运行）
	if session.IsProcessAlive() {
		// 实际终止进程的逻辑在 adapter 中实现
		sessionPoolLogInfo("SessionPool: terminating process",
			zap.String("sessionId", session.ID),
			zap.Int("pid", session.ProcessPID))
	}

	sessionPoolLogInfo("SessionPool: session sealed successfully",
		zap.String("sessionId", session.ID),
		zap.String("recordId", record.ID.String()))

	// 6. 广播 session_sealed 事件
	p.broadcastSessionEvent(session.ThreadID, "session_sealed", map[string]interface{}{
		"sessionId":   session.ID,
		"threadId":    session.ThreadID,
		"agentId":     session.AgentID,
		"reason":      string(reason),
		"turnCount":   session.TurnCount,
		"recoverable": true,
	})
}

// PersistConversation 定期持久化对话内容
func (p *SessionPool) PersistConversation(sessionKey string) error {
	p.mu.RLock()
	session := p.sessions[sessionKey]
	p.mu.RUnlock()

	if session == nil {
		return fmt.Errorf("session not found: %s", sessionKey)
	}

	// 持久化对话内容
	conversationJSON, err := session.Persist()
	if err != nil {
		return err
	}

	// 更新数据库中的记录
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	record, err := p.repo.FindByThreadAndAgent(ctx, session.ThreadID, session.AgentID)
	if err != nil {
		// 创建新记录
		record = &model.SessionRecord{
			ID:           uuid.New(),
			ThreadID:     uuid.MustParse(session.ThreadID),
			AgentID:      uuid.MustParse(session.AgentID),
			AgentType:    model.BaseAgentType("open_code"),
			Status:       string(session.Status),
			TurnCount:    session.TurnCount,
			TotalTokens:  session.Conversation.TotalTokens,
			Conversation: conversationJSON,
			LastActiveAt: session.LastActiveAt.Unix(),
			CreatedAt:    session.CreatedAt.Unix(),
			UpdatedAt:    time.Now().Unix(),
		}
		return p.repo.Create(ctx, record)
	}

	// 更新现有记录
	record.Conversation = conversationJSON
	record.TurnCount = session.TurnCount
	record.TotalTokens = session.Conversation.TotalTokens
	record.LastActiveAt = session.LastActiveAt.Unix()
	record.UpdatedAt = time.Now().Unix()

	return p.repo.Update(ctx, record)
}

// OnExecutionComplete 执行完成后的处理
func (p *SessionPool) OnExecutionComplete(sessionKey string, output string, contentBlocks []ContentBlockData) {
	session := p.Get(sessionKey)
	if session == nil {
		return
	}

	// 1. 累积 Agent 输出
	session.AppendAgentTurn(output, contentBlocks)

	// 2. 标记为空闲
	p.MarkIdle(sessionKey)

	// 3. 定期持久化检查
	if session.TurnCount % p.persistInterval == 0 {
		go p.PersistConversation(sessionKey)
	}

	sessionPoolLogInfo("SessionPool: execution completed",
		zap.String("sessionKey", sessionKey),
		zap.Int("turnCount", session.TurnCount),
		zap.Int("totalTokens", session.Conversation.TotalTokens))
}

// Cancel 取消 session（用户主动取消）
func (p *SessionPool) Cancel(sessionKey string) error {
	p.mu.Lock()
	session := p.sessions[sessionKey]
	if session == nil {
		p.mu.Unlock()
		return nil
	}

	// 从 pool 移除
	delete(p.sessions, sessionKey)
	p.mu.Unlock()

	// 异步封存
	go p.sealSession(session, SealReasonManual)

	sessionPoolLogInfo("SessionPool: session cancelled",
		zap.String("sessionKey", sessionKey))
	return nil
}

// Remove 移除 session（不封存）
func (p *SessionPool) Remove(sessionKey string) {
	p.mu.Lock()
	delete(p.sessions, sessionKey)
	p.mu.Unlock()
}

// Size 获取当前 session 数量
func (p *SessionPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.sessions)
}

// GetAll 获取所有 session（用于监控）
func (p *SessionPool) GetAll() map[string]*LongRunningSession {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*LongRunningSession)
	for k, v := range p.sessions {
		result[k] = v
	}
	return result
}

// startIdleMonitor 启动空闲监控和进程存活监控
func (p *SessionPool) startIdleMonitor() {
	ctx, cancel := context.WithCancel(context.Background())
	p.idleMonitorCancel = cancel

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		// 进程存活检查使用更短的间隔
		processTicker := time.NewTicker(30 * time.Second)
		defer processTicker.Stop()

		for {
			select {
			case <-ticker.C:
				p.checkIdleSessions()
			case <-processTicker.C:
				p.checkProcessHealth()
			case <-ctx.Done():
				return
			case <-p.stopCh:
				return
			}
		}
	}()

	sessionPoolLogInfo("SessionPool: idle monitor and process health monitor started",
		zap.Duration("idleTimeout", p.idleTimeout))
}

// checkIdleSessions 检查空闲 session
func (p *SessionPool) checkIdleSessions() {
	p.mu.RLock()
	sessions := make([]*LongRunningSession, 0)
	sessionKeys := make([]string, 0)

	for key, session := range p.sessions {
		sessions = append(sessions, session)
		sessionKeys = append(sessionKeys, key)
	}
	p.mu.RUnlock()

	now := time.Now()
	for i, session := range sessions {
		if session.IsIdle() && now.Sub(session.LastActiveAt) > p.idleTimeout {
			sessionPoolLogInfo("SessionPool: session idle timeout",
				zap.String("sessionKey", sessionKeys[i]),
				zap.Duration("idleTime", now.Sub(session.LastActiveAt)))
			p.SealIdleSession(sessionKeys[i], SealReasonTimeout)
		}
	}
}

// checkProcessHealth 检查进程存活状态
// 如果进程崩溃，触发紧急封存和恢复流程
func (p *SessionPool) checkProcessHealth() {
	p.mu.RLock()
	sessions := make([]*LongRunningSession, 0)
	sessionKeys := make([]string, 0)

	for key, session := range p.sessions {
		sessions = append(sessions, session)
		sessionKeys = append(sessionKeys, key)
	}
	p.mu.RUnlock()

	for i, session := range sessions {
		// 只检查 active 和 idle 状态的 session
		if session.Status != SessionStatusActive && session.Status != SessionStatusIdle {
			continue
		}

		// 检查进程是否存活
		if !session.IsProcessAlive() {
			sessionPoolLogInfo("SessionPool: process death detected",
				zap.String("sessionKey", sessionKeys[i]),
				zap.String("sessionId", session.ID),
				zap.String("status", string(session.Status)))

			// 触发进程崩溃处理
			p.handleProcessDeath(sessionKeys[i], session)
		}
	}
}

// handleProcessHealth 处理进程崩溃
// 执行紧急持久化 + 恢复流程
func (p *SessionPool) handleProcessDeath(sessionKey string, session *LongRunningSession) {
	// 1. 更新状态为 Recovering
	session.UpdateStatus(SessionStatusRecovering)

	// 2. 紧急持久化当前对话
	go p.emergencyPersistAndRecover(sessionKey, session)
}

// emergencyPersistAndRecover 紧急持久化并尝试恢复
func (p *SessionPool) emergencyPersistAndRecover(sessionKey string, session *LongRunningSession) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sessionPoolLogInfo("SessionPool: emergency persist and recover started",
		zap.String("sessionKey", sessionKey),
		zap.String("sessionId", session.ID),
		zap.Int("turnCount", session.TurnCount))

	// 1. 持久化对话内容
	conversationJSON, err := session.Persist()
	if err != nil {
		sessionPoolLogError("SessionPool: emergency persist failed",
			zap.Error(err),
			zap.String("sessionId", session.ID))
		// 持久化失败，标记为 Error 状态
		session.UpdateStatus(SessionStatusError)
		return
	}

	// 2. 保存到数据库
	keyEntitiesJSON, _ := json.Marshal(session.Conversation.KeyEntities)
	record := &model.SessionRecord{
		ID:           uuid.New(),
		ThreadID:     uuid.MustParse(session.ThreadID),
		AgentID:      uuid.MustParse(session.AgentID),
		AgentType:    model.BaseAgentType("open_code"),
		Status:       string(SessionStatusSealed),
		TurnCount:    session.TurnCount,
		TotalTokens:  session.Conversation.TotalTokens,
		Conversation: conversationJSON,
		KeyEntities:  keyEntitiesJSON,
		ProcessPID:   session.ProcessPID,
		LastActiveAt: session.LastActiveAt.Unix(),
		SealedAt:     time.Now().Unix(),
		CreatedAt:    session.CreatedAt.Unix(),
		UpdatedAt:    time.Now().Unix(),
	}

	if err := p.repo.Create(ctx, record); err != nil {
		sessionPoolLogError("SessionPool: failed to save emergency record",
			zap.Error(err),
			zap.String("sessionId", session.ID))
		session.UpdateStatus(SessionStatusError)
		return
	}

	// 3. 从 pool 中移除
	p.mu.Lock()
	delete(p.sessions, sessionKey)
	p.mu.Unlock()

	// 4. 更新状态为 Sealed
	session.UpdateStatus(SessionStatusSealed)

	sessionPoolLogInfo("SessionPool: emergency persist completed, session sealed",
		zap.String("sessionKey", sessionKey),
		zap.String("sessionId", session.ID),
		zap.String("recordId", record.ID.String()))

	// 5. 广播 session_error 事件（进程崩溃）
	p.broadcastSessionEvent(session.ThreadID, "session_error", map[string]interface{}{
		"sessionId":   session.ID,
		"threadId":    session.ThreadID,
		"agentId":     session.AgentID,
		"reason":      "process_crash",
		"turnCount":   session.TurnCount,
		"recoverable": true,
		"message":     "Session process crashed, conversation saved for recovery",
	})

	// 6. 下次用户输入时，会通过 GetOrCreate 触发 RecoverFromSealed
}

// evictOneSession 驱逐一个 session
// 优先驱逐空闲的 session
func (p *SessionPool) evictOneSession() bool {
	// 首先尝试驱逐空闲 session
	for key, session := range p.sessions {
		if session.IsIdle() {
			delete(p.sessions, key)
			go p.sealSession(session, SealReasonTimeout)
			sessionPoolLogInfo("SessionPool: evicted idle session",
				zap.String("sessionKey", key))
			return true
		}
	}

	// 其次尝试驱逐 sealing session
	for key, session := range p.sessions {
		if session.Status == SessionStatusSealing {
			delete(p.sessions, key)
			sessionPoolLogInfo("SessionPool: evicted sealing session",
				zap.String("sessionKey", key))
			return true
		}
	}

	// 无法驱逐
	return false
}

// SetWSHub 设置 WebSocket 广播器
func (p *SessionPool) SetWSHub(broadcaster SessionBroadcaster) {
	p.mu.Lock()
	p.wsHub = broadcaster
	p.mu.Unlock()
}

// Stop 停止 SessionPool
func (p *SessionPool) Stop() {
	// 停止空闲监控
	if p.idleMonitorCancel != nil {
		p.idleMonitorCancel()
	}

	// 停止信号
	close(p.stopCh)

	// 封存所有活跃 session
	p.mu.Lock()
	sessions := make([]*LongRunningSession, 0)
	for _, session := range p.sessions {
		sessions = append(sessions, session)
	}
	p.sessions = make(map[string]*LongRunningSession)
	p.mu.Unlock()

	for _, session := range sessions {
		p.sealSession(session, SealReasonServerError)
	}

	sessionPoolLogInfo("SessionPool: stopped",
		zap.Int("sealedSessions", len(sessions)))
}

// HistoryCompressor 历史压缩器
type HistoryCompressor struct {
	maxTokens int
}

// NewHistoryCompressor 创建历史压缩器
func NewHistoryCompressor(maxTokens int) *HistoryCompressor {
	return &HistoryCompressor{maxTokens: maxTokens}
}

// Compress 压缩对话历史
func (c *HistoryCompressor) Compress(buffer *ConversationBuffer) string {
	if buffer.TotalTokens <= c.maxTokens {
		return buffer.FormatFullHistory()
	}

	// 需要压缩
	var result string

	// 1. 优先保留关键实体
	keyEntities := buffer.FormatKeyEntities()
	result += keyEntities

	// 2. 保留最近的 N 轮完整对话
	recentTurns := buffer.GetRecentTurns(5)
	result += "\n## 最近对话\n\n"
	for _, turn := range recentTurns {
		role := "用户"
		if turn.Role == "agent" {
			role = "Agent"
		}
		content := turn.Content
		if len(content) > 500 {
			content = TruncateHeadTail(content, 500)
		}
		result += fmt.Sprintf("**%s**: %s\n\n", role, content)
	}

	// 3. 早期对话摘要
	oldTurns := buffer.GetOldTurns(5)
	if len(oldTurns) > 0 {
		result += "\n## 早期对话摘要\n\n"
		result += c.summarizeTurns(oldTurns)
	}

	return result
}

// summarizeTurns 摘要化早期对话
func (c *HistoryCompressor) summarizeTurns(turns []ConversationTurn) string {
	var sb strings.Builder

	// 提取关键结论
	for _, turn := range turns {
		conclusions := ExtractConclusions(turn.Content)
		for _, conclusion := range conclusions {
			sb.WriteString(fmt.Sprintf("- %s\n", conclusion))
		}
	}

	return sb.String()
}

// sessionPoolLogInfo 记录信息级别日志
func sessionPoolLogInfo(msg string, fields ...zap.Field) {
	if logger := zap.L(); logger != nil {
		logger.Info(msg, fields...)
	}
}

// sessionPoolLogError 记录错误级别日志
func sessionPoolLogError(msg string, fields ...zap.Field) {
	if logger := zap.L(); logger != nil {
		logger.Error(msg, fields...)
	}
}

// ExtractConclusions 从内容中提取结论性语句
func ExtractConclusions(content string) []string {
	// 简化实现：查找包含关键词的句子
	keywords := []string{"结论", "决定", "结果", "建议", "完成", "总结"}
	var conclusions []string

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		for _, keyword := range keywords {
			if strings.Contains(line, keyword) {
				conclusions = append(conclusions, strings.TrimSpace(line))
				break
			}
		}
	}

	return conclusions
}
// Shutdown gracefully closes all sessions in the pool.
// Seals active sessions to database for recovery after restart.
func (p *SessionPool) Shutdown() {
	p.mu.Lock()
	sessions := make([]*LongRunningSession, 0, len(p.sessions))
	sessionKeys := make([]string, 0, len(p.sessions))
	for key, session := range p.sessions {
		sessions = append(sessions, session)
		sessionKeys = append(sessionKeys, key)
	}
	// Clear sessions map immediately to prevent new requests
	p.sessions = make(map[string]*LongRunningSession)
	p.mu.Unlock()

	sessionPoolLogInfo("SessionPool: shutting down",
		zap.Int("activeCount", len(sessions)))

	// Seal all active sessions (synchronously, not async)
	for i, session := range sessions {
		if session.IsActive() {
			// Call sealSession to properly save to database
			p.sealSessionSync(session, SealReasonGracefulShutdown)
			sessionPoolLogInfo("SessionPool: sealed session",
				zap.String("sessionKey", sessionKeys[i]),
				zap.String("sessionId", session.ID))
		}

		// Stop CLI process
		if session.Adapter != nil {
			longRunning, ok := session.Adapter.(LongRunningSessionCapable)
			if ok {
				longRunning.StopLongRunningSession(session.AcpSessionID)
			}
		}
	}

	// Stop idle monitor
	if p.idleMonitorCancel != nil {
		p.idleMonitorCancel()
	}

	// Clear sessions map
	p.mu.Lock()
	p.sessions = make(map[string]*LongRunningSession)
	p.mu.Unlock()

	sessionPoolLogInfo("SessionPool: shutdown completed")
}

// sealSessionSync executes seal flow synchronously (for graceful shutdown).
// Unlike sealSession which runs async, this version blocks until complete.
func (p *SessionPool) sealSessionSync(session *LongRunningSession, reason SealReason) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sessionPoolLogInfo("SessionPool: sealing session (sync)",
		zap.String("sessionId", session.ID),
		zap.String("reason", string(reason)),
		zap.Int("turnCount", session.TurnCount))

	// 1. Persist conversation
	conversationJSON, err := session.Persist()
	if err != nil {
		sessionPoolLogError("SessionPool: failed to persist conversation",
			zap.Error(err),
			zap.String("sessionId", session.ID))
	}

	// 2. Persist key entities
	keyEntitiesJSON, _ := json.Marshal(session.Conversation.KeyEntities)

	// 3. Save to database
	record := &model.SessionRecord{
		ID:           uuid.New(),
		ThreadID:     uuid.MustParse(session.ThreadID),
		AgentID:      uuid.MustParse(session.AgentID),
		AgentType:    model.BaseAgentType("open_code"),
		Status:       string(SessionStatusSealed),
		TurnCount:    session.TurnCount,
		TotalTokens:  session.Conversation.TotalTokens,
		Conversation: conversationJSON,
		KeyEntities:  keyEntitiesJSON,
		ProcessPID:   session.ProcessPID,
		LastActiveAt: session.LastActiveAt.Unix(),
		SealedAt:     time.Now().Unix(),
		CreatedAt:    session.CreatedAt.Unix(),
		UpdatedAt:    time.Now().Unix(),
	}

	if err := p.repo.Create(ctx, record); err != nil {
		sessionPoolLogError("SessionPool: failed to save sealed record",
			zap.Error(err),
			zap.String("sessionId", session.ID))
	} else {
		sessionPoolLogInfo("SessionPool: sealed record saved",
			zap.String("sessionId", session.ID),
			zap.String("recordId", record.ID.String()))
	}

	// 4. Update status
	session.UpdateStatus(SessionStatusSealed)

	// 5. Broadcast event
	p.broadcastSessionEvent(session.ThreadID, "session_sealed", map[string]interface{}{
		"sessionId":   session.ID,
		"threadId":    session.ThreadID,
		"agentId":     session.AgentID,
		"reason":      string(reason),
		"turnCount":   session.TurnCount,
		"recoverable": true,
	})
}
