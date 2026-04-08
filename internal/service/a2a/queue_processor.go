package a2a

import (
	"context"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
)

// QueueProcessorDeps QueueProcessor 依赖
type QueueProcessorDeps struct {
	Queue          *InvocationQueue
	Registry       *InvocationRegistry
	WSHub          *ws.Hub
	SpawnAgent     func(ctx context.Context, threadID uuid.UUID, catID string, content string) error
	MessageUpdater func(ctx context.Context, messageID string, deliveredAt int64) error
}

// QueueProcessor 队列处理器
// 处理 InvocationQueue 中的排队条目：自动出队 + 暂停管理
type QueueProcessor struct {
	deps           QueueProcessorDeps
	processingSlots sync.Map // threadID:catID -> bool (per-slot mutex)
	pausedSlots     sync.Map // threadID:catID -> string (pause reason: "canceled" | "failed")
}

// NewQueueProcessor 创建队列处理器
func NewQueueProcessor(deps QueueProcessorDeps) *QueueProcessor {
	return &QueueProcessor{
		deps: deps,
	}
}

// OnInvocationComplete invocation 完成后调用
// - succeeded → 自动出队下一个
// - canceled/failed → 暂停 slot，通知用户
func (p *QueueProcessor) OnInvocationComplete(ctx context.Context, threadID uuid.UUID, catID string, status string) error {
	slotKey := slotKey(threadID, catID)

	if status == "succeeded" {
		// 清除暂停状态
		p.pausedSlots.Delete(slotKey)

		// 检查是否有排队条目
		if p.deps.Queue != nil && p.deps.Queue.HasQueuedForThread(threadID.String()) {
			// 尝试执行下一个
			if err := p.tryExecuteNextAcrossUsers(ctx, threadID); err != nil {
				return err
			}
			// 尝试自动执行
			if err := p.tryAutoExecute(ctx, threadID); err != nil {
				return err
			}
		}
	} else {
		// canceled 或 failed
		if p.deps.Queue != nil && p.deps.Queue.HasQueuedForThread(threadID.String()) {
			p.pausedSlots.Store(slotKey, status)
			p.emitPausedToQueuedUsers(threadID, status)
		}
	}

	return nil
}

// IsPaused 检查 slot 是否暂停
func (p *QueueProcessor) IsPaused(threadID uuid.UUID, catID string) bool {
	slotKey := slotKey(threadID, catID)
	_, paused := p.pausedSlots.Load(slotKey)
	if !paused {
		return false
	}
	// 检查是否还有排队条目
	if p.deps.Queue != nil {
		return p.deps.Queue.HasQueuedForThread(threadID.String())
	}
	return false
}

// GetPauseReason 获取暂停原因
func (p *QueueProcessor) GetPauseReason(threadID uuid.UUID, catID string) string {
	slotKey := slotKey(threadID, catID)
	if reason, ok := p.pausedSlots.Load(slotKey); ok {
		return reason.(string)
	}
	return ""
}

// ClearPause 清除暂停状态
func (p *QueueProcessor) ClearPause(threadID uuid.UUID, catID string) {
	if catID != "" {
		p.pausedSlots.Delete(slotKey(threadID, catID))
	} else {
		// 清除该线程的所有暂停状态
		p.pausedSlots.Range(func(key, value interface{}) bool {
			k := key.(string)
			if len(k) > 36 && k[:36] == threadID.String() {
				p.pausedSlots.Delete(key)
			}
			return true
		})
	}
}

// ReleaseSlot 释放 slot mutex
func (p *QueueProcessor) ReleaseSlot(threadID uuid.UUID, catID string) {
	p.processingSlots.Delete(slotKey(threadID, catID))
}

// HasQueuedForThread 检查线程是否有排队条目
func (p *QueueProcessor) HasQueuedForThread(threadID uuid.UUID) bool {
	if p.deps.Queue == nil {
		return false
	}
	return p.deps.Queue.HasQueuedForThread(threadID.String())
}

// HasQueuedAgentForCat 检查指定 Agent 是否有排队条目 (A2A 去重)
func (p *QueueProcessor) HasQueuedAgentForCat(threadID uuid.UUID, catID string) bool {
	if p.deps.Queue == nil {
		return false
	}
	return p.deps.Queue.HasQueuedAgent(threadID.String(), catID)
}

