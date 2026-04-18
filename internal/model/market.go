// 文件路径: isdp/internal/model/market.go
package model

import (
	"time"

	"github.com/google/uuid"
)

// Market 市场配置
type Market struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	URL           string     `json:"url"`
	Branch        string     `json:"branch"`
	Enabled       bool       `json:"enabled"`
	AutoUpdate    bool       `json:"autoUpdate"`
	CheckInterval string     `json:"checkInterval"`
	LastSyncedAt  *time.Time `json:"lastSyncedAt"`
	LastError     string     `json:"lastError"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

func (m *Market) TableName() string {
	return "markets"
}

// Marketplace marketplace.json 结构
type Marketplace struct {
	Name        string  `json:"name"`
	Version     string  `json:"version"`
	Description string  `json:"description"`
	Owner       Owner   `json:"owner"`
	Plugins     []Plugin `json:"plugins"`
}

// Owner 市场所有者
type Owner struct {
	Name string `json:"name"`
}

// Plugin 市场插件/包
type Plugin struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Repository  string `json:"repository"` // 包所在仓库
	Source      string `json:"source"`     // 相对路径
	Category    string `json:"category"`   // team/skill/command等
}

// MarketPackage 市场团队包（用于前端展示）
type MarketPackage struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Description  string `json:"description"`
	MarketID     string `json:"marketId"`
	MarketName   string `json:"marketName"`
	Repository   string `json:"repository"`
	Source       string `json:"source"`
	LocalVersion string `json:"localVersion"` // 本地版本
	LocalStatus  string `json:"localStatus"`  // new/update/latest
}