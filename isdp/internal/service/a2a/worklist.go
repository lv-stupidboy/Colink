package a2a

import (
	"context"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// WorklistItem 工作列表项
type WorklistItem struct {
	ID          uuid.UUID
	ThreadID    uuid.UUID
	TargetRole  model.AgentRole
	SourceRole  model.AgentRole
	Priority    int
	Payload     string
	CreatedAt   time.Time
	ProcessAfter *time.Time
}

// Worklist A2A工作队列
type Worklist struct {
	items   []WorklistItem
	mu      sync.RWMutex
	notifier chan struct{}
}

// NewWorklist 创建工作列表
func NewWorklist() *Worklist {
	return &Worklist{
		items:    make([]WorklistItem, 0),
		notifier: make(chan struct{}, 1),
	}
}

// Enqueue 入队
func (w *Worklist) Enqueue(ctx context.Context, item WorklistItem) error {
	item.ID = uuid.New()
	item.CreatedAt = time.Now()

	w.mu.Lock()
	// 按优先级插入
	inserted := false
	for i, existing := range w.items {
		if item.Priority > existing.Priority {
			w.items = append(w.items[:i], append([]WorklistItem{item}, w.items[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		w.items = append(w.items, item)
	}
	w.mu.Unlock()

	// 通知有新任务
	select {
	case w.notifier <- struct{}{}:
	default:
	}

	return nil
}

// Dequeue 出队
func (w *Worklist) Dequeue(ctx context.Context) (*WorklistItem, error) {
	for {
		w.mu.Lock()
		if len(w.items) == 0 {
			w.mu.Unlock()
			// 等待新任务
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-w.notifier:
				continue
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}

		// 检查是否有可处理的项目
		now := time.Now()
		for i, item := range w.items {
			if item.ProcessAfter == nil || item.ProcessAfter.Before(now) {
				// 移除并返回
				w.items = append(w.items[:i], w.items[i+1:]...)
				w.mu.Unlock()
				return &item, nil
			}
		}
		w.mu.Unlock()

		// 等待或重试
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Peek 查看队首但不移除
func (w *Worklist) Peek() *WorklistItem {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for _, item := range w.items {
		if item.ProcessAfter == nil || item.ProcessAfter.Before(time.Now()) {
			return &item
		}
	}
	return nil
}

// Size 获取队列大小
func (w *Worklist) Size() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.items)
}

// Clear 清空队列
func (w *Worklist) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.items = make([]WorklistItem, 0)
}

// GetByThread 获取指定Thread的工作项
func (w *Worklist) GetByThread(threadID uuid.UUID) []WorklistItem {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var result []WorklistItem
	for _, item := range w.items {
		if item.ThreadID == threadID {
			result = append(result, item)
		}
	}
	return result
}

// Remove 移除指定项
func (w *Worklist) Remove(id uuid.UUID) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	for i, item := range w.items {
		if item.ID == id {
			w.items = append(w.items[:i], w.items[i+1:]...)
			return true
		}
	}
	return false
}

// Prioritize 提升优先级
func (w *Worklist) Prioritize(id uuid.UUID, newPriority int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	for i, item := range w.items {
		if item.ID == id {
			w.items[i].Priority = newPriority
			// 重新排序
			w.reorder(i)
			return true
		}
	}
	return false
}

// reorder 重新排序
func (w *Worklist) reorder(index int) {
	item := w.items[index]
	w.items = append(w.items[:index], w.items[index+1:]...)

	inserted := false
	for i, existing := range w.items {
		if item.Priority > existing.Priority {
			w.items = append(w.items[:i], append([]WorklistItem{item}, w.items[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		w.items = append(w.items, item)
	}
}