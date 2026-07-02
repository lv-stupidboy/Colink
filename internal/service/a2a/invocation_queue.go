package a2a

import (
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/google/uuid"
)

// 队列配置常量
const (
	MaxQueueDepth = 5 // 每用户每线程最多排队条目
)

// QueueEntry 队列条目
type QueueEntry struct {
	ID            uuid.UUID   // 条目 ID
	ThreadID      uuid.UUID   // 线程 ID
	UserID        string      // 用户 ID
	Content       string      // 消息内容
	MessageID     *uuid.UUID  // 关联的消息 ID
	MergedMsgIDs  []uuid.UUID // 合并的消息 IDs
	Source        string      // 来源: "user", "connector", "agent"
	TargetAgents  []string    // 目标 Agent IDs
	Intent        string      // 意图: "execute", "ideate"
	Status        string      // 状态: "queued", "processing"
	CreatedAt     time.Time   // 创建时间
	AutoExecute   bool        // A2A 自动执行标记
	CallerAgentID string      // A2A 调用者 ID

	// A2A 交接信息（上游 Agent → 下游 Agent）
	ChainHistory *agent.A2AChainContext // 上游链路历史快照（含上游输出）
	TriggeredBy  uuid.UUID              // 上游 invocation ID（SpawnRequest.TriggeredBy）
}

// EnqueueResult 入队结果
type EnqueueResult struct {
	Outcome     string       // "enqueued", "merged", "full"
	Entry       *QueueEntry  // 条目
	QueuePos    int          // 队列位置
}

// InvocationQueue 调用队列
// 用于管理 Agent 执行请求的排队和调度
type InvocationQueue struct {
	queues map[string][]*QueueEntry // scopeKey (threadID:userID) -> entries
	mu     sync.RWMutex
}

// NewInvocationQueue 创建调用队列
func NewInvocationQueue() *InvocationQueue {
	return &InvocationQueue{
		queues: make(map[string][]*QueueEntry),
	}
}

// scopeKey 生成作用域键
func (q *InvocationQueue) scopeKey(threadID uuid.UUID, userID string) string {
	return threadID.String() + ":" + userID
}

// Enqueue 入队
// 返回入队结果，支持同源同目标消息合并
func (q *InvocationQueue) Enqueue(entry *QueueEntry) (*EnqueueResult, error) {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	entry.Status = "queued"

	key := q.scopeKey(entry.ThreadID, entry.UserID)

	q.mu.Lock()
	defer q.mu.Unlock()

	queue := q.queues[key]
	if queue == nil {
		queue = make([]*QueueEntry, 0)
		q.queues[key] = queue
	}

	// 检查是否可以合并（同源同目标的连续消息）
	if len(queue) > 0 {
		tail := queue[len(queue)-1]
		if canMerge(tail, entry) {
			// 合并内容
			tail.Content += "\n" + entry.Content
			if entry.MessageID != nil {
				tail.MergedMsgIDs = append(tail.MergedMsgIDs, *entry.MessageID)
			}
			// A2A 交接信息以最新的为准（后到的 @mention 携带更完整的上游输出）
			if entry.ChainHistory != nil {
				tail.ChainHistory = entry.ChainHistory
			}
			if entry.TriggeredBy != uuid.Nil {
				tail.TriggeredBy = entry.TriggeredBy
			}
			return &EnqueueResult{
				Outcome: "merged",
				Entry:   tail,
				QueuePos: len(queue),
			}, nil
		}
	}

	// 容量检查
	queuedCount := countQueued(queue)
	if queuedCount >= MaxQueueDepth {
		return &EnqueueResult{Outcome: "full"}, nil
	}

	// 入队
	q.queues[key] = append(queue, entry)
	return &EnqueueResult{
		Outcome: "enqueued",
		Entry:   entry,
		QueuePos: len(q.queues[key]),
	}, nil
}

