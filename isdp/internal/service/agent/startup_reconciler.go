package agent

import (
	"context"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"go.uber.org/zap"
)

// StartupReconciler 启动时协调器，用于检测和恢复孤儿 invocation
type StartupReconciler struct {
	invocationRepo   *repo.AgentInvocationRepository
	contentBlockRepo *repo.ContentBlockRepository
}

// NewStartupReconciler 创建启动协调器
func NewStartupReconciler(
	invocationRepo *repo.AgentInvocationRepository,
	contentBlockRepo *repo.ContentBlockRepository,
) *StartupReconciler {
	return &StartupReconciler{
		invocationRepo:   invocationRepo,
		contentBlockRepo: contentBlockRepo,
	}
}

// Reconcile 执行启动时的协调逻辑
// 1. 查找所有 running 状态的 invocation
// 2. 检查进程是否存活（通过 process_id）
// 3. 标记孤儿 invocation 为 interrupted
func (r *StartupReconciler) Reconcile(ctx context.Context) {
	logInfo("StartupReconciler: starting reconciliation")

	// 查找所有 running 状态的 invocation
	invocations, err := r.invocationRepo.FindByStatus(ctx, model.InvocationStatusRunning)
	if err != nil {
		logError("StartupReconciler: failed to find running invocations", zap.Error(err))
		return
	}

	if len(invocations) == 0 {
		logInfo("StartupReconciler: no running invocations found")
		return
	}

	logInfo("StartupReconciler: found running invocations", zap.Int("count", len(invocations)))

	interruptedCount := 0
	for _, inv := range invocations {
		// 检查进程是否存活
		if inv.ProcessID != nil && *inv.ProcessID != "" {
			alive := r.isProcessAlive(*inv.ProcessID)
			if !alive {
				// 进程已死，标记为 interrupted
				logInfo("StartupReconciler: marking orphan invocation as interrupted",
					zap.String("invocationID", inv.ID.String()),
					zap.String("processID", *inv.ProcessID))

				inv.Status = model.InvocationStatusInterrupted
				now := time.Now()
				inv.CompletedAt = &now
				inv.Output = "Agent process terminated unexpectedly (detected on server restart)"

				if err := r.invocationRepo.Update(ctx, inv); err != nil {
					logError("StartupReconciler: failed to update invocation status",
						zap.Error(err),
						zap.String("invocationID", inv.ID.String()))
				} else {
					interruptedCount++
				}
			} else {
				logInfo("StartupReconciler: invocation process still alive",
					zap.String("invocationID", inv.ID.String()),
					zap.String("processID", *inv.ProcessID))
			}
		} else {
			// 没有 process_id，可能是旧版本创建的，也标记为 interrupted
			logInfo("StartupReconciler: marking invocation without process_id as interrupted",
				zap.String("invocationID", inv.ID.String()))

			inv.Status = model.InvocationStatusInterrupted
			now := time.Now()
			inv.CompletedAt = &now
			inv.Output = "Agent execution interrupted (server restart, no process tracking)"

			if err := r.invocationRepo.Update(ctx, inv); err != nil {
				logError("StartupReconciler: failed to update invocation status",
					zap.Error(err),
					zap.String("invocationID", inv.ID.String()))
			} else {
				interruptedCount++
			}
		}
	}

	logInfo("StartupReconciler: reconciliation complete",
		zap.Int("totalRunning", len(invocations)),
		zap.Int("interrupted", interruptedCount))
}

// isProcessAlive 检查进程是否存活
func (r *StartupReconciler) isProcessAlive(processID string) bool {
	pid, err := strconv.Atoi(processID)
	if err != nil {
		return false
	}

	// Windows 和 Unix 都使用 FindProcess + Signal(0)
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Signal(0) 不会实际发送信号，只检查进程是否存在
	err = process.Signal(syscall.Signal(0))
	return err == nil
}