// tryExecuteNextAcrossUsers 尝试跨用户执行下一个排队条目
func (p *QueueProcessor) tryExecuteNextAcrossUsers(ctx context.Context, threadID uuid.UUID) error {
	if p.deps.Queue == nil {
		return nil
	}

	// 获取最早的排队条目
	entry := p.deps.Queue.PeekOldestAcrossUsers(threadID.String())
	if entry == nil {
		return nil
	}

	// 检查 slot mutex
	entryCat := ""
	if len(entry.TargetAgents) > 0 {
		entryCat = entry.TargetAgents[0]
	}
	slotKeyStr := slotKey(threadID, entryCat)

	if _, busy := p.processingSlots.Load(slotKeyStr); busy {
		return nil
	}

	// 检查是否已有活跃调用
	if p.deps.Registry != nil && p.deps.Registry.HasActiveSlot(threadID, entryCat) {
		return nil
	}

	// 标记为处理中
	p.processingSlots.Store(slotKeyStr, true)

	// 标记条目为 processing
	markedEntry := p.deps.Queue.MarkProcessing(threadID, entry.UserID)
	if markedEntry == nil || markedEntry.ID != entry.ID {
		p.processingSlots.Delete(slotKeyStr)
		return nil
	}

	// 执行条目
	go p.executeEntry(context.Background(), markedEntry)

	return nil
}

// tryAutoExecute 尝试自动执行 autoExecute 条目
func (p *QueueProcessor) tryAutoExecute(ctx context.Context, threadID uuid.UUID) error {
	if p.deps.Queue == nil {
		return nil
	}

	// 获取所有 autoExecute 条目
	entries := p.deps.Queue.ListAutoExecute(threadID.String())

	for _, entry := range entries {
		if entry == nil {
			continue
		}

		entryCat := ""
		if len(entry.TargetAgents) > 0 {
			entryCat = entry.TargetAgents[0]
		}
		slotKeyStr := slotKey(threadID, entryCat)

		// 跳过忙碌的 slot
		if _, busy := p.processingSlots.Load(slotKeyStr); busy {
			continue
		}

		// 跳过已有活跃调用的 slot
		if p.deps.Registry != nil && p.deps.Registry.HasActiveSlot(threadID, entryCat) {
			continue
		}

		// 标记为处理中
		if !p.deps.Queue.MarkProcessingById(threadID.String(), entry.ID.String()) {
			continue
		}

		p.processingSlots.Store(slotKeyStr, true)

		// 异步执行
		go p.executeEntry(context.Background(), entry)
	}

	return nil
}

// TryAutoExecute 公共方法：尝试自动执行 autoExecute 条目
func (p *QueueProcessor) TryAutoExecute(ctx context.Context, threadID uuid.UUID) error {
	return p.tryAutoExecute(ctx, threadID)
}

// executeEntry 执行队列条目
func (p *QueueProcessor) executeEntry(ctx context.Context, entry *QueueEntry) {
	if entry == nil {
		return
	}

	defer func() {
		// 清理 slot mutex
		if len(entry.TargetAgents) > 0 {
			p.ReleaseSlot(entry.ThreadID, entry.TargetAgents[0])
		}
	}()

	threadID := entry.ThreadID
	primaryCat := ""
	if len(entry.TargetAgents) > 0 {
		primaryCat = entry.TargetAgents[0]
	}

	var finalStatus string = "failed"

	// 调用 SpawnAgent
	if p.deps.SpawnAgent != nil {
		if err := p.deps.SpawnAgent(ctx, threadID, primaryCat, entry.Content); err != nil {
			finalStatus = "failed"
		} else {
			finalStatus = "succeeded"
		}
	}

	// 标记消息已投递
	if p.deps.MessageUpdater != nil && entry.MessageID != nil {
		_ = p.deps.MessageUpdater(ctx, entry.MessageID.String(), time.Now().UnixMilli())
	}

	// 移除已处理的条目
	if p.deps.Queue != nil {
		p.deps.Queue.RemoveProcessedAcrossUsers(threadID.String(), entry.ID.String())
	}

	// 广播队列更新
	if p.deps.WSHub != nil {
		p.deps.WSHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "queue_updated",
			ThreadID:  threadID.String(),
			Timestamp: model.Now(),
			Payload: map[string]interface{}{
				"action": "completed",
				"status": finalStatus,
			},
		})
	}

	// 触发下一个
	if finalStatus == "succeeded" {
		_ = p.OnInvocationComplete(ctx, threadID, primaryCat, finalStatus)
	}
}

// emitPausedToQueuedUsers 通知有排队条目的用户队列已暂停
func (p *QueueProcessor) emitPausedToQueuedUsers(threadID uuid.UUID, reason string) {
	if p.deps.Queue == nil || p.deps.WSHub == nil {
		return
	}

	users := p.deps.Queue.ListUsersForThread(threadID.String())
	for _, userID := range users {
		queue := p.deps.Queue.List(threadID, userID)
		hasQueued := false
		for _, e := range queue {
			if e.Status == "queued" {
				hasQueued = true
				break
			}
		}
		if hasQueued {
			p.deps.WSHub.BroadcastToThread(threadID.String(), ws.WSMessage{
				Type:      "queue_paused",
				ThreadID:  threadID.String(),
				Timestamp: model.Now(),
				Payload: map[string]interface{}{
					"reason": reason,
					"queue":  queue,
				},
			})
		}
	}
}