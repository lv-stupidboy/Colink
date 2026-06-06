// SessionManager 统一 Session 管理器
// 根据不同的 CLI 类型选择不同的 session 策略
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

	// IsLongRunning 是否使用长连接模式
	IsLongRunning() bool

	// GetAgentType 获取 agent 类型
	GetAgentType() model.BaseAgentType

	// GetLongRunningSession 获取长连接 session（仅长连接模式有效）
	// 返回 nil 表示非长连接模式
	GetLongRunningSession() *LongRunningSession
}

// ResumeSessionHandle Claude CLI 的 resume session 句柄
type ResumeSessionHandle struct {
	SessionID  string            `json:"sessionId"`
	Strategy   SessionStrategy   `json:"strategy"`  // new 或 resume
	AgentType  model.BaseAgentType `json:"agentType"`
	ThreadID   string            `json:"threadId"`
	AgentID    string            `json:"agentId"`
	Record     *model.SessionRecord `json:"record"` // 数据库记录
}

// LongRunningSessionHandle OpenCode/CodeAgent 的长连接 session 句柄
type LongRunningSessionHandle struct {
	Session    *LongRunningSession `json:"session"`
	AgentType  model.BaseAgentType `json:"agentType"`
}

// NewSessionHandle 创建新的 session 句柄
type NewSessionHandle struct {
	SessionID  string            `json:"sessionId"`
	AgentType  model.BaseAgentType `json:"agentType"`
	ThreadID   string            `json:"threadId"`
	AgentID    string            `json:"agentId"`
}

// === ResumeSessionHandle 方法实现 ===

func (h *ResumeSessionHandle) GetSessionID() string { return h.SessionID }
func (h *ResumeSessionHandle) GetStrategy() SessionStrategy { return h.Strategy }
func (h *ResumeSessionHandle) IsLongRunning() bool { return false }
func (h *ResumeSessionHandle) GetAgentType() model.BaseAgentType { return h.AgentType }
func (h *ResumeSessionHandle) GetLongRunningSession() *LongRunningSession { return nil }

// === LongRunningSessionHandle 方法实现 ===

func (h *LongRunningSessionHandle) GetSessionID() string { return h.Session.ID }
func (h *LongRunningSessionHandle) GetStrategy() SessionStrategy { return SessionStrategyResume }
func (h *LongRunningSessionHandle) IsLongRunning() bool { return true }
func (h *LongRunningSessionHandle) GetAgentType() model.BaseAgentType { return h.AgentType }
func (h *LongRunningSessionHandle) GetLongRunningSession() *LongRunningSession { return h.Session }

// === NewSessionHandle 方法实现 ===

func (h *NewSessionHandle) GetSessionID() string { return h.SessionID }
func (h *NewSessionHandle) GetStrategy() SessionStrategy { return SessionStrategyNew }
func (h *NewSessionHandle) IsLongRunning() bool { return false }
func (h *NewSessionHandle) GetAgentType() model.BaseAgentType { return h.AgentType }
func (h *NewSessionHandle) GetLongRunningSession() *LongRunningSession { return nil }

// SessionManager 统一 Session 管理器
// 根据不同的 CLI 类型选择不同的 session 策略
type SessionManager struct {
	// 长连接池（OpenCode/CodeAgent 使用）
	pool *SessionPool

	// Session 记录 Repository（所有类型使用）
	repo repo.SessionRecordRepository

	// 配置
	config SessionManagerConfig

	// 清理任务
	cleanupCancel context.CancelFunc
	mu            sync.RWMutex
}

// SessionManagerConfig SessionManager 配置
type SessionManagerConfig struct {
	// 长连接配置
	LongRunning SessionPoolConfig

	// Claude CLI resume 有效期（小时）
	ResumeExpiry int

	// Sealed 记录过期时间（小时）
	SealedExpiry int

	// 后台清理间隔（分钟）
	CleanupInterval int
}

