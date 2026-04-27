package market

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/pkg/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service 市场管理服务
type Service struct {
	marketRepo  *repo.MarketRepository
	versionRepo *repo.TeamPackageVersionRepository
	gitClient   *GitClient
	tempBase    string
	logger      *zap.Logger
	cache       *MarketCache // 新增：缓存模块
}

// NewService 创建市场服务
func NewService(
	marketRepo *repo.MarketRepository,
	versionRepo *repo.TeamPackageVersionRepository,
	tempBase string,
	logger *zap.Logger,
) *Service {
	return &Service{
		marketRepo:  marketRepo,
		versionRepo: versionRepo,
		gitClient:   NewGitClient(logger),
		tempBase:    tempBase,
		logger:      logger,
		cache:       NewMarketCache(5 * time.Minute), // 5分钟缓存
	}
}

// ListMarkets 列出所有市场
func (s *Service) ListMarkets(ctx context.Context) ([]model.Market, error) {
	return s.marketRepo.List(ctx)
}

// AddMarket 添加市场
func (s *Service) AddMarket(ctx context.Context, req AddMarketRequest) (*model.Market, error) {
	if req.Branch == "" {
		req.Branch = "main"
	}

	market := &model.Market{
		Name:          req.Name,
		URL:           req.URL,
		Branch:        req.Branch,
		Enabled:       true,
		AutoUpdate:    false,
		CheckInterval: "24h",
	}

	if err := s.marketRepo.Create(ctx, market); err != nil {
		return nil, err
	}

	return market, nil
}

// UpdateMarket 更新市场配置
func (s *Service) UpdateMarket(ctx context.Context, id uuid.UUID, req UpdateMarketRequest) (*model.Market, error) {
	market, err := s.marketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.WrapError(err)
	}
	if market == nil {
		return nil, errors.WithDetail(errors.ErrRepoNotFound, "market not found: "+id.String())
	}

	if req.Name != nil && *req.Name != "" {
		market.Name = *req.Name
	}
	if req.URL != nil && *req.URL != "" {
		market.URL = *req.URL
	}
	if req.Branch != nil && *req.Branch != "" {
		market.Branch = *req.Branch
	}
	if req.Enabled != nil {
		market.Enabled = *req.Enabled
	}
	if req.AutoUpdate != nil {
		market.AutoUpdate = *req.AutoUpdate
	}
	if req.CheckInterval != nil && *req.CheckInterval != "" {
		if !ValidateCron(*req.CheckInterval) {
			return nil, errors.NewInvalidParam(
				fmt.Sprintf("invalid cron expression: %s (format: min hour day month weekday)", *req.CheckInterval))
		}
		market.CheckInterval = *req.CheckInterval
	}

	if err := s.marketRepo.Update(ctx, market); err != nil {
		return nil, errors.WrapError(err)
	}

	return market, nil
}

// DeleteMarket 删除市场
func (s *Service) DeleteMarket(ctx context.Context, id uuid.UUID) error {
	return s.marketRepo.Delete(ctx, id)
}

// GetMarketByID 根据ID获取市场
func (s *Service) GetMarketByID(ctx context.Context, id uuid.UUID) (*model.Market, error) {
	return s.marketRepo.FindByID(ctx, id)
}

// RefreshMarket 刷新市场（重新克隆并解析 marketplace.json）
func (s *Service) RefreshMarket(ctx context.Context, id uuid.UUID) (*model.Marketplace, error) {
	market, err := s.marketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.WrapError(err)
	}
	if market == nil {
		return nil, errors.WithDetail(errors.ErrRepoNotFound, "market not found: "+id.String())
	}

	cloneDir, err := s.gitClient.Clone(ctx, market.URL, market.Branch, s.tempBase)
	if err != nil {
		// err 已是 AppError
		s.marketRepo.UpdateSyncStatus(ctx, id, nil, err.Error())
		return nil, err
	}
	defer s.gitClient.Cleanup(cloneDir)

	marketplace, err := s.gitClient.ParseMarketplaceJSON(cloneDir)
	if err != nil {
		// err 已是 AppError
		s.marketRepo.UpdateSyncStatus(ctx, id, nil, err.Error())
		return nil, err
	}


	// 更新同步状态
	now := time.Now()
	s.marketRepo.UpdateSyncStatus(ctx, id, &now, "")

	s.logger.Info("market refreshed successfully",
		zap.String("market", market.Name),
		zap.Int("plugins", len(marketplace.Plugins)),
	)

	return marketplace, nil
}

