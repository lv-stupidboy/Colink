package a2a

import (
	"context"
	"time"

	"github.com/anthropic/isdp/internal/service/agent"
	"go.uber.org/zap"
)

// SessionRecorderImpl 实现 agent.SessionRecorder 接口
// 用于记录执行失败和成功的会话
type SessionRecorderImpl struct{}

// NewSessionRecorderImpl 创建 SessionRecorder 实现
func NewSessionRecorderImpl() *SessionRecorderImpl {
	return &SessionRecorderImpl{}
}

// RecordFailedSession 记录执行失败的会话
// 创建 SessionRecord 并封存（标记为错误结束）
func (r *SessionRecorderImpl) RecordFailedSession(threadID, configID, sessionID string) {
	if sessionID == "" {
		return
	}

	store := GetSessionChainStore()
	sealer := GetSessionSealer()

	// 创建会话记录
	record := store.Create(CreateSessionInput{
		CLISessionID: sessionID,
		ThreadID:     threadID,
		CatID:        configID,
		UserID:       "",
	})

	if record != nil {
		// 封存会话（标记为错误结束）
		sealResult := sealer.RequestSeal(context.Background(), record.ID, SealReasonError)
		if sealResult.Accepted {
			// 异步完成封存（不阻塞主流程）
			go func() {
				_ = sealer.Finalize(context.Background(), record.ID)
			}()
		}
		zap.L().Info("Failed session recorded and sealed",
			zap.String("sessionId", sessionID),
			zap.String("threadId", threadID),
			zap.String("catId", configID),
			zap.String("recordId", record.ID),
			zap.Bool("sealAccepted", sealResult.Accepted))
	}
}

// RecordSuccessfulSession 记录执行成功的会话
// 创建 SessionRecord（默认为 active 状态）
func (r *SessionRecorderImpl) RecordSuccessfulSession(threadID, configID, sessionID string) {
	if sessionID == "" {
		return
	}

	store := GetSessionChainStore()

	// 创建会话记录
	record := store.Create(CreateSessionInput{
		CLISessionID: sessionID,
		ThreadID:     threadID,
		CatID:        configID,
		UserID:       "",
	})

	if record != nil {
		zap.L().Info("Successful session recorded",
			zap.String("sessionId", sessionID),
			zap.String("threadId", threadID),
			zap.String("catId", configID),
			zap.String("recordId", record.ID))
	}
}

// RecordPendingSession 记录待执行会话（CLI 执行前持久化）
// 确保 CLI 取消或崩溃后仍能通过 sessionID resume 上下文
func (r *SessionRecorderImpl) RecordPendingSession(threadID, configID, sessionID string) {
	if sessionID == "" {
		return
	}

	store := GetSessionChainStore()

	// 创建会话记录（active 状态，表示正在执行）
	record := store.Create(CreateSessionInput{
		CLISessionID: sessionID,
		ThreadID:     threadID,
		CatID:        configID,
		UserID:       "",
	})

	if record != nil {
		zap.L().Info("Pending session recorded (before CLI execution)",
			zap.String("sessionId", sessionID),
			zap.String("threadId", threadID),
			zap.String("catId", configID),
			zap.String("recordId", record.ID))
	}
}

// IncrementConsecutiveFailures 增加连续恢复失败计数
func (r *SessionRecorderImpl) IncrementConsecutiveFailures(threadID, configID, sessionID string) {
	if sessionID == "" {
		return
	}

	store := GetSessionChainStore()
	store.IncrementConsecutiveFailures(configID, sessionID)
	zap.L().Warn("ConsecutiveRestoreFailures incremented",
		zap.String("sessionId", sessionID),
		zap.String("threadId", threadID),
		zap.String("catId", configID))
}

// ResetConsecutiveFailures 重置连续恢复失败计数
func (r *SessionRecorderImpl) ResetConsecutiveFailures(threadID, configID, sessionID string) {
	if sessionID == "" {
		return
	}

	store := GetSessionChainStore()
	store.ResetConsecutiveFailures(configID, sessionID)
	zap.L().Info("ConsecutiveRestoreFailures reset",
		zap.String("sessionId", sessionID),
		zap.String("threadId", threadID),
		zap.String("catId", configID))
}

// CheckAndSealOnOverflow Circuit Breaker 检查，返回是否应该终止执行
func (r *SessionRecorderImpl) CheckAndSealOnOverflow(threadID, configID string) bool {
	store := GetSessionChainStore()

	// 获取活跃 session
	sessionRecord := store.GetActive(configID, threadID)
	if sessionRecord == nil {
		return false
	}

	// 检查连续失败次数是否达到阈值
	if sessionRecord.ConsecutiveRestoreFailures >= agent.MAX_CONSECUTIVE_FAILURES {
		// 标记 session 为 sealed 状态
		sessionRecord.Status = SessionStatusSealed
		sessionRecord.SealReason = SealReasonThreshold
		sessionRecord.SealedAt = new(int64)
		*sessionRecord.SealedAt = time.Now().Unix()

		zap.L().Warn("Circuit Breaker triggered, sealing session",
			zap.String("threadId", threadID),
			zap.String("configId", configID),
			zap.Int("consecutiveFailures", sessionRecord.ConsecutiveRestoreFailures))
		return true
	}

	return false
}