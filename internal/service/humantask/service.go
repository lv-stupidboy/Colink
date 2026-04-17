package humantask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
)

// A2ATriggerFunc A2A 触发函数类型
type A2ATriggerFunc func(ctx context.Context, threadID uuid.UUID, targetAgentID uuid.UUID, content string, callerAgentID uuid.UUID) error

// Service HumanTask 服务
type Service struct {
	taskRepo     *repo.HumanTaskRepository
	msgRepo      *repo.MessageRepository
	threadRepo   *repo.ThreadRepository
	workflowRepo *repo.WorkflowTemplateRepository
	agentRepo    *repo.AgentConfigRepository
	wsHub        *ws.Hub
	a2aTrigger   A2ATriggerFunc
}

// NewService 创建 HumanTask 服务
func NewService(
	taskRepo *repo.HumanTaskRepository,
	msgRepo *repo.MessageRepository,
	threadRepo *repo.ThreadRepository,
	workflowRepo *repo.WorkflowTemplateRepository,
	agentRepo *repo.AgentConfigRepository,
	wsHub *ws.Hub,
) *Service {
	return &Service{
		taskRepo:     taskRepo,
		msgRepo:      msgRepo,
		threadRepo:   threadRepo,
		workflowRepo: workflowRepo,
		agentRepo:    agentRepo,
		wsHub:        wsHub,
	}
}

// SetA2ATrigger 设置 A2A 触发函数（解决循环依赖）
func (s *Service) SetA2ATrigger(trigger A2ATriggerFunc) {
	s.a2aTrigger = trigger
}

// CreateTask 创建人工任务
// 参数:
// - threadID: 线程 ID
// - roleConfigID: 角色配置 ID
// - taskContent: 任务内容
// - sourceInvocationID: 来源 Agent invocation ID
// - sourceAgentName: 来源 Agent 名称
func (s *Service) CreateTask(
	ctx context.Context,
	threadID uuid.UUID,
	roleConfigID uuid.UUID,
	taskContent string,
	sourceInvocationID uuid.UUID,
	sourceAgentName string,
) (*model.HumanTask, error) {
	// 1. 获取角色配置信息
	roleConfig, err := s.agentRepo.FindByID(ctx, roleConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to find role config: %w", err)
	}

	// 2. 从 SystemPrompt 提取期望交付物
	expectedOutput := extractExpectedOutput(roleConfig.SystemPrompt)

	// 3. 创建人工任务记录
	now := time.Now()
	task := &model.HumanTask{
		ID:              uuid.New(),
		ThreadID:        threadID,
		RoleConfigID:    roleConfigID,
		RoleName:        roleConfig.Name,
		TaskType:        model.HumanTaskTypeDispatch, // 默认为任务分发类型
		TaskContent:     taskContent,
		ExpectedOutput:  expectedOutput,
		SourceAgentID:   sourceInvocationID,
		SourceAgentName: sourceAgentName,
		Status:          model.HumanTaskStatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create human task: %w", err)
	}

	// 4. 创建消息记录（任务卡片）
	metadata := buildTaskCardMetadata(task)
	metadataJSON, _ := json.Marshal(metadata)

	msg := &model.Message{
		ThreadID:    threadID,
		Role:        model.MessageRoleAgent,
		AgentID:     roleConfigID.String(),
		Content:     taskContent,
		MessageType: model.MessageTypeText,
		Metadata:    metadataJSON,
	}

	if err := s.msgRepo.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	// 5. 广播 WebSocket 事件
	if s.wsHub != nil {
		s.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "human_task_created",
			ThreadID:  threadID.String(),
			Timestamp: model.Now(),
			Payload: map[string]interface{}{
				"taskId":       task.ID.String(),
				"roleConfigId": roleConfigID.String(),
				"roleName":     task.RoleName,
				"taskContent":  taskContent,
				"expectedOutput": expectedOutput,
				"sourceAgentName": sourceAgentName,
				"status":      string(task.Status),
				"messageId":   msg.ID.String(),
			},
		})
	}

	return task, nil
}