// NewSessionManager 创建 SessionManager
func NewSessionManager(sessionRepo repo.SessionRecordRepository, config SessionManagerConfig) *SessionManager {
	// 默认配置
	if config.ResumeExpiry == 0 {
		config.ResumeExpiry = 168 // 7 天
	}
	if config.SealedExpiry == 0 {
		config.SealedExpiry = 24 // 24 小时
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 30 // 30 分钟
	}

	manager := &SessionManager{
		repo:   sessionRepo,
		config: config,
	}

	// 创建长连接池（仅在需要时初始化）
	// 实际初始化在 InitializeLongRunningPool 中

	// 启动后台清理任务
	manager.startCleanupTask()

	return manager
}

// InitializeLongRunningPool 初始化长连接池
// 当有 OpenCode/CodeAgent 类型的请求时调用
func (sm *SessionManager) InitializeLongRunningPool(config SessionPoolConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.pool != nil {
		return // 已初始化
	}

	sm.pool = NewSessionPool(config, sm.repo)
	sessionManagerLogInfo("SessionManager: long running pool initialized")
}

// GetOrCreateSession 获取或创建 Session
// 这是统一入口，根据 agentType 选择不同的策略
func (sm *SessionManager) GetOrCreateSession(ctx context.Context, threadID uuid.UUID, agentID uuid.UUID, baseAgent *model.BaseAgent) (SessionHandle, error) {
	agentType := baseAgent.Type
	strategy := GetSessionStrategy(agentType)

	threadIDStr := threadID.String()
	agentIDStr := agentID.String()

	sessionManagerLogInfo("SessionManager: GetOrCreateSession",
		zap.String("threadId", threadIDStr),
		zap.String("agentId", agentIDStr),
		zap.String("agentType", string(agentType)),
		zap.Bool("useLongRunning", strategy.IsLongRunningMode()),
		zap.Bool("useNativeResume", strategy.UseNativeResume))

	// === Claude CLI: 使用原生 resume ===
	if strategy.UseNativeResume {
		return sm.getOrCreateResumeSession(ctx, threadIDStr, agentIDStr, agentType, strategy)
	}

	// === OpenCode/CodeAgent: 使用长连接 ===
	if strategy.IsLongRunningMode() {
		return sm.getOrCreateLongRunningSession(ctx, threadIDStr, agentIDStr, agentType)
	}

	// === 默认: 创建新 session（无历史恢复）===
	return &NewSessionHandle{
		SessionID: uuid.New().String(),
		AgentType: agentType,
		ThreadID:  threadIDStr,
		AgentID:   agentIDStr,
	}, nil
}

// getOrCreateResumeSession Claude CLI resume 模式
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

	// 2. 检查是否有有效的 session ID
	if record != nil && record.CliSessionID != "" {
		// 检查是否过期
		if !record.IsExpired(strategy.ResumeExpiry) {
			sessionManagerLogInfo("SessionManager: using existing resume session",
				zap.String("sessionId", record.CliSessionID),
				zap.String("threadId", threadID),
				zap.String("agentId", agentID))

			return &ResumeSessionHandle{
				SessionID: record.CliSessionID,
				Strategy:  SessionStrategyResume,
				AgentType: agentType,
				ThreadID:  threadID,
				AgentID:   agentID,
				Record:    record,
			}, nil
		}

		// 过期，删除旧记录
		sessionManagerLogInfo("SessionManager: session expired, creating new",
			zap.String("oldSessionId", record.CliSessionID))
		sm.repo.Delete(ctx, record.ID)
	}

	// 3. 无有效历史，创建新 session
	return sm.createNewResumeSession(ctx, threadID, agentID, agentType)
}

