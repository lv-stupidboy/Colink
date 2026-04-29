package settings

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// DirectoryContent 目录内容结构
type DirectoryContent struct {
	Path    string         `json:"path"`
	Files   []FileInfo     `json:"files"`
	Subdirs []SubdirInfo   `json:"subdirs"`
}

// FileInfo 文件信息
type FileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

// SubdirInfo 子目录信息
type SubdirInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// CreateFromFileRequest 从文件创建Settings的请求
type CreateFromFileRequest struct {
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	Version         string     `json:"version"`
	SupportedAgents []string   `json:"supportedAgents"` // 支持的Agent类型
	Files           []FileData `json:"files"`
}

// FileData 文件数据
type FileData struct {
	RelativePath string `json:"relativePath"`
	Content      io.Reader `json:"-"`
}

// Service Settings业务服务
type Service struct {
	settingsRepo         *repo.SettingsRepository
	agentSettingsBinding *repo.AgentSettingsBindingRepository
	agentRepo            *repo.AgentConfigRepository
	storagePath          string
	logger               *zap.Logger
}

// NewService 创建Settings Service
func NewService(
	settingsRepo *repo.SettingsRepository,
	agentSettingsBinding *repo.AgentSettingsBindingRepository,
	agentRepo *repo.AgentConfigRepository,
	storagePath string,
	logger *zap.Logger,
) *Service {
	return &Service{
		settingsRepo:         settingsRepo,
		agentSettingsBinding: agentSettingsBinding,
		agentRepo:            agentRepo,
		storagePath:          storagePath,
		logger:               logger,
	}
}

// Create 创建Settings（基本元数据创建）
func (s *Service) Create(ctx context.Context, req *model.CreateSettingsRequest) (*model.Settings, error) {
	// 检查名称是否重复
	existing, err := s.settingsRepo.FindByName(ctx, req.Name)
	if err != nil {
		// 如果不是"未找到"错误，返回实际错误
		if !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("检查Settings名称失败: %w", err)
		}
		// 名称不存在，可以创建
	} else if existing != nil {
		return nil, errors.New("Settings名称已存在")
	}

	// 处理 SupportedAgents：空数组默认为 ["claude_code"]
	supportedAgents := req.SupportedAgents
	if len(supportedAgents) == 0 {
		supportedAgents = []string{"claude_code"}
	}

	now := time.Now()
	settings := &model.Settings{
		ID:              uuid.New(),
		Name:            req.Name,
		Description:     req.Description,
		SupportedAgents: supportedAgents,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// 创建存储目录
	if s.storagePath != "" {
		settingsDir := filepath.Join(s.storagePath, req.Name)
		if err := os.MkdirAll(settingsDir, 0755); err != nil {
			return nil, fmt.Errorf("创建Settings目录失败: %w", err)
		}
		settings.DirectoryPath = settingsDir
	}

	if err := s.settingsRepo.Create(ctx, settings); err != nil {
		// 如果创建失败，清理已创建的目录
		if settings.DirectoryPath != "" {
			os.RemoveAll(settings.DirectoryPath)
		}
		return nil, fmt.Errorf("创建Settings失败: %w", err)
	}

	s.logger.Info("创建Settings成功",
		zap.String("id", settings.ID.String()),
		zap.String("name", settings.Name),
		zap.String("path", settings.DirectoryPath))

	return settings, nil
}

// CreateFromFile 从上传的文件创建Settings
func (s *Service) CreateFromFile(ctx context.Context, req *CreateFromFileRequest) (*model.Settings, error) {
	// 检查名称是否重复
	existing, err := s.settingsRepo.FindByName(ctx, req.Name)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("检查Settings名称失败: %w", err)
		}
	} else if existing != nil {
		return nil, errors.New("Settings名称已存在")
	}

	// 处理 SupportedAgents：空数组默认为 ["claude_code"]
	supportedAgents := req.SupportedAgents
	if len(supportedAgents) == 0 {
		supportedAgents = []string{"claude_code"}
	}

	// 创建存储目录
	settingsDir := filepath.Join(s.storagePath, req.Name)
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return nil, fmt.Errorf("创建Settings目录失败: %w", err)
	}

	// 写入文件
	for i, file := range req.Files {
		// 构建完整路径
		fullPath := filepath.Join(settingsDir, file.RelativePath)

		s.logger.Info("写入文件",
			zap.Int("index", i),
			zap.String("relativePath", file.RelativePath),
			zap.String("fullPath", fullPath))

		// 确保父目录存在
		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			// 清理已创建的目录
			os.RemoveAll(settingsDir)
			return nil, fmt.Errorf("创建子目录失败: %w", err)
		}

		// 写入文件内容
		dstFile, err := os.Create(fullPath)
		if err != nil {
			os.RemoveAll(settingsDir)
			return nil, fmt.Errorf("创建文件失败: %w", err)
		}

		_, err = io.Copy(dstFile, file.Content)
		dstFile.Close()
		if err != nil {
			os.RemoveAll(settingsDir)
			return nil, fmt.Errorf("写入文件失败: %w", err)
		}
	}

	now := time.Now()

	settings := &model.Settings{
		ID:              uuid.New(),
		Name:            req.Name,
		Description:     req.Description,
		DirectoryPath:   settingsDir,
		SupportedAgents: supportedAgents,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.settingsRepo.Create(ctx, settings); err != nil {
		// 如果数据库创建失败，清理已创建的目录
		os.RemoveAll(settingsDir)
		return nil, fmt.Errorf("创建Settings失败: %w", err)
	}

	s.logger.Info("从文件创建Settings成功",
		zap.String("id", settings.ID.String()),
		zap.String("name", settings.Name),
		zap.Int("fileCount", len(req.Files)))

	return settings, nil
}

