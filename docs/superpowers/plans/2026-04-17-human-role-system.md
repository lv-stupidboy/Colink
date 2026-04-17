# 人角色系统实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将人角色纳入团队工作流，支持 Agent 将无法完成的任务分发给指定的人角色，人提交交付物后自动流转下游 Agent。

**Architecture:** Agent 通过 @mention 触发人角色时，创建 HumanTask 任务卡片而非启动 CLI。任务卡片嵌入 Thread 消息流，同时在任务中心页面汇总展示。用户提交交付物后，自动触发下游 Agent。

**Tech Stack:** Go (Gin + SQLite) + React (Ant Design + Zustand) + WebSocket

---

## 文件结构

### 新增文件

| 文件 | 职责 |
|------|------|
| `internal/model/human_task.go` | HumanTask 数据模型 |
| `internal/repo/human_task.go` | HumanTaskRepository 数据访问层 |
| `internal/service/humantask/service.go` | HumanTaskService 业务逻辑（创建、提交、流转） |
| `internal/api/human_task_handler.go` | HTTP API 处理器 |
| `sql-change/v1.3.0/sqlite/00003_human_tasks.sql` | 数据库表创建 |
| `sql-change/v1.3.0/sqlite/00004_role_migration.sql` | Role 字段值迁移 |
| `web/src/types/humanTask.ts` | HumanTask TypeScript 类型定义 |
| `web/src/pages/MyTasks/index.tsx` | 任务中心页面 |
| `web/src/components/HumanTaskCard/index.tsx` | 任务卡片组件 |
| `web/src/components/HumanTaskCard/TaskExecuteModal.tsx` | 任务执行弹窗 |

### 修改文件

| 文件 | 变更内容 |
|------|---------|
| `internal/model/agent_config.go` | AgentRole 常量简化为 agent/human |
| `internal/repo/agent_config.go` | FindByRole 支持 agent/human 查询 |
| `internal/service/a2a/a2a_trigger.go` | EnqueueA2ATargets 增加 Role 判断分支 |
| `internal/service/a2a/queue_processor.go` | 增加 CreateHumanTask 方法 |
| `web/src/types/index.ts` | AgentRole 类型修改 + 导入 HumanTask |
| `web/src/api/client.ts` | 新增 humanTasks API 方法 |
| `web/src/layouts/MainLayout.tsx` | 新增"我的任务"菜单项 |
| `web/src/App.tsx` | 新增 /tasks 路由 |

---

## Task 1: 数据库表创建

**Files:**
- Create: `sql-change/v1.3.0/sqlite/00003_human_tasks.sql`

- [ ] **Step 1: 创建 human_tasks 表 SQL 文件**

```sql
-- +goose Up
-- +goose StatementBegin

-- human_tasks 人工任务表
CREATE TABLE IF NOT EXISTS human_tasks (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    role_config_id TEXT NOT NULL,
    role_name TEXT NOT NULL,
    task_type TEXT NOT NULL DEFAULT 'task_dispatch',
    task_content TEXT NOT NULL,
    expected_output TEXT,
    source_agent_id TEXT,
    source_agent_name TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    submitted_at TEXT,
    submitted_by TEXT,
    output_content TEXT,
    output_files TEXT,  -- JSON 数组
    target_agent_id TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_human_tasks_thread ON human_tasks(thread_id);
CREATE INDEX IF NOT EXISTS idx_human_tasks_status ON human_tasks(status);
CREATE INDEX IF NOT EXISTS idx_human_tasks_role_config ON human_tasks(role_config_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS human_tasks;
-- +goose StatementEnd
```

- [ ] **Step 2: 提交 SQL 文件**

```bash
git add sql-change/v1.3.0/sqlite/00003_human_tasks.sql
git commit -m "feat: human_tasks 表结构"
```

---

## Task 2: Role 字段值迁移

**Files:**
- Create: `sql-change/v1.3.0/sqlite/00004_role_migration.sql`

- [ ] **Step 1: 创建 Role 字段迁移 SQL**

```sql
-- +goose Up
-- +goose StatementBegin

-- 将现有细分角色类型统一迁移为 'agent'
-- 现有值: requirement, architect, developer, reviewer, testengineer, devops, fullstack_engineer, custom
-- 新值: agent 或 human
UPDATE agent_configs SET role = 'agent' 
WHERE role IN ('requirement', 'architect', 'developer', 'reviewer', 'testengineer', 'devops', 'fullstack_engineer', 'custom');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 无法精确回滚，保留现有值
-- +goose StatementEnd
```

- [ ] **Step 2: 提交迁移文件**

```bash
git add sql-change/v1.3.0/sqlite/00004_role_migration.sql
git commit -m "feat: agent_configs Role 字段迁移"
```

---

## Task 3: HumanTask 模型定义

**Files:**
- Create: `internal/model/human_task.go`

- [ ] **Step 1: 创建 HumanTask 模型**

