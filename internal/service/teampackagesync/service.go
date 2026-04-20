package teampackagesync

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/market"
	"github.com/anthropic/isdp/internal/service/teampackage"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SyncService 团队包同步服务
type SyncService struct {
	versionRepo    *repo.TeamPackageVersionRepository
	workflowRepo   *repo.WorkflowTemplateRepository
	teamPackageSvc *teampackage.Service
	marketSvc      *market.Service // 市场服务
	config         config.TeamPackageSyncConfig
	gitClient      *GitClient
	logger         *zap.Logger
}

// NewSyncService 创建同步服务
func NewSyncService(
	versionRepo *repo.TeamPackageVersionRepository,
	workflowRepo *repo.WorkflowTemplateRepository,
	teamPackageSvc *teampackage.Service,
	marketSvc *market.Service, // 新增参数
	cfg config.TeamPackageSyncConfig,
	basePath string,
	logger *zap.Logger,
) *SyncService {
	return &SyncService{
		versionRepo:    versionRepo,
		workflowRepo:   workflowRepo,
		teamPackageSvc: teamPackageSvc,
		marketSvc:      marketSvc, // 新增
		config:         cfg,
		gitClient:      NewGitClient(cfg, basePath, logger),
		logger:         logger,
	}
}

// CheckUpdates 检查本地版本与远程版本的差异
func (s *SyncService) CheckUpdates(ctx context.Context) (*UpdateCheckResult, error) {
	// 获取本地版本列表
	localVersions, err := s.versionRepo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get local versions: %w", err)
	}

	result := &UpdateCheckResult{
		NeedUpdate:  []PackageUpdateInfo{},
		NewPackages: []RemotePackage{},
		Removed:     []string{},
	}

	// 从所有市场获取远程包列表进行对比
	markets, err := s.marketSvc.ListMarkets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list markets: %w", err)
	}

	// 构建本地版本映射
	localMap := make(map[string]model.TeamPackageVersion)
	for _, v := range localVersions {
		localMap[v.Name] = v
	}

	for _, market := range markets {
		if !market.Enabled {
			continue
		}

		marketplace, err := s.marketSvc.RefreshMarket(ctx, market.ID)
		if err != nil {
			s.logger.Warn("failed to refresh market for update check",
				zap.String("market", market.Name),
				zap.Error(err))
			continue
		}

		for _, plugin := range marketplace.Plugins {
			if strings.ToLower(plugin.Category) != "team" {
				continue
			}

			if local, exists := localMap[plugin.Name]; exists {
				// 比较版本
				if CompareVersions(local.Version, plugin.Version) < 0 {
					result.NeedUpdate = append(result.NeedUpdate, PackageUpdateInfo{
						Local:  local,
						Remote: RemotePackage{
							Name:        plugin.Name,
							Version:     plugin.Version,
							Description: plugin.Description,
						},
					})
				}
				delete(localMap, plugin.Name)
			} else {
				result.NewPackages = append(result.NewPackages, RemotePackage{
					Name:        plugin.Name,
					Version:     plugin.Version,
					Description: plugin.Description,
				})
			}
		}
	}

	// 映射中剩余的是远程已移除的包
	for name := range localMap {
		result.Removed = append(result.Removed, name)
	}

	return result, nil
}

// GetLocalVersions 获取本地版本记录列表
func (s *SyncService) GetLocalVersions(ctx context.Context) ([]model.TeamPackageVersion, error) {
	versions, err := s.versionRepo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local versions: %w", err)
	}
	return versions, nil
}

