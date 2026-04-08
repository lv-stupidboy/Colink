package agent

import (
	"context"
	"errors"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// InvocationTracker 调用追踪器
type InvocationTracker struct {
	repo      *repo.AgentInvocationRepository
	processes map[uuid.UUID]*ProcessInfo
	mu        sync.RWMutex
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	InvocationID uuid.UUID
	PID          int
	Status       string // running, cancelled
	StartTime    time.Time
	CancelFunc   context.CancelFunc
}

// NewInvocationTracker 创建追踪器
func NewInvocationTracker(repo *repo.AgentInvocationRepository) *InvocationTracker {
	return &InvocationTracker{
		repo:      repo,
		processes: make(map[uuid.UUID]*ProcessInfo),
	}
}

// StartTracking 开始追踪
func (t *InvocationTracker) StartTracking(ctx context.Context, invocation *model.AgentInvocation, pid int) error {
	invocation.Status = model.InvocationStatusRunning
	invocation.StartedAt = timePtr(time.Now())

	if err := t.repo.Update(ctx, invocation); err != nil {
		return err
	}

	t.mu.Lock()
	t.processes[invocation.ID] = &ProcessInfo{
		InvocationID: invocation.ID,
		PID:          pid,
		Status:       "running",
		StartTime:    time.Now(),
	}
	t.mu.Unlock()

	return nil
}

// StopTracking 停止追踪
func (t *InvocationTracker) StopTracking(ctx context.Context, invocationID uuid.UUID, status model.InvocationStatus, output string) error {
	t.mu.Lock()
	delete(t.processes, invocationID)
	t.mu.Unlock()

	invocation, err := t.repo.FindByID(ctx, invocationID)
	if err != nil {
		return err
	}

	invocation.Status = status
	invocation.Output = output
	invocation.CompletedAt = timePtr(time.Now())

	return t.repo.Update(ctx, invocation)
}

// Cancel 取消调用
func (t *InvocationTracker) Cancel(ctx context.Context, invocationID uuid.UUID) error {
	t.mu.Lock()
	info, exists := t.processes[invocationID]
	if exists {
		info.Status = "cancelled"
	}
	t.mu.Unlock()

	if !exists {
		return ErrProcessNotFound
	}

	// 通过PID发送终止信号
	if info.PID > 0 {
		if err := killProcess(info.PID); err != nil {
			return err
		}
	}

	invocation, err := t.repo.FindByID(ctx, invocationID)
	if err != nil {
		return err
	}

	invocation.Status = model.InvocationStatusCancelled
	invocation.CompletedAt = timePtr(time.Now())

	return t.repo.Update(ctx, invocation)
}

// GetActiveInvocations 获取活跃调用
func (t *InvocationTracker) GetActiveInvocations(threadID uuid.UUID) []uuid.UUID {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var invocations []uuid.UUID
	for id, info := range t.processes {
		if info.Status == "running" {
			invocations = append(invocations, id)
		}
	}
	return invocations
}

// GetProcessInfo 获取进程信息
func (t *InvocationTracker) GetProcessInfo(invocationID uuid.UUID) (*ProcessInfo, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	info, exists := t.processes[invocationID]
	if !exists {
		return nil, ErrProcessNotFound
	}
	return info, nil
}

// killProcess 终止进程
func killProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}

var (
	ErrProcessNotFound = errors.New("process not found")
)