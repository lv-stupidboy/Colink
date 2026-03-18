package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// RunMode 运行模式
type RunMode string

const (
	RunModeDocker RunMode = "docker"
	RunModeLocal  RunMode = "local"
)

// RunResult 运行结果
type LocalRunResult struct {
	ID        string
	ExitCode  int
	Output    string
	Error     string
	StartedAt time.Time
	EndedAt   time.Time
}

// LocalProcessCmd 本地进程命令（支持异步杀死）
type LocalProcessCmd struct {
	cmd    *exec.Cmd
	Result chan *LocalRunResult
}

// Kill 杀死进程及其子进程
func (l *LocalProcessCmd) Kill() error {
	if l.cmd == nil || l.cmd.Process == nil {
		fmt.Printf("[Kill] Process is nil, nothing to kill\n")
		return nil
	}

	pid := l.cmd.Process.Pid
	fmt.Printf("[Kill] Attempting to kill process tree with PID: %d\n", pid)

	// 在 Windows 上使用 taskkill 杀死整个进程树
	if runtime.GOOS == "windows" {
		// 先尝试使用 taskkill 杀死进程树
		killCmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
		output, err := killCmd.CombinedOutput()
		fmt.Printf("[Kill] taskkill output: %s, err: %v\n", string(output), err)

		// 如果 taskkill 失败，尝试直接杀死进程
		if err != nil {
			fmt.Printf("[Kill] taskkill failed, trying direct kill\n")
			if killErr := l.cmd.Process.Kill(); killErr != nil {
				fmt.Printf("[Kill] direct kill error: %v\n", killErr)
				return killErr
			}
		}
	} else {
		// Unix 系统使用进程组
		if err := l.cmd.Process.Kill(); err != nil {
			fmt.Printf("[Kill] kill error: %v\n", err)
			return err
		}
	}

	fmt.Printf("[Kill] Successfully killed process %d\n", pid)
	return nil
}

// LocalProcessRunner 本地进程运行器
type LocalProcessRunner struct {
	workDir string
}

// NewLocalProcessRunner 创建本地进程运行器
func NewLocalProcessRunner(workDir string) *LocalProcessRunner {
	if workDir == "" {
		workDir = "./repos"
	}
	return &LocalProcessRunner{workDir: workDir}
}

// Run 在工作目录执行命令
func (r *LocalProcessRunner) Run(ctx context.Context, projectPath string, cmd string, args ...string) (*LocalRunResult, error) {
	// 如果是绝对路径，直接使用；否则拼接到 workDir
	var fullPath string
	if filepath.IsAbs(projectPath) {
		fullPath = projectPath
	} else {
		fullPath = filepath.Join(r.workDir, projectPath)
	}

	// 确保目录存在
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("project path not found: %s", fullPath)
	}

	result := &LocalRunResult{
		ID:        fmt.Sprintf("run_%d", time.Now().UnixNano()),
		StartedAt: time.Now(),
	}

	execCmd := exec.CommandContext(ctx, cmd, args...)
	execCmd.Dir = fullPath

	output, err := execCmd.CombinedOutput()
	result.EndedAt = time.Now()
	result.Output = string(output)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Error = exitErr.Error()
		} else {
			result.Error = err.Error()
		}
	}

	return result, nil
}

// RunWithOutput 带实时输出的运行（异步版本）
func (r *LocalProcessRunner) RunWithOutput(ctx context.Context, projectPath string, cmd string, onOutput func(string), args ...string) (*LocalProcessCmd, error) {
	// 如果是绝对路径，直接使用；否则拼接到 workDir
	var fullPath string
	if filepath.IsAbs(projectPath) {
		fullPath = projectPath
	} else {
		fullPath = filepath.Join(r.workDir, projectPath)
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("project path not found: %s", fullPath)
	}

	execCmd := exec.CommandContext(ctx, cmd, args...)
	execCmd.Dir = fullPath

	// 设置进程组，以便能够杀死整个进程树
	setProcessGroup(execCmd)

	stdout, err := execCmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := execCmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := execCmd.Start(); err != nil {
		return nil, err
	}

	result := &LocalRunResult{
		ID:        fmt.Sprintf("run_%d", time.Now().UnixNano()),
		StartedAt: time.Now(),
	}

	processCmd := &LocalProcessCmd{
		cmd:    execCmd,
		Result: make(chan *LocalRunResult, 1),
	}

	// 读取输出
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				output := string(buf[:n])
				result.Output += output
				if onOutput != nil {
					onOutput(output)
				}
			}
			if err != nil {
				break
			}
		}
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				output := string(buf[:n])
				result.Error += output
				if onOutput != nil {
					onOutput(output)
				}
			}
			if err != nil {
				break
			}
		}
	}()

	// 等待进程完成
	go func() {
		err := execCmd.Wait()
		result.EndedAt = time.Now()

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			}
		}

		processCmd.Result <- result
	}()

	return processCmd, nil
}

