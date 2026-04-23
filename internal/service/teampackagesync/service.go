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

// SyncService 团队包同步服务（扩展：支持冲突检测）
type SyncService struct {
	versionRepo    *repo.TeamPackageVersionRepository
	workflowRepo   *repo.WorkflowTemplateRepository
	agentRepo      *repo.AgentConfigRepository      // 新增：用于检测 Role 冲突
	skillRepo      *repo.SkillRepository            // 新增：用于检测 Skill 冲突
	commandRepo    *repo.CommandRepository          // 新增：用于检测 Command 冲突
	subagentRepo   *repo.SubagentRepository         // 新增：用于检测 Subagent 冲突
	ruleRepo       *repo.RuleRepository             // 新增：用于检测 Rule 冲突
	settingsRepo   *repo.SettingsRepository         // 新增：用于检测 Settings 冲突
	teamPackageSvc *teampackage.Service
	marketSvc      *market.Service // 市场服务
	config         config.TeamPackageSyncConfig
	gitClient      *GitClient
	logger         *zap.Logger
}

// NewSyncService 创建同步服务（扩展：支持冲突检测）
func NewSyncService(
	versionRepo *repo.TeamPackageVersionRepository,
	workflowRepo *repo.WorkflowTemplateRepository,
	agentRepo *repo.AgentConfigRepository,      // 新增
	skillRepo *repo.SkillRepository,            // 新增
	commandRepo *repo.CommandRepository,        // 新增
	subagentRepo *repo.SubagentRepository,      // 新增
	ruleRepo *repo.RuleRepository,              // 新增
	settingsRepo *repo.SettingsRepository,      // 新增
	teamPackageSvc *teampackage.Service,
	marketSvc *market.Service,
	cfg config.TeamPackageSyncConfig,
	basePath string,
	logger *zap.Logger,
) *SyncService {
	return &SyncService{
		versionRepo:    versionRepo,
		workflowRepo:   workflowRepo,
		agentRepo:      agentRepo,
		skillRepo:      skillRepo,
		commandRepo:    commandRepo,
		subagentRepo:   subagentRepo,
		ruleRepo:       ruleRepo,
		settingsRepo:   settingsRepo,
		teamPackageSvc: teamPackageSvc,
		marketSvc:      marketSvc,
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
			Exists:      false, // 默认值，后续检测
		},
		Roles:         []PreviewRoleInfo{},
		Assets: PreviewAssetsInfo{
			Skills:    []PreviewAssetInfo{},
			Commands:  []PreviewAssetInfo{},
			Subagents: []PreviewAssetInfo{},
			Rules:     []PreviewAssetInfo{},
			Settings:  []PreviewAssetInfo{},
		},
		ConflictCount: 0, // 默认值，后续计算
	}

	// === 冲突检测逻辑 ===
	// 检查工作流是否已存在（按名称匹配）
	workflows, err := s.workflowRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取工作流列表失败: %w", err)
	}
	for _, wf := range workflows {
		if wf.Name == manifest.Workflow.Name {
			response.Workflow.Exists = true
			break
		}
	}

	// 收集角色信息并检测冲突
	for _, role := range manifest.Roles {
		roleInfo := PreviewRoleInfo{
			Name:        role.Name,
			Role:        role.Role,
			Description: role.Description,
			Assets:      []string{},
			Exists:      false, // 默认值，后续检测
		}

		// 检查角色是否已存在（按ID匹配）
		roleID, err := uuid.Parse(role.ID)
		if err == nil {
			existing, err := s.agentRepo.FindByID(ctx, roleID)
			if err == nil && existing != nil {
				roleInfo.Exists = true
			}
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

	// 收集资产信息并检测冲突
	for _, skill := range manifest.Assets.Skills {
		info := PreviewAssetInfo{
			Name:        skill.Name,
			Description: skill.Description,
			Exists:      false,
		}
		// 检查 Skill 是否已存在
		if s.skillRepo != nil {
			existing, err := s.skillRepo.FindByName(ctx, skill.Name)
			if err == nil && existing != nil {
				info.Exists = true
			}
		}
		response.Assets.Skills = append(response.Assets.Skills, info)
	}
	for _, cmd := range manifest.Assets.Commands {
		info := PreviewAssetInfo{
			Name:        cmd.Name,
			Description: cmd.Description,
			Exists:      false,
		}
		// 检查 Command 是否已存在
		if s.commandRepo != nil {
			existing, err := s.commandRepo.FindByName(ctx, cmd.Name)
			if err == nil && existing != nil {
				info.Exists = true
			}
		}
		response.Assets.Commands = append(response.Assets.Commands, info)
	}
	for _, sub := range manifest.Assets.Subagents {
		info := PreviewAssetInfo{
			Name:        sub.Name,
			Description: sub.Description,
			Exists:      false,
		}
		// 检查 Subagent 是否已存在
		if s.subagentRepo != nil {
			existing, err := s.subagentRepo.FindByName(ctx, sub.Name)
			if err == nil && existing != nil {
				info.Exists = true
			}
		}
		response.Assets.Subagents = append(response.Assets.Subagents, info)
	}
	for _, rule := range manifest.Assets.Rules {
		info := PreviewAssetInfo{
			Name:        rule.Name,
			Description: rule.Description,
			Exists:      false,
		}
		// 检查 Rule 是否已存在
		if s.ruleRepo != nil {
			existing, err := s.ruleRepo.FindByName(ctx, rule.Name)
			if err == nil && existing != nil {
				info.Exists = true
			}
		}
		response.Assets.Rules = append(response.Assets.Rules, info)
	}
	for _, settings := range manifest.Assets.Settings {
		info := PreviewAssetInfo{
			Name:        settings.Name,
			Description: settings.Description,
			Exists:      false,
		}
		// 检查 Settings 是否已存在
		if s.settingsRepo != nil {
			existing, err := s.settingsRepo.FindByName(ctx, settings.Name)
			if err == nil && existing != nil {
				info.Exists = true
			}
		}
		response.Assets.Settings = append(response.Assets.Settings, info)
	}

	// 计算冲突总数
	conflictCount := 0
	if response.Workflow.Exists {
		conflictCount++
	}
	for _, role := range response.Roles {
		if role.Exists {
			conflictCount++
		}
	}
	for _, skill := range response.Assets.Skills {
		if skill.Exists {
			conflictCount++
		}
	}
	for _, cmd := range response.Assets.Commands {
		if cmd.Exists {
			conflictCount++
		}
	}
	for _, sub := range response.Assets.Subagents {
		if sub.Exists {
			conflictCount++
		}
	}
	for _, rule := range response.Assets.Rules {
		if rule.Exists {
			conflictCount++
		}
	}
	for _, settings := range response.Assets.Settings {
		if settings.Exists {
			conflictCount++
		}
	}
	response.ConflictCount = conflictCount

	s.logger.Info("团队包预览完成",
		zap.String("package", packageName),
		zap.Int("roles", len(response.Roles)),
		zap.Int("skills", len(response.Assets.Skills)))

	return response, nil
}