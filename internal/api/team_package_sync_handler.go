// 文件路径: isdp/internal/api/team_package_sync_handler.go
package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/service/teampackagesync"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TeamPackageSyncHandler 团队包同步 API 处理器
type TeamPackageSyncHandler struct {
	syncSvc *teampackagesync.SyncService
	logger  *zap.Logger
}

// NewTeamPackageSyncHandler 创建 TeamPackageSyncHandler
func NewTeamPackageSyncHandler(syncSvc *teampackagesync.SyncService, logger *zap.Logger) *TeamPackageSyncHandler {
	return &TeamPackageSyncHandler{
		syncSvc: syncSvc,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由
func (h *TeamPackageSyncHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/team-package-sync")
	g.GET("/remote", h.GetRemotePackages)
	g.GET("/check-update", h.CheckUpdates)
	g.GET("/local-versions", h.GetLocalVersions)
	g.POST("/sync", h.SyncPackage)
}

// GetRemotePackages 获取远程团队包列表
func (h *TeamPackageSyncHandler) GetRemotePackages(c *gin.Context) {
	result, err := h.syncSvc.GetRemotePackages(c.Request.Context())
	if err != nil {
		h.logger.Error("get remote packages failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// CheckUpdates 检查可用更新
func (h *TeamPackageSyncHandler) CheckUpdates(c *gin.Context) {
	result, err := h.syncSvc.CheckUpdates(c.Request.Context())
	if err != nil {
		h.logger.Error("check updates failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetLocalVersions 获取本地版本记录列表
func (h *TeamPackageSyncHandler) GetLocalVersions(c *gin.Context) {
	versions, err := h.syncSvc.GetLocalVersions(c.Request.Context())
	if err != nil {
		h.logger.Error("get local versions failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": versions, "total": len(versions)})
}

// SyncPackage 同步指定的团队包
func (h *TeamPackageSyncHandler) SyncPackage(c *gin.Context) {
	var req teampackagesync.SyncPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.syncSvc.SyncPackage(c.Request.Context(), req.PackageName, req.Confirm)
	if err != nil {
		h.logger.Error("sync package failed", zap.Error(err), zap.String("package", req.PackageName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}