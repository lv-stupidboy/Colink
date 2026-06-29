// internal/service/agent/session_mutex.go
// SessionMutex - per-sessionKey 并发锁
// 锁粒度: threadID:agentID，而非 sessionId
// 原因: sessionId 在 CLI 执行后才生成，并发请求时可能为空导致锁失效
package agent

import (
	"context"
	"sync"
)

// sessionLock 会话锁
type sessionLock struct {
	release func()
}

// waiter 等待者
type waiter struct {
	resolve chan func()  // 获取锁后调用
	reject  chan error   // 取消时调用
	cleanup func()       // 清理函数
}

// SessionMutex 会话互斥锁
// 防止并发 resume 同一 sessionKey (threadID:agentID)
type SessionMutex struct {
	held    map[string]*sessionLock // key: sessionKey → lock
	waiters map[string][]*waiter    // key: sessionKey → queue
	mu      sync.RWMutex
}

// NewSessionMutex 创建 SessionMutex
func NewSessionMutex() *SessionMutex {
	return &SessionMutex{
		held:    make(map[string]*sessionLock),
		waiters: make(map[string][]*waiter),
	}
}

// Acquire 获取锁
// sessionKey = threadID:agentID
func (sm *SessionMutex) Acquire(threadID, agentID string, ctx context.Context) (func(), error) {
	sessionKey := threadID + ":" + agentID

	// 检查 ctx 是否已取消
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	sm.mu.Lock()

	// 无竞争 → 立即获取
	_, exists := sm.held[sessionKey]
	if !exists {
		releaseOnce := false
		lock := &sessionLock{
			release: func() {
				if releaseOnce {
					return
				}
				releaseOnce = true
				sm.release(sessionKey)
			},
		}
		sm.held[sessionKey] = lock
		sm.mu.Unlock()
		return lock.release, nil
	}

	// 有竞争 → 排队等待
	sm.mu.Unlock()

	resolveCh := make(chan func(), 1)
	rejectCh := make(chan error, 1)

	w := &waiter{
		resolve: resolveCh,
		reject:  rejectCh,
	}

	sm.mu.Lock()
	if sm.waiters[sessionKey] == nil {
		sm.waiters[sessionKey] = []*waiter{}
	}
	sm.waiters[sessionKey] = append(sm.waiters[sessionKey], w)
	sm.mu.Unlock()

	// 等待获取或取消
	select {
	case release := <-resolveCh:
		return release, nil
	case err := <-rejectCh:
		return nil, err
	case <-ctx.Done():
		// 取消，从队列移除
		sm.mu.Lock()
		queue := sm.waiters[sessionKey]
		for i, q := range queue {
			if q == w {
				sm.waiters[sessionKey] = append(queue[:i], queue[i+1:]...)
				if len(sm.waiters[sessionKey]) == 0 {
					delete(sm.waiters, sessionKey)
				}
				break
			}
		}
		sm.mu.Unlock()
		return nil, ctx.Err()
	}
}

// release 释放锁（内部方法）
func (sm *SessionMutex) release(sessionKey string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// 删除当前锁
	delete(sm.held, sessionKey)

	// 唤醒下一个 waiter
	queue := sm.waiters[sessionKey]
	if len(queue) == 0 {
		delete(sm.waiters, sessionKey)
		return
	}

	// 取第一个 waiter
	next := queue[0]
	sm.waiters[sessionKey] = queue[1:]
	if len(sm.waiters[sessionKey]) == 0 {
		delete(sm.waiters, sessionKey)
	}

	// 创建新锁给 waiter
	releaseOnce := false
	lock := &sessionLock{
		release: func() {
			if releaseOnce {
				return
			}
			releaseOnce = true
			sm.release(sessionKey)
		},
	}
	sm.held[sessionKey] = lock

	// 唤醒 waiter
	next.resolve <- lock.release
}

// Release 释放锁（幂等）
func (sm *SessionMutex) Release(threadID, agentID string) {
	sessionKey := threadID + ":" + agentID

	sm.mu.RLock()
	lock, exists := sm.held[sessionKey]
	sm.mu.RUnlock()

	if exists && lock != nil && lock.release != nil {
		lock.release()
	}
}