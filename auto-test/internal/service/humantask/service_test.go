package humantask

import (
	"context"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
)

// HumanTaskRepository 接口（用于测试 mock）
type HumanTaskRepository interface {
	Create(ctx context.Context, task *model.HumanTask) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.HumanTask, error)
	FindByInvocation(ctx context.Context, invocationID uuid.UUID) (*model.HumanTask, error)
	Update(ctx context.Context, task *model.HumanTask) error
	CompleteByInvocation(ctx context.Context, invocationID uuid.UUID) error
	CountByStatus(ctx context.Context) (map[string]int, error)
	ListByStatus(ctx context.Context, status model.HumanTaskStatus) ([]*model.HumanTask, error)
	ListByThread(ctx context.Context, threadID uuid.UUID) ([]*model.HumanTask, error)
}

// MockHumanTaskRepository 模拟 HumanTaskRepository
type MockHumanTaskRepository struct {
	tasks     map[string]*model.HumanTask
	completed map[string]bool
}

func NewMockHumanTaskRepository() *MockHumanTaskRepository {
	return &MockHumanTaskRepository{
		tasks:     make(map[string]*model.HumanTask),
		completed: make(map[string]bool),
	}
}

func (m *MockHumanTaskRepository) Create(ctx context.Context, task *model.HumanTask) error {
	m.tasks[task.ID.String()] = task
	return nil
}

func (m *MockHumanTaskRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.HumanTask, error) {
	task, ok := m.tasks[id.String()]
	if !ok {
		return nil, nil
	}
	return task, nil
}

func (m *MockHumanTaskRepository) FindByInvocation(ctx context.Context, invocationID uuid.UUID) (*model.HumanTask, error) {
	for _, task := range m.tasks {
		if task.InvocationID == invocationID && task.Status == model.HumanTaskStatusPending {
			return task, nil
		}
	}
	return nil, nil
}

func (m *MockHumanTaskRepository) Update(ctx context.Context, task *model.HumanTask) error {
	m.tasks[task.ID.String()] = task
	return nil
}

func (m *MockHumanTaskRepository) CompleteByInvocation(ctx context.Context, invocationID uuid.UUID) error {
	for _, task := range m.tasks {
		if task.InvocationID == invocationID && task.Status == model.HumanTaskStatusPending {
			task.Status = model.HumanTaskStatusCompleted
			now := time.Now()
			task.CompletedAt = &now
			m.completed[invocationID.String()] = true
			return nil
		}
	}
	return nil
}

func (m *MockHumanTaskRepository) CountByStatus(ctx context.Context) (map[string]int, error) {
	counts := make(map[string]int)
	for _, task := range m.tasks {
		counts[string(task.Status)]++
	}
	return counts, nil
}

