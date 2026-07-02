// internal/service/agent/session_mutex_test.go
package agent

import (
	"context"
	"testing"
	"time"
)

func TestNewSessionMutex(t *testing.T) {
	sm := NewSessionMutex()
	if sm == nil {
		t.Error("NewSessionMutex() returned nil")
	}
	if len(sm.held) != 0 {
		t.Error("new SessionMutex should have empty held map")
	}
	if len(sm.waiters) != 0 {
		t.Error("new SessionMutex should have empty waiters map")
	}
}

func TestSessionMutexAcquireNoContention(t *testing.T) {
	sm := NewSessionMutex()

	release, err := sm.Acquire("thread-1", "agent-1", context.Background())
	if err != nil {
		t.Errorf("Acquire() error = %v", err)
	}
	if release == nil {
		t.Error("Acquire() returned nil release function")
	}

	// 检查锁被持有
	sessionKey := "thread-1:agent-1"
	sm.mu.RLock()
	_, held := sm.held[sessionKey]
	sm.mu.RUnlock()

	if !held {
		t.Error("Acquire() did not add entry to held map")
	}

	// 释放锁
	release()

	sm.mu.RLock()
	_, held = sm.held[sessionKey]
	sm.mu.RUnlock()

	if held {
		t.Error("Release() did not remove entry from held map")
	}
}

func TestSessionMutexAcquireWithContention(t *testing.T) {
	sm := NewSessionMutex()

	// 第一个获取
	release1, err := sm.Acquire("thread-1", "agent-1", context.Background())
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}

	// 第二个应该等待
	done := make(chan struct{})
	go func() {
		release2, err := sm.Acquire("thread-1", "agent-1", context.Background())
		if err != nil {
			t.Errorf("second Acquire() error = %v", err)
		}
		release2()
		close(done)
	}()

	// 等待一小段时间，确保第二个 goroutine 进入等待
	time.Sleep(100 * time.Millisecond)

	// 检查 waiters
	sessionKey := "thread-1:agent-1"
	sm.mu.RLock()
	waiters := sm.waiters[sessionKey]
	sm.mu.RUnlock()

	if len(waiters) != 1 {
		t.Errorf("waiters count = %d, want 1", len(waiters))
	}

	// 释放第一个锁
	release1()

	// 等待第二个完成
	select {
	case <-done:
		// 成功
	case <-time.After(1 * time.Second):
		t.Error("second Acquire() did not complete after first Release()")
	}
}

func TestSessionMutexAcquireWithCancellation(t *testing.T) {
	sm := NewSessionMutex()

	// 第一个获取
	release1, err := sm.Acquire("thread-1", "agent-1", context.Background())
	if err != nil {
		t.Fatalf("first Acquire() error = %v", err)
	}

	// 第二个带取消
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() {
		_, err := sm.Acquire("thread-1", "agent-1", ctx)
		done <- err
	}()

	// 等待进入等待队列
	time.Sleep(100 * time.Millisecond)

	// 取消
	cancel()

	// 等待结果
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Acquire() with cancelled context error = %v, want context.Canceled", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Acquire() with cancelled context did not return")
	}

	// 释放第一个
	release1()
}

func TestSessionMutexReleaseIdempotent(t *testing.T) {
	sm := NewSessionMutex()

	release, err := sm.Acquire("thread-1", "agent-1", context.Background())
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	// 第一次释放
	release()

	// 第二次释放应该无效果（幂等）
	release()

	// 检查锁确实被释放
	sessionKey := "thread-1:agent-1"
	sm.mu.RLock()
	_, held := sm.held[sessionKey]
	sm.mu.RUnlock()

	if held {
		t.Error("lock still held after idempotent Release()")
	}
}