// GetByID 根据ID获取Settings
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*model.Settings, error) {
	return s.settingsRepo.FindByID(ctx, id)
}

// GetByName 根据名称获取Settings
func (s *Service) GetByName(ctx context.Context, name string) (*model.Settings, error) {
	return s.settingsRepo.FindByName(ctx, name)
}

// List 分页列表查询
func (s *Service) List(ctx context.Context, query *model.SettingsListQuery) ([]*model.Settings, int64, error) {
	return s.settingsRepo.List(ctx, query)
}

// Update 更新Settings元数据
func (s *Service) Update(ctx context.Context, id uuid.UUID, req *model.UpdateSettingsRequest) (*model.Settings, error) {
	settings, err := s.settingsRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Settings不存在: %w", err)
	}

	// 更新字段
	if req.Description != "" {
		settings.Description = req.Description
	}
	// 更新 SupportedAgents（如果提供了）
	if req.SupportedAgents != nil {
		settings.SupportedAgents = req.SupportedAgents
	}
	settings.UpdatedAt = time.Now()

	if err := s.settingsRepo.Update(ctx, settings); err != nil {
		return nil, fmt.Errorf("更新Settings失败: %w", err)
	}

	return settings, nil
}

// Delete 删除Settings（检查绑定关系）
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// 获取Settings信息
	settings, err := s.settingsRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("Settings不存在: %w", err)
	}

	// 检查是否有Agent绑定
	agentRoleIDs, err := s.agentSettingsBinding.FindBySettingsID(ctx, id)
	if err != nil {
		return fmt.Errorf("检查Agent绑定关系失败: %w", err)
	}
	if len(agentRoleIDs) > 0 {
		// 获取Agent名称列表
		agentNames := make([]string, 0, len(agentRoleIDs))
		for _, agentID := range agentRoleIDs {
			agent, err := s.agentRepo.FindByID(ctx, agentID)
			if err == nil {
				agentNames = append(agentNames, agent.Name)
			}
		}
		return fmt.Errorf("无法删除Settings：该Settings已被以下Agent绑定：%s", strings.Join(agentNames, "、"))
	}

	// 删除数据库记录
	if err := s.settingsRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除Settings失败: %w", err)
	}

	// 删除对应的目录
	if settings.DirectoryPath != "" {
		if _, err := os.Stat(settings.DirectoryPath); err == nil {
			if err := os.RemoveAll(settings.DirectoryPath); err != nil {
				s.logger.Warn("删除Settings目录失败",
					zap.String("path", settings.DirectoryPath),
					zap.Error(err))
			} else {
				s.logger.Info("删除Settings目录成功",
					zap.String("path", settings.DirectoryPath))
			}
		}
	}

	s.logger.Info("删除Settings成功",
		zap.String("id", id.String()),
		zap.String("name", settings.Name))

	return nil
}

// BindSettings 绑定Settings到AgentRole（全量替换）
func (s *Service) BindSettings(ctx context.Context, agentRoleID uuid.UUID, settingsIDs []uuid.UUID) error {
	// 验证Agent是否存在
	_, err := s.agentRepo.FindByID(ctx, agentRoleID)
	if err != nil {
		return fmt.Errorf("Agent角色不存在: %w", err)
	}

	// 验证所有Settings存在
	for _, settingsID := range settingsIDs {
		_, err := s.settingsRepo.FindByID(ctx, settingsID)
		if err != nil {
			return fmt.Errorf("Settings %s 不存在: %w", settingsID.String(), err)
		}
	}

	// 先删除所有现有绑定
	if err := s.agentSettingsBinding.DeleteByAgentRoleID(ctx, agentRoleID); err != nil {
		return fmt.Errorf("清理旧绑定失败: %w", err)
	}

	// 创建新的绑定
	for _, settingsID := range settingsIDs {
		binding := &model.AgentSettingsBinding{
			ID:          uuid.New(),
			AgentRoleID: agentRoleID,
			SettingsID:  settingsID,
			CreatedAt:   time.Now(),
		}
		if err := s.agentSettingsBinding.Create(ctx, binding); err != nil {
			return err
		}
	}

	s.logger.Info("绑定Settings成功",
		zap.String("agentRoleID", agentRoleID.String()),
		zap.Int("settingsCount", len(settingsIDs)))

	return nil
}

