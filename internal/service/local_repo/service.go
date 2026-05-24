package local_repo

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/workspace"
	"github.com/google/uuid"
)

// Service 本地代码仓服务
type Service struct {
	repo      *repo.LocalRepoRepository
	workspace *workspace.Guard
}

// NewService 创建本地代码仓服务
func NewService(repo *repo.LocalRepoRepository, workspaceGuard *workspace.Guard) *Service {
	return &Service{repo: repo, workspace: workspaceGuard}
}

func (s *Service) validateWorkspacePath(path string) error {
	if s.workspace == nil {
		return nil
	}
	return s.workspace.Validate(path)
}

func (s *Service) validateRepoName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("仓库名称不能为空")
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\\`) {
		return errors.New("仓库名称不能包含路径分隔符")
	}
	return nil
}

func pathWithin(basePath, targetPath string) bool {
	baseAbs, err := filepath.Abs(filepath.Clean(basePath))
	if err != nil {
		return false
	}
	targetAbs, err := filepath.Abs(filepath.Clean(targetPath))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func isSSHGitURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, " \t\r\n") {
		return false
	}
	if strings.HasPrefix(raw, "ssh://") {
		rest := strings.TrimPrefix(raw, "ssh://")
		at := strings.Index(rest, "@")
		if at <= 0 || at == len(rest)-1 {
			return false
		}
		return strings.Contains(rest[at+1:], "/")
	}
	at := strings.Index(raw, "@")
	colon := strings.Index(raw, ":")
	return at > 0 && colon > at+1 && colon < len(raw)-1 && !strings.Contains(raw[:at], "/")
}

// List 列出所有本地代码仓
func (s *Service) List(ctx context.Context) ([]*model.LocalRepo, error) {
	return s.repo.FindAll(ctx)
}

// GetByID 根据ID获取本地代码仓
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.LocalRepo, error) {
	return s.repo.FindByID(ctx, id)
}

// Upload 上传ZIP并创建本地代码仓
func (s *Service) Upload(ctx context.Context, fileBytes []byte, originalName string, req *model.UploadRepoRequest) (*model.LocalRepo, error) {
	name := req.Name
	if name == "" {
		name = strings.TrimSuffix(originalName, ".zip")
		name = strings.TrimSuffix(name, ".ZIP")
		if name == "" {
			name = "uploaded-repo"
		}
	}

	if err := s.validateRepoName(name); err != nil {
		return nil, err
	}

	targetPath := req.TargetPath
	if targetPath == "" {
		return nil, errors.New("目标路径不能为空")
	}
	if err := s.validateWorkspacePath(targetPath); err != nil {
		return nil, err
	}

	localPath := filepath.Join(targetPath, name)
	if err := s.validateWorkspacePath(localPath); err != nil {
		return nil, err
	}

	if _, err := os.Stat(localPath); err == nil {
		return nil, fmt.Errorf("目录已存在: %s", localPath)
	}

	if err := os.MkdirAll(localPath, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}

	if err := extractZip(fileBytes, localPath); err != nil {
		os.RemoveAll(localPath)
		return nil, fmt.Errorf("解压ZIP失败: %w", err)
	}

	gitUrl, branch, commit := probeGitInfo(localPath)

	status := model.RepoStatusPending
	if gitUrl != "" {
		status = model.RepoStatusReady
	}

	localRepo := &model.LocalRepo{
		ID:         uuid.New(),
		Name:       name,
		GitUrl:     gitUrl,
		LocalPath:  localPath,
		Branch:     branch,
		LastCommit: commit,
		Status:     status,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.repo.Create(ctx, localRepo); err != nil {
		os.RemoveAll(localPath)
		return nil, fmt.Errorf("创建数据库记录失败: %w", err)
	}

	return localRepo, nil
}

// Clone 从远程URL克隆代码仓
func (s *Service) Clone(ctx context.Context, req *model.CloneRepoRequest) (*model.LocalRepo, error) {
	if !isSSHGitURL(req.GitUrl) {
		return nil, errors.New("仅支持 SSH 格式的 Git URL，例如 git@github.com:owner/repo.git")
	}

	name := req.Name
	if name == "" {
		name = inferRepoNameFromGitUrl(req.GitUrl)
	}
	if err := s.validateRepoName(name); err != nil {
		return nil, err
	}

	targetPath := req.TargetPath
	if targetPath == "" {
		return nil, errors.New("目标路径不能为空")
	}
	if err := s.validateWorkspacePath(targetPath); err != nil {
		return nil, err
	}

	localPath := filepath.Join(targetPath, name)
	if err := s.validateWorkspacePath(localPath); err != nil {
		return nil, err
	}

	if _, err := os.Stat(localPath); err == nil {
		return nil, fmt.Errorf("目录已存在: %s", localPath)
	}

	branch := req.Branch
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "--branch", branch, "--depth", "1")
	} else {
		args = append(args, "--depth", "1")
	}
	args = append(args, req.GitUrl, localPath)

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("克隆失败: %w, output: %s", err, string(output))
	}

	gitUrl, probedBranch, commit := probeGitInfo(localPath)

	localRepo := &model.LocalRepo{
		ID:         uuid.New(),
		Name:       name,
		GitUrl:     gitUrl,
		LocalPath:  localPath,
		Branch:     probedBranch,
		LastCommit: commit,
		Status:     model.RepoStatusReady,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.repo.Create(ctx, localRepo); err != nil {
		os.RemoveAll(localPath)
		return nil, fmt.Errorf("创建数据库记录失败: %w", err)
	}

	return localRepo, nil
}

// Sync 同步本地代码仓（git pull）
func (s *Service) Sync(ctx context.Context, id uuid.UUID) (*model.LocalRepo, error) {
	localRepo, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.validateWorkspacePath(localRepo.LocalPath); err != nil {
		return nil, err
	}

	if localRepo.GitUrl == "" {
		return nil, errors.New("GIT地址未配置")
	}

	localRepo.Status = model.RepoStatusSyncing
	localRepo.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, localRepo); err != nil {
		return nil, fmt.Errorf("更新状态为syncing失败: %w", err)
	}

	cmd := exec.Command("git", "-C", localRepo.LocalPath, "pull")
	output, err := cmd.CombinedOutput()
	if err != nil {
		localRepo.Status = model.RepoStatusError
		localRepo.UpdatedAt = time.Now()
		s.repo.Update(ctx, localRepo)
		return nil, fmt.Errorf("同步失败: %w, output: %s", err, string(output))
	}

	gitUrl, branch, commit := probeGitInfo(localRepo.LocalPath)
	if gitUrl != "" {
		localRepo.GitUrl = gitUrl
	}
	if branch != nil {
		localRepo.Branch = branch
	}
	if commit != nil {
		localRepo.LastCommit = commit
	}

	localRepo.Status = model.RepoStatusReady
	localRepo.ErrorMessage = nil
	localRepo.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, localRepo); err != nil {
		return nil, fmt.Errorf("更新状态为ready失败: %w", err)
	}

	return localRepo, nil
}

// ConfigureGit 配置代码仓的 git URL
func (s *Service) ConfigureGit(ctx context.Context, id uuid.UUID, req *model.GitConfigRequest) (*model.LocalRepo, error) {
	localRepo, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	localRepo.GitUrl = req.GitUrl
	localRepo.Branch = &req.Branch
	localRepo.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, localRepo); err != nil {
		return nil, fmt.Errorf("更新gitUrl失败: %w", err)
	}

	gitDir := filepath.Join(localRepo.LocalPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		cmd := exec.Command("git", "-C", localRepo.LocalPath, "remote", "set-url", "origin", req.GitUrl)
		output, err := cmd.CombinedOutput()
		if err != nil {
			cmd = exec.Command("git", "-C", localRepo.LocalPath, "remote", "add", "origin", req.GitUrl)
			output, err = cmd.CombinedOutput()
			if err != nil {
				return localRepo, fmt.Errorf("设置git remote失败: %w, output: %s", err, string(output))
			}
		}
	}

	gitUrl, _, commit := probeGitInfo(localRepo.LocalPath)
	if gitUrl != "" {
		localRepo.GitUrl = gitUrl
	}
	if commit != nil {
		localRepo.LastCommit = commit
	}

	if localRepo.Status != model.RepoStatusReady && localRepo.GitUrl != "" {
		localRepo.Status = model.RepoStatusReady
		localRepo.ErrorMessage = nil
	}
	localRepo.UpdatedAt = time.Now()
	s.repo.Update(ctx, localRepo)

	return localRepo, nil
}

// Delete 删除本地代码仓（文件 + 数据库记录）
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	localRepo, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.validateWorkspacePath(localRepo.LocalPath); err != nil {
		return err
	}

	if localRepo.LocalPath != "" {
		if err := os.RemoveAll(localRepo.LocalPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除本地文件失败: %w", err)
		}
	}

	return s.repo.Delete(ctx, id)
}

// CreateFolder 创建本地代码仓目标目录下的文件夹
func (s *Service) CreateFolder(_ context.Context, parentPath, name string) error {
	parentPath = filepath.Clean(parentPath)
	fullPath := filepath.Join(parentPath, strings.TrimSpace(name))
	if s.workspace != nil {
		var err error
		fullPath, err = s.workspace.ValidateChild(parentPath, name)
		if err != nil {
			return err
		}
	}
	parentInfo, err := os.Stat(parentPath)
	if err != nil {
		return errors.New("父目录不存在")
	}
	if !parentInfo.IsDir() {
		return errors.New("父路径不是目录")
	}
	if _, err := os.Stat(fullPath); err == nil {
		return errors.New("文件夹已存在")
	}
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return errors.New("创建文件夹失败: " + err.Error())
	}
	return nil
}

// GetRemoteBranches 获取远程仓库的分支列表
func (s *Service) GetRemoteBranches(gitUrl string) ([]*model.RemoteBranch, error) {
	if !isSSHGitURL(gitUrl) {
		return nil, errors.New("仅支持 SSH 格式的 Git URL，例如 git@github.com:owner/repo.git")
	}
	cmd := exec.Command("git", "ls-remote", "--heads", "--tags", gitUrl)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("获取远程分支失败: %w, output: %s", err, string(output))
	}

	var branches []*model.RemoteBranch
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		refPath := parts[1]

		if strings.HasPrefix(refPath, "refs/heads/") {
			name := strings.TrimPrefix(refPath, "refs/heads/")
			branches = append(branches, &model.RemoteBranch{
				Name: name,
				Type: "branch",
			})
		} else if strings.HasPrefix(refPath, "refs/tags/") {
			name := strings.TrimPrefix(refPath, "refs/tags/")
			if strings.HasSuffix(name, "^{}") {
				continue
			}
			branches = append(branches, &model.RemoteBranch{
				Name: name,
				Type: "tag",
			})
		}
	}

	return branches, nil
}

// BrowsePath 浏览文件系统路径
func (s *Service) BrowsePath(ctx context.Context, path string) (*model.BrowsePathResponse, error) {
	resp := &model.BrowsePathResponse{
		CurrentPath: path,
		Entries:     make([]model.FileInfo, 0),
	}

	if s.workspace != nil {
		path = s.workspace.NormalizeStart(path)
	}
	if strings.TrimSpace(path) == "" {
		if runtime.GOOS == "windows" {
			drives, err := getWindowsDrives()
			if err != nil {
				resp.Error = err.Error()
				return resp, nil
			}
			resp.Drives = drives
			resp.IsValid = true
			return resp, nil
		}
		path = string(filepath.Separator)
	}

	if runtime.GOOS == "windows" && len(path) == 2 && path[1] == ':' {
		path = path + string(filepath.Separator)
	}

	path = filepath.Clean(path)
	resp.CurrentPath = path
	if err := s.validateWorkspacePath(path); err != nil {
		resp.Error = err.Error()
		return resp, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			resp.Error = "路径不存在"
			return resp, nil
		}
		resp.Error = err.Error()
		return resp, nil
	}

	if !info.IsDir() {
		resp.Error = "路径不是目录"
		return resp, nil
	}

	resp.IsValid = true

	if path != "/" && path != "" {
		parentPath := filepath.Dir(path)
		if s.workspace == nil || !s.workspace.Enabled() || pathWithin(s.workspace.Root(), parentPath) {
			resp.ParentPath = parentPath
		}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		resp.Error = "无法读取目录: " + err.Error()
		return resp, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if strings.HasPrefix(entry.Name(), ".") || strings.HasPrefix(entry.Name(), "$") {
			continue
		}

		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		resp.Entries = append(resp.Entries, model.FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(path, entry.Name()),
			IsDir:   true,
			Size:    0,
			ModTime: entryInfo.ModTime().Format(time.RFC3339),
		})
	}

	sortFiles(resp.Entries)

	return resp, nil
}

// ========== 辅助函数 ==========

func extractZip(fileBytes []byte, dest string) error {
	tmpFile, err := os.CreateTemp("", "upload-*.zip")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(fileBytes); err != nil {
		tmpFile.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	tmpFile.Close()

	r, err := zip.OpenReader(tmpPath)
	if err != nil {
		return fmt.Errorf("打开ZIP文件失败: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if strings.Contains(f.Name, "..") {
			continue
		}

		fpath := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0755); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return fmt.Errorf("创建父目录失败: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("打开ZIP内文件失败: %w", err)
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return fmt.Errorf("创建目标文件失败: %w", err)
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return fmt.Errorf("写入文件失败: %w", err)
		}
	}

	return nil
}

func probeGitInfo(localPath string) (gitUrl string, branch *string, commit *string) {
	gitDir := filepath.Join(localPath, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return "", nil, nil
	}

	cmd := exec.Command("git", "-C", localPath, "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err == nil {
		url := strings.TrimSpace(string(output))
		if url != "" {
			gitUrl = url
		}
	}

	cmd = exec.Command("git", "-C", localPath, "rev-parse", "--abbrev-ref", "HEAD")
	output, err = cmd.Output()
	if err == nil {
		b := strings.TrimSpace(string(output))
		if b != "" {
			branch = &b
		}
	}

	cmd = exec.Command("git", "-C", localPath, "rev-parse", "--short", "HEAD")
	output, err = cmd.Output()
	if err == nil {
		c := strings.TrimSpace(string(output))
		if c != "" {
			commit = &c
		}
	}

	return gitUrl, branch, commit
}

func inferRepoNameFromGitUrl(gitUrl string) string {
	url := strings.TrimSuffix(gitUrl, ".git")
	url = strings.TrimSuffix(url, ".GIT")

	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		name = strings.SplitN(name, "?", 2)[0]
		name = strings.SplitN(name, "@", 2)[0]
		if name != "" {
			return name
		}
	}

	return "cloned-repo"
}

func getWindowsDrives() ([]string, error) {
	var drives []string
	for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		path := string(drive) + ":"
		if _, err := os.Stat(path); err == nil {
			drives = append(drives, path)
		}
	}
	if len(drives) == 0 {
		return nil, errors.New("未找到可用驱动器")
	}
	return drives, nil
}

func sortFiles(files []model.FileInfo) {
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if !files[i].IsDir && files[j].IsDir {
				files[i], files[j] = files[j], files[i]
			} else if files[i].IsDir == files[j].IsDir && files[i].Name > files[j].Name {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
}
