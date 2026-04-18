package teampackagesync

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// SyncChecker 定时检查团队包更新的后台任务
type SyncChecker struct {
	syncSvc  *SyncService
	interval time.Duration
	stopChan chan struct{}
	logger   *zap.Logger
}

// NewSyncChecker 创建同步检查器
func NewSyncChecker(syncSvc *SyncService, interval time.Duration, logger *zap.Logger) *SyncChecker {
	return &SyncChecker{
		syncSvc:  syncSvc,
		interval: interval,
		stopChan: make(chan struct{}),
		logger:   logger,
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

// check 执行更新检查
func (c *SyncChecker) check() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	c.logger.Info("checking for team package updates")

	result, err := c.syncSvc.CheckUpdates(ctx)
	if err != nil {
		c.logger.Error("check updates failed", zap.Error(err))
		return
	}

	if len(result.NeedUpdate) > 0 {
		c.logger.Info("packages need update",
			zap.Int("count", len(result.NeedUpdate)),
			zap.Strings("packages", getPackageNames(result.NeedUpdate)))
	}

	if len(result.NewPackages) > 0 {
		c.logger.Info("new packages available",
			zap.Int("count", len(result.NewPackages)))
	}

	if len(result.Removed) > 0 {
		c.logger.Warn("packages removed from remote",
			zap.Int("count", len(result.Removed)),
			zap.Strings("packages", result.Removed))
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