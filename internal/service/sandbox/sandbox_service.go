package sandbox

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	pkgexec "github.com/anthropic/isdp/pkg/exec"
	"github.com/google/uuid"
)

// SandboxService 沙箱服务
type SandboxService struct {
	docker        *DockerClient
	sandboxRepo   *repo.SandboxRepository
	localRunner   *LocalProcessRunner
	activeRuns    map[uuid.UUID]*SandboxRun
	activeServers map[uuid.UUID]*ProjectServer
	mu            sync.RWMutex
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

// ProjectServer 项目服务器实例
type ProjectServer struct {
	ID          uuid.UUID
	ThreadID    uuid.UUID
	ProjectPath string
	Mode        RunMode
	Port        int
	URL         string
	Process     interface{} // *exec.Cmd for local
	ContainerID string      // Docker容器ID
	Status      string
	StartedAt   time.Time
	CancelFunc  context.CancelFunc
	KillFunc    func() error // 用于杀死进程树（Windows兼容）
}

// NewSandboxService 创建沙箱服务
func NewSandboxService(docker *DockerClient, sandboxRepo *repo.SandboxRepository) *SandboxService {
	return &SandboxService{
		docker:        docker,
		sandboxRepo:   sandboxRepo,
		localRunner:   NewLocalProcessRunner("./repos"),
		activeRuns:    make(map[uuid.UUID]*SandboxRun),
		activeServers: make(map[uuid.UUID]*ProjectServer),
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
func (s *SandboxService) RunInSandbox(ctx context.Context, req *RunRequest) (*DockerRunResult, error) {
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

	return &DockerRunResult{
		RunID:    run.ID,
		ExitCode: int(exitCode),
		Output:   output,
		Duration: now.Sub(run.StartedAt),
	}, nil
}

// RunWithFiles 带文件的沙箱运行
func (s *SandboxService) RunWithFiles(ctx context.Context, req *RunWithFilesRequest) (*DockerRunResult, error) {
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

	return &DockerRunResult{
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

// DockerRunResult Docker运行结果
type DockerRunResult struct {
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
	ErrServerNotFound = fmt.Errorf("project server not found")
)

// RunProjectRequest 运行项目请求
type RunProjectRequest struct {
	ThreadID    uuid.UUID
	ProjectPath string
	Mode        RunMode
}

// RunProjectResponse 运行项目响应
type RunProjectResponse struct {
	ID          uuid.UUID `json:"id"`
	ThreadID    uuid.UUID `json:"threadId"`
	ProjectPath string    `json:"projectPath"`
	Mode        RunMode   `json:"mode"`
	Port        int       `json:"port"`
	URL         string    `json:"url"`
	Status      string    `json:"status"`
}

// RunProject 运行项目到沙箱
func (s *SandboxService) RunProject(ctx context.Context, req *RunProjectRequest) (*RunProjectResponse, error) {
	if req.Mode == "" {
		req.Mode = RunModeLocal
	}

	// 检查项目路径是否存在
	if req.ProjectPath == "" {
		return nil, fmt.Errorf("project path is required")
	}

	switch req.Mode {
	case RunModeDocker:
		return s.runProjectInDocker(ctx, req)
	case RunModeLocal:
		return s.runProjectLocal(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported run mode: %s", req.Mode)
	}
}

// runProjectLocal 本地运行项目
func (s *SandboxService) runProjectLocal(ctx context.Context, req *RunProjectRequest) (*RunProjectResponse, error) {
	server := &ProjectServer{
		ID:          uuid.New(),
		ThreadID:    req.ThreadID,
		ProjectPath: req.ProjectPath,
		Mode:        RunModeLocal,
		Status:      "starting",
		StartedAt:   time.Now(),
	}

	// 获取启动命令
	cmd, args, port := s.localRunner.GetStartCommand(req.ProjectPath)
	if cmd == "" {
		projectType := s.localRunner.DetectProjectType(req.ProjectPath)
		return nil, fmt.Errorf("unable to determine start command for project (detected type: %s, path: %s). Please ensure the project has package.json, requirements.txt, go.mod, or .html files", projectType, req.ProjectPath)
	}

	// 检查端口是否被占用，如果被占用则杀掉占用的进程
	if err := s.killProcessOnPort(port); err != nil {
		fmt.Printf("Warning: failed to kill process on port %d: %v\n", port, err)
	}

	server.Port = port
	server.URL = fmt.Sprintf("http://localhost:%d", port)

	// 创建可取消的上下文
	serverCtx, cancel := context.WithCancel(context.Background())
	server.CancelFunc = cancel

	// 记录活跃服务器
	s.mu.Lock()
	s.activeServers[server.ID] = server
	s.mu.Unlock()

	// 异步启动服务
	var execCmd *LocalProcessCmd
	execCmd, err := s.localRunner.RunWithOutput(serverCtx, req.ProjectPath, cmd, func(output string) {
		// 可以在这里广播输出到WebSocket
	}, args...)

	if err != nil {
		cancel()
		s.mu.Lock()
		delete(s.activeServers, server.ID)
		s.mu.Unlock()
		return nil, err
	}

	// 设置 KillFunc 用于杀死进程树
	server.KillFunc = execCmd.Kill

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.activeServers, server.ID)
			s.mu.Unlock()
		}()

		result := <-execCmd.Result
		if result.Error != "" {
			server.Status = "error"
		} else {
			server.Status = "stopped"
		}
		_ = result
	}()

	// 等待服务启动 - 检查端口是否可连接
	if err := s.waitForServer(port, 10*time.Second); err != nil {
		cancel()
		s.mu.Lock()
		delete(s.activeServers, server.ID)
		s.mu.Unlock()
		return nil, err
	}

	server.Status = "running"

	return &RunProjectResponse{
		ID:          server.ID,
		ThreadID:    server.ThreadID,
		ProjectPath: server.ProjectPath,
		Mode:        server.Mode,
		Port:        server.Port,
		URL:         server.URL,
		Status:      server.Status,
	}, nil
}

// runProjectInDocker 在Docker容器中运行项目
func (s *SandboxService) runProjectInDocker(ctx context.Context, req *RunProjectRequest) (*RunProjectResponse, error) {
	fmt.Printf("[runProjectInDocker] Starting docker project, path: %s\n", req.ProjectPath)

	server := &ProjectServer{
		ID:          uuid.New(),
		ThreadID:    req.ThreadID,
		ProjectPath: req.ProjectPath,
		Mode:        RunModeDocker,
		Status:      "starting",
		StartedAt:   time.Now(),
	}

	// 检测项目类型
	projectType := s.localRunner.DetectProjectType(req.ProjectPath)
	fmt.Printf("[runProjectInDocker] Detected project type: %s\n", projectType)
	if projectType == "unknown" {
		return nil, fmt.Errorf("unable to detect project type for path: %s", req.ProjectPath)
	}

	// 获取Docker镜像
	image := s.getDockerImage(projectType)
	fmt.Printf("[runProjectInDocker] Using image: %s\n", image)

	// 获取可用端口
	hostPort, err := s.getAvailablePort()
	if err != nil {
		fmt.Printf("[runProjectInDocker] Failed to get port: %v\n", err)
		return nil, fmt.Errorf("failed to get available port: %w", err)
	}
	fmt.Printf("[runProjectInDocker] Using host port: %d\n", hostPort)

	// 获取容器内服务端口
	_, _, containerPort := s.localRunner.GetStartCommand(req.ProjectPath)
	if containerPort == 0 {
		containerPort = 8080 // 默认端口
	}
	fmt.Printf("[runProjectInDocker] Container port: %d\n", containerPort)

	// 创建容器配置
	config := &ContainerConfig{
		Image:       image,
		Cmd:         []string{"tail", "-f", "/dev/null"}, // 先启动一个保持运行的进程
		WorkDir:     "/workspace",
		MemoryLimit: 1024, // 1GB
		Mounts: []MountConfig{
			{
				Source:   req.ProjectPath,
				Target:   "/workspace",
				ReadOnly: false,
			},
		},
		PortBindings: []PortBinding{
			{ContainerPort: containerPort, HostPort: hostPort},
		},
	}

	// 创建容器
	containerName := fmt.Sprintf("isdp-project-%s", server.ID.String()[:8])
	fmt.Printf("[runProjectInDocker] Creating container: %s\n", containerName)
	containerID, err := s.docker.CreateContainer(ctx, containerName, config)
	if err != nil {
		fmt.Printf("[runProjectInDocker] Failed to create container: %v\n", err)
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	server.ContainerID = containerID
	fmt.Printf("[runProjectInDocker] Container created: %s\n", containerID[:12])

	// 启动容器
	fmt.Printf("[runProjectInDocker] Starting container...\n")
	if err := s.docker.StartContainer(ctx, containerID); err != nil {
		s.docker.RemoveContainer(ctx, containerID)
		fmt.Printf("[runProjectInDocker] Failed to start container: %v\n", err)
		return nil, fmt.Errorf("failed to start container: %w", err)
	}
	fmt.Printf("[runProjectInDocker] Container started\n")

	// 安装依赖
	if err := s.installDepsInContainer(ctx, containerID, projectType); err != nil {
		// 记录警告但不失败
		fmt.Printf("Warning: failed to install dependencies: %v\n", err)
	}

	// 获取实际端口映射
	ports, err := s.docker.GetContainerPorts(ctx, containerID)
	if err == nil {
		if p, ok := ports[containerPort]; ok {
			hostPort = p
		}
	}

	server.Port = hostPort
	server.URL = fmt.Sprintf("http://localhost:%d", hostPort)

	// 启动开发服务器
	startCmd, startArgs := s.getDockerStartCommand(projectType)
	if startCmd != "" {
		go func() {
			cmd := append([]string{startCmd}, startArgs...)
			_, _, _ = s.docker.ExecInContainer(ctx, containerID, cmd)
		}()
	}

	// 创建可取消的上下文（用于未来可能的取消操作）
	_, cancel := context.WithCancel(context.Background())
	server.CancelFunc = cancel

	// 记录活跃服务器
	s.mu.Lock()
	s.activeServers[server.ID] = server
	s.mu.Unlock()

	// 等待服务启动
	if err := s.waitForServer(hostPort, 30*time.Second); err != nil {
		cancel()
		s.mu.Lock()
		delete(s.activeServers, server.ID)
		s.mu.Unlock()
		s.docker.StopContainer(ctx, containerID, intPtr(5))
		s.docker.RemoveContainer(ctx, containerID)
		return nil, fmt.Errorf("failed to start project server: %w", err)
	}

	server.Status = "running"

	return &RunProjectResponse{
		ID:          server.ID,
		ThreadID:    server.ThreadID,
		ProjectPath: server.ProjectPath,
		Mode:        server.Mode,
		Port:        server.Port,
		URL:         server.URL,
		Status:      server.Status,
	}, nil
}

// getDockerImage 根据项目类型选择镜像
func (s *SandboxService) getDockerImage(projectType string) string {
	images := map[string]string{
		"node":   "node:20-alpine",
		"react":  "node:22-alpine",
		"vue":    "node:22-alpine",
		"python": "python:3.11-slim",
		"go":     "golang:1.21-alpine",
		"static": "python:3.11-slim",
	}
	if img, ok := images[projectType]; ok {
		return img
	}
	return "node:20-alpine" // 默认镜像
}

// getAvailablePort 获取可用端口
func (s *SandboxService) getAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// installDepsInContainer 在容器中安装依赖
func (s *SandboxService) installDepsInContainer(ctx context.Context, containerID string, projectType string) error {
	var cmds [][]string

	switch projectType {
	case "node", "react", "vue":
		cmds = [][]string{
			{"sh", "-c", "cd /workspace && npm install"},
		}
	case "python":
		cmds = [][]string{
			{"sh", "-c", "cd /workspace && pip install -r requirements.txt"},
		}
	case "go":
		cmds = [][]string{
			{"sh", "-c", "cd /workspace && go mod download"},
		}
	}

	for _, cmd := range cmds {
		stdout, stderr, err := s.docker.ExecInContainer(ctx, containerID, cmd)
		if err != nil {
			return fmt.Errorf("exec failed: %w", err)
		}
		if stderr != "" {
			fmt.Printf("Install output: %s\n", stdout+stderr)
		}
	}
	return nil
}

// getDockerStartCommand 获取Docker中的启动命令
func (s *SandboxService) getDockerStartCommand(projectType string) (string, []string) {
	switch projectType {
	case "react", "vue":
		return "sh", []string{"-c", "cd /workspace && npm run dev -- --host 0.0.0.0"}
	case "node":
		return "sh", []string{"-c", "cd /workspace && npm start"}
	case "python":
		return "sh", []string{"-c", "cd /workspace && python -m http.server 8080"}
	case "go":
		return "sh", []string{"-c", "cd /workspace && go run ."}
	case "static":
		return "sh", []string{"-c", "cd /workspace && python -m http.server 8080"}
	default:
		return "", nil
	}
}

// waitForServer 等待服务启动
func (s *SandboxService) waitForServer(port int, timeout time.Duration) error {
	maxWait := timeout
	checkInterval := 500 * time.Millisecond

	for i := 0; i < int(maxWait/checkInterval); i++ {
		time.Sleep(checkInterval)
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 1*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
	}
	return fmt.Errorf("server not ready on port %d within %v", port, maxWait)
}

// killProcessOnPort 杀死占用指定端口的进程（仅 Windows）
func (s *SandboxService) killProcessOnPort(port int) error {
	// 检查端口是否被占用
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 1*time.Second)
	if err != nil {
		// 端口未被占用
		return nil
	}
	conn.Close()

	fmt.Printf("[killProcessOnPort] Port %d is in use, attempting to kill the process\n", port)

	// 在 Windows 上使用 netstat 找到占用端口的 PID
	if runtime.GOOS == "windows" {
		// 使用 netstat -ano 查找占用端口的进程
		cmd := pkgexec.Command("cmd", "/c", fmt.Sprintf("netstat -ano | findstr :%d", port))
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to find process on port %d: %w", port, err)
		}

		// 解析输出，找到 PID
		lines := strings.Split(string(output), "\n")
		pids := make(map[string]bool)
		for _, line := range lines {
			// 格式: TCP    0.0.0.0:5173    0.0.0.0:0    LISTENING    12345
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				pid := fields[len(fields)-1]
				if pid != "0" {
					pids[pid] = true
				}
			}
		}

		// 杀死找到的进程
		for pid := range pids {
			fmt.Printf("[killProcessOnPort] Killing process with PID: %s\n", pid)
			killCmd := pkgexec.Command("taskkill", "/F", "/T", "/PID", pid)
			if out, err := killCmd.CombinedOutput(); err != nil {
				fmt.Printf("[killProcessOnPort] Failed to kill PID %s: %v, output: %s\n", pid, err, string(out))
			} else {
				fmt.Printf("[killProcessOnPort] Successfully killed PID %s\n", pid)
			}
		}

		// 等待端口释放
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// GetServerLogs 获取服务日志
func (s *SandboxService) GetServerLogs(ctx context.Context, serverID uuid.UUID) (string, error) {
	s.mu.RLock()
	server, exists := s.activeServers[serverID]
	s.mu.RUnlock()

	if !exists {
		return "", ErrServerNotFound
	}

	if server.Mode == RunModeDocker && server.ContainerID != "" {
		return s.docker.GetContainerLogs(ctx, server.ContainerID)
	}

	return "", nil
}

// IsDockerAvailable 检查Docker是否可用
func (s *SandboxService) IsDockerAvailable() bool {
	if s.docker == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.docker.ListContainers(ctx, false)
	return err == nil
}

// StopProject 停止项目服务
func (s *SandboxService) StopProject(ctx context.Context, serverID uuid.UUID) error {
	s.mu.Lock()
	server, exists := s.activeServers[serverID]
	if exists {
		fmt.Printf("[StopProject] Stopping server %s, mode: %s\n", serverID, server.Mode)
		// 本地进程杀死
		if server.KillFunc != nil {
			fmt.Printf("[StopProject] Calling KillFunc\n")
			if err := server.KillFunc(); err != nil {
				fmt.Printf("Warning: failed to kill process: %v\n", err)
			}
		} else {
			fmt.Printf("[StopProject] KillFunc is nil!\n")
		}
		// 取消上下文
		if server.CancelFunc != nil {
			server.CancelFunc()
		}
		// Docker容器停止
		if server.ContainerID != "" {
			s.docker.StopContainer(ctx, server.ContainerID, intPtr(5))
			s.docker.RemoveContainer(ctx, server.ContainerID)
		}
		delete(s.activeServers, serverID)
	}
	s.mu.Unlock()

	if !exists {
		return ErrServerNotFound
	}

	return nil
}

// GetProjectServer 获取项目服务状态
func (s *SandboxService) GetProjectServer(ctx context.Context, serverID uuid.UUID) (*ProjectServer, error) {
	s.mu.RLock()
	server, exists := s.activeServers[serverID]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrServerNotFound
	}

	return server, nil
}

// GetProjectServerByThread 按Thread获取项目服务
func (s *SandboxService) GetProjectServerByThread(ctx context.Context, threadID uuid.UUID) (*ProjectServer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, server := range s.activeServers {
		if server.ThreadID == threadID {
			return server, nil
		}
	}

	return nil, ErrServerNotFound
}

// ListProjectServers 列出所有活跃的项目服务
func (s *SandboxService) ListProjectServers() []*ProjectServer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	servers := make([]*ProjectServer, 0, len(s.activeServers))
	for _, server := range s.activeServers {
		servers = append(servers, server)
	}
	return servers
}