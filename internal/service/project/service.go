package project

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/workspace"
	"github.com/google/uuid"
)

// Service 项目服务
type Service struct {
	repo         *repo.ProjectRepository
	workflowRepo *repo.WorkflowTemplateRepository // 新增依赖
	workspace    *workspace.Guard
}

// NewService 创建项目服务
func NewService(repo *repo.ProjectRepository, workflowRepo *repo.WorkflowTemplateRepository, workspaceGuard *workspace.Guard) *Service {
	return &Service{
		repo:         repo,
		workflowRepo: workflowRepo,
		workspace:    workspaceGuard,
	}
}

func (s *Service) validateWorkspacePath(path string) error {
	if s.workspace == nil {
		return nil
	}
	return s.workspace.Validate(path)
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

// List 列出项目
func (s *Service) List(ctx context.Context) ([]*model.Project, error) {
	return s.repo.FindAll(ctx, 100, 0)
}

// GetByID 根据ID获取项目
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	return s.repo.FindByID(ctx, id)
}

// Create 创建项目
func (s *Service) Create(ctx context.Context, req *model.CreateProjectRequest) (*model.Project, error) {
	if err := s.validateWorkspacePath(req.LocalPath); err != nil {
		return nil, err
	}
	// 默认值
	projectType := req.Type
	if projectType == "" {
		projectType = model.ProjectTypeService
	}
	projectMode := req.Mode
	if projectMode == "" {
		projectMode = model.ProjectModeNew
	}
	// 处理 RepositoryUrl，优先使用新字段，兼容旧字段
	repositoryUrl := req.RepositoryUrl
	if repositoryUrl == nil && req.ExistingRepoURL != "" {
		repositoryUrl = &req.ExistingRepoURL
	}
	project := &model.Project{
		ID:                 uuid.New(),
		Name:               req.Name,
		Description:        req.Description,
		Type:               projectType,
		Mode:               projectMode,
		Status:             model.ProjectStatusDraft,
		LocalPath:          req.LocalPath,
		RepositoryUrl: repositoryUrl,
		WorkflowTemplateID: req.WorkflowTemplateID,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := s.repo.Create(ctx, project); err != nil {
		return nil, err
	}

	return project, nil
}

// Update 更新项目
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateProjectRequest) (*model.Project, error) {
	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 如果设置了工作流ID，验证工作流是否存在
	if req.WorkflowTemplateID != nil && *req.WorkflowTemplateID != uuid.Nil {
		_, err := s.workflowRepo.FindByID(ctx, *req.WorkflowTemplateID)
		if err != nil {
			return nil, errors.New("指定的工作流模板不存在")
		}
	}

	// 更新字段
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = req.Description
	}
	if req.Type != nil {
		project.Type = *req.Type
	}
	if req.Mode != nil {
		project.Mode = *req.Mode
	}
	if req.Status != nil {
		project.Status = *req.Status
	}
	if req.LocalPath != nil {
		if err := s.validateWorkspacePath(*req.LocalPath); err != nil {
			return nil, err
		}
		project.LocalPath = *req.LocalPath
	}
	if req.RepositoryUrl != nil {
		project.RepositoryUrl = req.RepositoryUrl
	}
	project.WorkflowTemplateID = req.WorkflowTemplateID

	if err := s.repo.Update(ctx, project); err != nil {
		return nil, err
	}

	return project, nil
}

// Delete 删除项目
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// ListFiles 列出项目文件夹内容
func (s *Service) ListFiles(ctx context.Context, id uuid.UUID, subPath string) (*model.ListFilesResponse, error) {
	// 获取项目
	project, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 构建完整路径
	basePath := project.LocalPath
	if basePath == "" {
		return nil, errors.New("项目未设置本地路径")
	}

	if err := s.validateWorkspacePath(basePath); err != nil {
		return nil, err
	}

	// 安全检查：防止路径遍历攻击
	subPath = filepath.Clean("/" + subPath)
	fullPath := filepath.Join(basePath, subPath)

	// 确保完整路径在项目路径内
	if !pathWithin(basePath, fullPath) {
		return nil, errors.New("无效的路径")
	}

	// 读取目录
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("路径不存在")
		}
		return nil, err
	}

	// 构建文件列表
	var files []model.FileInfo
	for _, entry := range entries {
		// 跳过隐藏文件
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		relPath := filepath.Join(subPath, entry.Name())
		if subPath == "" || subPath == "/" || subPath == "\\" {
			relPath = entry.Name()
		}

		files = append(files, model.FileInfo{
			Name:    entry.Name(),
			Path:    relPath,
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}

	// 排序：目录在前，然后按名称排序
	sortFiles(files)

	return &model.ListFilesResponse{
		Path:    subPath,
		Files:   files,
		HasMore: false,
	}, nil
}

// BrowsePath 浏览文件系统路径
func (s *Service) BrowsePath(ctx context.Context, path string) (*model.BrowsePathResponse, error) {
	resp := &model.BrowsePathResponse{
		CurrentPath: path,
		Entries:     make([]model.FileInfo, 0),
	}

	// 如果路径为空，返回驱动器列表或根目录
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
		// 非 Windows 系统从根目录开始
		path = string(filepath.Separator)
	}

	// Windows 驱动器路径处理：将 "D:" 转换为 "D:\"
	if runtime.GOOS == "windows" && len(path) == 2 && path[1] == ':' {
		path = path + string(filepath.Separator)
	}

	// 规范化路径
	path = filepath.Clean(path)
	resp.CurrentPath = path

	if err := s.validateWorkspacePath(path); err != nil {
		resp.Error = err.Error()
		return resp, nil
	}

	// 检查路径是否存在
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

	// 设置父目录路径
	if path != "/" && path != "" {
		parentPath := filepath.Dir(path)
		if s.workspace == nil || !s.workspace.Enabled() || pathWithin(s.workspace.Root(), parentPath) {
			resp.ParentPath = parentPath
		}
	}

	// 读取目录内容
	entries, err := os.ReadDir(path)
	if err != nil {
		resp.Error = "无法读取目录: " + err.Error()
		return resp, nil
	}

	// 构建文件列表（只显示目录）
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // 只显示目录
		}

		// 跳过隐藏文件和系统文件
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

	// 按名称排序
	sortFiles(resp.Entries)

	return resp, nil
}

