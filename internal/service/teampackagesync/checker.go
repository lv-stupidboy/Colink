package teampackagesync

import (
	"context"
	"strings"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"go.uber.org/zap"
)

// SyncChecker 定时检查团队包更新的后台任务
type SyncChecker struct {
	syncSvc   *SyncService
	marketRepo *repo.MarketRepository
	versionRepo *repo.TeamPackageVersionRepository
	interval  time.Duration
	stopChan  chan struct{}
	logger    *zap.Logger
}

// NewSyncChecker 创建同步检查器
func NewSyncChecker(
	syncSvc *SyncService,
	marketRepo *repo.MarketRepository,
	versionRepo *repo.TeamPackageVersionRepository,
	interval time.Duration,
	logger *zap.Logger,
) *SyncChecker {
	return &SyncChecker{
		syncSvc:    syncSvc,
		marketRepo: marketRepo,
		versionRepo: versionRepo,
		interval:   interval,
		stopChan:   make(chan struct{}),
		logger:     logger,
	}
}

// Start 启动定时检查循环
func (c *SyncChecker) Start() {
	c.check() // 启动时立即检查一次
	go c.runLoop()
}

// Stop 停止检查器
func (c *SyncChecker) Stop() {
	close(c.stopChan)
}

// runLoop 运行定时检查循环
func (c *SyncChecker) runLoop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.check()
		case <-c.stopChan:
			c.logger.Info("sync checker stopped")
			return
		}
	}
}

// check 执行更新检查并自动导入
func (c *SyncChecker) check() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c.logger.Info("checking for team package auto-update")

	// 获取所有启用自动更新的市场
	markets, err := c.marketRepo.List(ctx)
	if err != nil {
		c.logger.Error("failed to list markets", zap.Error(err))
		return
	}

	// 获取本地版本列表
	localVersions, err := c.versionRepo.ListAll(ctx)
	if err != nil {
		c.logger.Warn("failed to get local versions", zap.Error(err))
		localVersions = []model.TeamPackageVersion{}
	}

	// 构建本地版本映射
	localMap := make(map[string]model.TeamPackageVersion)
	for _, v := range localVersions {
		localMap[v.Name] = v
	}

	for _, market := range markets {
		// 只处理启用且开启自动更新的市场
		if !market.Enabled || !market.AutoUpdate {
			continue
		}

		c.logger.Info("checking market for auto-update",
			zap.String("market", market.Name),
			zap.String("marketId", market.ID.String()))

		// 刷新市场获取最新包列表
		marketplace, err := c.syncSvc.marketSvc.RefreshMarket(ctx, market.ID)
		if err != nil {
			c.logger.Warn("failed to refresh market for auto-update",
				zap.String("market", market.Name),
				zap.Error(err))
			continue
		}

		// 检查每个团队包是否需要更新
		for _, plugin := range marketplace.Plugins {
			// 只处理 category=team 的包
			if strings.ToLower(plugin.Category) != "team" {
				continue
			}

			localVer, exists := localMap[plugin.Name]
			needUpdate := false

			if exists {
				// 本地存在，检查版本是否需要更新
				if CompareVersions(localVer.Version, plugin.Version) < 0 {
					needUpdate = true
				}
			} else {
				// 本地不存在，视为新包（根据配置决定是否自动导入新包）
				// 目前只自动更新已有包，不自动导入新包
				c.logger.Info("new package found in market (not auto-imported)",
					zap.String("package", plugin.Name),
					zap.String("market", market.Name))
				continue
			}

			if needUpdate {
				c.logger.Info("auto-updating package",
					zap.String("package", plugin.Name),
					zap.String("localVersion", localVer.Version),
					zap.String("remoteVersion", plugin.Version),
					zap.String("market", market.Name))

				// 执行同步导入
				_, err := c.syncSvc.SyncPackage(ctx, plugin.Name, market.ID.String(), nil)
				if err != nil {
					c.logger.Error("failed to auto-update package",
						zap.String("package", plugin.Name),
						zap.String("market", market.Name),
						zap.Error(err))
				} else {
					c.logger.Info("package auto-updated successfully",
						zap.String("package", plugin.Name),
						zap.String("market", market.Name),
						zap.String("version", plugin.Version))
				}
			}
		}
	}
}

// getPackageNames 从更新信息列表中提取包名
func getPackageNames(updates []PackageUpdateInfo) []string {
	names := make([]string, len(updates))
	for i, u := range updates {
		names[i] = u.Local.Name
	}
	return names
}