// Dequeue 出队
func (q *InvocationQueue) Dequeue(threadID uuid.UUID, userID string) *QueueEntry {
	key := q.scopeKey(threadID, userID)

	q.mu.Lock()
	defer q.mu.Unlock()

	queue := q.queues[key]
	if len(queue) == 0 {
		return nil
	}

	entry := queue[0]
	q.queues[key] = queue[1:]
	return entry
}

// Peek 查看队首但不移除
func (q *InvocationQueue) Peek(threadID uuid.UUID, userID string) *QueueEntry {
	key := q.scopeKey(threadID, userID)

	q.mu.RLock()
	defer q.mu.RUnlock()

	queue := q.queues[key]
	if len(queue) == 0 {
		return nil
	}
	return queue[0]
}

// Size 获取队列大小
func (q *InvocationQueue) Size(threadID uuid.UUID, userID string) int {
	key := q.scopeKey(threadID, userID)

	q.mu.RLock()
	defer q.mu.RUnlock()

	return countQueued(q.queues[key])
}

// MarkProcessing 标记为处理中
func (q *InvocationQueue) MarkProcessing(threadID uuid.UUID, userID string) *QueueEntry {
	key := q.scopeKey(threadID, userID)

	q.mu.Lock()
	defer q.mu.Unlock()

	queue := q.queues[key]
	for _, entry := range queue {
		if entry.Status == "queued" {
			entry.Status = "processing"
			return entry
		}
	}
	return nil
}

// Remove 移除指定条目
func (q *InvocationQueue) Remove(threadID uuid.UUID, userID string, entryID uuid.UUID) *QueueEntry {
	key := q.scopeKey(threadID, userID)

	q.mu.Lock()
	defer q.mu.Unlock()

	queue := q.queues[key]
	for i, entry := range queue {
		if entry.ID == entryID {
			q.queues[key] = append(queue[:i], queue[i+1:]...)
			return entry
		}
	}
	return nil
}

// List 列出队列条目
func (q *InvocationQueue) List(threadID uuid.UUID, userID string) []*QueueEntry {
	key := q.scopeKey(threadID, userID)

	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*QueueEntry, len(q.queues[key]))
	copy(result, q.queues[key])
	return result
}

// Clear 清空队列
func (q *InvocationQueue) Clear(threadID uuid.UUID, userID string) {
	key := q.scopeKey(threadID, userID)

	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.queues, key)
}

// HasQueuedAgent 检查是否有指定 Agent 的排队条目（用于去重）
func (q *InvocationQueue) HasQueuedAgent(threadID string, agentID string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for key, queue := range q.queues {
		// 检查是否属于该线程
		if len(key) > 36 && key[:36] == threadID {
			for _, entry := range queue {
				if entry.Status == "queued" && containsAgent(entry.TargetAgents, agentID) {
					return true
				}
			}
		}
	}
	return false
}

// CountAgentEntriesForThread 统计线程中 Agent 来源的条目数（用于深度检查）
func (q *InvocationQueue) CountAgentEntriesForThread(threadID string) int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	count := 0
	for key, queue := range q.queues {
		if len(key) > 36 && key[:36] == threadID {
			for _, entry := range queue {
				if entry.Source == "agent" {
					count++
				}
			}
		}
	}
	return count
}

// PeekOldestAcrossUsers 获取线程中最早的排队条目（跨用户）
func (q *InvocationQueue) PeekOldestAcrossUsers(threadID string) *QueueEntry {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var oldest *QueueEntry
	for key, queue := range q.queues {
		if len(key) > 36 && key[:36] == threadID {
			for _, entry := range queue {
				if entry.Status == "queued" {
					if oldest == nil || entry.CreatedAt.Before(oldest.CreatedAt) {
						oldest = entry
					}
				}
			}
		}
	}
	return oldest
}

// ListUsersForThread 获取线程中有排队条目的用户列表
func (q *InvocationQueue) ListUsersForThread(threadID string) []string {
	q.mu.RLock()
	defer q.mu.RUnlock()

	users := make(map[string]bool)
	for key, queue := range q.queues {
		if len(key) > 36 && key[:36] == threadID && len(queue) > 0 {
			userID := key[37:] // threadID + ":" + userID
			if userID != "" {
				users[userID] = true
			}
		}
	}

	result := make([]string, 0, len(users))
	for user := range users {
		result = append(result, user)
	}
	return result
}

