package market

import (
	"github.com/anthropic/isdp/internal/model"
)

// AddMarketRequest 添加市场请求
type AddMarketRequest struct {
	Name   string `json:"name" binding:"required"`
	URL    string `json:"url" binding:"required"`
	Branch string `json:"branch"`
}

// UpdateMarketRequest 更新市场请求
type UpdateMarketRequest struct {
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	AutoUpdate    bool   `json:"autoUpdate"`
	CheckInterval string `json:"checkInterval"`
}

// MarketSyncResult 市场同步结果
type MarketSyncResult struct {
	Market      model.Market      `json:"market"`
	Marketplace model.Marketplace `json:"marketplace"`
}