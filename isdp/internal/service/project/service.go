package project

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// Service 项目服务
type Service struct {
	repo         *repo.ProjectRepository
	workflowRepo *repo.WorkflowTemplateRepository // 新增依赖
}

// NewService 创建项目服务
func NewService(repo *repo.ProjectRepository, workflowRepo *repo.WorkflowTemplateRepository) *Service {
	return &Service{
		repo:         repo,
		workflowRepo: workflowRepo,
	}
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
	project := &model.Project{
		ID:        uuid.New(),
		Name:      req.Name,
		Type:      req.Type,
		Mode:      req.Mode,
		Status:    model.ProjectStatusDraft,
		LocalPath: req.LocalPath,
		GitRepo:   req.ExistingRepoURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
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
		project.LocalPath = *req.LocalPath
	}
	if req.GitRepo != nil {
		project.GitRepo = *req.GitRepo
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

	// 安全检查：防止路径遍历攻击
	subPath = filepath.Clean("/" + subPath)
	fullPath := filepath.Join(basePath, subPath)

	// 确保完整路径在项目路径内
	if !strings.HasPrefix(fullPath, basePath) {
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