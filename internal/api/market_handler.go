package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/service/market"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/anthropic/isdp/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MarketHandler 市场 API 处理器
type MarketHandler struct {
	marketSvc *market.Service
	cfg       *config.Config
	logger    *zap.Logger
}

// NewMarketHandler 创建 MarketHandler
func NewMarketHandler(marketSvc *market.Service, cfg *config.Config, logger *zap.Logger) *MarketHandler {
	return &MarketHandler{
		marketSvc: marketSvc,
		cfg:       cfg,
		logger:    logger,
	}
}

// RegisterRoutes 注册路由
func (h *MarketHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/markets")
	g.GET("", h.ListMarkets)
	g.GET("/default", h.GetDefaultMarketConfig)
	g.POST("", h.AddMarket)
	g.POST("/default", h.AddDefaultMarket)
	g.PUT("/:id", h.UpdateMarket)
	g.DELETE("/:id", h.DeleteMarket)
	g.POST("/:id/refresh", h.RefreshMarket)
	g.GET("/packages", h.GetTeamPackages)
	g.POST("/packages/refresh", h.RefreshPackages) // 新增
}

// ListMarkets 获取市场列表
func (h *MarketHandler) ListMarkets(c *gin.Context) {
	markets, err := h.marketSvc.ListMarkets(c.Request.Context())
	if err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("list markets failed",
			zap.String("code", string(appErr.Code)),
			zap.String("detail", appErr.Detail))
		statusCode := errors.ToHTTPStatus(appErr.Code)
		c.JSON(statusCode, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": markets, "total": len(markets)})
}

// GetDefaultMarketConfig 获取默认市场配置
func (h *MarketHandler) GetDefaultMarketConfig(c *gin.Context) {
	cfg := h.cfg.Market
	c.JSON(http.StatusOK, gin.H{
		"name":   cfg.Name,
		"url":    cfg.URL,
		"branch": cfg.Branch,
	})
}

// AddDefaultMarket 添加默认市场
func (h *MarketHandler) AddDefaultMarket(c *gin.Context) {
	cfg := h.cfg.Market
	if cfg.URL == "" {
		appErr := errors.NewInvalidParam("默认市场URL未配置")
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}

	req := market.AddMarketRequest{
		Name:   cfg.Name,
		URL:    cfg.URL,
		Branch: cfg.Branch,
	}

	// 如果名称为空，使用默认名称
	if req.Name == "" {
		req.Name = "Colink官方市场"
	}
	// 如果分支为空，使用默认分支
	if req.Branch == "" {
		req.Branch = "main"
	}

	m, err := h.marketSvc.AddMarket(c.Request.Context(), req)
	if err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("add default market failed", zap.String("detail", appErr.Detail))
		statusCode := errors.ToHTTPStatus(appErr.Code)
		c.JSON(statusCode, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, m)
}

// AddMarket 添加市场
func (h *MarketHandler) AddMarket(c *gin.Context) {
	var req market.AddMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewInvalidParam(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}

	m, err := h.marketSvc.AddMarket(c.Request.Context(), req)
	if err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("add market failed", zap.String("detail", appErr.Detail))
		statusCode := errors.ToHTTPStatus(appErr.Code)
		c.JSON(statusCode, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, m)
}

// UpdateMarket 更新市场
func (h *MarketHandler) UpdateMarket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		appErr := errors.NewInvalidParam("invalid market id")
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}

	var req market.UpdateMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewInvalidParam(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}

	m, err := h.marketSvc.UpdateMarket(c.Request.Context(), id, req)
	if err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("update market failed",
			zap.String("marketId", idStr),
			zap.String("code", string(appErr.Code)),
			zap.String("detail", appErr.Detail))
		statusCode := errors.ToHTTPStatus(appErr.Code)
		c.JSON(statusCode, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, m)
}

// DeleteMarket 删除市场
func (h *MarketHandler) DeleteMarket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		appErr := errors.NewInvalidParam("invalid market id")
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}

	if err := h.marketSvc.DeleteMarket(c.Request.Context(), id); err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("delete market failed", zap.String("detail", appErr.Detail))
		statusCode := errors.ToHTTPStatus(appErr.Code)
		c.JSON(statusCode, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "market deleted"})
}

// RefreshMarket 刷新市场
func (h *MarketHandler) RefreshMarket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		appErr := errors.NewInvalidParam("invalid market id")
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}

	marketplace, err := h.marketSvc.RefreshMarket(c.Request.Context(), id)
	if err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("refresh market failed",
			zap.String("marketId", idStr),
			zap.String("code", string(appErr.Code)),
			zap.String("detail", appErr.Detail))
		statusCode := errors.ToHTTPStatus(appErr.Code)
		c.JSON(statusCode, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "market refreshed",
		"plugins": len(marketplace.Plugins),
	})
}

// GetTeamPackages 获取所有市场的团队包
func (h *MarketHandler) GetTeamPackages(c *gin.Context) {
	// 解析 forceRefresh 参数
	forceRefresh := c.Query("forceRefresh") == "true"

	packages, err := h.marketSvc.GetTeamPackages(c.Request.Context(), forceRefresh)
	if err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("get team packages failed",
			zap.String("code", string(appErr.Code)),
			zap.String("detail", appErr.Detail))
		statusCode := errors.ToHTTPStatus(appErr.Code)
		c.JSON(statusCode, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": packages, "total": len(packages)})
}

// RefreshPackages 手动刷新所有市场缓存
func (h *MarketHandler) RefreshPackages(c *gin.Context) {
	if err := h.marketSvc.RefreshPackages(c.Request.Context()); err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("refresh packages failed", zap.String("detail", appErr.Detail))
		statusCode := errors.ToHTTPStatus(appErr.Code)
		c.JSON(statusCode, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "packages refreshed"})
}