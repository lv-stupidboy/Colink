package market

import (
	"regexp"
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
	Name          *string `json:"name"`
	URL           *string `json:"url"`
	Branch        *string `json:"branch"`
	Enabled       *bool   `json:"enabled"`
	AutoUpdate    *bool   `json:"autoUpdate"`
	CheckInterval *string `json:"checkInterval"`
}

// MarketSyncResult 市场同步结果
type MarketSyncResult struct {
	Market      model.Market      `json:"market"`
	Marketplace model.Marketplace `json:"marketplace"`
}

// ValidateCron 校验 cron 表达式（5 位标准格式）
func ValidateCron(cron string) bool {
	if cron == "" {
		return true
	}
	// 5位标准 cron: 分 时 日 月 周
	pattern := `^(\*|([0-5]?[0-9])|(\*/[0-9]+)|([0-5]?[0-9]-[0-5]?[0-9])|([0-5]?[0-9](,[0-5]?[0-9])*)) ` +
		`(\*|([0-9]|1[0-9]|2[0-3])|(\*/[0-9]+)|([0-9]-[0-9]|1[0-9]-1[0-9]|2[0-3]-[0-9])|([0-9](,[0-9])*)) ` +
		`(\*|([1-9]|[12][0-9]|3[01])|(\*/[0-9]+)|([1-9]-[1-9]|[12][0-9]-[0-9]|3[01]-[0-9])) ` +
		`(\*|([1-9]|1[0-2])|(\*/[0-9]+)|([1-9]-[1-9]|1[0-2]-[0-9])) ` +
		`(\*|([0-6])|(\*/[0-9]+)|([0-6]-[0-6]))$`
	matched, _ := regexp.MatchString(pattern, cron)
	return matched
}