// GetTeamPackages 获取所有市场的团队包列表
func (s *Service) GetTeamPackages(ctx context.Context, forceRefresh bool) ([]model.MarketPackage, error) {
	// 非强制刷新时，先尝试从缓存读取
	if !forceRefresh {
		cached := s.cache.GetTeamPackages()
		if cached != nil && len(cached) > 0 {
			return cached, nil
		}
	}

	markets, err := s.marketRepo.List(ctx)
	if err != nil {
		// 降级：尝试使用过期缓存
		cached := s.cache.GetExpiredTeamPackages()
		if cached != nil && len(cached) > 0 {
			s.logger.Warn("using expired cache due to market list failure", zap.Error(err))
			return cached, nil
		}
		return nil, err
	}

	// 获取本地版本列表
	localVersions, err := s.versionRepo.ListAll(ctx)
	if err != nil {
		s.logger.Warn("failed to get local versions", zap.Error(err))
		localVersions = []model.TeamPackageVersion{}
	}

	// 构建本地版本映射（包含版本和最后导入时间）
	localMap := make(map[string]model.TeamPackageVersion)
	for _, v := range localVersions {
		localMap[v.Name] = v
	}

	packages := []model.MarketPackage{}
	hasErrors := false

	for _, market := range markets {
		if !market.Enabled {
			continue
		}

		// 每次都刷新市场数据，不使用缓存
		marketplace, err := s.RefreshMarket(ctx, market.ID)
		if err != nil {
			s.logger.Warn("failed to refresh market",
				zap.String("market", market.Name),
				zap.Error(err),
			)
			hasErrors = true
			continue
		}

		// 只筛选 category=team 的包
		for _, plugin := range marketplace.Plugins {
			if strings.ToLower(plugin.Category) != "team" {
				continue
			}

			pkg := model.MarketPackage{
				Name:        plugin.Name,
				Version:     plugin.Version,
				Description: plugin.Description,
				MarketID:    market.ID.String(),
				MarketName:  market.Name,
				Repository:  plugin.Repository,
				Source:      plugin.Source,
			}

			// 对比本地版本
			if localVer, exists := localMap[plugin.Name]; exists {
				pkg.LocalVersion = localVer.Version
				pkg.LastImportedAt = localVer.LastSyncedAt
				if compareVersions(localVer.Version, plugin.Version) < 0 {
					pkg.LocalStatus = "update"
				} else {
					pkg.LocalStatus = "latest"
				}
			} else {
				pkg.LocalStatus = "new"
			}

			packages = append(packages, pkg)
		}
	}

	// 如果有错误且没有获取到任何数据，尝试使用过期缓存
	if hasErrors && len(packages) == 0 {
		cached := s.cache.GetExpiredTeamPackages()
		if cached != nil && len(cached) > 0 {
			s.logger.Warn("using expired cache due to all markets failed")
			return cached, nil
		}
	}

	// 更新缓存
	if len(packages) > 0 {
		s.cache.SetTeamPackages(packages)
	}

	return packages, nil
}

// compareVersions 比较版本号（语义化版本）
func compareVersions(v1, v2 string) int {
	// 解析版本号：major.minor.patch
	v1Parts := parseVersionParts(v1)
	v2Parts := parseVersionParts(v2)

	for i := 0; i < 3; i++ {
		if v1Parts[i] < v2Parts[i] {
			return -1
		}
		if v1Parts[i] > v2Parts[i] {
			return 1
		}
	}
	return 0
}

// parseVersionParts 解析版本号为 [major, minor, patch]
func parseVersionParts(v string) [3]int {
	parts := strings.SplitN(v, ".", 3)
	result := [3]int{}
	for i, p := range parts {
		if i >= 3 {
			break
		}
		// 提取数字部分（处理类似 "1.0.0-beta" 的情况）
		numStr := p
		for j, c := range p {
			if c < '0' || c > '9' {
				numStr = p[:j]
				break
			}
		}
		if numStr != "" {
			result[i], _ = strconv.Atoi(numStr)
		}
	}
	return result
}

// StartAutoUpdateChecker 启动自动更新检查器
func (s *Service) StartAutoUpdateChecker(ctx context.Context) {
	markets, err := s.marketRepo.List(ctx)
	if err != nil {
		s.logger.Error("failed to list markets for auto update", zap.Error(err))
		return
	}

	for _, market := range markets {
		if market.AutoUpdate && market.Enabled {
			s.scheduleAutoUpdate(ctx, market)
		}
	}
}

// scheduleAutoUpdate 为单个市场调度自动更新
func (s *Service) scheduleAutoUpdate(ctx context.Context, market model.Market) {
	duration, err := time.ParseDuration(market.CheckInterval)
	if err != nil {
		duration = 24 * time.Hour
	}

	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.logger.Info("auto refreshing market", zap.String("market", market.Name))
				_, err := s.RefreshMarket(ctx, market.ID)
				if err != nil {
					s.logger.Warn("auto refresh failed", zap.String("market", market.Name), zap.Error(err))
				}
			}
		}
	}()

	s.logger.Info("auto update scheduled",
		zap.String("market", market.Name),
		zap.Duration("interval", duration),
	)
}

// RefreshPackages 手动刷新所有市场缓存
func (s *Service) RefreshPackages(ctx context.Context) error {
	s.cache.InvalidateTeamPackages()

	// 强制刷新，获取最新数据
	_, err := s.GetTeamPackages(ctx, true)
	return err
}