// SyncPackage 同步指定的团队包（必须指定 marketId）
func (s *SyncService) SyncPackage(ctx context.Context, packageName string, marketId string, confirm *model.TeamPackageImportConfirm) (*model.ImportResult, error) {
	if marketId == "" {
		return nil, fmt.Errorf("marketId is required")
	}
	if s.marketSvc == nil {
		return nil, fmt.Errorf("market service not available")
	}

	var remotePkg *RemotePackage
	var packageCloneDir string

	marketUUID, err := uuid.Parse(marketId)
	if err != nil {
		return nil, fmt.Errorf("invalid market id: %w", err)
	}

	market, err := s.marketSvc.GetMarketByID(ctx, marketUUID)
	if err != nil {
		return nil, fmt.Errorf("get market: %w", err)
	}
	if market == nil {
		return nil, fmt.Errorf("market not found: %s", marketId)
	}

	// 克隆市场仓库获取 marketplace.json
	marketCloneDir, err := s.gitClient.CloneFromURL(ctx, market.URL, market.Branch)
	if err != nil {
		return nil, fmt.Errorf("clone market repo: %w", err)
	}
	defer s.gitClient.Cleanup(marketCloneDir)

	// 解析 marketplace.json
	marketplace, err := s.parseMarketplaceJSON(marketCloneDir)
	if err != nil {
		return nil, fmt.Errorf("parse marketplace.json: %w", err)
	}

	// 查找指定的包
	for _, plugin := range marketplace.Plugins {
		if strings.ToLower(plugin.Category) == "team" && plugin.Name == packageName {
			remotePkg = &RemotePackage{
				Name:        plugin.Name,
				Version:     plugin.Version,
				Description: plugin.Description,
				Path:        "", // 将在克隆后设置
				Repository:  plugin.Repository,
				Source:      plugin.Source,
			}
			break
		}
	}

	if remotePkg == nil {
		return nil, fmt.Errorf("package not found in marketplace: %s", packageName)
	}

	// 克隆包仓库
	packageCloneDir, err = s.gitClient.CloneFromURL(ctx, remotePkg.Repository, "master")
	if err != nil {
		return nil, fmt.Errorf("clone package repo: %w", err)
	}
	defer s.gitClient.Cleanup(packageCloneDir)

	// 设置包的实际路径
	remotePkg.Path = filepath.Join(packageCloneDir, remotePkg.Source)

	// 将目录转换为 zip 数据
	zipData, err := s.createZipFromDir(remotePkg.Path)
	if err != nil {
		return nil, fmt.Errorf("create zip: %w", err)
	}

	// 如果 confirm 为空，创建一个默认的（全部覆盖导入）
	if confirm == nil {
		confirm = &model.TeamPackageImportConfirm{
			Mode:           "overwrite",
			WorkflowAction: "overwrite",
			AssetActions: []model.TeamPackageAssetAction{
				{AssetType: "skill", Name: "*", Action: "overwrite"},
				{AssetType: "command", Name: "*", Action: "overwrite"},
				{AssetType: "rule", Name: "*", Action: "overwrite"},
			},
		}
	}

	// 调用现有的 ImportConfirm 方法（零侵入复用）
	result, err := s.teamPackageSvc.ImportConfirm(ctx, zipData, confirm)
	if err != nil {
		return nil, fmt.Errorf("import confirm: %w", err)
	}

	// 更新版本记录
	if err := s.updateVersionRecord(ctx, packageName, remotePkg, result); err != nil {
		s.logger.Warn("failed to update version record", zap.Error(err))
	}

	return result, nil
}

// createZipFromDir 将目录创建为 zip 文件
func (s *SyncService) createZipFromDir(dirPath string) ([]byte, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	err := s.addDirToZip(w, dirPath, "")
	if err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// addDirToZip 递归添加目录到 zip
func (s *SyncService) addDirToZip(w *zip.Writer, basePath, zipPath string) error {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fullZipPath := filepath.Join(zipPath, entry.Name())
		fullPath := filepath.Join(basePath, entry.Name())

		if entry.IsDir() {
			// 添加目录条目
			if _, err := w.Create(fullZipPath + "/"); err != nil {
				return err
			}
			if err := s.addDirToZip(w, fullPath, fullZipPath); err != nil {
				return err
			}
		} else {
			// 添加文件
			data, err := os.ReadFile(fullPath)
			if err != nil {
				return err
			}
			f, err := w.Create(fullZipPath)
			if err != nil {
				return err
			}
			if _, err := f.Write(data); err != nil {
				return err
			}
		}
	}
	return nil
}