// getWindowsDrives 获取 Windows 驱动器列表
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

// ValidatePath 验证路径是否可用于项目
func (s *Service) ValidatePath(ctx context.Context, path string) (*model.ValidatePathResponse, error) {
	resp := &model.ValidatePathResponse{}

	if path == "" {
		resp.Error = "路径不能为空"
		return resp, nil
	}

	// 规范化路径
	path = filepath.Clean(path)

	// Windows 驱动器路径处理
	if runtime.GOOS == "windows" && len(path) == 2 && path[1] == ':' {
		path = path + string(filepath.Separator)
	}

	if err := s.validateWorkspacePath(path); err != nil {
		resp.Error = err.Error()
		return resp, nil
	}

	// 检查路径是否存在
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 路径不存在，检查是否可以创建
			parentDir := filepath.Dir(path)
			parentInfo, parentErr := os.Stat(parentDir)
			if parentErr != nil {
				resp.Error = "父目录不存在"
				return resp, nil
			}
			if !parentInfo.IsDir() {
				resp.Error = "父路径不是目录"
				return resp, nil
			}
			resp.IsValid = true
			resp.CanCreate = true
			return resp, nil
		}
		resp.Error = err.Error()
		return resp, nil
	}

	resp.Exists = true
	resp.IsDir = info.IsDir()

	if !info.IsDir() {
		resp.Error = "路径不是目录"
		return resp, nil
	}

	// 目录存在且有效
	resp.IsValid = true
	resp.Writable = true
	resp.CanCreate = true
	return resp, nil
}

// CreateFolder 创建文件夹
func (s *Service) CreateFolder(ctx context.Context, parentPath, name string) error {
	if parentPath == "" || name == "" {
		return errors.New("父路径和文件夹名称不能为空")
	}

	// 规范化路径
	parentPath = filepath.Clean(parentPath)
	fullPath := filepath.Join(parentPath, strings.TrimSpace(name))

	if s.workspace != nil {
		var err error
		fullPath, err = s.workspace.ValidateChild(parentPath, name)
		if err != nil {
			return err
		}
	}

	// 检查父目录是否存在
	parentInfo, err := os.Stat(parentPath)
	if err != nil {
		return errors.New("父目录不存在")
	}
	if !parentInfo.IsDir() {
		return errors.New("父路径不是目录")
	}

	// 检查目标是否已存在
	if _, err := os.Stat(fullPath); err == nil {
		return errors.New("文件夹已存在")
	}

	// 创建目录（包含所有父目录）
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return errors.New("创建文件夹失败: " + err.Error())
	}

	return nil
}

// checkWritable 检查目录是否可写
func checkWritable(path string) error {
	// 尝试创建临时文件
	testFile := filepath.Join(path, ".isdp_write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return err
	}
	f.Close()
	os.Remove(testFile)
	return nil
}

