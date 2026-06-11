// SessionManager 统一 Session 管理器
// 使用 ACP 原生 session/resume 能力
package agent

import (
	"context"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SessionHandle Session 句柄接口
// 统一不同 CLI 类型的 session 处理方式
type SessionHandle interface {
	// GetSessionID 获取 session ID
	GetSessionID() string

	// GetStrategy 获取 session 策略
	GetStrategy() SessionStrategy

	// GetAgentType 获取 agent 类型
	GetAgentType() model.BaseAgentType

	// GetACPSessionID 获取 ACP session ID（用于 resume）
	GetACPSessionID() string
}

// ResumeSessionHandle Resume session 句柄
type ResumeSessionHandle struct {
	SessionID     string              `json:"sessionId"`
	ACPSessionID  string              `json:"acpSessionId"` // ACP 协议的 session ID
	Strategy      SessionStrategy     `json:"strategy"`     // new 或 resume
	AgentType     model.BaseAgentType `json:"agentType"`
	ThreadID      string              `json:"threadId"`
	AgentID       string              `json:"agentId"`
	Record        *model.SessionRecord `json:"record"` // 数据库记录
}

// NewSessionHandle 创建新的 session 句柄
type NewSessionHandle struct {
	SessionID  string              `json:"sessionId"`
	AgentType  model.BaseAgentType `json:"agentType"`
	ThreadID   string              `json:"threadId"`
	AgentID    string              `json:"agentId"`
}

// === ResumeSessionHandle 方法实现 ===

func (h *ResumeSessionHandle) GetSessionID() string { return h.SessionID }
func (h *ResumeSessionHandle) GetStrategy() SessionStrategy { return h.Strategy }
func (h *ResumeSessionHandle) GetAgentType() model.BaseAgentType { return h.AgentType }
func (h *ResumeSessionHandle) GetACPSessionID() string { return h.ACPSessionID }

// === NewSessionHandle 方法实现 ===

func (h *NewSessionHandle) GetSessionID() string { return h.SessionID }
func (h *NewSessionHandle) GetStrategy() SessionStrategy { return SessionStrategyNew }
func (h *NewSessionHandle) GetAgentType() model.BaseAgentType { return h.AgentType }
func (h *NewSessionHandle) GetACPSessionID() string { return "" } // 新 session 无 ACP ID

// SessionManager 统一 Session 管理器
// 使用 ACP 原生 session/resume 能力管理会话
type SessionManager struct {
	// Session 记录 Repository
	repo repo.SessionRecordRepository

	// 配置
	config SessionManagerConfig

	// 清理任务
	cleanupCancel context.CancelFunc
	mu            sync.RWMutex
}

// SessionManagerConfig SessionManager 配置
type SessionManagerConfig struct {
	// Resume 有效期（小时）
	ResumeExpiry int

	// 后台清理间隔（分钟）
	CleanupInterval int
}

// NewSessionManager 创建 SessionManager
func NewSessionManager(sessionRepo repo.SessionRecordRepository, config SessionManagerConfig) *SessionManager {
	// 默认配置
	if config.ResumeExpiry == 0 {
		config.ResumeExpiry = 168 // 7 天
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 30 // 30 分钟
	}

	manager := &SessionManager{
		repo:   sessionRepo,
		config: config,
	}

	// 启动后台清理任务
	manager.startCleanupTask()

	return manager
}

// GetOrCreateSession 获取或创建 Session
// 使用 ACP 原生 session/resume 策略
func (sm *SessionManager) GetOrCreateSession(ctx context.Context, threadID uuid.UUID, agentID uuid.UUID, baseAgent *model.BaseAgent) (SessionHandle, error) {
	agentType := baseAgent.Type
	strategy := GetSessionStrategy(agentType)

	threadIDStr := threadID.String()
	agentIDStr := agentID.String()

	sessionManagerLogInfo("SessionManager: GetOrCreateSession",
		zap.String("threadId", threadIDStr),
		zap.String("agentId", agentIDStr),
		zap.String("agentType", string(agentType)),
		zap.Bool("useNativeResume", strategy.UseNativeResume))

	// === 使用 ACP 原生 resume ===
	if strategy.UseNativeResume {
		return sm.getOrCreateResumeSession(ctx, threadIDStr, agentIDStr, agentType, strategy)
	}

	// === 默认: 创建新 session（无历史恢复）===
	return &NewSessionHandle{
		SessionID: uuid.New().String(),
		AgentType: agentType,
		ThreadID:  threadIDStr,
		AgentID:   agentIDStr,
	}, nil
}

// getOrCreateResumeSession 获取或创建 resume session
func (sm *SessionManager) getOrCreateResumeSession(ctx context.Context, threadID, agentID string, agentType model.BaseAgentType, strategy SessionStrategyConfig) (SessionHandle, error) {
	// 1. 从数据库查询历史 session ID
	record, err := sm.repo.FindByThreadAndAgent(ctx, threadID, agentID)
	if err != nil {
		sessionManagerLogError("SessionManager: failed to find session record",
			zap.Error(err),
			zap.String("threadId", threadID),
			zap.String("agentId", agentID))
		// 查询失败，创建新 session
		return sm.createNewResumeSession(ctx, threadID, agentID, agentType)
	}

	// 2. 检查是否有有效的 ACP session ID
	if record != nil && record.AcpSessionID != "" {
		// 检查是否过期
		if !record.IsExpired(strategy.ResumeExpiry) {
			sessionManagerLogInfo("SessionManager: using existing resume session",
				zap.String("acpSessionId", record.AcpSessionID),
				zap.String("threadId", threadID),
				zap.String("agentId", agentID))

			return &ResumeSessionHandle{
				SessionID:    record.ID.String(),
				ACPSessionID: record.AcpSessionID,
				Strategy:     SessionStrategyResume,
				AgentType:    agentType,
				ThreadID:     threadID,
				AgentID:      agentID,
				Record:       record,
			}, nil
		}

		// 过期，删除旧记录
		sessionManagerLogInfo("SessionManager: session expired, creating new",
			zap.String("oldAcpSessionId", record.AcpSessionID))
		sm.repo.Delete(ctx, record.ID)
	}

	// 3. 无有效历史，创建新 session
	return sm.createNewResumeSession(ctx, threadID, agentID, agentType)
}

// createNewResumeSession 创建新的 resume session
func (sm *SessionManager) createNewResumeSession(ctx context.Context, threadID, agentID string, agentType model.BaseAgentType) (SessionHandle, error) {
	newSessionID := uuid.New().String()

	sessionManagerLogInfo("SessionManager: created new resume session",
		zap.String("sessionId", newSessionID),
		zap.String("threadId", threadID),
		zap.String("agentId", agentID))

	return &ResumeSessionHandle{
		SessionID:    newSessionID,
		ACPSessionID: "", // 执行后会填充
		Strategy:     SessionStrategyNew,
		AgentType:    agentType,
		ThreadID:     threadID,
		AgentID:      agentID,
		Record:       nil,
	}, nil
}

// SaveACPSessionID 保存 ACP session ID（执行完成后）
func (sm *SessionManager) SaveACPSessionID(ctx context.Context, threadID, agentID string, acpSessionID string, agentType model.BaseAgentType) error {
	threadUUID := uuid.MustParse(threadID)
	agentUUID := uuid.MustParse(agentID)

	// 查找现有记录
	record, err := sm.repo.FindByThreadAndAgent(ctx, threadID, agentID)
	if err != nil {
		sessionManagerLogInfo("SessionManager: no existing record, creating new",
			zap.String("threadId", threadID),
			zap.String("agentId", agentID))
	}

	if record != nil {
		// 更新现有记录
		record.AcpSessionID = acpSessionID
		record.LastActiveAt = time.Now().Unix()
		record.SetResumeExpiry(sm.config.ResumeExpiry)
		return sm.repo.Update(ctx, record)
	}

	// 创建新记录
	newRecord := &model.SessionRecord{
		ID:           uuid.New(),
		ThreadID:     threadUUID,
		AgentID:      agentUUID,
		AgentType:    agentType,
		AcpSessionID: acpSessionID,
		Status:       "active",
	}
	newRecord.SetResumeExpiry(sm.config.ResumeExpiry)

	return sm.repo.Create(ctx, newRecord)
}

// Cancel 取消 session
func (sm *SessionManager) Cancel(ctx context.Context, threadID, agentID string, agentType model.BaseAgentType) error {
	// 删除数据库记录
	record, err := sm.repo.FindByThreadAndAgent(ctx, threadID, agentID)
	if err != nil {
		return nil // 无记录，无需删除
	}

	return sm.repo.Delete(ctx, record.ID)
}

// GetMetrics 获取 session 统计信息
func (sm *SessionManager) GetMetrics(ctx context.Context) SessionMetrics {
	metrics := SessionMetrics{}
	// 数据库统计（可选）
	// ...
	return metrics
}

// startCleanupTask 启动后台清理任务
func (sm *SessionManager) startCleanupTask() {
	ctx, cancel := context.WithCancel(context.Background())
	sm.cleanupCancel = cancel

	go func() {
		ticker := time.NewTicker(time.Duration(sm.config.CleanupInterval) * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				sm.cleanupExpiredRecords(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()

	sessionManagerLogInfo("SessionManager: cleanup task started",
		zap.Int("intervalMinutes", sm.config.CleanupInterval))
}

// cleanupExpiredRecords 清理过期记录
func (sm *SessionManager) cleanupExpiredRecords(ctx context.Context) {
	// 清理过期的 resume 记录
	expiryDuration := time.Duration(sm.config.ResumeExpiry) * time.Hour
	if err := sm.repo.DeleteExpiredRecords(ctx, expiryDuration); err != nil {
		sessionManagerLogError("SessionManager: failed to delete expired resume records",
			zap.Error(err))
	}

	sessionManagerLogInfo("SessionManager: cleanup completed")
}

// Stop 停止 SessionManager
func (sm *SessionManager) Stop() {
	// 停止清理任务
	if sm.cleanupCancel != nil {
		sm.cleanupCancel()
	}

	sessionManagerLogInfo("SessionManager: stopped")
}

// SessionMetrics Session 统计信息
type SessionMetrics struct {
	ActiveSessions     int `json:"activeSessions"`
	TotalSessionsCreated int `json:"totalSessionsCreated"`
}

// sessionManagerLogInfo 记录信息级别日志
func sessionManagerLogInfo(msg string, fields ...zap.Field) {
	if logger := zap.L(); logger != nil {
		logger.Info(msg, fields...)
	}
}

// sessionManagerLogError 记录错误级别日志
func sessionManagerLogError(msg string, fields ...zap.Field) {
	if logger := zap.L(); logger != nil {
		logger.Error(msg, fields...)
	}
}

// Shutdown 优雅关闭 SessionManager
func (sm *SessionManager) Shutdown() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 停止清理任务
	if sm.cleanupCancel != nil {
		sm.cleanupCancel()
	}

	sessionManagerLogInfo("SessionManager: shutdown completed")
	return nil
}