// createNewResumeSession 创建新的 Claude CLI resume session
func (sm *SessionManager) createNewResumeSession(ctx context.Context, threadID, agentID string, agentType model.BaseAgentType) (SessionHandle, error) {
	newSessionID := uuid.New().String()

	// 保存到数据库
	record := &model.SessionRecord{
		ThreadID:     uuid.MustParse(threadID),
		AgentID:      uuid.MustParse(agentID),
		AgentType:    agentType,
		CliSessionID: newSessionID,
		Status:       "active",
	}
	record.SetResumeExpiry(sm.config.ResumeExpiry)

	if err := sm.repo.Create(ctx, record); err != nil {
		sessionManagerLogError("SessionManager: failed to save new session record",
			zap.Error(err))
		// 保存失败，但继续执行（不影响用户）
	}

	sessionManagerLogInfo("SessionManager: created new resume session",
		zap.String("sessionId", newSessionID),
		zap.String("threadId", threadID),
		zap.String("agentId", agentID))

	return &ResumeSessionHandle{
		SessionID: newSessionID,
		Strategy:  SessionStrategyNew,
		AgentType: agentType,
		ThreadID:  threadID,
		AgentID:   agentID,
		Record:    record,
	}, nil
}

// getOrCreateLongRunningSession OpenCode/CodeAgent 长连接模式
func (sm *SessionManager) getOrCreateLongRunningSession(ctx context.Context, threadID, agentID string, agentType model.BaseAgentType) (SessionHandle, error) {
	// 确保 pool 已初始化
	sm.mu.RLock()
	pool := sm.pool
	sm.mu.RUnlock()

	if pool == nil {
		// 初始化 pool
		sm.InitializeLongRunningPool(SessionPoolConfig{
			IdleTimeout:      10 * time.Minute,
			MaxSessions:      20,
			PersistInterval:  3,
			MaxHistoryTokens: 4000,
		})
		pool = sm.pool
	}

	// 通过 pool 获取或创建 session
	session, err := pool.GetOrCreate(ctx, threadID, agentID, agentType)
	if err != nil {
		return nil, err
	}

	return &LongRunningSessionHandle{
		Session:   session,
		AgentType: agentType,
	}, nil
}

// SaveSessionID 保存 session ID（执行完成后）
func (sm *SessionManager) SaveSessionID(ctx context.Context, threadID, agentID string, sessionID string, agentType model.BaseAgentType) error {
	// 对于 Claude CLI resume 模式，更新数据库记录
	strategy := GetSessionStrategy(agentType)
	if !strategy.UseNativeResume {
		return nil // 长连接模式由 pool 管理
	}

	threadUUID := uuid.MustParse(threadID)
	agentUUID := uuid.MustParse(agentID)

	// 查找现有记录
	record, err := sm.repo.FindByThreadAndAgent(ctx, threadID, agentID)
	if err != nil {
		// 查询失败，创建新记录
		sessionManagerLogInfo("SessionManager: no existing record, creating new",
			zap.String("threadId", threadID),
			zap.String("agentId", agentID))
	}

	if record != nil {
		// 更新现有记录
		record.CliSessionID = sessionID
		record.LastActiveAt = time.Now().Unix()
		record.SetResumeExpiry(sm.config.ResumeExpiry)
		return sm.repo.Update(ctx, record)
	}

	// 创建新记录
	newRecord := &model.SessionRecord{
		ThreadID:     threadUUID,
		AgentID:      agentUUID,
		AgentType:    agentType,
		CliSessionID: sessionID,
		Status:       "active",
	}
	newRecord.SetResumeExpiry(sm.config.ResumeExpiry)

	return sm.repo.Create(ctx, newRecord)
}

// MarkIdle 标记 session 为空闲（长连接模式）
func (sm *SessionManager) MarkIdle(sessionKey string) {
	sm.mu.RLock()
	pool := sm.pool
	sm.mu.RUnlock()

	if pool != nil {
		pool.MarkIdle(sessionKey)
	}
}

