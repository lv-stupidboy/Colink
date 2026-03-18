// isdp/internal/service/agent/debug_thread_manager.go
package agent

import (
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
)

// DebugThread 状态常量
const (
	DebugThreadStatusIdle      = "idle"
	DebugThreadStatusRunning   = "running"
	DebugThreadStatusCompleted = "completed"
	DebugThreadStatusError     = "error"
)

// DebugThread 内存中的调试线程
type DebugThread struct {
	ID          uuid.UUID
	Status      string // idle, running, completed, error
	CreatedAt   time.Time
	Messages    []*model.Message
	ProjectPath string // 工作目录路径
}

// DebugThreadManager 调试线程管理器
type DebugThreadManager struct {
	threads         map[uuid.UUID]*DebugThread
	mu              sync.RWMutex
	wsHub           *ws.Hub
	maxAge          time.Duration // 线程最大存活时间
	cleanupInterval time.Duration // 清理间隔
	stopCleanup     chan struct{} // 停止清理信号
	stopOnce        sync.Once     // 确保 Stop() 只执行一次
}

// NewDebugThreadManager 创建调试线程管理器
func NewDebugThreadManager(wsHub *ws.Hub) *DebugThreadManager {
	m := &DebugThreadManager{
		threads:         make(map[uuid.UUID]*DebugThread),
		wsHub:           wsHub,
		maxAge:          2 * time.Hour,
		cleanupInterval: 30 * time.Minute,
		stopCleanup:     make(chan struct{}),
	}
	go m.startCleanupRoutine()
	return m
}

// startCleanupRoutine 定期清理过期线程
func (m *DebugThreadManager) startCleanupRoutine() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredThreads()
		case <-m.stopCleanup:
			return
		}
	}
}

// cleanupExpiredThreads 清理过期线程
func (m *DebugThreadManager) cleanupExpiredThreads() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, thread := range m.threads {
		if now.Sub(thread.CreatedAt) > m.maxAge {
			// 广播线程关闭消息
			if m.wsHub != nil {
				m.wsHub.BroadcastToThread(id.String(), ws.WSMessage{
					Type: "thread_expired",
					Payload: map[string]interface{}{
						"threadId": id.String(),
						"message":  "调试会话已过期，请重新开始",
					},
					Timestamp: time.Now().Unix(),
				})
			}
			delete(m.threads, id)
		}
	}
}

// Stop 停止清理协程（用于优雅关闭）
func (m *DebugThreadManager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCleanup)
	})
}

// CreateThread 创建调试线程
func (m *DebugThreadManager) CreateThread(projectPath string) *DebugThread {
	m.mu.Lock()
	defer m.mu.Unlock()

	thread := &DebugThread{
		ID:          uuid.New(),
		Status:      DebugThreadStatusIdle,
		CreatedAt:   time.Now(),
		Messages:    make([]*model.Message, 0),
		ProjectPath: projectPath,
	}
	m.threads[thread.ID] = thread
	return thread
}

// GetThread 获取调试线程
func (m *DebugThreadManager) GetThread(id uuid.UUID) *DebugThread {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.threads[id]
}

// AddMessage 添加消息
func (m *DebugThreadManager) AddMessage(threadID uuid.UUID, msg *model.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if thread, ok := m.threads[threadID]; ok {
		thread.Messages = append(thread.Messages, msg)
	}
}

// GetMessages 获取消息列表
func (m *DebugThreadManager) GetMessages(threadID uuid.UUID) []*model.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if thread, ok := m.threads[threadID]; ok {
		// 返回副本，避免外部修改影响内部状态
		msgs := make([]*model.Message, len(thread.Messages))
		copy(msgs, thread.Messages)
		return msgs
	}
	return nil
}

// SetStatus 设置线程状态
func (m *DebugThreadManager) SetStatus(threadID uuid.UUID, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if thread, ok := m.threads[threadID]; ok {
		thread.Status = status
	}
}

// CompareAndSwapStatus 原子地比较并交换状态
// 只有当当前状态等于 expected 时，才设置为 newStatus
// 返回是否成功交换
func (m *DebugThreadManager) CompareAndSwapStatus(threadID uuid.UUID, expected, newStatus string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if thread, ok := m.threads[threadID]; ok {
		if thread.Status == expected {
			thread.Status = newStatus
			return true
		}
	}
	return false
}

// TryStartExecution 尝试启动执行
// 原子地将状态从 idle 或 completed 转换为 running
// 返回是否成功转换
func (m *DebugThreadManager) TryStartExecution(threadID uuid.UUID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if thread, ok := m.threads[threadID]; ok {
		if thread.Status == DebugThreadStatusIdle || thread.Status == DebugThreadStatusCompleted {
			thread.Status = DebugThreadStatusRunning
			return true
		}
	}
	return false
}

// GetProjectPath 获取线程的工作目录
func (m *DebugThreadManager) GetProjectPath(threadID uuid.UUID) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if thread, ok := m.threads[threadID]; ok {
		return thread.ProjectPath
	}
	return ""
}

// SetProjectPath 设置线程的工作目录
func (m *DebugThreadManager) SetProjectPath(threadID uuid.UUID, projectPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if thread, ok := m.threads[threadID]; ok {
		// 只有当新路径不为空且与当前不同时才更新
		if projectPath != "" && thread.ProjectPath != projectPath {
			thread.ProjectPath = projectPath
		}
	}
}

// DeleteThread 删除调试线程（清理资源）
func (m *DebugThreadManager) DeleteThread(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.threads, id)
}

// Broadcast 向线程广播消息（WebSocket集成）
func (m *DebugThreadManager) Broadcast(threadID uuid.UUID, msgType string, payload map[string]interface{}) {
	if m.wsHub == nil {
		return
	}

	msg := ws.WSMessage{
		Type:      msgType,
		Payload:   payload,
		ThreadID:  threadID.String(),
		Timestamp: time.Now().Unix(),
	}

	m.wsHub.BroadcastToThread(threadID.String(), msg)
}

// BroadcastChunk 广播流式输出块
func (m *DebugThreadManager) BroadcastChunk(threadID, invocationID, agentID, agentName, chunk string) {
	id, err := uuid.Parse(threadID)
	if err != nil {
		return
	}
	m.Broadcast(id, "agent_output_chunk", map[string]interface{}{
		"chunk":        chunk,
		"invocationId": invocationID,
		"agentId":      agentID,
		"agentName":    agentName,
	})
}

// BroadcastMessage 广播完整消息
func (m *DebugThreadManager) BroadcastMessage(threadID, messageID, agentID, agentName, agentRole, content string) {
	id, err := uuid.Parse(threadID)
	if err != nil {
		return
	}
	m.Broadcast(id, "agent_message", map[string]interface{}{
		"messageId":  messageID,
		"agentId":    agentID,
		"agentName":  agentName,
		"agentRole":  agentRole,
		"content":    content,
	})
}

// BroadcastSandboxReady 广播沙箱就绪
func (m *DebugThreadManager) BroadcastSandboxReady(threadID, sandboxURL string) {
	id, err := uuid.Parse(threadID)
	if err != nil {
		return
	}
	m.Broadcast(id, "sandbox_ready", map[string]interface{}{
		"url": sandboxURL,
	})
}

// BroadcastError 广播错误消息
func (m *DebugThreadManager) BroadcastError(threadID, errorMsg string) {
	id, err := uuid.Parse(threadID)
	if err != nil {
		return
	}
	m.Broadcast(id, "system_message", map[string]interface{}{
		"content": errorMsg,
		"level":   "error",
	})
}