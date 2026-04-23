package market

import (
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
)

// TeamPackagesCache 团队包列表缓存（完整聚合结果）
type TeamPackagesCache struct {
	data      []model.MarketPackage
	expiredAt time.Time
}

// MarketCache 市场数据缓存
type MarketCache struct {
	teamPackages *TeamPackagesCache
	ttl          time.Duration
	mutex        sync.RWMutex
}

// NewMarketCache 创建缓存实例
func NewMarketCache(ttl time.Duration) *MarketCache {
	return &MarketCache{
		ttl: ttl,
	}
}

// GetTeamPackages 获取团队包缓存（有效期内）
func (c *MarketCache) GetTeamPackages() []model.MarketPackage {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.teamPackages == nil || time.Now().After(c.teamPackages.expiredAt) {
		return nil
	}
	return c.teamPackages.data
}

// GetExpiredTeamPackages 获取过期缓存（用于降级）
func (c *MarketCache) GetExpiredTeamPackages() []model.MarketPackage {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.teamPackages == nil {
		return nil
	}
	return c.teamPackages.data
}

// SetTeamPackages 设置团队包缓存
func (c *MarketCache) SetTeamPackages(packages []model.MarketPackage) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.teamPackages = &TeamPackagesCache{
		data:      packages,
		expiredAt: time.Now().Add(c.ttl),
	}
}

// InvalidateTeamPackages 清除团队包缓存
func (c *MarketCache) InvalidateTeamPackages() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.teamPackages = nil
}