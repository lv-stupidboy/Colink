package teampackagesync

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
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
	config         config.TeamPackageSyncConfig
	gitClient      *GitClient
	logger         *zap.Logger
}

// NewSyncService 创建同步服务
func NewSyncService(
	versionRepo *repo.TeamPackageVersionRepository,
	workflowRepo *repo.WorkflowTemplateRepository,
	teamPackageSvc *teampackage.Service,
	cfg config.TeamPackageSyncConfig,
	logger *zap.Logger,
) *SyncService {
	return &SyncService{
		versionRepo:    versionRepo,
		workflowRepo:   workflowRepo,
		teamPackageSvc: teamPackageSvc,
		config:         cfg,
		gitClient:      NewGitClient(cfg, logger),
		logger:         logger,
	}
}

// GetRemotePackages 获取远程团队包列表
func (s *SyncService) GetRemotePackages(ctx context.Context) (*RemotePackageList, error) {
	cloneDir, err := s.gitClient.Clone(ctx)
	if err != nil {
		return nil, fmt.Errorf("clone repo: %w", err)
	}
	defer s.gitClient.Cleanup(cloneDir)

	list, err := s.gitClient.GetPackageList(cloneDir)
	if err != nil {
		return nil, fmt.Errorf("get package list: %w", err)
	}

	return list, nil
}

// CheckUpdates 检查本地版本与远程版本的差异
func (s *SyncService) CheckUpdates(ctx context.Context) (*UpdateCheckResult, error) {
	// 获取本地版本列表
	localVersions, err := s.versionRepo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get local versions: %w", err)
	}

	// 获取远程包列表
	remoteList, err := s.GetRemotePackages(ctx)
	if err != nil {
		return nil, fmt.Errorf("get remote packages: %w", err)
	}

	result := &UpdateCheckResult{
		NeedUpdate:  []PackageUpdateInfo{},
		NewPackages: []RemotePackage{},
		Removed:     []string{},
	}

	// 构建本地版本映射
	localMap := make(map[string]model.TeamPackageVersion)
	for _, v := range localVersions {
		localMap[v.Name] = v
	}

	// 检查每个远程包
	for _, category := range remoteList.Categories {
		for _, remote := range category.Packages {
			if local, exists := localMap[remote.Name]; exists {
				// 比较版本
				if CompareVersions(local.Version, remote.Version) < 0 {
					result.NeedUpdate = append(result.NeedUpdate, PackageUpdateInfo{
						Local:  local,
						Remote: remote,
					})
				}
				// 从映射中删除，用于追踪已移除的包
				delete(localMap, remote.Name)
			} else {
				// 新包，本地不存在
				result.NewPackages = append(result.NewPackages, remote)
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

// SyncPackage 同步指定的团队包
func (s *SyncService) SyncPackage(ctx context.Context, packageName string, confirm *model.TeamPackageImportConfirm) (*model.ImportResult, error) {
	// 克隆仓库
	cloneDir, err := s.gitClient.Clone(ctx)
	if err != nil {
		return nil, fmt.Errorf("clone repo: %w", err)
	}
	defer s.gitClient.Cleanup(cloneDir)

	// 获取远程包列表
	remoteList, err := s.gitClient.GetPackageList(cloneDir)
	if err != nil {
		return nil, fmt.Errorf("get package list: %w", err)
	}

	// 查找指定的包
	var remotePkg *RemotePackage
	for _, category := range remoteList.Categories {
		for _, pkg := range category.Packages {
			if pkg.Name == packageName {
				remotePkg = &pkg
				break
			}
		}
		if remotePkg != nil {
			break
		}
	}

	if remotePkg == nil {
		return nil, fmt.Errorf("package not found: %s", packageName)
	}

	// 将目录转换为 zip 数据
	zipData, err := s.createZipFromDir(remotePkg.Path)
	if err != nil {
		return nil, fmt.Errorf("create zip: %w", err)
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

	// 从导入结果中获取 workflow ID
	var workflowID string
	for _, detail := range result.Details {
		if detail.AssetType == "workflow" && detail.Status == "success" {
			// 尝试从 workflowRepo 获取刚导入的 workflow ID
			workflows, err := s.workflowRepo.FindAll(ctx)
			if err == nil {
				for _, wf := range workflows {
					if wf.Name == packageName {
						workflowID = wf.ID.String()
						break
					}
				}
			}
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