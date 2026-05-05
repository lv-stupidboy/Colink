package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	pkgexec "github.com/anthropic/isdp/pkg/exec"
)

// 常量定义（参考 ClaudeCode prompt.rs）
const (
	MaxInstructionFileChars     = 4000  // 单个文件最大字符数
	MaxTotalInstructionChars    = 12000 // 所有文件总字符数
	MaxRecentCommits            = 5     // 默认获取的最近 commit 数量
)

// InstructionFile 指令文件
type InstructionFile struct {
	Path    string // 文件绝对路径
	Content string // 文件内容（可能截断）
	Scope   string // 作用域：workspace 或目录路径
}

// CommitInfo Commit 信息
type CommitInfo struct {
	Hash    string // Commit hash（短格式）
	Subject string // Commit subject（第一行）
}

// ProjectContext 项目上下文
// 参考 ClaudeCode prompt.rs:ProjectContext
type ProjectContext struct {
	Cwd              string            // 当前工作目录
	CurrentDate       string            // 当前日期
	GitStatus         string            // git status --short --branch 输出
	RecentCommits     []CommitInfo      // 最近 N 条 commit
	InstructionFiles  []InstructionFile // 发现的指令文件
	LoadedAt          time.Time         // 加载时间
}

// DiscoverInstructionFiles 祖先目录链扫描
// 参考 ClaudeCode prompt.rs:discover_instruction_files
// 扫描顺序：CLAUDE.md → CLAUDE.local.md → .claude/CLAUDE.md → .claude/instructions.md
func DiscoverInstructionFiles(cwd string) []InstructionFile {
	var files []InstructionFile
	totalChars := 0
	seenHashes := make(map[string]bool) // 用于去重

	// 从 cwd 向上遍历祖先目录
	dir := cwd
	for {
		// 检查 4 个候选文件
		candidates := []string{
			"CLAUDE.md",
			"CLAUDE.local.md",
				filepath.Join(".claude", "CLAUDE.md"),
			filepath.Join(".claude", "instructions.md"),
		}

		for _, candidate := range candidates {
			filePath := filepath.Join(dir, candidate)
			content, err := readFileContent(filePath)
			if err != nil {
				continue // 文件不存在或无法读取，跳过
			}

			// 计算内容 hash 用于去重
			hash := sha256.Sum256([]byte(content))
			hashStr := hex.EncodeToString(hash[:])
			if seenHashes[hashStr] {
				continue // 已见过相同内容，跳过
			}
			seenHashes[hashStr] = true

			// 截断处理
			if len(content) > MaxInstructionFileChars {
				content = content[:MaxInstructionFileChars] + "...[truncated]"
			}

			// 总字符数限制
			if totalChars + len(content) > MaxTotalInstructionChars {
				// 达到总限制，停止
				return files
			}

			// 确定作用域
			scope := "workspace"
			if dir != cwd {
				scope = dir
			}

			files = append(files, InstructionFile{
				Path:    filePath,
				Content: content,
				Scope:   scope,
			})
			totalChars += len(content)
		}

		// 向上移动到父目录
		parent := filepath.Dir(dir)
		if parent == dir {
			break // 到达根目录，停止
		}
		dir = parent
	}

	return files
}

// ReadGitStatus 执行 git status --short --branch
func ReadGitStatus(cwd string) string {
	cmd := pkgexec.Command("git", "status", "--short", "--branch", "--show-stash")
	cmd.Dir = cwd

	output, err := cmd.Output()
	if err != nil {
		// 非 git 目录或 git 命令失败，返回空
		return ""
	}

	return strings.TrimSpace(string(output))
}

// ReadRecentCommits 执行 git log -n --oneline
func ReadRecentCommits(cwd string, n int) []CommitInfo {
	if n <= 0 {
		n = MaxRecentCommits
	}

	cmd := pkgexec.Command("git", "log", fmt.Sprintf("-n%d", n), "--oneline", "--no-decorate")
	cmd.Dir = cwd

	output, err := cmd.Output()
	if err != nil {
		// 非 git 目录或 git 命令失败，返回空
		return nil
	}

	var commits []CommitInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// 格式: "hash subject"
		parts := strings.SplitN(line, " ", 2)
		if len(parts) >= 1 {
			commit := CommitInfo{
				Hash: parts[0],
			}
			if len(parts) >= 2 {
				commit.Subject = parts[1]
			}
			commits = append(commits, commit)
		}
	}

	return commits
}

// LoadProjectContext 加载完整的项目上下文
func LoadProjectContext(cwd string) *ProjectContext {
	return &ProjectContext{
		Cwd:              cwd,
		CurrentDate:       time.Now().Format("2006-01-02"),
		GitStatus:         ReadGitStatus(cwd),
		RecentCommits:     ReadRecentCommits(cwd, MaxRecentCommits),
		InstructionFiles:  DiscoverInstructionFiles(cwd),
		LoadedAt:          time.Now(),
	}
}

// readFileContent 读取文件内容
func readFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FormatGitStatus 格式化 Git Status 用于上下文注入
func (pc *ProjectContext) FormatGitStatus() string {
	if pc.GitStatus == "" {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Git Status\n")
	sb.WriteString(pc.GitStatus)
	sb.WriteString("\n")
	return sb.String()
}

// FormatRecentCommits 格式化最近 Commits 用于上下文注入
func (pc *ProjectContext) FormatRecentCommits() string {
	if len(pc.RecentCommits) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Recent Commits\n")
	for _, c := range pc.RecentCommits {
		sb.WriteString(fmt.Sprintf("  %s %s\n", c.Hash, c.Subject))
	}
	return sb.String()
}

// FormatInstructionFiles 格式化指令文件用于上下文注入
func (pc *ProjectContext) FormatInstructionFiles() string {
	if len(pc.InstructionFiles) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Claude Instructions\n")
	for _, f := range pc.InstructionFiles {
		sb.WriteString(fmt.Sprintf("### %s (scope: %s)\n", f.Path, f.Scope))
		sb.WriteString(f.Content)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// HasGitContext 检查是否有 Git 上下文
func (pc *ProjectContext) HasGitContext() bool {
	return pc.GitStatus != "" || len(pc.RecentCommits) > 0
}

// HasInstructionFiles 检查是否有指令文件
func (pc *ProjectContext) HasInstructionFiles() bool {
	return len(pc.InstructionFiles) > 0
}

// ExecuteGitCommand 执行 git 命令并返回输出（通用方法）
func ExecuteGitCommand(cwd string, args ...string) (string, error) {
	cmd := pkgexec.Command("git", args...)
	cmd.Dir = cwd

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git command failed: %v, stderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}