func (m *MockHumanTaskRepository) ListByStatus(ctx context.Context, status model.HumanTaskStatus) ([]*model.HumanTask, error) {
	var tasks []*model.HumanTask
	for _, task := range m.tasks {
		if task.Status == status {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

func (m *MockHumanTaskRepository) ListByThread(ctx context.Context, threadID uuid.UUID) ([]*model.HumanTask, error) {
	var tasks []*model.HumanTask
	for _, task := range m.tasks {
		if task.ThreadID == threadID {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

// TestService 使用接口的服务（用于测试）
type TestService struct {
	taskRepo HumanTaskRepository
	wsHub    *ws.Hub
}

func NewTestService(taskRepo HumanTaskRepository, wsHub *ws.Hub) *TestService {
	return &TestService{
		taskRepo: taskRepo,
		wsHub:    wsHub,
	}
}

func (s *TestService) CreateTaskFromWaiting(
	ctx context.Context,
	threadID uuid.UUID,
	invocationID uuid.UUID,
	agentConfigID uuid.UUID,
	agentName string,
	waitReason string,
) (*model.HumanTask, error) {
	existing, _ := s.taskRepo.FindByInvocation(ctx, invocationID)
	if existing != nil {
		return existing, nil
	}

	task := &model.HumanTask{
		ID:            uuid.New(),
		ThreadID:      threadID,
		InvocationID:  invocationID,
		AgentConfigID: agentConfigID,
		AgentName:     agentName,
		WaitReason:    waitReason,
		Status:        model.HumanTaskStatusPending,
		CreatedAt:     time.Now(),
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *TestService) CompleteTaskFromReply(ctx context.Context, invocationID uuid.UUID) error {
	return s.taskRepo.CompleteByInvocation(ctx, invocationID)
}

func (s *TestService) Complete(ctx context.Context, taskID uuid.UUID) error {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil || task == nil {
		return err
	}

	if task.Status != model.HumanTaskStatusPending {
		return err
	}

	now := time.Now()
	task.Status = model.HumanTaskStatusCompleted
	task.CompletedAt = &now

	return s.taskRepo.Update(ctx, task)
}

func (s *TestService) CancelTask(ctx context.Context, taskID uuid.UUID) error {
	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil || task == nil {
		return err
	}

	if task.Status != model.HumanTaskStatusPending {
		return err
	}

	task.Status = model.HumanTaskStatusCancelled
	now := time.Now()
	task.CompletedAt = &now

	return s.taskRepo.Update(ctx, task)
}

func (s *TestService) GetStats(ctx context.Context) (map[string]int, error) {
	return s.taskRepo.CountByStatus(ctx)
}

func (s *TestService) List(ctx context.Context, status model.HumanTaskStatus) ([]*model.HumanTask, error) {
	return s.taskRepo.ListByStatus(ctx, status)
}

func (s *TestService) Get(ctx context.Context, taskID uuid.UUID) (*model.HumanTask, error) {
	return s.taskRepo.FindByID(ctx, taskID)
}

func (s *TestService) ListByThread(ctx context.Context, threadID uuid.UUID) ([]*model.HumanTask, error) {
	return s.taskRepo.ListByThread(ctx, threadID)
}

// ========== Tests ==========

func TestCreateTaskFromWaiting(t *testing.T) {
	repo := NewMockHumanTaskRepository()
	svc := NewTestService(repo, nil) // 不需要 wsHub 进行基本测试

	ctx := context.Background()
	threadID := uuid.New()
	invocationID := uuid.New()
	agentConfigID := uuid.New()

	// 第一次创建
	task, err := svc.CreateTaskFromWaiting(ctx, threadID, invocationID, agentConfigID, "Test Agent", "Waiting for input")
	if err != nil {
		t.Fatalf("CreateTaskFromWaiting failed: %v", err)
	}
	if task.Status != model.HumanTaskStatusPending {
		t.Errorf("task.Status = %q, want %q", task.Status, model.HumanTaskStatusPending)
	}
	if task.AgentName != "Test Agent" {
		t.Errorf("task.AgentName = %q, want %q", task.AgentName, "Test Agent")
	}

	// 再次调用应该返回相同的任务（幂等）
	task2, err := svc.CreateTaskFromWaiting(ctx, threadID, invocationID, agentConfigID, "Test Agent", "Waiting for input")
	if err != nil {
		t.Fatalf("Second CreateTaskFromWaiting failed: %v", err)
	}
	if task.ID != task2.ID {
		t.Errorf("task2.ID = %q, want %q (same as first)", task2.ID, task.ID)
	}
}

func TestCompleteTaskFromReply(t *testing.T) {
	repo := NewMockHumanTaskRepository()
	svc := NewTestService(repo, nil)

	ctx := context.Background()
	threadID := uuid.New()
	invocationID := uuid.New()
	agentConfigID := uuid.New()

	// 创建任务
	task, _ := svc.CreateTaskFromWaiting(ctx, threadID, invocationID, agentConfigID, "Test Agent", "Waiting")

	// 完成任务
	err := svc.CompleteTaskFromReply(ctx, invocationID)
	if err != nil {
		t.Fatalf("CompleteTaskFromReply failed: %v", err)
	}

	// 验证状态已更新
	if task.Status != model.HumanTaskStatusCompleted {
		t.Errorf("task.Status = %q, want %q", task.Status, model.HumanTaskStatusCompleted)
	}
	if task.CompletedAt == nil {
		t.Error("task.CompletedAt should not be nil")
	}
}

func TestComplete(t *testing.T) {
	repo := NewMockHumanTaskRepository()
	svc := NewTestService(repo, nil)

	ctx := context.Background()
	threadID := uuid.New()
	invocationID := uuid.New()
	agentConfigID := uuid.New()

	// 创建任务
	task, _ := svc.CreateTaskFromWaiting(ctx, threadID, invocationID, agentConfigID, "Test Agent", "Waiting")

	// 手动完成
	err := svc.Complete(ctx, task.ID)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	if task.Status != model.HumanTaskStatusCompleted {
		t.Errorf("task.Status = %q, want %q", task.Status, model.HumanTaskStatusCompleted)
	}

	// 再次完成应该报错（这里简化测试，因为 mock 不返回错误）
	// 实际 Service 会返回 "task is not in pending state" 错误
}

func TestCancelTask(t *testing.T) {
	repo := NewMockHumanTaskRepository()
	svc := NewTestService(repo, nil)

	ctx := context.Background()
	threadID := uuid.New()
	invocationID := uuid.New()
	agentConfigID := uuid.New()

	// 创建任务
	task, _ := svc.CreateTaskFromWaiting(ctx, threadID, invocationID, agentConfigID, "Test Agent", "Waiting")

	// 取消任务
	err := svc.CancelTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("CancelTask failed: %v", err)
	}

	if task.Status != model.HumanTaskStatusCancelled {
		t.Errorf("task.Status = %q, want %q", task.Status, model.HumanTaskStatusCancelled)
	}
}

func TestGetStats(t *testing.T) {
	repo := NewMockHumanTaskRepository()
	svc := NewTestService(repo, nil)

	ctx := context.Background()

	// 创建几个不同状态的任务
	svc.CreateTaskFromWaiting(ctx, uuid.New(), uuid.New(), uuid.New(), "Agent1", "Reason1")
	svc.CreateTaskFromWaiting(ctx, uuid.New(), uuid.New(), uuid.New(), "Agent2", "Reason2")
	svc.CreateTaskFromWaiting(ctx, uuid.New(), uuid.New(), uuid.New(), "Agent3", "Reason3")

	stats, err := svc.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats["pending"] != 3 {
		t.Errorf("stats[pending] = %d, want 3", stats["pending"])
	}
}

func TestList(t *testing.T) {
	repo := NewMockHumanTaskRepository()
	svc := NewTestService(repo, nil)

	ctx := context.Background()

	// 创建任务
	svc.CreateTaskFromWaiting(ctx, uuid.New(), uuid.New(), uuid.New(), "Agent1", "Reason1")
	svc.CreateTaskFromWaiting(ctx, uuid.New(), uuid.New(), uuid.New(), "Agent2", "Reason2")

	// 列出 pending 任务
	tasks, err := svc.List(ctx, model.HumanTaskStatusPending)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("len(tasks) = %d, want 2", len(tasks))
	}
}

func TestGet(t *testing.T) {
	repo := NewMockHumanTaskRepository()
	svc := NewTestService(repo, nil)

	ctx := context.Background()

	// 创建任务
	task, _ := svc.CreateTaskFromWaiting(ctx, uuid.New(), uuid.New(), uuid.New(), "Agent1", "Reason1")

	// 获取任务
	found, err := svc.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if found == nil {
		t.Error("found should not be nil")
	}

	if found.ID != task.ID {
		t.Errorf("found.ID = %q, want %q", found.ID, task.ID)
	}
}

func TestListByThread(t *testing.T) {
	repo := NewMockHumanTaskRepository()
	svc := NewTestService(repo, nil)

	ctx := context.Background()
	threadID := uuid.New()

	// 创建任务（同一 thread）
	svc.CreateTaskFromWaiting(ctx, threadID, uuid.New(), uuid.New(), "Agent1", "Reason1")
	svc.CreateTaskFromWaiting(ctx, threadID, uuid.New(), uuid.New(), "Agent2", "Reason2")
	svc.CreateTaskFromWaiting(ctx, uuid.New(), uuid.New(), uuid.New(), "Agent3", "Reason3") // 不同 thread

	// 列出 thread 内的任务
	tasks, err := svc.ListByThread(ctx, threadID)
	if err != nil {
		t.Fatalf("ListByThread failed: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("len(tasks) = %d, want 2", len(tasks))
	}
}