// Submit 提交交付物
// 更新任务状态、创建消息、触发下游 Agent
func (s *Service) Submit(ctx context.Context, taskID uuid.UUID, req *model.SubmitHumanTaskRequest) (*model.SubmitHumanTaskResponse, error) {
	// 1. 获取任务
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to find task: %w", err)
	}

	// 2. 检查状态
	if task.Status != model.HumanTaskStatusPending && task.Status != model.HumanTaskStatusInProgress {
		return nil, fmt.Errorf("task is not in a submittable state: %s", task.Status)
	}

	// 3. 更新任务状态和交付物
	now := time.Now()
	task.Status = model.HumanTaskStatusCompleted
	task.OutputContent = req.OutputContent
	task.OutputFiles = req.OutputFiles
	task.SubmittedAt = &now
	task.UpdatedAt = now

	if err := s.taskRepo.Update(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	// 4. 创建提交消息
	submitContent := fmt.Sprintf("已提交交付物：\n%s", req.OutputContent)
	if len(req.OutputFiles) > 0 {
		submitContent += fmt.Sprintf("\n\n附件文件：\n%s", strings.Join(req.OutputFiles, "\n"))
	}

	msg := &model.Message{
		ThreadID:    task.ThreadID,
		Role:        model.MessageRoleUser,
		Content:     submitContent,
		MessageType: model.MessageTypeText,
	}

	if err := s.msgRepo.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to create submit message: %w", err)
	}

	// 5. 广播 WebSocket 事件
	if s.wsHub != nil {
		s.wsHub.BroadcastToThread(task.ThreadID.String(), ws.WSMessage{
			Type:      "human_task_completed",
			ThreadID:  task.ThreadID.String(),
			Timestamp: model.Now(),
			Payload: map[string]interface{}{
				"taskId":       task.ID.String(),
				"status":       string(task.Status),
				"outputContent": req.OutputContent,
				"outputFiles":  req.OutputFiles,
				"submittedAt":  now.UnixMilli(),
			},
		})
	}

	// 6. 触发下游 Agent
	nextAgent, triggered := s.triggerDownstream(ctx, task)

	return &model.SubmitHumanTaskResponse{
		Success:   true,
		NextAgent: nextAgent,
		Triggered: triggered,
	}, nil
}

// Start 开始执行任务（更改状态为 in_progress）
func (s *Service) Start(ctx context.Context, taskID uuid.UUID) error {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to find task: %w", err)
	}

	if task.Status != model.HumanTaskStatusPending {
		return fmt.Errorf("task is not in pending state: %s", task.Status)
	}

	task.Status = model.HumanTaskStatusInProgress
	task.UpdatedAt = time.Now()

	if err := s.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	// 广播状态变更
	if s.wsHub != nil {
		s.wsHub.BroadcastToThread(task.ThreadID.String(), ws.WSMessage{
			Type:      "human_task_status_changed",
			ThreadID:  task.ThreadID.String(),
			Timestamp: model.Now(),
			Payload: map[string]interface{}{
				"taskId": task.ID.String(),
				"status": string(task.Status),
			},
		})
	}

	return nil
}

// Reject 拒绝任务（更改状态为 rejected）
func (s *Service) Reject(ctx context.Context, taskID uuid.UUID) error {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to find task: %w", err)
	}

	if task.Status != model.HumanTaskStatusPending && task.Status != model.HumanTaskStatusInProgress {
		return fmt.Errorf("task is not in a rejectable state: %s", task.Status)
	}

	task.Status = model.HumanTaskStatusRejected
	task.UpdatedAt = time.Now()

	if err := s.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	// 广播状态变更
	if s.wsHub != nil {
		s.wsHub.BroadcastToThread(task.ThreadID.String(), ws.WSMessage{
			Type:      "human_task_status_changed",
			ThreadID:  task.ThreadID.String(),
			Timestamp: model.Now(),
			Payload: map[string]interface{}{
				"taskId": task.ID.String(),
				"status": string(task.Status),
			},
		})
	}

	return nil
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

// ========== 辅助函数 ==========

