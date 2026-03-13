package sandbox

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// SandboxService 沙箱服务
type SandboxService struct {
	docker      *DockerClient
	sandboxRepo *repo.SandboxRepository
	activeRuns  map[uuid.UUID]*SandboxRun
	mu          sync.RWMutex
}

// SandboxRun 沙箱运行实例
type SandboxRun struct {
	ID          uuid.UUID
	ThreadID    uuid.UUID
	ContainerID string
	Status      model.SandboxStatus
	StartedAt   time.Time
	EndedAt     *time.Time
	ExitCode    int
	Output      string
}

// NewSandboxService 创建沙箱服务
func NewSandboxService(docker *DockerClient, sandboxRepo *repo.SandboxRepository) *SandboxService {
	return &SandboxService{
		docker:      docker,
		sandboxRepo: sandboxRepo,
		activeRuns:  make(map[uuid.UUID]*SandboxRun),
	}
}

// CreateSandbox 创建沙箱
func (s *SandboxService) CreateSandbox(ctx context.Context, req *CreateSandboxRequest) (*model.Sandbox, error) {
	sandbox := &model.Sandbox{
		ID:        uuid.New(),
		ThreadID:  req.ThreadID,
		Name:      req.Name,
		Image:     req.Image,
		Status:    model.SandboxStatusCreated,
		CreatedAt: time.Now(),
	}

	if err := s.sandboxRepo.Create(ctx, sandbox); err != nil {
		return nil, err
	}

	return sandbox, nil
}

// RunInSandbox 在沙箱中运行代码
func (s *SandboxService) RunInSandbox(ctx context.Context, req *RunRequest) (*RunResult, error) {
	// 创建运行记录
	run := &SandboxRun{
		ID:        uuid.New(),
		ThreadID:  req.ThreadID,
		Status:    model.SandboxStatusRunning,
		StartedAt: time.Now(),
	}

	// 构建容器配置
	config := &ContainerConfig{
		Image:       req.Image,
		Cmd:         req.Command,
		Env:         req.Env,
		WorkDir:     "/workspace",
		MemoryLimit: req.MemoryLimit,
		CPUQuota:    req.CPUQuota,
		Timeout:     req.Timeout,
		Mounts:      req.Mounts,
	}

	// 如果没有指定镜像，使用默认
	if config.Image == "" {
		config.Image = "python:3.11-slim"
	}

	// 如果没有指定内存限制，使用默认
	if config.MemoryLimit == 0 {
		config.MemoryLimit = 512 // 512MB
	}

	// 如果没有指定超时，使用默认
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Minute
	}

	// 创建容器
	containerName := fmt.Sprintf("isdp-run-%s", run.ID.String()[:8])
	containerID, err := s.docker.CreateContainer(ctx, containerName, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	run.ContainerID = containerID

	// 记录活跃运行
	s.mu.Lock()
	s.activeRuns[run.ID] = run
	s.mu.Unlock()

	// 清理函数
	defer func() {
		s.mu.Lock()
		delete(s.activeRuns, run.ID)
		s.mu.Unlock()
	}()

	// 启动容器
	if err := s.docker.StartContainer(ctx, containerID); err != nil {
		s.docker.RemoveContainer(ctx, containerID)
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// 等待完成或超时
	runCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	exitCode, err := s.docker.WaitContainer(runCtx, containerID)
	if err != nil {
		s.docker.StopContainer(ctx, containerID, intPtr(5))
		s.docker.RemoveContainer(ctx, containerID)
		return nil, fmt.Errorf("container wait failed: %w", err)
	}

	// 获取输出
	output, err := s.docker.GetContainerLogs(ctx, containerID)
	if err != nil {
		output = fmt.Sprintf("failed to get logs: %v", err)
	}

	// 清理容器
	s.docker.RemoveContainer(ctx, containerID)

	// 更新状态
	run.Status = model.SandboxStatusComplete
	run.ExitCode = int(exitCode)
	run.Output = output
	now := time.Now()
	run.EndedAt = &now

	return &RunResult{
		RunID:    run.ID,
		ExitCode: int(exitCode),
		Output:   output,
		Duration: now.Sub(run.StartedAt),
	}, nil
}

// RunWithFiles 带文件的沙箱运行
func (s *SandboxService) RunWithFiles(ctx context.Context, req *RunWithFilesRequest) (*RunResult, error) {
	// 创建临时容器
	config := &ContainerConfig{
		Image:       req.Image,
		Cmd:         []string{"tail", "-f", "/dev/null"}, // 保持运行
		WorkDir:     "/workspace",
		MemoryLimit: req.MemoryLimit,
		CPUQuota:    req.CPUQuota,
	}

	containerName := fmt.Sprintf("isdp-workspace-%s", uuid.New().String()[:8])
	containerID, err := s.docker.CreateContainer(ctx, containerName, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// 确保清理
	defer s.docker.RemoveContainer(ctx, containerID)

	// 复制文件到容器
	for path, content := range req.Files {
		if err := s.docker.CopyToContainer(ctx, containerID, path, []byte(content)); err != nil {
			return nil, fmt.Errorf("failed to copy file %s: %w", path, err)
		}
	}

	// 执行命令
	stdout, stderr, err := s.docker.ExecInContainer(ctx, containerID, req.Command)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	return &RunResult{
		ExitCode: 0,
		Output:   stdout + "\n" + stderr,
	}, nil
}

// StopRun 停止运行
func (s *SandboxService) StopRun(ctx context.Context, runID uuid.UUID) error {
	s.mu.RLock()
	run, exists := s.activeRuns[runID]
	s.mu.RUnlock()

	if !exists {
		return ErrRunNotFound
	}

	if err := s.docker.StopContainer(ctx, run.ContainerID, intPtr(5)); err != nil {
		return err
	}

	run.Status = model.SandboxStatusStopped
	now := time.Now()
	run.EndedAt = &now

	return nil
}

// GetRunStatus 获取运行状态
func (s *SandboxService) GetRunStatus(ctx context.Context, runID uuid.UUID) (*SandboxRun, error) {
	s.mu.RLock()
	run, exists := s.activeRuns[runID]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrRunNotFound
	}

	return run, nil
}

// ListActiveRuns 列出活跃运行
func (s *SandboxService) ListActiveRuns(threadID uuid.UUID) []SandboxRun {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var runs []SandboxRun
	for _, run := range s.activeRuns {
		if threadID == uuid.Nil || run.ThreadID == threadID {
			runs = append(runs, *run)
		}
	}
	return runs
}

// CreateSandboxRequest 创建沙箱请求
type CreateSandboxRequest struct {
	ThreadID uuid.UUID
	Name     string
	Image    string
}

// RunRequest 运行请求
type RunRequest struct {
	ThreadID    uuid.UUID
	Image       string
	Command     []string
	Env         []string
	MemoryLimit int64
	CPUQuota    int64
	Timeout     time.Duration
	Mounts      []MountConfig
}

// RunWithFilesRequest 带文件的运行请求
type RunWithFilesRequest struct {
	Image       string
	Files       map[string]string // path -> content
	Command     []string
	MemoryLimit int64
	CPUQuota    int64
}

// RunResult 运行结果
type RunResult struct {
	RunID    uuid.UUID
	ExitCode int
	Output   string
	Duration time.Duration
}

func intPtr(i int) *int {
	return &i
}

var (
	ErrRunNotFound = fmt.Errorf("sandbox run not found")
)