// sortFiles 对文件列表排序：目录在前，然后按名称
func sortFiles(files []model.FileInfo) {
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			// 目录排在前面
			if !files[i].IsDir && files[j].IsDir {
				files[i], files[j] = files[j], files[i]
			} else if files[i].IsDir == files[j].IsDir && files[i].Name > files[j].Name {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
}

// ListFilesByPath 根据路径列出文件（用于调试模式，不需要项目ID）
func (s *Service) ListFilesByPath(ctx context.Context, basePath string, subPath string) (*model.ListFilesResponse, error) {
	if basePath == "" {
		return nil, errors.New("基础路径不能为空")
	}

	if err := s.validateWorkspacePath(basePath); err != nil {
		return nil, err
	}

	// 安全检查：防止路径遍历攻击
	subPath = filepath.Clean("/" + subPath)
	fullPath := filepath.Join(basePath, subPath)

	// 确保完整路径在基础路径内
	if !pathWithin(basePath, fullPath) {
		return nil, errors.New("无效的路径")
	}

	// 读取目录
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("路径不存在")
		}
		return nil, err
	}

	// 构建文件列表
	var files []model.FileInfo
	for _, entry := range entries {
		// 跳过隐藏文件
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		relPath := filepath.Join(subPath, entry.Name())
		if subPath == "" || subPath == "/" || subPath == "\\" {
			relPath = entry.Name()
		}

		files = append(files, model.FileInfo{
			Name:    entry.Name(),
			Path:    relPath,
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}

	// 排序：目录在前，然后按名称排序
	sortFiles(files)

	return &model.ListFilesResponse{
		Path:    subPath,
		Files:   files,
		HasMore: false,
	}, nil
}

// maxFileSize 文件最大读取大小（1MB）
const maxFileSize = 1 * 1024 * 1024

// binaryExtensions 常见二进制文件扩展名
var binaryExtensions = map[string]bool{
	".exe":   true,
	".dll":   true,
	".so":    true,
	".dylib": true,
	".bin":   true,
	".dat":   true,
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".gif":   true,
	".bmp":   true,
	".ico":   true,
	".svg":   true, // SVG 虽然是文本，但通常是图片资源
	".webp":  true,
	".pdf":   true,
	".zip":   true,
	".tar":   true,
	".gz":    true,
	".rar":   true,
	".7z":    true,
	".mp3":   true,
	".mp4":   true,
	".avi":   true,
	".mov":   true,
	".wav":   true,
	".flv":   true,
	".mkv":   true,
	".woff":  true,
	".woff2": true,
	".ttf":   true,
	".otf":   true,
	".eot":   true,
	".class": true,
	".jar":   true,
	".war":   true,
	".pyc":   true,
	".pyd":   true,
	".node":  true,
	".db":    true,
	".sqlite": true,
	".sqlite3": true,
}

// isBinaryFile 判断是否为二进制文件
func isBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return binaryExtensions[ext]
}

// GetFileContent 获取文件内容
func (s *Service) GetFileContent(ctx context.Context, basePath string, filePath string) (*model.FileContentResponse, error) {
	if basePath == "" {
		return nil, errors.New("基础路径不能为空")
	}

	if err := s.validateWorkspacePath(basePath); err != nil {
		return nil, err
	}

	// 规范化 basePath，确保有尾部分隔符
	basePath = filepath.Clean(basePath)
	if !strings.HasSuffix(basePath, string(filepath.Separator)) {
		basePath += string(filepath.Separator)
	}

	// 规范化 filePath，去掉前导分隔符（防止被当作绝对路径）
	filePath = filepath.Clean(filePath)
	// 去掉前导的路径分隔符（Windows: \ 或 /，Unix: /）
	for strings.HasPrefix(filePath, string(filepath.Separator)) {
		filePath = strings.TrimPrefix(filePath, string(filepath.Separator))
	}
	for strings.HasPrefix(filePath, "/") {
		filePath = strings.TrimPrefix(filePath, "/")
	}

	fullPath := filepath.Join(basePath, filePath)

	// 确保完整路径在基础路径内
	if !pathWithin(basePath, fullPath) {
		return nil, errors.New("无效的路径")
	}

	// 检查文件是否存在
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("文件不存在")
		}
		return nil, err
	}

	// 检查是否为目录
	if info.IsDir() {
		return nil, errors.New("路径是目录，不是文件")
	}

	// 解析 symlink，防止 symlink bypass 攻击
	evaluatedPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		return nil, errors.New("无法解析路径: " + err.Error())
	}
	// 再次检查 resolved path 是否仍在 basePath 内
	// 去掉 basePath 的尾部分隔符进行 EvalSymlinks
	evaluatedBase, err := filepath.EvalSymlinks(basePath[:len(basePath)-1])
	if err != nil {
		return nil, errors.New("无法解析基础路径: " + err.Error())
	}
	// 确保解析后的路径仍在解析后的基础路径内
	if !pathWithin(evaluatedBase, evaluatedPath) {
		return nil, errors.New("无效的路径（symlink指向外部目录）")
	}

	// 判断是否为二进制文件
	isBinary := isBinaryFile(fullPath)

	resp := &model.FileContentResponse{
		Path:     filePath,
		Size:     info.Size(),
		IsBinary: isBinary,
	}

	// 如果是二进制文件，不读取内容
	if isBinary {
		resp.Content = ""
		resp.Truncated = false
		return resp, nil
	}

	// 使用 LimitReader 限制读取大小，防止大文件占用过多内存
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, errors.New("打开文件失败: " + err.Error())
	}
	defer file.Close()

	// +1 用于检测是否超过限制
	limitedReader := io.LimitReader(file, maxFileSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, errors.New("读取文件失败: " + err.Error())
	}

	// 检查是否超过最大大小
	truncated := len(data) > maxFileSize
	if truncated {
		data = data[:maxFileSize]
	}
	resp.Truncated = truncated
	resp.Content = string(data)

	return resp, nil
}