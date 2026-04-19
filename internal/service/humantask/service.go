package humantask

import (
	"context"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
)

// Service HumanTask 服务
type Service struct {
	taskRepo    *repo.HumanTaskRepository
	threadRepo  *repo.ThreadRepository
	projectRepo *repo.ProjectRepository
	wsHub       *ws.Hub
}

// NewService 创建 HumanTask 服务
func NewService(
	taskRepo *repo.HumanTaskRepository,
	threadRepo *repo.ThreadRepository,
	projectRepo *repo.ProjectRepository,
	wsHub *ws.Hub,
) *Service {
	return &Service{
		taskRepo:    taskRepo,
		threadRepo:  threadRepo,
		projectRepo: projectRepo,
		wsHub:       wsHub,
	}
}

// CreateTaskFromWaiting 从 Agent waiting 状态创建待办任务
func (s *Service) CreateTaskFromWaiting(
	ctx context.Context,
	threadID uuid.UUID,
	invocationID uuid.UUID,
	agentConfigID uuid.UUID,
	agentName string,
	waitReason string,
) (*model.HumanTask, error) {
	// 检查是否已有 pending 任务（幂等）
	existing, _ := s.taskRepo.FindByInvocation(ctx, invocationID)
	if existing != nil {
		return existing, nil // 已存在，直接返回
	}

	// 获取 Thread 信息
	var projectName, threadName string
	var projectID uuid.UUID
	if s.threadRepo != nil {
		thread, err := s.threadRepo.FindByID(ctx, threadID)
		if err == nil && thread != nil {
			threadName = thread.Name
			projectID = thread.ProjectID
			// 获取 Project 信息
			if s.projectRepo != nil && projectID != uuid.Nil {
				project, err := s.projectRepo.FindByID(ctx, projectID)
				if err == nil && project != nil {
					projectName = project.Name
				}
			}
		}
	}

	task := &model.HumanTask{
		ID:            uuid.New(),
		ThreadID:      threadID,
		InvocationID:  invocationID,
		AgentConfigID: agentConfigID,
		AgentName:     agentName,
		WaitReason:    waitReason,
		ProjectID:     projectID,
		ProjectName:   projectName,
		ThreadName:    threadName,
		Status:        model.HumanTaskStatusPending,
		CreatedAt:     time.Now(),
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		return nil, err
	}

	// 广播 WebSocket 事件（全局广播）
	s.broadcastTaskCreated(task)

	return task, nil
}

// broadcastTaskCreated 广播任务创建事件（全局广播）
func (s *Service) broadcastTaskCreated(task *model.HumanTask) {
	if s.wsHub != nil {
		// 使用全局广播，让所有页面都能收到（包括 Tasks 页面）
		s.wsHub.BroadcastGlobal(ws.WSMessage{
			Type:      "human_task_created",
			Timestamp: model.Now(),
			Payload: map[string]interface{}{
				"taskId":       task.ID.String(),
				"threadId":     task.ThreadID.String(),
				"invocationId": task.InvocationID.String(),
				"agentName":    task.AgentName,
				"waitReason":   task.WaitReason,
				"status":       string(task.Status),
				"projectId":    task.ProjectID.String(),
				"projectName":  task.ProjectName,
				"threadName":   task.ThreadName,
				"createdAt":    task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			},
		})
	}
}

// CompleteTaskFromReply 用户回复后关闭待办任务
func (s *Service) CompleteTaskFromReply(ctx context.Context, invocationID uuid.UUID) error {
	if err := s.taskRepo.CompleteByInvocation(ctx, invocationID); err != nil {
		return err
	}

	// 广播 WebSocket 事件
	s.broadcastTaskCompleted(invocationID)

	return nil
}

// broadcastTaskCompleted 广播任务完成事件
func (s *Service) broadcastTaskCompleted(invocationID uuid.UUID) {
	if s.wsHub != nil {
		s.wsHub.BroadcastGlobal(ws.WSMessage{
			Type:      "human_task_completed",
			Timestamp: model.Now(),
			Payload: map[string]interface{}{
				"invocationId": invocationID.String(),
			},
		})
	}
}

// Complete 手动完成待办任务（通过 taskID）
func (s *Service) Complete(ctx context.Context, taskID uuid.UUID) error {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	if task.Status != model.HumanTaskStatusPending {
		return fmt.Errorf("task is not in pending state: %s", task.Status)
	}

	now := time.Now()
	task.Status = model.HumanTaskStatusCompleted
	task.CompletedAt = &now

	if err := s.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}

	// 广播 WebSocket 事件
	s.broadcastTaskCompleted(task.InvocationID)

	return nil
}

// CancelTask 取消待办任务
func (s *Service) CancelTask(ctx context.Context, taskID uuid.UUID) error {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return err
	}

	if task.Status != model.HumanTaskStatusPending {
		return fmt.Errorf("task is not in pending state")
	}

	task.Status = model.HumanTaskStatusCancelled
	now := time.Now()
	task.CompletedAt = &now

	if err := s.taskRepo.Update(ctx, task); err != nil {
		return err
	}

	// 广播 WebSocket 事件
	s.broadcastTaskCancelled(task)

	return nil
}

// broadcastTaskCancelled 广播任务取消事件
func (s *Service) broadcastTaskCancelled(task *model.HumanTask) {
	if s.wsHub != nil {
		s.wsHub.BroadcastGlobal(ws.WSMessage{
			Type:      "human_task_cancelled",
			Timestamp: model.Now(),
			Payload: map[string]interface{}{
				"taskId": task.ID.String(),
				"status": string(task.Status),
			},
		})
	}
}

// GetStats 获取待办任务统计
func (s *Service) GetStats(ctx context.Context) (map[string]int, error) {
	return s.taskRepo.CountByStatus(ctx)
}

// List 根据状态列出任务
func (s *Service) List(ctx context.Context, status model.HumanTaskStatus) ([]*model.HumanTask, error) {
	return s.taskRepo.ListByStatus(ctx, status)
}

// Get 获取任务详情
func (s *Service) Get(ctx context.Context, taskID uuid.UUID) (*model.HumanTask, error) {
	return s.taskRepo.FindByID(ctx, taskID)
}

// ListByThread 列出线程内的所有任务
func (s *Service) ListByThread(ctx context.Context, threadID uuid.UUID) ([]*model.HumanTask, error) {
	return s.taskRepo.ListByThread(ctx, threadID)
}