package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// RegistryService 联邦技能源服务
type RegistryService struct {
	registryRepo  *repo.SkillRegistryRepository
	skillRepo     *repo.SkillRepository
	skillScanner  *SkillScanner
}

// NewRegistryService 创建 Registry Service
func NewRegistryService(registryRepo *repo.SkillRegistryRepository, skillRepo *repo.SkillRepository, skillScanner *SkillScanner) *RegistryService {
	return &RegistryService{
		registryRepo:  registryRepo,
		skillRepo:     skillRepo,
		skillScanner:  skillScanner,
	}
}

// Create 创建注册表
func (s *RegistryService) Create(ctx context.Context, req *model.CreateRegistryRequest) (*model.SkillRegistry, error) {
	// 检查名称是否重复
	existing, err := s.registryRepo.FindByName(ctx, req.Name)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("检查注册表名称失败: %w", err)
		}
	} else if existing != nil {
		return nil, errors.New("注册表名称已存在")
	}

	registry := &model.SkillRegistry{
		ID:           uuid.New(),
		Name:         req.Name,
		DisplayName:  req.DisplayName,
		Type:         req.Type,
		URL:          req.URL,
		AuthConfig:   req.AuthConfig,
		SyncInterval: req.SyncInterval,
		SyncStatus:   model.RegistrySyncPending,
		SkillCount:   0,
		Status:       model.RegistryStatusActive,
		CreatedAt:    time.Now(),
	}

	// 默认同步间隔为1小时
	if registry.SyncInterval == 0 {
		registry.SyncInterval = 3600
	}

	if err := s.registryRepo.Create(ctx, registry); err != nil {
		return nil, fmt.Errorf("创建注册表失败: %w", err)
	}

	return registry, nil
}

// GetByID 根据ID获取注册表
func (s *RegistryService) GetByID(ctx context.Context, id uuid.UUID) (*model.SkillRegistry, error) {
	return s.registryRepo.FindByID(ctx, id)
}

// GetByName 根据名称获取注册表
func (s *RegistryService) GetByName(ctx context.Context, name string) (*model.SkillRegistry, error) {
	return s.registryRepo.FindByName(ctx, name)
}

// List 列出注册表
func (s *RegistryService) List(ctx context.Context, query *repo.RegistryListQuery) ([]*model.SkillRegistry, int64, error) {
	return s.registryRepo.List(ctx, query)
}

// Update 更新注册表
func (s *RegistryService) Update(ctx context.Context, id uuid.UUID, req *model.UpdateRegistryRequest) (*model.SkillRegistry, error) {
	registry, err := s.registryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("注册表不存在: %w", err)
	}

	// 更新字段
	if req.DisplayName != "" {
		registry.DisplayName = req.DisplayName
	}
	if req.URL != "" {
		registry.URL = req.URL
	}
	if req.AuthConfig != nil {
		registry.AuthConfig = req.AuthConfig
	}
	if req.SyncInterval > 0 {
		registry.SyncInterval = req.SyncInterval
	}
	if req.Status != "" {
		registry.Status = req.Status
	}

	if err := s.registryRepo.Update(ctx, registry); err != nil {
		return nil, fmt.Errorf("更新注册表失败: %w", err)
	}

	return registry, nil
}

// Delete 删除注册表
func (s *RegistryService) Delete(ctx context.Context, id uuid.UUID) error {
	// TODO: 检查是否有技能关联此注册表
	return s.registryRepo.Delete(ctx, id)
}