// extractExpectedOutput 从 SystemPrompt 提取期望交付物
// 解析格式：职责：xxx\n交付物：yyy
// 返回交付物部分的内容
func extractExpectedOutput(systemPrompt string) string {
	// 首先找到 "交付物：" 或 "交付物:" 的位置
	deliveryIdx := -1
	for _, marker := range []string{"交付物：", "交付物:"} {
		idx := strings.Index(systemPrompt, marker)
		if idx >= 0 {
			deliveryIdx = idx + len(marker)
			break
		}
	}

	if deliveryIdx < 0 {
		return ""
	}

	// 从标记位置开始提取内容
	content := systemPrompt[deliveryIdx:]

	// 检查是否以换行开始（多行交付物格式）
	if strings.HasPrefix(content, "\n") || strings.HasPrefix(content, " ") {
		content = strings.TrimSpace(content)
		// 多行交付物：提取到下一个段落分隔符（空行）或文档结束
		// 或者遇到新的"职责："等字段标记
		endIdx := strings.Index(content, "\n\n")
		if endIdx < 0 {
			// 尝试匹配下一个字段标记
			fieldMarkers := []string{"职责：", "职责:", "说明：", "说明:", "备注：", "备注:"}
			for _, marker := range fieldMarkers {
				idx := strings.Index(content, marker)
				if idx >= 0 && (endIdx < 0 || idx < endIdx) {
					endIdx = idx
				}
			}
		}
		if endIdx >= 0 {
			content = content[:endIdx]
		}
		return strings.TrimSpace(content)
	}

	// 单行交付物：提取到换行或文档结束
	endIdx := strings.Index(content, "\n")
	if endIdx < 0 {
		return strings.TrimSpace(content)
	}
	return strings.TrimSpace(content[:endIdx])
}

// TaskCardMetadata 任务卡片消息元数据
type TaskCardMetadata struct {
	Type           string          `json:"type"`           // "human_task"
	TaskID         string          `json:"taskId"`         // 任务 ID
	RoleName       string          `json:"roleName"`       // 角色名称
	TaskType       model.HumanTaskType `json:"taskType"`   // 任务类型
	ExpectedOutput string          `json:"expectedOutput"` // 期望交付物
	SourceAgentName string         `json:"sourceAgentName"` // 来源 Agent 名称
	Status         string          `json:"status"`         // 任务状态
	CreatedAt      int64           `json:"createdAt"`      // 创建时间
}

// buildTaskCardMetadata 构建任务卡片消息元数据
func buildTaskCardMetadata(task *model.HumanTask) *TaskCardMetadata {
	return &TaskCardMetadata{
		Type:           "human_task",
		TaskID:         task.ID.String(),
		RoleName:       task.RoleName,
		TaskType:       task.TaskType,
		ExpectedOutput: task.ExpectedOutput,
		SourceAgentName: task.SourceAgentName,
		Status:         string(task.Status),
		CreatedAt:      task.CreatedAt.UnixMilli(),
	}
}

// triggerDownstream 触发下游 Agent
// 返回下游 Agent 信息和是否触发成功
func (s *Service) triggerDownstream(ctx context.Context, task *model.HumanTask) (*model.NextAgentInfo, bool) {
	// 1. 获取线程的工作流模板
	thread, err := s.threadRepo.FindByID(ctx, task.ThreadID)
	if err != nil || thread.WorkflowTemplateID == nil {
		return nil, false
	}

	// 2. 获取工作流模板
	workflow, err := s.workflowRepo.FindByID(ctx, *thread.WorkflowTemplateID)
	if err != nil {
		return nil, false
	}

	// 3. 解析 transitions，找到下游 Agent
	var transitions []model.Transition
	if len(workflow.Transitions) > 0 {
		if err := json.Unmarshal(workflow.Transitions, &transitions); err != nil {
			return nil, false
		}
	}

	// 查找以当前角色为源头的转换规则
	roleConfigIDStr := task.RoleConfigID.String()
	for _, t := range transitions {
		if t.FromAgentID == roleConfigIDStr {
			// 找到下游 Agent
			targetAgentID, err := uuid.Parse(t.ToAgentID)
			if err != nil {
				continue
			}

			// 获取目标 Agent 信息
			targetAgent, err := s.agentRepo.FindByID(ctx, targetAgentID)
			if err != nil {
				continue
			}

			// 构建触发内容（包含交付物）
			triggerContent := fmt.Sprintf("收到交付物，请继续执行。\n\n交付内容：\n%s", task.OutputContent)
			if len(task.OutputFiles) > 0 {
				triggerContent += fmt.Sprintf("\n\n附件文件：\n%s", strings.Join(task.OutputFiles, "\n"))
			}

			// 执行 A2A 触发（如果设置了触发函数）
			if s.a2aTrigger != nil {
				go func() {
					// 异步触发下游 Agent
					triggerCtx := context.Background()
					if err := s.a2aTrigger(triggerCtx, task.ThreadID, targetAgentID, triggerContent, task.RoleConfigID); err != nil {
						// 记录错误但不影响任务完成
						println("[WARN] Failed to trigger downstream agent:", err.Error())
					}
				}()
			}

			return &model.NextAgentInfo{
				ID:   t.ToAgentID,
				Name: targetAgent.Name,
			}, true
		}
	}

	return nil, false
}