// updateVersionRecord 更新或创建版本记录
func (s *SyncService) updateVersionRecord(ctx context.Context, packageName string, remote *RemotePackage, result *model.ImportResult) error {
	// 查找已存在的版本记录
	existing, err := s.versionRepo.FindByName(ctx, packageName)
	if err != nil {
		return err
	}

	now := time.Now()

	// 从导入结果中获取 workflow ID（直接使用 ImportDetail.ID）
	var workflowID string
	for _, detail := range result.Details {
		// workflow 可能是 success 或 skipped（已存在），都需要获取 ID
		if detail.AssetType == "workflow" && (detail.Status == "success" || detail.Status == "skipped") && detail.ID != "" {
			workflowID = detail.ID
			break
		}
	}

	if existing != nil {
		// 更新已存在的记录
		existing.Version = remote.Version
		existing.Description = remote.Description
		existing.LastSyncedAt = &now
		if workflowID != "" {
			existing.WorkflowID, _ = uuid.Parse(workflowID)
		}
		return s.versionRepo.Update(ctx, existing)
	}

	// 创建新记录
	if workflowID == "" {
		// 没有找到 workflow ID，记录警告但不创建版本记录
		s.logger.Warn("cannot create version record without workflow ID",
			zap.String("package", packageName))
		return nil
	}

	wfUUID, err := uuid.Parse(workflowID)
	if err != nil {
		return fmt.Errorf("parse workflow ID: %w", err)
	}

	newVersion := &model.TeamPackageVersion{
		ID:           uuid.New(),
		WorkflowID:   wfUUID,
		Name:         packageName,
		Version:      remote.Version,
		Description:  remote.Description,
		LastSyncedAt: &now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	return s.versionRepo.Create(ctx, newVersion)
}

// parseMarketplaceJSON 解析 marketplace.json 文件
func (s *SyncService) parseMarketplaceJSON(cloneDir string) (*model.Marketplace, error) {
	marketplaceFile := filepath.Join(cloneDir, "marketplace.json")

	data, err := os.ReadFile(marketplaceFile)
	if err != nil {
		return nil, fmt.Errorf("read marketplace.json: %w", err)
	}

	var marketplace model.Marketplace
	if err := json.Unmarshal(data, &marketplace); err != nil {
		return nil, fmt.Errorf("parse marketplace.json: %w", err)
	}

	return &marketplace, nil
}

// PreviewPackage 预览团队包内容（不实际导入，必须指定 marketId）
func (s *SyncService) PreviewPackage(ctx context.Context, packageName string, marketId string) (*PreviewPackageResponse, error) {
	if marketId == "" {
		return nil, fmt.Errorf("marketId is required")
	}
	if s.marketSvc == nil {
		return nil, fmt.Errorf("market service not available")
	}

	var remotePkg *RemotePackage
	var packageCloneDir string

	marketUUID, err := uuid.Parse(marketId)
	if err != nil {
		return nil, fmt.Errorf("invalid market id: %w", err)
	}

	market, err := s.marketSvc.GetMarketByID(ctx, marketUUID)
	if err != nil {
		return nil, fmt.Errorf("get market: %w", err)
	}
	if market == nil {
		return nil, fmt.Errorf("market not found: %s", marketId)
	}

	// 克隆市场仓库获取 marketplace.json
	marketCloneDir, err := s.gitClient.CloneFromURL(ctx, market.URL, market.Branch)
	if err != nil {
		return nil, fmt.Errorf("clone market repo: %w", err)
	}
	defer s.gitClient.Cleanup(marketCloneDir)

	// 解析 marketplace.json
	marketplace, err := s.parseMarketplaceJSON(marketCloneDir)
	if err != nil {
		return nil, fmt.Errorf("parse marketplace.json: %w", err)
	}

	// 查找指定的包
	for _, plugin := range marketplace.Plugins {
		if strings.ToLower(plugin.Category) == "team" && plugin.Name == packageName {
			remotePkg = &RemotePackage{
				Name:        plugin.Name,
				Version:     plugin.Version,
				Description: plugin.Description,
				Path:        "",
				Repository:  plugin.Repository,
				Source:      plugin.Source,
			}
			break
		}
	}

	if remotePkg == nil {
		return nil, fmt.Errorf("package not found in marketplace: %s", packageName)
	}

	// 克隆包仓库
	packageCloneDir, err = s.gitClient.CloneFromURL(ctx, remotePkg.Repository, "master")
	if err != nil {
		return nil, fmt.Errorf("clone package repo: %w", err)
	}
	defer s.gitClient.Cleanup(packageCloneDir)

	// 设置包的实际路径
	remotePkg.Path = filepath.Join(packageCloneDir, remotePkg.Source)

	// 解析包的 manifest.json
	manifestPath := filepath.Join(remotePkg.Path, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest.json: %w", err)
	}

	var manifest model.TeamPackageManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest.json: %w", err)
	}

	// 构建预览响应
	response := &PreviewPackageResponse{
		PackageName: remotePkg.Name,
		Version:     remotePkg.Version,
		Description: remotePkg.Description,
		Workflow: PreviewWorkflowInfo{
			Name:        manifest.Workflow.Name,
			Description: manifest.Workflow.Description,
		},
		Roles:  []PreviewRoleInfo{},
		Assets: PreviewAssetsInfo{},
	}

	// 收集角色信息
	for _, role := range manifest.Roles {
		roleInfo := PreviewRoleInfo{
			Name:        role.Name,
			Role:        role.Role,
			Description: role.Description,
			Assets:      []string{},
		}

		// 收集角色绑定的资产名称
		for _, skill := range role.Bindings.Skills {
			roleInfo.Assets = append(roleInfo.Assets, fmt.Sprintf("Skill: %s", skill))
		}
		for _, cmd := range role.Bindings.Commands {
			roleInfo.Assets = append(roleInfo.Assets, fmt.Sprintf("Command: %s", cmd))
		}
		for _, sub := range role.Bindings.Subagents {
			roleInfo.Assets = append(roleInfo.Assets, fmt.Sprintf("Subagent: %s", sub))
		}
		for _, rule := range role.Bindings.Rules {
			roleInfo.Assets = append(roleInfo.Assets, fmt.Sprintf("Rule: %s", rule))
		}
		for _, settings := range role.Bindings.Settings {
			roleInfo.Assets = append(roleInfo.Assets, fmt.Sprintf("Settings: %s", settings))
		}

		response.Roles = append(response.Roles, roleInfo)
	}

	// 收集资产信息
	for _, skill := range manifest.Assets.Skills {
		response.Assets.Skills = append(response.Assets.Skills, PreviewAssetInfo{
			Name:        skill.Name,
			Description: skill.Description,
		})
	}
	for _, cmd := range manifest.Assets.Commands {
		response.Assets.Commands = append(response.Assets.Commands, PreviewAssetInfo{
			Name:        cmd.Name,
			Description: cmd.Description,
		})
	}
	for _, sub := range manifest.Assets.Subagents {
		response.Assets.Subagents = append(response.Assets.Subagents, PreviewAssetInfo{
			Name:        sub.Name,
			Description: sub.Description,
		})
	}
	for _, rule := range manifest.Assets.Rules {
		response.Assets.Rules = append(response.Assets.Rules, PreviewAssetInfo{
			Name:        rule.Name,
			Description: rule.Description,
		})
	}
	for _, settings := range manifest.Assets.Settings {
		response.Assets.Settings = append(response.Assets.Settings, PreviewAssetInfo{
			Name:        settings.Name,
			Description: settings.Description,
		})
	}

	s.logger.Info("团队包预览完成",
		zap.String("package", packageName),
		zap.Int("roles", len(response.Roles)),
		zap.Int("skills", len(response.Assets.Skills)))

	return response, nil
}