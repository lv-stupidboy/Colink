package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/service/market"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MarketHandler 市场 API 处理器
type MarketHandler struct {
	marketSvc *market.Service
	logger    *zap.Logger
}

// NewMarketHandler 创建 MarketHandler
func NewMarketHandler(marketSvc *market.Service, logger *zap.Logger) *MarketHandler {
	return &MarketHandler{
		marketSvc: marketSvc,
		logger:    logger,
	}
}

// RegisterRoutes 注册路由
func (h *MarketHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/markets")
	g.GET("", h.ListMarkets)
	g.POST("", h.AddMarket)
	g.PUT("/:id", h.UpdateMarket)
	g.DELETE("/:id", h.DeleteMarket)
	g.POST("/:id/refresh", h.RefreshMarket)
	g.GET("/packages", h.GetTeamPackages)
}

// ListMarkets 获取市场列表
func (h *MarketHandler) ListMarkets(c *gin.Context) {
	markets, err := h.marketSvc.ListMarkets(c.Request.Context())
	if err != nil {
		h.logger.Error("list markets failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": markets, "total": len(markets)})
}

// AddMarket 添加市场
func (h *MarketHandler) AddMarket(c *gin.Context) {
	var req market.AddMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.marketSvc.AddMarket(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("add market failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, m)
}

// UpdateMarket 更新市场
func (h *MarketHandler) UpdateMarket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market id"})
		return
	}

	var req market.UpdateMarketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m, err := h.marketSvc.UpdateMarket(c.Request.Context(), id, req)
	if err != nil {
		h.logger.Error("update market failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, m)
}

// DeleteMarket 删除市场
func (h *MarketHandler) DeleteMarket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market id"})
		return
	}

	if err := h.marketSvc.DeleteMarket(c.Request.Context(), id); err != nil {
		h.logger.Error("delete market failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "market deleted"})
}

// RefreshMarket 刷新市场
func (h *MarketHandler) RefreshMarket(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid market id"})
		return
	}

	marketplace, err := h.marketSvc.RefreshMarket(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("refresh market failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "market refreshed",
		"plugins": len(marketplace.Plugins),
	})
}

// GetTeamPackages 获取所有市场的团队包
func (h *MarketHandler) GetTeamPackages(c *gin.Context) {
	packages, err := h.marketSvc.GetTeamPackages(c.Request.Context())
	if err != nil {
		h.logger.Error("get team packages failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": packages, "total": len(packages)})
}