// UnbindSettings 解除Settings绑定
func (s *Service) UnbindSettings(ctx context.Context, agentRoleID, settingsID uuid.UUID) error {
	// 检查绑定是否存在
	exists, err := s.agentSettingsBinding.ExistsBinding(ctx, agentRoleID, settingsID)
	if err != nil {
		return fmt.Errorf("检查绑定关系失败: %w", err)
	}
	if !exists {
		return errors.New("绑定关系不存在")
	}

	if err := s.agentSettingsBinding.DeleteBinding(ctx, agentRoleID, settingsID); err != nil {
		return fmt.Errorf("解除绑定失败: %w", err)
	}

	s.logger.Info("解除Settings绑定成功",
		zap.String("agentRoleID", agentRoleID.String()),
		zap.String("settingsID", settingsID.String()))

	return nil
}

// GetBoundSettings 获取Agent绑定的所有Settings
func (s *Service) GetBoundSettings(ctx context.Context, agentRoleID uuid.UUID) ([]*model.Settings, error) {
	settingsIDs, err := s.agentSettingsBinding.FindByAgentRoleID(ctx, agentRoleID)
	if err != nil {
		return nil, err
	}

	settingsList := make([]*model.Settings, 0, len(settingsIDs))
	for _, settingsID := range settingsIDs {
		settings, err := s.settingsRepo.FindByID(ctx, settingsID)
		if err != nil {
			continue // 跳过不存在的settings
		}
		settingsList = append(settingsList, settings)
	}

	return settingsList, nil
}

// GetBoundAgents 获取Settings绑定的所有Agents
func (s *Service) GetBoundAgents(ctx context.Context, settingsID uuid.UUID) ([]*model.AgentRoleConfig, error) {
	agentRoleIDs, err := s.agentSettingsBinding.FindBySettingsID(ctx, settingsID)
	if err != nil {
		return nil, err
	}

	agents := make([]*model.AgentRoleConfig, 0, len(agentRoleIDs))
	for _, agentRoleID := range agentRoleIDs {
		agent, err := s.agentRepo.FindByID(ctx, agentRoleID)
		if err != nil {
			continue // 跳过不存在的agent
		}
		agents = append(agents, agent)
	}

	return agents, nil
}

// ReadDirectoryContent 读取Settings目录内容
func (s *Service) ReadDirectoryContent(ctx context.Context, id uuid.UUID, subPath string) (*DirectoryContent, error) {
	settings, err := s.settingsRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Settings不存在: %w", err)
	}

	if settings.DirectoryPath == "" {
		return nil, errors.New("Settings没有关联的目录")
	}

	// 构建完整路径
	fullPath := settings.DirectoryPath
	if subPath != "" {
		fullPath = filepath.Join(settings.DirectoryPath, subPath)
	}

	// 安全检查：确保路径在Settings目录内
	if !strings.HasPrefix(fullPath, settings.DirectoryPath) {
		return nil, errors.New("路径超出Settings目录范围")
	}

	// 检查路径是否存在
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("目录不存在")
		}
		return nil, fmt.Errorf("读取目录信息失败: %w", err)
	}

	if !info.IsDir() {
		return nil, errors.New("路径不是目录")
	}

	// 读取目录内容
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录内容失败: %w", err)
	}

	content := &DirectoryContent{
		Path:    subPath,
		Files:   make([]FileInfo, 0),
		Subdirs: make([]SubdirInfo, 0),
	}

	for _, entry := range entries {
		if entry.IsDir() {
			content.Subdirs = append(content.Subdirs, SubdirInfo{
				Name: entry.Name(),
				Path: filepath.Join(subPath, entry.Name()),
			})
		} else {
			info, err := entry.Info()
			if err != nil {
				continue // 跳过无法读取的文件
			}
			content.Files = append(content.Files, FileInfo{
				Name:    entry.Name(),
				Size:    info.Size(),
				ModTime: info.ModTime().Format(time.RFC3339),
			})
		}
	}

	return content, nil
}

// ReadFileContent 读取Settings目录中的文件内容
func (s *Service) ReadFileContent(ctx context.Context, id uuid.UUID, filePath string) ([]byte, error) {
	settings, err := s.settingsRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Settings不存在: %w", err)
	}

	if settings.DirectoryPath == "" {
		return nil, errors.New("Settings没有关联的目录")
	}

	// 构建完整路径
	fullPath := filepath.Join(settings.DirectoryPath, filePath)

	// 安全检查：确保路径在Settings目录内
	if !strings.HasPrefix(fullPath, settings.DirectoryPath) {
		return nil, errors.New("路径超出Settings目录范围")
	}

	// 检查文件是否存在
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("文件不存在")
		}
		return nil, fmt.Errorf("读取文件信息失败: %w", err)
	}

	if info.IsDir() {
		return nil, errors.New("路径是目录，不是文件")
	}

	// 读取文件内容
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件内容失败: %w", err)
	}

	return content, nil
}