```go
package model

import (
	"time"

	"github.com/google/uuid"
)

// HumanTaskStatus 人工任务状态
type HumanTaskStatus string

const (
	HumanTaskStatusPending    HumanTaskStatus = "pending"
	HumanTaskStatusInProgress HumanTaskStatus = "in_progress"
	HumanTaskStatusCompleted  HumanTaskStatus = "completed"
	HumanTaskStatusRejected   HumanTaskStatus = "rejected"
	HumanTaskStatusFailed     HumanTaskStatus = "failed"
)

// HumanTaskType 任务类型
type HumanTaskType string

const (
	HumanTaskTypeDispatch HumanTaskType = "task_dispatch" // 任务分发
	HumanTaskTypeReview   HumanTaskType = "review"        // 审核决策
	HumanTaskTypeConfirm  HumanTaskType = "confirm"       // 人工确认
)

// HumanTask 人工任务
type HumanTask struct {
	ID              uuid.UUID      `json:"id"`
	ThreadID        uuid.UUID      `json:"threadId"`
	RoleConfigID    uuid.UUID      `json:"roleConfigId"`
	RoleName        string         `json:"roleName"`        // 角色名称
	TaskType        HumanTaskType  `json:"taskType"`        // 任务类型
	TaskContent     string         `json:"taskContent"`     // 任务描述
	ExpectedOutput  string         `json:"expectedOutput"`  // 期望交付物
	SourceAgentID   uuid.UUID      `json:"sourceAgentId"`   // 来源 Agent invocation ID
	SourceAgentName string         `json:"sourceAgentName"` // 来源 Agent 名称
	Status          HumanTaskStatus `json:"status"`          // 任务状态
	SubmittedAt     *time.Time     `json:"submittedAt"`     // 提交时间
	SubmittedBy     string         `json:"submittedBy"`     // 提交人
	OutputContent   string         `json:"outputContent"`   // 交付物内容
	OutputFiles     []string       `json:"outputFiles"`     // 交付物文件路径
	TargetAgentID   uuid.UUID      `json:"targetAgentId"`   // 下游目标 Agent ID
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

func (t *HumanTask) TableName() string {
	return "human_tasks"
}

// SubmitHumanTaskRequest 提交交付物请求
type SubmitHumanTaskRequest struct {
	OutputContent string   `json:"outputContent"`
	OutputFiles   []string `json:"outputFiles"`
}

// SubmitHumanTaskResponse 提交响应
type SubmitHumanTaskResponse struct {
	Success   bool              `json:"success"`
	NextAgent *NextAgentInfo    `json:"nextAgent,omitempty"`
	Triggered bool              `json:"triggered"`
}

// NextAgentInfo 下游 Agent 信息
type NextAgentInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
```

- [ ] **Step 2: 提交模型文件**

```bash
git add internal/model/human_task.go
git commit -m "feat: HumanTask 模型定义"
```

---

## Task 4: AgentRole 常量修改

**Files:**
- Modify: `internal/model/agent_config.go:15-23`

- [ ] **Step 1: 修改 AgentRole 常量定义**

将现有细分角色类型改为大类区分：

```go
// AgentRole 角色大类
type AgentRole string

const (
	AgentRoleAgent AgentRole = "agent"  // Agent 角色（CLI 执行）
	AgentRoleHuman AgentRole = "human"  // 人角色（任务卡片）
	
	// 旧角色类型常量（向后兼容，已弃用）
	// 现有数据已迁移为 agent
)
```

保留旧常量定义以向后兼容（可选，标记为 deprecated）。

- [ ] **Step 2: 提交修改**

```bash
git add internal/model/agent_config.go
git commit -m "feat: AgentRole 简化为 agent/human 大类"
```

---

## Task 5: HumanTaskRepository

**Files:**
- Create: `internal/repo/human_task.go`

- [ ] **Step 1: 创建 HumanTaskRepository**

```go
package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

// HumanTaskRepository 人工任务数据访问
type HumanTaskRepository struct {
	BaseRepository
}

// NewHumanTaskRepository 创建 HumanTaskRepository
func NewHumanTaskRepository(db *sql.DB, dbType DBType) *HumanTaskRepository {
	return &HumanTaskRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Create 创建任务
func (r *HumanTaskRepository) Create(ctx context.Context, task *model.HumanTask) error {
	query := `
		INSERT INTO human_tasks (
			id, thread_id, role_config_id, role_name, task_type, task_content, 
			expected_output, source_agent_id, source_agent_name, status,
			submitted_at, submitted_by, output_content, output_files, target_agent_id,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	
	outputFiles, _ := json.Marshal(task.OutputFiles)
	
	var sourceAgentID, targetAgentID, submittedAt interface{}
	if task.SourceAgentID != uuid.Nil {
		sourceAgentID = task.SourceAgentID.String()
	}
	if task.TargetAgentID != uuid.Nil {
		targetAgentID = task.TargetAgentID.String()
	}
	if task.SubmittedAt != nil {
		submittedAt = task.SubmittedAt.Format(time.RFC3339)
	}
	
	_, err := r.DB().ExecContext(ctx, query,
		task.ID.String(), task.ThreadID.String(), task.RoleConfigID.String(),
		task.RoleName, task.TaskType, task.TaskContent,
		task.ExpectedOutput, sourceAgentID, task.SourceAgentName, task.Status,
		submittedAt, task.SubmittedBy, task.OutputContent, outputFiles, targetAgentID,
		task.CreatedAt.Format(time.RFC3339), task.UpdatedAt.Format(time.RFC3339),
	)
	return err
}

// FindByID 查找任务
func (r *HumanTaskRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.HumanTask, error) {
	query := `
		SELECT id, thread_id, role_config_id, role_name, task_type, task_content,
			expected_output, source_agent_id, source_agent_name, status,
			submitted_at, submitted_by, output_content, output_files, target_agent_id,
			created_at, updated_at
		FROM human_tasks WHERE id = ?
	`
	
	task := &model.HumanTask{}
	var outputFiles []byte
	var sourceAgentID, targetAgentID, submittedAt sql.NullString
	
	err := r.DB().QueryRowContext(ctx, query, id.String()).Scan(
		&task.ID, &task.ThreadID, &task.RoleConfigID, &task.RoleName, &task.TaskType, &task.TaskContent,
		&task.ExpectedOutput, &sourceAgentID, &task.SourceAgentName, &task.Status,
		&submittedAt, &task.SubmittedBy, &task.OutputContent, &outputFiles, &targetAgentID,
		&task.CreatedAt, &task.UpdatedAt,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to find human task: %w", err)
	}
	
	json.Unmarshal(outputFiles, &task.OutputFiles)
	return task, nil
}