// Sync 同步注册表技能
func (s *RegistryService) Sync(ctx context.Context, id uuid.UUID) (*model.SyncResult, error) {
	registry, err := s.registryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("注册表不存在: %w", err)
	}

	result := &model.SyncResult{
		RegistryID:   registry.ID,
		RegistryName: registry.Name,
	}

	// 根据注册表类型选择同步策略
	var skills []*RemoteSkill
	switch registry.Type {
	case model.RegistryTypeGitHub:
		skills, err = s.syncFromGitHub(ctx, registry)
	case model.RegistryTypeGitLab:
		skills, err = s.syncFromGitLab(ctx, registry)
	case model.RegistryTypeAPI:
		skills, err = s.syncFromAPI(ctx, registry)
	case model.RegistryTypeCustom, model.RegistryTypeCodeHub:
		// Custom 和 CodeHub 类型使用 SkillScanner 进行 Git 仓库同步
		skills, err = s.syncFromGitRepo(ctx, registry)
	default:
		err = fmt.Errorf("不支持的注册表类型: %s", registry.Type)
	}

	if err != nil {
		result.Error = err.Error()
		// 更新同步状态为失败
		s.registryRepo.UpdateSyncStatus(ctx, id, model.RegistrySyncFailed, 0)
		return result, err
	}

	// 同步技能到本地
	for _, remoteSkill := range skills {
		// 检查是否已存在
		existing, err := s.skillRepo.FindByName(ctx, remoteSkill.Name)
		if err != nil {
			// 不存在，创建新技能
			skill := &model.Skill{
				ID:               uuid.New(),
				Name:             remoteSkill.Name,
				Description:      remoteSkill.Description,
				Tags:             remoteSkill.Tags,
				SourceType:       model.SkillSourceFederated,
				SourceRegistryID: registry.ID,
				SupportedAgents:  remoteSkill.SupportedAgents,
				IsPublic:         true, // 联邦技能固定公开
				Status:           model.SkillStatusActive,
				UseCount:         0,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}
			if err := s.skillRepo.Create(ctx, skill); err != nil {
				continue
			}
			result.SkillsAdded++
		} else {
			// 已存在，更新技能
			existing.Description = remoteSkill.Description
			existing.Tags = remoteSkill.Tags
			existing.SupportedAgents = remoteSkill.SupportedAgents
			existing.UpdatedAt = time.Now()
			if err := s.skillRepo.Update(ctx, existing); err != nil {
				continue
			}
			result.SkillsUpdated++
		}
	}

	// 更新同步状态
	s.registryRepo.UpdateSyncStatus(ctx, id, model.RegistrySyncSuccess, result.SkillsAdded+result.SkillsUpdated)

	return result, nil
}

// RemoteSkill 远程技能结构
type RemoteSkill struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Tags            []string `json:"tags"`
	SupportedAgents []string `json:"supported_agents"`
	Version         string   `json:"version"`
}

// syncFromGitHub 从 GitHub 同步
func (s *RegistryService) syncFromGitHub(ctx context.Context, registry *model.SkillRegistry) ([]*RemoteSkill, error) {
	// GitHub 同步逻辑：读取仓库中的 skills 目录
	// URL 格式: https://github.com/owner/repo
	// 技能文件: skills/*.md

	client := &http.Client{Timeout: 30 * time.Second}

	// 构建 API URL
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/skills",
		strings.TrimPrefix(registry.URL, "https://github.com/"))

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加认证头
	if token, ok := registry.AuthConfig["token"]; ok && token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 返回错误: %d", resp.StatusCode)
	}

	// 解析目录内容
	var contents []struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		DownloadURL string `json:"download_url"`
		Type        string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, fmt.Errorf("解析 GitHub 响应失败: %w", err)
	}

	skills := make([]*RemoteSkill, 0)
	for _, content := range contents {
		if content.Type != "file" || !strings.HasSuffix(content.Name, ".md") {
			continue
		}

		// 下载技能文件内容
		skill, err := s.downloadGitHubSkill(ctx, client, content.DownloadURL, registry.AuthConfig["token"])
		if err != nil {
			continue
		}
		skills = append(skills, skill)
	}

	return skills, nil
}

// downloadGitHubSkill 下载 GitHub 技能文件
func (s *RegistryService) downloadGitHubSkill(ctx context.Context, client *http.Client, url string, token string) (*RemoteSkill, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败: %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析技能文件（简化处理，实际应解析 frontmatter）
	skillName := strings.TrimSuffix(req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:], ".md")
	return &RemoteSkill{
		Name:        skillName,
		Description: string(content),
	}, nil
}

