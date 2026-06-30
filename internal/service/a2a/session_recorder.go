package a2a

import (
	"context"

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