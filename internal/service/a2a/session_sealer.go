package a2a

import (
	"context"
	"sync"
	"time"
)

// SealResult 封存结果
type SealResult struct {
	Accepted  bool          `json:"accepted"`
	Status    SessionStatus `json:"status"`
	SessionID string        `json:"sessionId,omitempty"`
}

// SessionSealerDeps SessionSealer 依赖
type SessionSealerDeps struct {
	Store *SessionChainStore
}

const finalizeTimeoutMs = 30000

// SessionSealer 会话封存器
// 参考 Clowder AI 的 SessionSealer
// 管理会话生命周期转换：active → sealing → sealed
type SessionSealer struct {
	deps    SessionSealerDeps
	pending sync.Map // sessionId -> bool (正在 finalize 的会话)
}

// NewSessionSealer 创建会话封存器
func NewSessionSealer(deps SessionSealerDeps) *SessionSealer {
	return &SessionSealer{
		deps: deps,
	}
}

// RequestSeal 请求封存会话
// 幂等：如果已经是 sealing/sealed，返回 accepted=false
// 快速路径：仅修改状态 + 清除活跃指针
func (s *SessionSealer) RequestSeal(ctx context.Context, sessionID string, reason SealReason) SealResult {
	if s.deps.Store == nil {
		return SealResult{Accepted: false, Status: SessionStatusSealed}
	}

	record := s.deps.Store.Get(sessionID)
	if record == nil {
		return SealResult{Accepted: false, Status: SessionStatusSealed}
	}

	// CAS: 只有 active 会话才能封存
	currentStatus := record.Status
	if currentStatus != SessionStatusActive {
		return SealResult{Accepted: false, Status: currentStatus}
	}

	// 转换 active → sealing
	now := time.Now().UnixMilli()
	updated := s.deps.Store.Update(sessionID, SessionRecordPatch{
		Status:    ptrSessionStatus(SessionStatusSealing),
		SealReason: &reason,
		UpdatedAt: &now,
	})

	if updated == nil || updated.Status != SessionStatusSealing {
		// 竞态条件：其他调用者已经修改了状态
		status := SessionStatusSealed
		if updated != nil {
			status = updated.Status
		}
		return SealResult{Accepted: false, Status: status}
	}

	return SealResult{
		Accepted:  true,
		Status:    SessionStatusSealing,
		SessionID: sessionID,
	}
}

// Finalize 完成封存
// 慢速路径：transcript JSONL flush + digest + mark sealed
// 当前 Phase B：仅转换 sealing → sealed
func (s *SessionSealer) Finalize(ctx context.Context, sessionID string) error {
	if s.deps.Store == nil {
		return nil
	}

	record := s.deps.Store.Get(sessionID)
	if record == nil {
		return nil
	}

	// 只能 finalize sealing 状态的会话
	if record.Status != SessionStatusSealing {
		return nil
	}

	// 防止重复 finalize
	if _, pending := s.pending.Load(sessionID); pending {
		return nil
	}
	s.pending.Store(sessionID, true)
	defer s.pending.Delete(sessionID)

	now := time.Now().UnixMilli()

	// Phase B: 直接转换 sealing → sealed
	// Phase C 会添加 transcript + digest 逻辑
	_, err := s.doFinalize(ctx, record, now)
	if err != nil {
		// Finalize 失败 — 强制封存以防止卡在 sealing 状态
		// 记录日志（简化处理）
	}

	// 始终尝试终端转换 — 即使 doFinalize 失败/超时
	// 缺少 transcript 的 sealed 会话可恢复；卡在 sealing 的会话不可恢复
	s.deps.Store.Update(sessionID, SessionRecordPatch{
		Status:    ptrSessionStatus(SessionStatusSealed),
		SealedAt:  &now,
		UpdatedAt: &now,
	})

	return nil
}