// syncFromGitLab 从 GitLab 同步
func (s *RegistryService) syncFromGitLab(ctx context.Context, registry *model.SkillRegistry) ([]*RemoteSkill, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// 从 URL 提取项目路径
	// URL 格式: https://gitlab.com/owner/repo
	projectPath := strings.TrimPrefix(registry.URL, "https://gitlab.com/")
	projectPath = strings.TrimSuffix(projectPath, ".git")
	projectPath = strings.TrimSuffix(projectPath, "/")

	// GitLab API: 编码项目路径
	encodedPath := urlEncodePath(projectPath)

	// 获取项目 ID
	projectAPI := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s", encodedPath)
	req, err := http.NewRequestWithContext(ctx, "GET", projectAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加认证头
	if token, ok := registry.AuthConfig["token"]; ok && token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitLab API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab API 返回错误: %d", resp.StatusCode)
	}

	var projectInfo struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&projectInfo); err != nil {
		return nil, fmt.Errorf("解析项目信息失败: %w", err)
	}

	// 获取仓库目录树
	treeAPI := fmt.Sprintf("https://gitlab.com/api/v4/projects/%d/repository/tree?recursive=true", projectInfo.ID)
	req, err = http.NewRequestWithContext(ctx, "GET", treeAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if token, ok := registry.AuthConfig["token"]; ok && token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	resp, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 GitLab API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab API 返回错误: %d", resp.StatusCode)
	}

	var treeNodes []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&treeNodes); err != nil {
		return nil, fmt.Errorf("解析目录树失败: %w", err)
	}

	skills := make([]*RemoteSkill, 0)
	for _, node := range treeNodes {
		if node.Type == "blob" && strings.ToLower(node.Name) == "skill.md" {
			// 下载文件内容
			skill, err := s.downloadGitLabSkill(ctx, client, projectInfo.ID, node.Path, registry.AuthConfig["token"])
			if err != nil {
				continue
			}
			skills = append(skills, skill)
		}
	}

	return skills, nil
}

// downloadGitLabSkill 从 GitLab 下载 Skill 文件
func (s *RegistryService) downloadGitLabSkill(ctx context.Context, client *http.Client, projectID int, filePath string, token string) (*RemoteSkill, error) {
	encodedPath := urlEncodePath(filePath)
	fileAPI := fmt.Sprintf("https://gitlab.com/api/v4/projects/%d/repository/files/%s/raw?ref=HEAD", projectID, encodedPath)

	req, err := http.NewRequestWithContext(ctx, "GET", fileAPI, nil)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败: %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析 Skill 名称（从文件路径提取）
	// 路径格式: skills/xxx/SKILL.md 或 SKILL.md
	parts := strings.Split(filePath, "/")
	skillName := ""
	if len(parts) > 1 {
		skillName = parts[len(parts)-2] // 取倒数第二部分作为名称
	} else {
		skillName = "root-skill"
	}

	return &RemoteSkill{
		Name:        skillName,
		Description: string(content),
	}, nil
}

// urlEncodePath URL 编码路径（GitLab 要求）
func urlEncodePath(path string) string {
	return strings.ReplaceAll(path, "/", "%2F")
}

// syncFromAPI 从自定义 API 同步
func (s *RegistryService) syncFromAPI(ctx context.Context, registry *model.SkillRegistry) ([]*RemoteSkill, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", registry.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 添加认证头
	if token, ok := registry.AuthConfig["token"]; ok && token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回错误: %d", resp.StatusCode)
	}

	var response struct {
		Skills []*RemoteSkill `json:"skills"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("解析 API 响应失败: %w", err)
	}

	return response.Skills, nil
}

// syncFromGitRepo 从 Git 仓库同步（支持 custom 和 codehub 类型）
func (s *RegistryService) syncFromGitRepo(ctx context.Context, registry *model.SkillRegistry) ([]*RemoteSkill, error) {
	// 使用 SkillScanner 扫描仓库
	if s.skillScanner == nil {
		return nil, fmt.Errorf("SkillScanner 未初始化")
	}

	scanResult, err := s.skillScanner.ScanRegistry(ctx, registry.ID)
	if err != nil {
		return nil, fmt.Errorf("扫描 Git 仓库失败: %w", err)
	}

	// 将 ScanResult 中的 RemoteSkill 转换为 RemoteSkill
	skills := make([]*RemoteSkill, 0, len(scanResult.Skills))
	for _, scannedSkill := range scanResult.Skills {
		skills = append(skills, &RemoteSkill{
			Name:            scannedSkill.Name,
			Description:     scannedSkill.Description,
			Tags:            []string{},
			SupportedAgents: []string{},
			Version:         "",
		})
	}

	return skills, nil
}

// SyncAll 同步所有活跃注册表
func (s *RegistryService) SyncAll(ctx context.Context) ([]*model.SyncResult, error) {
	registries, err := s.registryRepo.FindByStatus(ctx, model.RegistryStatusActive)
	if err != nil {
		return nil, fmt.Errorf("获取活跃注册表失败: %w", err)
	}

	results := make([]*model.SyncResult, 0, len(registries))
	for _, registry := range registries {
		result, err := s.Sync(ctx, registry.ID)
		if err != nil {
			// 记录错误但继续同步其他注册表
			results = append(results, result)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}