// Cancel 取消 session
func (sm *SessionManager) Cancel(ctx context.Context, threadID, agentID string, agentType model.BaseAgentType) error {
	strategy := GetSessionStrategy(agentType)

	if strategy.IsLongRunningMode() {
		// 长连接模式：通过 pool 取消
		sessionKey := GetSessionKey(threadID, agentID)
		sm.mu.RLock()
		pool := sm.pool
		sm.mu.RUnlock()

		if pool != nil {
			return pool.Cancel(sessionKey)
		}
		return nil
	}

	// Claude CLI resume 模式：删除数据库记录
	record, err := sm.repo.FindByThreadAndAgent(ctx, threadID, agentID)
	if err != nil {
		return nil // 无记录，无需删除
	}

	return sm.repo.Delete(ctx, record.ID)
}

// GetMetrics 获取 session 统计信息
func (sm *SessionManager) GetMetrics(ctx context.Context) SessionMetrics {
	metrics := SessionMetrics{}

	sm.mu.RLock()
	pool := sm.pool
	sm.mu.RUnlock()

	// 长连接 pool 统计
	if pool != nil {
		metrics.ActiveSessions = 0
		metrics.IdleSessions = 0
		metrics.SealedSessions = 0

		for _, session := range pool.GetAll() {
			switch session.Status {
			case SessionStatusActive:
				metrics.ActiveSessions++
			case SessionStatusIdle:
				metrics.IdleSessions++
			case SessionStatusSealed:
				metrics.SealedSessions++
			}
		}
	}

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
	// 1. 清理过期的 Claude CLI resume 记录
	expiryDuration := time.Duration(sm.config.ResumeExpiry) * time.Hour
	if err := sm.repo.DeleteExpiredRecords(ctx, expiryDuration); err != nil {
		sessionManagerLogError("SessionManager: failed to delete expired resume records",
			zap.Error(err))
	}

	// 2. 清理过期的 Sealed 记录（长连接模式）
	sealedExpiry := time.Duration(sm.config.SealedExpiry) * time.Hour
	if err := sm.repo.DeleteSealedRecords(ctx, sealedExpiry); err != nil {
		sessionManagerLogError("SessionManager: failed to delete expired sealed records",
			zap.Error(err))
	}

	sessionManagerLogInfo("SessionManager: cleanup completed")
}

// SetWSHub 设置 WebSocket 广播器
// 用于向前端通知 session 状态变化（sealed、recovered 等）
func (sm *SessionManager) SetWSHub(broadcaster SessionBroadcaster) {
	sm.mu.Lock()
	if sm.pool != nil {
		sm.pool.SetWSHub(broadcaster)
	}
	sm.mu.Unlock()

	sessionManagerLogInfo("SessionManager: WSHub set for session pool")
}

// Stop 停止 SessionManager
func (sm *SessionManager) Stop() {
	// 停止清理任务
	if sm.cleanupCancel != nil {
		sm.cleanupCancel()
	}

	// 停止长连接 pool
	sm.mu.RLock()
	pool := sm.pool
	sm.mu.RUnlock()

	if pool != nil {
		pool.Stop()
	}

	sessionManagerLogInfo("SessionManager: stopped")
}

// SessionMetrics Session 统计信息
type SessionMetrics struct {
	ActiveSessions     int `json:"activeSessions"`
	IdleSessions       int `json:"idleSessions"`
	SealedSessions     int `json:"sealedSessions"`
	RecoveringSessions int `json:"recoveringSessions"`

	TotalSessionsCreated int `json:"totalSessionsCreated"`
	TotalSessionsSealed  int `json:"totalSessionsSealed"`
	TotalSessionsRecovered int `json:"totalSessionsRecovered"`
}

// sql.ErrNoRows 常量（避免导入 database/sql）
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
// 封存所有活跃的 session 到数据库，确保重启后可以恢复
func (sm *SessionManager) Shutdown() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 停止清理任务
	if sm.cleanupCancel != nil {
		sm.cleanupCancel()
	}

	// 关闭长连接池中的所有 session
	if sm.pool != nil {
		sm.pool.Shutdown()
	}

	sessionManagerLogInfo("SessionManager: shutdown completed")
	return nil
}