// doFinalize 执行实际封存逻辑
// Phase B: 简化实现
func (s *SessionSealer) doFinalize(ctx context.Context, record *SessionRecord, now int64) (interface{}, error) {
	// Phase B: 直接返回成功
	// Phase C 会添加：
	// 1. Transcript JSONL flush
	// 2. Extractive digest 生成
	// 3. ThreadMemory 更新
	// 4. Handoff digest 生成

	return nil, nil
}

// ReconcileStuck 调和卡在 sealing 状态的会话
// 扫描指定 cat/thread 的所有会话，强制封存超过 maxAgeMs 的 sealing 会话
// 返回调和的会话数量
func (s *SessionSealer) ReconcileStuck(ctx context.Context, catID, threadID string, maxAgeMs ...int64) int {
	if s.deps.Store == nil {
		return 0
	}

	maxAge := int64(5 * 60 * 1000) // 默认 5 分钟
	if len(maxAgeMs) > 0 && maxAgeMs[0] > 0 {
		maxAge = maxAgeMs[0]
	}

	sessions := s.deps.Store.GetChain(catID, threadID)
	now := time.Now().UnixMilli()
	count := 0

	for _, session := range sessions {
		if session.Status == SessionStatusSealing {
			updatedAt := session.UpdatedAt
			if updatedAt == 0 {
				updatedAt = session.CreatedAt
			}
			if now-updatedAt > maxAge {
				s.deps.Store.Update(session.ID, SessionRecordPatch{
					Status:    ptrSessionStatus(SessionStatusSealed),
					SealedAt:  &now,
					UpdatedAt: &now,
				})
				count++
			}
		}
	}

	return count
}

// ReconcileAllStuck 全局调和器
// 调和所有 cat/thread 中卡在 sealing 状态的会话
// 启动时和定期运行，捕获永远不会被 per-invoke 懒扫描访问的孤立 sealing 会话
// 返回调和的会话总数
func (s *SessionSealer) ReconcileAllStuck(ctx context.Context, maxAgeMs ...int64) int {
	if s.deps.Store == nil {
		return 0
	}

	maxAge := int64(5 * 60 * 1000) // 默认 5 分钟
	if len(maxAgeMs) > 0 && maxAgeMs[0] > 0 {
		maxAge = maxAgeMs[0]
	}

	sealingIDs := s.deps.Store.ListSealingSessions()
	if len(sealingIDs) == 0 {
		return 0
	}

	now := time.Now().UnixMilli()
	count := 0

	for _, id := range sealingIDs {
		session := s.deps.Store.Get(id)
		if session == nil || session.Status != SessionStatusSealing {
			continue
		}

		updatedAt := session.UpdatedAt
		if updatedAt == 0 {
			updatedAt = session.CreatedAt
		}

		if now-updatedAt > maxAge {
			s.deps.Store.Update(id, SessionRecordPatch{
				Status:    ptrSessionStatus(SessionStatusSealed),
				SealedAt:  &now,
				UpdatedAt: &now,
			})
			count++
		}
	}

	return count
}

// 辅助函数

func ptrSessionStatus(s SessionStatus) *SessionStatus {
	return &s
}

func ptrSealReason(r SealReason) *SealReason {
	return &r
}

func ptrInt64(v int64) *int64 {
	return &v
}

// 全局 SessionSealer 实例
var globalSessionSealer *SessionSealer
var sessionSealerOnce sync.Once

// GetSessionSealer 获取全局 SessionSealer
func GetSessionSealer() *SessionSealer {
	sessionSealerOnce.Do(func() {
		globalSessionSealer = NewSessionSealer(SessionSealerDeps{
			Store: globalSessionChainStore,
		})
	})
	return globalSessionSealer
}

// InitSessionSealer 初始化全局 SessionSealer（用于自定义依赖）
func InitSessionSealer(deps SessionSealerDeps) {
	sessionSealerOnce.Do(func() {
		globalSessionSealer = NewSessionSealer(deps)
	})
}