// InstallDependencies 安装依赖
func (r *LocalProcessRunner) InstallDependencies(ctx context.Context, projectPath string, lang string) (*LocalRunResult, error) {
	// 如果是绝对路径，直接使用；否则拼接到 workDir
	var fullPath string
	if filepath.IsAbs(projectPath) {
		fullPath = projectPath
	} else {
		fullPath = filepath.Join(r.workDir, projectPath)
	}

	// 检测项目类型
	detectedLang := lang
	if detectedLang == "" {
		detectedLang = r.DetectProjectType(fullPath)
	}

	var cmd string
	var args []string

	switch detectedLang {
	case "node", "react", "vue":
		// 检查使用 npm 还是 yarn
		if _, err := os.Stat(filepath.Join(fullPath, "yarn.lock")); err == nil {
			cmd = "yarn"
			args = []string{"install"}
		} else if _, err := os.Stat(filepath.Join(fullPath, "pnpm-lock.yaml")); err == nil {
			cmd = "pnpm"
			args = []string{"install"}
		} else {
			cmd = "npm"
			args = []string{"install"}
		}
	case "python":
		cmd = "pip"
		args = []string{"install", "-r", "requirements.txt"}
	case "go":
		cmd = "go"
		args = []string{"mod", "download"}
	default:
		return nil, fmt.Errorf("unsupported project type: %s", detectedLang)
	}

	return r.Run(ctx, projectPath, cmd, args...)
}

// DetectProjectType 检测项目类型
func (r *LocalProcessRunner) DetectProjectType(projectPath string) string {
	// 如果是绝对路径，直接使用；否则拼接到 workDir
	var fullPath string
	if filepath.IsAbs(projectPath) {
		fullPath = projectPath
	} else {
		fullPath = filepath.Join(r.workDir, projectPath)
	}

	// Node.js / React / Vue
	if _, err := os.Stat(filepath.Join(fullPath, "package.json")); err == nil {
		// 检查是否是 React
		pkgJson, err := os.ReadFile(filepath.Join(fullPath, "package.json"))
		if err == nil {
			content := string(pkgJson)
			if strings.Contains(content, "\"react\"") {
				return "react"
			}
			if strings.Contains(content, "\"vue\"") {
				return "vue"
			}
		}
		return "node"
	}

	// Python
	if _, err := os.Stat(filepath.Join(fullPath, "requirements.txt")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(fullPath, "pyproject.toml")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(fullPath, "setup.py")); err == nil {
		return "python"
	}

	// Go
	if _, err := os.Stat(filepath.Join(fullPath, "go.mod")); err == nil {
		return "go"
	}

	// 静态HTML
	if files, err := filepath.Glob(filepath.Join(fullPath, "*.html")); err == nil && len(files) > 0 {
		return "static"
	}

	return "unknown"
}

// GetStartCommand 获取启动命令
func (r *LocalProcessRunner) GetStartCommand(projectPath string) (string, []string, int) {
	// 如果是绝对路径，直接使用；否则拼接到 workDir
	var fullPath string
	if filepath.IsAbs(projectPath) {
		fullPath = projectPath
	} else {
		fullPath = filepath.Join(r.workDir, projectPath)
	}

	projectType := r.DetectProjectType(projectPath)

	switch projectType {
	case "react":
		// 检查 package.json 中的脚本
		port := 5173 // Vite 默认端口
		if _, err := os.Stat(filepath.Join(fullPath, "yarn.lock")); err == nil {
			return "yarn", []string{"dev"}, port
		}
		return "npm", []string{"run", "dev"}, port

	case "vue":
		port := 5173
		if _, err := os.Stat(filepath.Join(fullPath, "yarn.lock")); err == nil {
			return "yarn", []string{"dev"}, port
		}
		return "npm", []string{"run", "dev"}, port

	case "node":
		port := 3000
		// 检查是否有 start 脚本
		pkgJson, err := os.ReadFile(filepath.Join(fullPath, "package.json"))
		if err == nil && strings.Contains(string(pkgJson), "\"start\"") {
			return "npm", []string{"start"}, port
		}
		return "node", []string{"index.js"}, port

	case "python":
		// 检查是 Flask 还是 FastAPI
		files, _ := filepath.Glob(filepath.Join(fullPath, "*.py"))
		for _, f := range files {
			content, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			s := string(content)
			if strings.Contains(s, "flask") || strings.Contains(s, "Flask") {
				return "python", []string{filepath.Base(f)}, 5000
			}
			if strings.Contains(s, "fastapi") || strings.Contains(s, "FastAPI") {
				return "uvicorn", []string{strings.TrimSuffix(filepath.Base(f), ".py") + ":app", "--reload"}, 8000
			}
		}
		return "python", []string{"app.py"}, 5000

	case "go":
		return "go", []string{"run", "."}, 3001

	case "static":
		return "python", []string{"-m", "http.server", "3002"}, 3002

	default:
		return "", nil, 0
	}
}