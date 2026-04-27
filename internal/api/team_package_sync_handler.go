// 文件路径: isdp/internal/api/team_package_sync_handler.go
package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/service/teampackagesync"
	"github.com/anthropic/isdp/pkg/errors"
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
	g.GET("/check-update", h.CheckUpdates)
	g.GET("/local-versions", h.GetLocalVersions)
	g.POST("/preview", h.PreviewPackage)
	g.POST("/preview-batch", h.PreviewPackagesBatch) // 新增：批量预览
	g.POST("/sync", h.SyncPackage)
	g.POST("/sync-batch", h.SyncPackagesBatch)       // 新增：批量同步
}

// CheckUpdates 检查可用更新
func (h *TeamPackageSyncHandler) CheckUpdates(c *gin.Context) {
	result, err := h.syncSvc.CheckUpdates(c.Request.Context())
	if err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("check updates failed",
			zap.String("code", string(appErr.Code)),
			zap.String("detail", appErr.Detail))
		statusCode := errors.ToHTTPStatus(appErr.Code)
		c.JSON(statusCode, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetLocalVersions 获取本地版本记录列表
func (h *TeamPackageSyncHandler) GetLocalVersions(c *gin.Context) {
	versions, err := h.syncSvc.GetLocalVersions(c.Request.Context())
	if err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("get local versions failed",
			zap.String("code", string(appErr.Code)),
			zap.String("detail", appErr.Detail))
		statusCode := errors.ToHTTPStatus(appErr.Code)
		c.JSON(statusCode, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": versions, "total": len(versions)})
}

// PreviewPackage 预览团队包内容（不实际导入）
func (h *TeamPackageSyncHandler) PreviewPackage(c *gin.Context) {
	var req teampackagesync.PreviewPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewInvalidParam(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}

	result, err := h.syncSvc.PreviewPackage(c.Request.Context(), req.PackageName, req.MarketId)
	if err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("preview package failed",
			zap.String("package", req.PackageName),
			zap.String("code", string(appErr.Code)),
			zap.String("detail", appErr.Detail))
		statusCode := errors.ToHTTPStatus(appErr.Code)
		c.JSON(statusCode, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, result)
}

// SyncPackage 同步指定的团队包
func (h *TeamPackageSyncHandler) SyncPackage(c *gin.Context) {
	var req teampackagesync.SyncPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewInvalidParam(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}

	result, err := h.syncSvc.SyncPackage(c.Request.Context(), req.PackageName, req.MarketId, req.Confirm)
	if err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("sync package failed",
			zap.String("package", req.PackageName),
			zap.String("code", string(appErr.Code)),
			zap.String("detail", appErr.Detail))
		statusCode := errors.ToHTTPStatus(appErr.Code)
		c.JSON(statusCode, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, result)
}

// PreviewPackagesBatch 批量预览团队包
func (h *TeamPackageSyncHandler) PreviewPackagesBatch(c *gin.Context) {
	var req teampackagesync.BatchPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewInvalidParam(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}

	result, err := h.syncSvc.PreviewPackagesBatch(c.Request.Context(), req.Packages)
	if err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("batch preview failed",
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
		"previews":       result.Previews,
		"totalConflicts": result.TotalConflicts,
		"successCount":   result.SuccessCount,
		"failedCount":    result.FailedCount,
	})
}

// SyncPackagesBatch 批量同步团队包
func (h *TeamPackageSyncHandler) SyncPackagesBatch(c *gin.Context) {
	var req teampackagesync.BatchSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewInvalidParam(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}

	result, err := h.syncSvc.SyncPackagesBatch(c.Request.Context(), req.Packages)
	if err != nil {
		appErr := errors.WrapError(err)
		h.logger.Error("batch sync failed",
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
		"results":      result.Results,
		"successCount": result.SuccessCount,
		"failedCount":  result.FailedCount,
	})
}