// 辅助函数

func canMerge(existing, new *QueueEntry) bool {
	if existing.Status != "queued" || new.Status != "queued" {
		return false
	}
	if existing.Source != new.Source {
		return false
	}
	if existing.Intent != new.Intent {
		return false
	}
	if len(existing.TargetAgents) != len(new.TargetAgents) {
		return false
	}
	// 简化：目标 Agent 必须完全一致
	for i, a := range existing.TargetAgents {
		if a != new.TargetAgents[i] {
			return false
		}
	}
	return true
}

func countQueued(queue []*QueueEntry) int {
	count := 0
	for _, entry := range queue {
		if entry.Status == "queued" {
			count++
		}
	}
	return count
}

func containsAgent(agents []string, agentID string) bool {
	for _, a := range agents {
		if a == agentID {
			return true
		}
	}
	return false
}

// HasQueuedForThread 检查线程是否有任何排队条目
func (q *InvocationQueue) HasQueuedForThread(threadID string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for key, queue := range q.queues {
		if len(key) > 36 && key[:36] == threadID {
			for _, entry := range queue {
				if entry.Status == "queued" {
					return true
				}
			}
		}
	}
	return false
}

// MarkProcessingById 通过 ID 标记条目为处理中
func (q *InvocationQueue) MarkProcessingById(threadID string, entryID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for key, queue := range q.queues {
		if len(key) > 36 && key[:36] == threadID {
			for _, entry := range queue {
				if entry.ID.String() == entryID && entry.Status == "queued" {
					entry.Status = "processing"
					return true
				}
			}
		}
	}
	return false
}

// RemoveProcessedAcrossUsers 跨用户移除已处理的条目
func (q *InvocationQueue) RemoveProcessedAcrossUsers(threadID string, entryID string) *QueueEntry {
	q.mu.Lock()
	defer q.mu.Unlock()

	for key, queue := range q.queues {
		if len(key) > 36 && key[:36] == threadID {
			for i, entry := range queue {
				if entry.Status == "processing" && entry.ID.String() == entryID {
					q.queues[key] = append(queue[:i], queue[i+1:]...)
					return entry
				}
			}
		}
	}
	return nil
}

// ListAutoExecute 列出所有 autoExecute 条目
func (q *InvocationQueue) ListAutoExecute(threadID string) []*QueueEntry {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var result []*QueueEntry
	for key, queue := range q.queues {
		if len(key) > 36 && key[:36] == threadID {
			for _, entry := range queue {
				if entry.Status == "queued" && entry.AutoExecute {
					result = append(result, entry)
				}
			}
		}
	}
	// 按创建时间排序
	sortEntriesByCreatedAt(result)
	return result
}

// RemoveProcessed 移除已处理的条目
func (q *InvocationQueue) RemoveProcessed(threadID uuid.UUID, userID string, entryID uuid.UUID) *QueueEntry {
	key := q.scopeKey(threadID, userID)

	q.mu.Lock()
	defer q.mu.Unlock()

	queue := q.queues[key]
	for i, entry := range queue {
		if entry.Status == "processing" && entry.ID == entryID {
			q.queues[key] = append(queue[:i], queue[i+1:]...)
			return entry
		}
	}
	return nil
}

// RollbackProcessing 回滚处理中的条目
func (q *InvocationQueue) RollbackProcessing(threadID string, entryID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for key, queue := range q.queues {
		if len(key) > 36 && key[:36] == threadID {
			for _, entry := range queue {
				if entry.ID.String() == entryID && entry.Status == "processing" {
					entry.Status = "queued"
					return true
				}
			}
		}
	}
	return false
}

// sortEntriesByCreatedAt 按创建时间排序
func sortEntriesByCreatedAt(entries []*QueueEntry) {
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].CreatedAt.After(entries[j].CreatedAt) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}