// ListByThread 查找 Thread 内所有任务
func (r *HumanTaskRepository) ListByThread(ctx context.Context, threadID uuid.UUID) ([]*model.HumanTask, error) {
	query := `
		SELECT id, thread_id, role_config_id, role_name, task_type, task_content,
			expected_output, source_agent_id, source_agent_name, status,
			submitted_at, submitted_by, output_content, output_files, target_agent_id,
			created_at, updated_at
		FROM human_tasks WHERE thread_id = ? ORDER BY created_at DESC
	`
	
	rows, err := r.DB().QueryContext(ctx, query, threadID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	tasks := make([]*model.HumanTask, 0)
	for rows.Next() {
		task := &model.HumanTask{}
		var outputFiles []byte
		var sourceAgentID, targetAgentID, submittedAt sql.NullString
		
		err := rows.Scan(
			&task.ID, &task.ThreadID, &task.RoleConfigID, &task.RoleName, &task.TaskType, &task.TaskContent,
			&task.ExpectedOutput, &sourceAgentID, &task.SourceAgentName, &task.Status,
			&submittedAt, &task.SubmittedBy, &task.OutputContent, &outputFiles, &targetAgentID,
			&task.CreatedAt, &task.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		
		json.Unmarshal(outputFiles, &task.OutputFiles)
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// ListByStatus 查找指定状态的任务
func (r *HumanTaskRepository) ListByStatus(ctx context.Context, status model.HumanTaskStatus) ([]*model.HumanTask, error) {
	query := `
		SELECT id, thread_id, role_config_id, role_name, task_type, task_content,
			expected_output, source_agent_id, source_agent_name, status,
			submitted_at, submitted_by, output_content, output_files, target_agent_id,
			created_at, updated_at
		FROM human_tasks WHERE status = ? ORDER BY created_at DESC
	`
	
	rows, err := r.DB().QueryContext(ctx, query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	tasks := make([]*model.HumanTask, 0)
	for rows.Next() {
		task := &model.HumanTask{}
		var outputFiles []byte
		var sourceAgentID, targetAgentID, submittedAt sql.NullString
		
		err := rows.Scan(
			&task.ID, &task.ThreadID, &task.RoleConfigID, &task.RoleName, &task.TaskType, &task.TaskContent,
			&task.ExpectedOutput, &sourceAgentID, &task.SourceAgentName, &task.Status,
			&submittedAt, &task.SubmittedBy, &task.OutputContent, &outputFiles, &targetAgentID,
			&task.CreatedAt, &task.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		
		json.Unmarshal(outputFiles, &task.OutputFiles)
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// Update 更新任务
func (r *HumanTaskRepository) Update(ctx context.Context, task *model.HumanTask) error {
	query := `
		UPDATE human_tasks SET
			status = ?, submitted_at = ?, submitted_by = ?, output_content = ?, output_files = ?, updated_at = ?
		WHERE id = ?
	`
	
	outputFiles, _ := json.Marshal(task.OutputFiles)
	task.UpdatedAt = time.Now()
	
	var submittedAt interface{}
	if task.SubmittedAt != nil {
		submittedAt = task.SubmittedAt.Format(time.RFC3339)
	}
	
	_, err := r.DB().ExecContext(ctx, query,
		task.Status, submittedAt, task.SubmittedBy, task.OutputContent, outputFiles, task.UpdatedAt,
		task.ID.String(),
	)
	return err
}
```

- [ ] **Step 2: 提交 Repository 文件**

```bash
git add internal/repo/human_task.go
git commit -m "feat: HumanTaskRepository 数据访问层"
```

---

## Task 6: HumanTaskService 业务逻辑

**Files:**
- Create: `internal/service/humantask/service.go`

- [ ] **Step 1: 创建 HumanTaskService**

```go
package humantask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/a2a"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
)

// Service 人工任务服务
type Service struct {
	taskRepo     *repo.HumanTaskRepository
	msgRepo      *repo.MessageRepository
	threadRepo   *repo.ThreadRepository
	workflowRepo *repo.WorkflowTemplateRepository
	agentRepo    *repo.AgentConfigRepository
	a2aTrigger   func(ctx context.Context, deps *a2a.A2ATriggerDeps, opts *a2a.A2ATriggerOptions) (*a2a.A2AResult, error)
	wsHub        *ws.Hub
}

// NewService 创建服务
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

// SetA2ATrigger 设置 A2A 触发函数（延迟注入）
func (s *Service) SetA2ATrigger(trigger func(ctx context.Context, deps *a2a.A2ATriggerDeps, opts *a2a.A2ATriggerOptions) (*a2a.A2AResult, error)) {
	s.a2aTrigger = trigger
}

// CreateTask 创建人工任务（由 A2A 触发调用）
func (s *Service) CreateTask(
	ctx context.Context,
	threadID uuid.UUID,
	roleConfigID uuid.UUID,
	taskContent string,
	sourceInvocationID uuid.UUID,
	sourceAgentName string,
) (*model.HumanTask, error) {
	// 获取角色配置
	roleConfig, err := s.agentRepo.FindByID(ctx, roleConfigID)
	if err != nil {
		return nil, fmt.Errorf("failed to find role config: %w", err)
	}
	
	// 提取期望交付物
	expectedOutput := extractExpectedOutput(roleConfig.SystemPrompt)
	
	// 创建任务
	task := &model.HumanTask{
		ID:              uuid.New(),
		ThreadID:        threadID,
		RoleConfigID:    roleConfigID,
		RoleName:        roleConfig.Name,
		TaskType:        model.HumanTaskTypeDispatch,
		TaskContent:     taskContent,
		ExpectedOutput:  expectedOutput,
		SourceAgentID:   sourceInvocationID,
		SourceAgentName: sourceAgentName,
		Status:          model.HumanTaskStatusPending,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	
	if err := s.taskRepo.Create(ctx, task); err != nil {
		return nil, err
	}
	
	// Thread 内插入任务卡片消息
	msg := &model.Message{
		ID:           uuid.New(),
		ThreadID:     threadID,
		Role:         "system",
		Content:      "",
		MessageType:  "human_task",
		CreatedAt:    time.Now(),
		Metadata:     buildTaskCardMetadata(task),
	}
	
	if err := s.msgRepo.Create(ctx, msg); err != nil {
		// 消息创建失败不影响任务创建，记录日志即可
		fmt.Printf("[WARN] failed to create task card message: %v\n", err)
	}
	
	// 广播任务创建事件
	if s.wsHub != nil {
		s.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "human_task_created",
			ThreadID:  threadID.String(),
			Timestamp: model.Now(),
			Payload:   map[string]interface{}{"task": task},
		})
	}
	
	return task, nil
}

// Submit 提交交付物
func (s *Service) Submit(
	ctx context.Context,
	taskID uuid.UUID,
	req *model.SubmitHumanTaskRequest,
) (*model.SubmitHumanTaskResponse, error) {
	// 获取任务
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to find task: %w", err)
	}
	
	if task.Status != model.HumanTaskStatusPending && task.Status != model.HumanTaskStatusInProgress {
		return nil, fmt.Errorf("task already processed: %s", task.Status)
	}
	
	// 更新任务状态
	task.Status = model.HumanTaskStatusCompleted
	task.OutputContent = req.OutputContent
	task.OutputFiles = req.OutputFiles
	task.SubmittedAt = &time.Time{}
	*task.SubmittedAt = time.Now()
	task.SubmittedBy = "user" // 当前系统为用户自己
	
	if err := s.taskRepo.Update(ctx, task); err != nil {
		return nil, err
	}
	
	// 创建交付物消息
	msg := &model.Message{
		ID:           uuid.New(),
		ThreadID:     task.ThreadID,
		Role:         "human",
		AgentID:      task.RoleConfigID.String(),
		Content:      req.OutputContent,
		MessageType:  "human_output",
		CreatedAt:    time.Now(),
		Metadata:     map[string]interface{}{
			"taskId":     taskID.String(),
			"outputFiles": req.OutputFiles,
		},
	}
	
	if err := s.msgRepo.Create(ctx, msg); err != nil {
		fmt.Printf("[WARN] failed to create output message: %v\n", err)
	}
	
	// 查找下游 Transition
	nextAgent, triggered := s.triggerDownstream(ctx, task)
	
	// 广播任务完成事件
	if s.wsHub != nil {
		s.wsHub.BroadcastToThread(task.ThreadID.String(), ws.WSMessage{
			Type:      "human_task_completed",
			ThreadID:  task.ThreadID.String(),
			Timestamp: model.Now(),
			Payload:   map[string]interface{}{"task": task, "triggered": triggered},
		})
	}
	
	return &model.SubmitHumanTaskResponse{
		Success:   true,
		NextAgent: nextAgent,
		Triggered: triggered,
	}, nil
}

// Start 开始执行任务
func (s *Service) Start(ctx context.Context, taskID uuid.UUID) error {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	
	if task.Status != model.HumanTaskStatusPending {
		return fmt.Errorf("task not in pending status")
	}
	
	task.Status = model.HumanTaskStatusInProgress
	task.UpdatedAt = time.Now()
	
	return s.taskRepo.Update(ctx, task)
}

// Reject 拒绝任务
func (s *Service) Reject(ctx context.Context, taskID uuid.UUID) error {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	
	task.Status = model.HumanTaskStatusRejected
	task.UpdatedAt = time.Now()
	
	return s.taskRepo.Update(ctx, task)
}

// List 获取任务列表
func (s *Service) List(ctx context.Context, status model.HumanTaskStatus) ([]*model.HumanTask, error) {
	return s.taskRepo.ListByStatus(ctx, status)
}

// Get 获取任务详情
func (s *Service) Get(ctx context.Context, taskID uuid.UUID) (*model.HumanTask, error) {
	return s.taskRepo.FindByID(ctx, taskID)
}

// ListByThread 获取 Thread 内任务列表
func (s *Service) ListByThread(ctx context.Context, threadID uuid.UUID) ([]*model.HumanTask, error) {
	return s.taskRepo.ListByThread(ctx, threadID)
}

// extractExpectedOutput 从 SystemPrompt 提取期望交付物
func extractExpectedOutput(systemPrompt string) string {
	lines := strings.Split(systemPrompt, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "交付物：") || strings.HasPrefix(line, "交付物:") {
			return strings.TrimPrefix(strings.TrimPrefix(line, "交付物："), "交付物:")
		}
		if strings.HasPrefix(line, "期望交付物：") || strings.HasPrefix(line, "期望交付物:") {
			return strings.TrimPrefix(strings.TrimPrefix(line, "期望交付物："), "期望交付物:")
		}
	}
	return systemPrompt // 未找到明确交付物描述，返回完整 SystemPrompt
}

// buildTaskCardMetadata 构建任务卡片消息 metadata
func buildTaskCardMetadata(task *model.HumanTask) map[string]interface{} {
	return map[string]interface{}{
		"taskId":         task.ID.String(),
		"taskStatus":     task.Status,
		"taskType":       task.TaskType,
		"roleName":       task.RoleName,
		"taskContent":    task.TaskContent,
		"expectedOutput": task.ExpectedOutput,
		"sourceAgentName": task.SourceAgentName,
	}
}

// triggerDownstream 查找并触发下游 Agent
func (s *Service) triggerDownstream(ctx context.Context, task *model.HumanTask) (*model.NextAgentInfo, bool) {
	// 查找 WorkflowTemplate
	thread, err := s.threadRepo.FindByID(ctx, task.ThreadID)
	if err != nil || thread.WorkflowTemplateID == uuid.Nil {
		return nil, false
	}
	
	workflow, err := s.workflowRepo.FindByID(ctx, thread.WorkflowTemplateID)
	if err != nil {
		return nil, false
	}
	
	// 解析 Transitions
	var transitions []model.Transition
	if len(workflow.Transitions) > 0 {
		json.Unmarshal(workflow.Transitions, &transitions)
	}
	
	// 查找从当前人角色出发的 Transition
	for _, t := range transitions {
		if t.FromAgentID == task.RoleConfigID.String() {
			// 找到下游目标
			nextAgentID := t.ToAgentID
			
			// 获取目标 Agent 信息
			nextConfigID, _ := uuid.Parse(nextAgentID)
			nextConfig, err := s.agentRepo.FindByID(ctx, nextConfigID)
			if err != nil {
				fmt.Printf("[WARN] failed to find next agent config: %v\n", err)
				return nil, false
			}
			
			// 触发下游 Agent（通过 A2A trigger）
			// 这里需要调用 A2A 触发逻辑，传递交付物内容
			// 实际触发在 Task 7 中完善
			
			return &model.NextAgentInfo{
				ID:   nextAgentID,
				Name: nextConfig.Name,
			}, true
		}
	}
	
	return nil, false
}
```

- [ ] **Step 2: 提交 Service 文件**

```bash
git add internal/service/humantask/service.go
git commit -m "feat: HumanTaskService 业务逻辑"
```

---

## Task 7: A2A 触发机制扩展

**Files:**
- Modify: `internal/service/a2a/a2a_trigger.go:45-125`

- [ ] **Step 1: 修改 EnqueueA2ATargets 增加 Role 判断**

在 `EnqueueA2ATargets` 函数中，入队前检查目标角色的 Role 类型：

```go
// 在 EnqueueA2ATargets 函数开头添加依赖注入
// 需要在 A2ATriggerDeps 中添加 HumanTaskService

type A2ATriggerDeps struct {
	Registry      *InvocationRegistry
	Orchestrator  *agent.Orchestrator
	WSHub         *ws.Hub
	Queue         *InvocationQueue
	HumanTaskSvc  *humantask.Service // 新增
	AgentConfigRepo *repo.AgentConfigRepository // 新增
}
```

修改入队逻辑：

```go
// 在 for _, catID := range opts.TargetCats 循环内添加判断
// catID 是 roleConfigID，需要查询 Role 类型

configID, _ := uuid.Parse(catID)
targetConfig, err := deps.AgentConfigRepo.FindByID(ctx, configID)
if err != nil {
	continue // 配置不存在，跳过
}

if targetConfig.Role == model.AgentRoleHuman {
	// 人角色：创建任务卡片
	task, err := deps.HumanTaskSvc.CreateTask(
		ctx,
		opts.ThreadID,
		configID,
		opts.Content,
		opts.ParentInvocationID,
		opts.CallerCatID, // 需要传递来源 Agent 名称
	)
	if err != nil {
		fmt.Printf("[WARN] failed to create human task: %v\n", err)
	}
	enqueued = append(enqueued, catID)
	continue
}

// Agent 角色：继续原有逻辑（入队 SpawnAgent）
```

- [ ] **Step 2: 提交 A2A 触发修改**

```bash
git add internal/service/a2a/a2a_trigger.go
git commit -m "feat: A2A 触发支持人角色判断"
```

---

## Task 8: HumanTaskHandler API

**Files:**
- Create: `internal/api/human_task_handler.go`

- [ ] **Step 1: 创建 HumanTaskHandler**

```go
package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/humantask"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// HumanTaskHandler 人工任务 API 处理器
type HumanTaskHandler struct {
	svc *humantask.Service
}

// NewHumanTaskHandler 创建处理器
func NewHumanTaskHandler(svc *humantask.Service) *HumanTaskHandler {
	return &HumanTaskHandler{svc: svc}
}

// List 获取任务列表
// GET /api/v1/human-tasks?status=pending
func (h *HumanTaskHandler) List(c *gin.Context) {
	statusStr := c.Query("status")
	status := model.HumanTaskStatus(statusStr)
	
	if status == "" {
		// 默认返回所有待处理和进行中的任务
		// 可以改为返回所有状态的任务
		status = model.HumanTaskStatusPending
	}
	
	tasks, err := h.svc.List(c.Request.Context(), status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, tasks)
}

// Get 获取任务详情
// GET /api/v1/human-tasks/:id
func (h *HumanTaskHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	
	task, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	
	c.JSON(http.StatusOK, task)
}

// Submit 提交交付物
// POST /api/v1/human-tasks/:id/submit
func (h *HumanTaskHandler) Submit(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	
	var req model.SubmitHumanTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	resp, err := h.svc.Submit(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, resp)
}

// Start 开始执行任务
// PUT /api/v1/human-tasks/:id/start
func (h *HumanTaskHandler) Start(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	
	if err := h.svc.Start(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "task started"})
}

// Reject 拒绝任务
// PUT /api/v1/human-tasks/:id/reject
func (h *HumanTaskHandler) Reject(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	
	if err := h.svc.Reject(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "task rejected"})
}
```

- [ ] **Step 2: 在 main.go 中注册路由**

需要在 `cmd/server/main.go` 中：
1. 创建 HumanTaskRepository
2. 创建 HumanTaskService
3. 创建 HumanTaskHandler
4. 注册路由

```go
// 在依赖初始化部分添加
humanTaskRepo := repo.NewHumanTaskRepository(db, dbType)
humanTaskSvc := humantask.NewService(humanTaskRepo, msgRepo, threadRepo, workflowRepo, agentConfigRepo, wsHub)
humanTaskHandler := api.NewHumanTaskHandler(humanTaskSvc)

// 注册路由
v1.GET("/human-tasks", humanTaskHandler.List)
v1.GET("/human-tasks/:id", humanTaskHandler.Get)
v1.POST("/human-tasks/:id/submit", humanTaskHandler.Submit)
v1.PUT("/human-tasks/:id/start", humanTaskHandler.Start)
v1.PUT("/human-tasks/:id/reject", humanTaskHandler.Reject)
```

- [ ] **Step 3: 提交 API 文件**

```bash
git add internal/api/human_task_handler.go cmd/server/main.go
git commit -m "feat: HumanTask API 接口"
```

---

## Task 9: 前端类型定义

**Files:**
- Create: `web/src/types/humanTask.ts`
- Modify: `web/src/types/index.ts`

- [ ] **Step 1: 创建 HumanTask 类型文件**

```typescript
// web/src/types/humanTask.ts

// HumanTask 状态
export type HumanTaskStatus = 'pending' | 'in_progress' | 'completed' | 'rejected' | 'failed';

// HumanTask 类型
export type HumanTaskType = 'task_dispatch' | 'review' | 'confirm';

// HumanTask 人工任务
export interface HumanTask {
  id: string;
  threadId: string;
  roleConfigId: string;
  roleName: string;
  taskType: HumanTaskType;
  taskContent: string;
  expectedOutput: string;
  sourceAgentId: string;
  sourceAgentName: string;
  status: HumanTaskStatus;
  submittedAt?: string;
  submittedBy?: string;
  outputContent?: string;
  outputFiles?: string[];
  targetAgentId?: string;
  createdAt: string;
  updatedAt: string;
}

// 提交交付物请求
export interface SubmitHumanTaskRequest {
  outputContent: string;
  outputFiles?: string[];
}

// 提交响应
export interface SubmitHumanTaskResponse {
  success: boolean;
  nextAgent?: {
    id: string;
    name: string;
  };
  triggered: boolean;
}
```

- [ ] **Step 2: 修改 index.ts 导入 HumanTask**

```typescript
// 在 web/src/types/index.ts 末尾添加

// HumanTask 类型
export * from './humanTask';

// AgentRole 类型修改（简化为 agent/human）
export type AgentRole = 'agent' | 'human';
```

- [ ] **Step 3: 提交类型文件**

```bash
git add web/src/types/humanTask.ts web/src/types/index.ts
git commit -m "feat: HumanTask TypeScript 类型定义"
```

---

## Task 10: API Client 扩展

**Files:**
- Modify: `web/src/api/client.ts`

- [ ] **Step 1: 添加 humanTasks API 方法**

在 `APIClient` 类中添加 `humanTasks` 方法组：

```typescript
// 在 APIClient 类中添加

// HumanTask API
humanTasks = {
  list: (status?: HumanTaskStatus): Promise<HumanTask[]> => {
    const url = status ? `/human-tasks?status=${status}` : '/human-tasks';
    return this.request(url, 'GET');
  },
  get: (id: string): Promise<HumanTask> =>
    this.request(`/human-tasks/${id}`, 'GET'),
  submit: (id: string, data: SubmitHumanTaskRequest): Promise<SubmitHumanTaskResponse> =>
    this.request(`/human-tasks/${id}/submit`, 'POST', data),
  start: (id: string): Promise<{ message: string }> =>
    this.request(`/human-tasks/${id}/start`, 'PUT'),
  reject: (id: string): Promise<{ message: string }> =>
    this.request(`/human-tasks/${id}/reject`, 'PUT'),
};
```

同时在导入部分添加：

```typescript
import type {
  // ...existing imports
  HumanTask,
  HumanTaskStatus,
  SubmitHumanTaskRequest,
  SubmitHumanTaskResponse,
} from '@/types';
```

- [ ] **Step 2: 提交 API Client**

```bash
git add web/src/api/client.ts
git commit -m "feat: humanTasks API 方法"
```

---

## Task 11: 任务卡片组件

**Files:**
- Create: `web/src/components/HumanTaskCard/index.tsx`
- Create: `web/src/components/HumanTaskCard/TaskExecuteModal.tsx`

- [ ] **Step 1: 创建 HumanTaskCard 组件**

```tsx
// web/src/components/HumanTaskCard/index.tsx

import React from 'react';
import { Card, Button, Space, Tag, Typography } from 'antd';
import { ClockCircleOutlined, UserOutlined, FileTextOutlined } from '@ant-design/icons';
import type { HumanTask } from '@/types';

const { Text, Paragraph } = Typography;

interface HumanTaskCardProps {
  task: HumanTask;
  onExecute?: () => void;
  onViewContext?: () => void;
  compact?: boolean; // 紧凑模式（用于任务中心列表）
}

const statusColors: Record<string, string> = {
  pending: 'orange',
  in_progress: 'blue',
  completed: 'green',
  rejected: 'red',
  failed: 'red',
};

const statusLabels: Record<string, string> = {
  pending: '待处理',
  in_progress: '进行中',
  completed: '已完成',
  rejected: '已拒绝',
  failed: '失败',
};

const HumanTaskCard: React.FC<HumanTaskCardProps> = ({
  task,
  onExecute,
  onViewContext,
  compact = false,
}) => {
  const timeAgo = () => {
    const created = new Date(task.createdAt);
    const now = new Date();
    const diffMs = now.getTime() - created.getTime();
    const diffMin = Math.floor(diffMs / 60000);
    if (diffMin < 60) return `${diffMin}分钟前`;
    const diffHour = Math.floor(diffMin / 60);
    if (diffHour < 24) return `${diffHour}小时前`;
    return `${Math.floor(diffHour / 24)}天前`;
  };

  if (compact) {
    // 紧凑模式：任务中心列表项
    return (
      <Card
        size="small"
        style={{ marginBottom: 8 }}
        actions={task.status === 'pending' ? [
          <Button type="link" size="small" onClick={onExecute}>执行任务</Button>,
        ] : undefined}
      >
        <Space direction="vertical" size="small" style={{ width: '100%' }}>
          <Space>
            <FileTextOutlined />
            <Text strong>{task.roleName}: {task.taskContent.slice(0, 50)}...</Text>
          </Space>
          <Space split="|">
            <Text type="secondary">来源: @{task.sourceAgentName}</Text>
            <Text type="secondary"><ClockCircleOutlined /> {timeAgo()}</Text>
          </Space>
        </Space>
      </Card>
    );
  }

  // 完整模式：Thread 内卡片
  return (
    <Card
      title={
        <Space>
          <FileTextOutlined />
          <span>任务: {task.roleName}</span>
          <Tag color={statusColors[task.status]}>{statusLabels[task.status]}</Tag>
        </Space>
      }
      style={{ marginBottom: 16 }}
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Space>
          <UserOutlined />
          <Text type="secondary">来源: @{task.sourceAgentName}</Text>
        </Space>

        <div>
          <Text type="secondary">任务描述:</Text>
          <Paragraph
            style={{
              background: 'var(--bg-container)',
              padding: 12,
              borderRadius: 4,
              marginTop: 8,
            }}
          >
            {task.taskContent}
          </Paragraph>
        </div>

        <div>
          <Text type="secondary">期望交付物:</Text>
          <Paragraph type="secondary" style={{ marginTop: 4 }}>
            {task.expectedOutput}
          </Paragraph>
        </div>

        {task.status === 'pending' && (
          <Space>
            <Button type="primary" onClick={onExecute}>执行任务</Button>
            <Button onClick={onViewContext}>查看上下文</Button>
          </Space>
        )}

        {task.status === 'completed' && task.outputContent && (
          <div>
            <Text type="secondary">交付物:</Text>
            <Paragraph
              style={{
                background: 'var(--bg-container)',
                padding: 12,
                borderRadius: 4,
                marginTop: 8,
              }}
            >
              {task.outputContent}
            </Paragraph>
          </div>
        )}
      </Space>
    </Card>
  );
};

export default HumanTaskCard;
```

- [ ] **Step 2: 创建 TaskExecuteModal 组件**

```tsx
// web/src/components/HumanTaskCard/TaskExecuteModal.tsx

import React, { useState } from 'react';
import { Modal, Input, Button, Upload, Space, message } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import type { HumanTask, SubmitHumanTaskRequest } from '@/types';
import { api } from '@/api/client';

interface TaskExecuteModalProps {
  task: HumanTask;
  visible: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const TaskExecuteModal: React.FC<TaskExecuteModalProps> = ({
  task,
  visible,
  onClose,
  onSuccess,
}) => {
  const [outputContent, setOutputContent] = useState('');
  const [outputFiles, setOutputFiles] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async () => {
    if (!outputContent.trim()) {
      message.warning('请填写交付内容');
      return;
    }

    setLoading(true);
    try {
      const req: SubmitHumanTaskRequest = {
        outputContent,
        outputFiles,
      };
      const result = await api.humanTasks.submit(task.id, req);
      
      if (result.success) {
        message.success('提交成功');
        if (result.triggered && result.nextAgent) {
          message.info(`已触发下游 Agent: ${result.nextAgent.name}`);
        }
        onSuccess();
        onClose();
      }
    } catch (err: any) {
      message.error(err.message || '提交失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      title={`执行任务: ${task.roleName}`}
      open={visible}
      onCancel={onClose}
      footer={[
        <Button key="cancel" onClick={onClose}>取消</Button>,
        <Button key="submit" type="primary" loading={loading} onClick={handleSubmit}>
          提交
        </Button>,
      ]}
      width={600}
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <div>
          <div style={{ marginBottom: 8 }}>任务描述:</div>
          <div style={{ background: 'var(--bg-container)', padding: 12, borderRadius: 4 }}>
            {task.taskContent}
          </div>
        </div>

        <div>
          <div style={{ marginBottom: 8 }}>交付内容:</div>
          <Input.TextArea
            rows={6}
            value={outputContent}
            onChange={(e) => setOutputContent(e.target.value)}
            placeholder="请填写交付物内容..."
          />
        </div>

        <div>
          <div style={{ marginBottom: 8 }}>上传文件:</div>
          <Upload
            beforeUpload={(file) => {
              // 暂时只记录文件名，后续需要实现真实上传
              setOutputFiles([...outputFiles, file.name]);
              return false; // 阻止自动上传
            }}
            fileList={outputFiles.map((name, idx) => ({
              uid: `${idx}`,
              name,
              status: 'done',
            }))}
            onRemove={(file) => {
              setOutputFiles(outputFiles.filter((f) => f !== file.name));
            }}
          >
            <Button icon={<UploadOutlined />}>选择文件</Button>
          </Upload>
        </div>
      </Space>
    </Modal>
  );
};

export default TaskExecuteModal;
```

- [ ] **Step 3: 提交组件文件**

```bash
git add web/src/components/HumanTaskCard/index.tsx web/src/components/HumanTaskCard/TaskExecuteModal.tsx
git commit -m "feat: HumanTaskCard 任务卡片组件"
```

---

## Task 12: 任务中心页面

**Files:**
- Create: `web/src/pages/MyTasks/index.tsx`

- [ ] **Step 1: 创建 MyTasks 页面**

```tsx
// web/src/pages/MyTasks/index.tsx

import React, { useState, useEffect } from 'react';
import { Tabs, Card, Empty, Spin, Badge } from 'antd';
import { FileTextOutlined } from '@ant-design/icons';
import HumanTaskCard from '@/components/HumanTaskCard';
import TaskExecuteModal from '@/components/HumanTaskCard/TaskExecuteModal';
import { api } from '@/api/client';
import type { HumanTask, HumanTaskStatus } from '@/types';

const MyTasks: React.FC = () => {
  const [tasks, setTasks] = useState<HumanTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<HumanTaskStatus>('pending');
  const [executeTask, setExecuteTask] = useState<HumanTask | null>(null);

  const loadTasks = async (status?: HumanTaskStatus) => {
    setLoading(true);
    try {
      const data = await api.humanTasks.list(status);
      setTasks(data);
    } catch (err) {
      console.error('Failed to load tasks:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadTasks(activeTab);
  }, [activeTab]);

  const handleExecute = (task: HumanTask) => {
    setExecuteTask(task);
  };

  const handleExecuteSuccess = () => {
    loadTasks(activeTab);
    setExecuteTask(null);
  };

  const filteredTasks = tasks.filter((t) => t.status === activeTab);

  const tabItems = [
    {
      key: 'pending',
      label: (
        <Badge count={tasks.filter((t) => t.status === 'pending').length} offset={[10, 0]}>
          <span>待处理</span>
        </Badge>
      ),
    },
    {
      key: 'in_progress',
      label: (
        <Badge count={tasks.filter((t) => t.status === 'in_progress').length} offset={[10, 0]}>
          <span>进行中</span>
        </Badge>
      ),
    },
    {
      key: 'completed',
      label: '已完成',
    },
    {
      key: 'rejected',
      label: '已拒绝',
    },
  ];

  return (
    <Card
      title={
        <span>
          <FileTextOutlined style={{ marginRight: 8 }} />
          我的任务
        </span>
      }
    >
      <Tabs
        activeKey={activeTab}
        onChange={(key) => setActiveTab(key as HumanTaskStatus)}
        items={tabItems}
      />

      {loading ? (
        <Spin style={{ display: 'block', margin: '20px auto' }} />
      ) : filteredTasks.length === 0 ? (
        <Empty description="暂无任务" />
      ) : (
        <div>
          {filteredTasks.map((task) => (
            <HumanTaskCard
              key={task.id}
              task={task}
              compact
              onExecute={() => handleExecute(task)}
              onViewContext={() => {
                // TODO: 跳转到 Thread 页面
              }}
            />
          ))}
        </div>
      )}

      {executeTask && (
        <TaskExecuteModal
          task={executeTask}
          visible={!!executeTask}
          onClose={() => setExecuteTask(null)}
          onSuccess={handleExecuteSuccess}
        />
      )}
    </Card>
  );
};

export default MyTasks;
```

- [ ] **Step 2: 提交页面文件**

```bash
git add web/src/pages/MyTasks/index.tsx
git commit -m "feat: MyTasks 任务中心页面"
```

---

## Task 13: 路由和菜单配置

**Files:**
- Modify: `web/src/layouts/MainLayout.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: 添加菜单项**

在 `MainLayout.tsx` 的 `menuItems` 中添加"我的任务"菜单项：

```tsx
// 在 menuItems 数组开头添加
{
  key: '/tasks',
  icon: <FileTextOutlined />,
  label: '我的任务',
},
```

同时在导入部分确保 `FileTextOutlined` 已导入（已存在）。

- [ ] **Step 2: 添加路由**

在 `App.tsx` 中添加 `/tasks` 路由：

```tsx
// 在 routes 配置中添加
<Route path="/tasks" element={<MyTasks />} />
```

需要导入 MyTasks 组件：

```tsx
import MyTasks from '@/pages/MyTasks';
```

- [ ] **Step 3: 提交路由配置**

```bash
git add web/src/layouts/MainLayout.tsx web/src/App.tsx
git commit -m "feat: 我的任务路由和菜单"
```

---

## Task 14: Thread 内任务卡片展示

**Files:**
- Modify: `web/src/pages/ThreadView/MessageList.tsx` (或对应的消息渲染组件)

- [ ] **Step 1: 添加 human_task 消息类型渲染**

在消息渲染逻辑中，识别 `messageType === 'human_task'` 的消息，渲染 HumanTaskCard：

```tsx
// 在消息渲染组件中添加
if (message.messageType === 'human_task' && message.metadata) {
  const task: HumanTask = {
    id: message.metadata.taskId as string,
    threadId: message.threadId,
    roleConfigId: '', // 从 metadata 获取或通过 API 查询
    roleName: message.metadata.roleName as string,
    taskType: message.metadata.taskType as HumanTaskType,
    taskContent: message.metadata.taskContent as string,
    expectedOutput: message.metadata.expectedOutput as string,
    sourceAgentId: '', // 从 metadata 获取
    sourceAgentName: message.metadata.sourceAgentName as string,
    status: message.metadata.taskStatus as HumanTaskStatus,
    createdAt: message.createdAt,
    updatedAt: message.createdAt,
  };

  return (
    <HumanTaskCard
      task={task}
      onExecute={() => {
        // 打开执行弹窗
      }}
      onViewContext={() => {
        // 滚动到上下文位置
      }}
    />
  );
}
```

- [ ] **Step 2: 提交消息渲染修改**

```bash
git add web/src/pages/ThreadView/MessageList.tsx
git commit -m "feat: Thread 内任务卡片展示"
```

---

## Task 15: 集成测试

**Files:**
- Manual testing

- [ ] **Step 1: 测试人角色创建**

1. 创建一个人角色配置（Role="human"）
2. 设置 MentionPatterns（如 ["@人工审核员"]）
3. 配置 SystemPrompt（包含"交付物："描述）

- [ ] **Step 2: 测试触发机制**

1. Agent 输出 "@人工审核员 请审查..."
2. 验证任务卡片正确创建
3. Thread 内展示任务卡片
4. 任务中心页面显示任务

- [ ] **Step 3: 测试交付物提交**

1. 点击"执行任务"
2. 填写交付物内容
3. 提交后验证：
   - 任务状态变为 completed
   - Thread 内显示交付物消息
   - 下游 Agent 被触发（如果配置了 Transition）

- [ ] **Step 4: 测试自动流转**

1. 在 WorkflowTemplate 中配置从人角色到 Agent 的 Transition
2. 提交交付物后验证下游 Agent 自动启动

---

## 验证清单

| 功能点 | 验证方式 |
|-------|---------|
| 人角色创建 | 前端角色管理页面创建 Role="human" 的配置 |
| @mention 触发 | Agent 输出 @人角色，验证任务卡片创建 |
| Thread 内展示 | Thread 消息流中显示任务卡片 |
| 任务中心 | /tasks 页面显示所有状态任务 |
| 状态变更 | 点击执行任务，状态变为 in_progress |
| 交付物提交 | 提交文本内容，验证存储和展示 |
| 自动流转 | 配置 Transition 后，提交触发下游 Agent |
| WebSocket | 任务状态变更实时推送 |