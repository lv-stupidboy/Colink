package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDiscoverInstructionFilesBasic 测试基本文件发现功能
func TestDiscoverInstructionFilesBasic(t *testing.T) {
	// 使用系统临时目录（不在项目目录内，避免祖先扫描发现项目的 CLAUDE.md）
	baseTmp := os.TempDir()
	testDir := filepath.Join(baseTmp, "isdp_test_"+strings.ReplaceAll(t.Name(), "/", "_"))
	defer os.RemoveAll(testDir) // 手动清理

	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// 创建 CLAUDE.md 文件
	claudeFile := filepath.Join(testDir, "CLAUDE.md")
	content := "# Test Instructions\nThis is a test instruction file."
	if err := os.WriteFile(claudeFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write CLAUDE.md: %v", err)
	}

	files := DiscoverInstructionFiles(testDir)

	if len(files) < 1 {
		t.Errorf("Expected at least 1 instruction file, got %d", len(files))
	}

	// 验证内容
	found := false
	for _, f := range files {
		if filepath.Base(f.Path) == "CLAUDE.md" && f.Content == content {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected CLAUDE.md with exact content not found")
	}
}

// TestDiscoverInstructionFilesEmpty 测试无文件时的行为
// 注意：祖先目录扫描可能发现用户目录下的 CLAUDE.md，这是预期行为
// 本测试验证在测试目录本身范围内没有创建文件
func TestDiscoverInstructionFilesEmpty(t *testing.T) {
	// 创建一个完全隔离的临时目录
	baseTmp := os.TempDir()
	testDir := filepath.Join(baseTmp, "isdp_empty_test_"+strings.ReplaceAll(t.Name(), "/", "_"))
	defer os.RemoveAll(testDir)

	// 创建嵌套目录（不创建任何 CLAUDE.md）
	subDir := filepath.Join(testDir, "sub1", "sub2")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	files := DiscoverInstructionFiles(subDir)

	// 验证在测试目录范围内没有文件被创建
	// 注意：祖先目录可能有文件（如用户目录），这是预期行为
	for _, f := range files {
		// 如果文件在测试目录范围内，则是测试失败
		if strings.HasPrefix(f.Path, testDir) {
			t.Errorf("Unexpected file found in test directory: %s", f.Path)
		}
	}
}

// TestDiscoverInstructionFilesTruncate 测试字符限制截断
func TestDiscoverInstructionFilesTruncate(t *testing.T) {
	baseTmp := os.TempDir()
	testDir := filepath.Join(baseTmp, "isdp_truncate_test")
	defer os.RemoveAll(testDir)

	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// 创建超长内容的文件
	longFile := filepath.Join(testDir, "CLAUDE.md")
	longContent := strings.Repeat("x", 5000) // 超过 MAX_INSTRUCTION_FILE_CHARS (4000)
	if err := os.WriteFile(longFile, []byte(longContent), 0644); err != nil {
		t.Fatalf("Failed to write long CLAUDE.md: %v", err)
	}

	files := DiscoverInstructionFiles(testDir)

	// 只检查我们在隔离目录创建的文件
	var ourFile *InstructionFile
	for i := range files {
		if filepath.Dir(files[i].Path) == testDir {
			ourFile = &files[i]
			break
		}
	}

	if ourFile == nil {
		t.Fatalf("Test file not found in results")
	}

	// 内容应该被截断到 MAX_INSTRUCTION_FILE_CHARS
	maxChars := 4000
	if len(ourFile.Content) > maxChars+len("...[truncated]") {
		t.Errorf("Content not truncated properly: got %d chars, max is %d", len(ourFile.Content), maxChars)
	}

	// 截断后应该有截断标记
	if !strings.HasSuffix(ourFile.Content, "...[truncated]") {
		t.Error("Truncated content should end with marker")
	}
}

// TestReadGitStatus 测试 git status 执行
func TestReadGitStatus(t *testing.T) {
	// 使用当前项目目录（是 git 仓库）
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	status := ReadGitStatus(cwd)

	// 在 git 仓库中应该有输出（至少包含分支信息）
	if status == "" {
		t.Error("Expected non-empty git status in git repository")
	}

	// 应该包含 "##" 分支标记
	if !strings.Contains(status, "##") {
		t.Errorf("Git status should contain branch info (##), got: %q", status)
	}
}

// TestReadGitStatusNonGit 测试非 git 目录
func TestReadGitStatusNonGit(t *testing.T) {
	// 使用临时目录（不是 git 仓库）
	tmpDir := t.TempDir()

	status := ReadGitStatus(tmpDir)

	// 非 git 目录应该返回空字符串
	if status != "" {
		t.Errorf("Expected empty git status in non-git directory, got: %q", status)
	}
}

// TestReadRecentCommits 测试 git log 执行
func TestReadRecentCommits(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	commits := ReadRecentCommits(cwd, 5)

	// 在 git 仓库中应该有 commits
	if len(commits) == 0 {
		t.Error("Expected non-empty commits in git repository")
	}

	// 最多返回 5 条
	if len(commits) > 5 {
		t.Errorf("Expected at most 5 commits, got %d", len(commits))
	}

	// 验证 commit 格式
	for _, c := range commits {
		if c.Hash == "" {
			t.Error("Commit hash should not be empty")
		}
		if len(c.Hash) < 7 {
			t.Errorf("Commit hash too short: %s", c.Hash)
		}
		if c.Subject == "" {
			t.Error("Commit subject should not be empty")
		}
	}
}

// TestReadRecentCommitsNonGit 测试非 git 目录的 commits
func TestReadRecentCommitsNonGit(t *testing.T) {
	tmpDir := t.TempDir()

	commits := ReadRecentCommits(tmpDir, 5)

	// 非 git 目录应该返回空
	if len(commits) != 0 {
		t.Errorf("Expected 0 commits in non-git directory, got %d", len(commits))
	}
}