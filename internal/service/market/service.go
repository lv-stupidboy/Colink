package market

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service 市场管理服务
type Service struct {
	marketRepo  *repo.MarketRepository
	versionRepo *repo.TeamPackageVersionRepository
	gitClient   *GitClient
	tempBase    string
	cache       map[uuid.UUID]*model.Marketplace
	cacheMutex  sync.RWMutex
	logger      *zap.Logger
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
		cache:       make(map[uuid.UUID]*model.Marketplace),
		logger:      logger,
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
		return nil, err
	}
	if market == nil {
		return nil, fmt.Errorf("market not found")
	}

	if req.Name != "" {
		market.Name = req.Name
	}
	market.Enabled = req.Enabled
	market.AutoUpdate = req.AutoUpdate
	if req.CheckInterval != "" {
		market.CheckInterval = req.CheckInterval
	}

	if err := s.marketRepo.Update(ctx, market); err != nil {
		return nil, err
	}

	return market, nil
}

// DeleteMarket 删除市场
func (s *Service) DeleteMarket(ctx context.Context, id uuid.UUID) error {
	return s.marketRepo.Delete(ctx, id)
}

// RefreshMarket 刷新市场（重新克隆并解析 marketplace.json）
func (s *Service) RefreshMarket(ctx context.Context, id uuid.UUID) (*model.Marketplace, error) {
	market, err := s.marketRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if market == nil {
		return nil, fmt.Errorf("market not found")
	}

	cloneDir, err := s.gitClient.Clone(ctx, market.URL, market.Branch, s.tempBase)
	if err != nil {
		s.marketRepo.UpdateSyncStatus(ctx, id, nil, err.Error())
		return nil, err
	}
	defer s.gitClient.Cleanup(cloneDir)

	marketplace, err := s.gitClient.ParseMarketplaceJSON(cloneDir)
	if err != nil {
		s.marketRepo.UpdateSyncStatus(ctx, id, nil, err.Error())
		return nil, err
	}

	// 更新缓存
	s.cacheMutex.Lock()
	s.cache[id] = marketplace
	s.cacheMutex.Unlock()

	// 更新同步状态
	now := time.Now()
	s.marketRepo.UpdateSyncStatus(ctx, id, &now, "")

	s.logger.Info("market refreshed successfully",
		zap.String("market", market.Name),
		zap.Int("plugins", len(marketplace.Plugins)),
	)

	return marketplace, nil
}

// GetCachedMarketplace 获取缓存的市场数据
func (s *Service) GetCachedMarketplace(id uuid.UUID) *model.Marketplace {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()
	return s.cache[id]
}

// GetTeamPackages 获取所有市场的团队包列表
func (s *Service) GetTeamPackages(ctx context.Context) ([]model.MarketPackage, error) {
	markets, err := s.marketRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	// 获取本地版本列表
	localVersions, err := s.versionRepo.ListAll(ctx)
	if err != nil {
		s.logger.Warn("failed to get local versions", zap.Error(err))
		localVersions = []model.TeamPackageVersion{}
	}

	// 构建本地版本映射
	localMap := make(map[string]string)
	for _, v := range localVersions {
		localMap[v.Name] = v.Version
	}

	packages := []model.MarketPackage{}

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
				pkg.LocalVersion = localVer
				if compareVersions(localVer, plugin.Version) < 0 {
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