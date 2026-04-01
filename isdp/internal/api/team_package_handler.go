// 文件路径: isdp/internal/api/team_package_handler.go
package api

import (
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/teampackage"
	"github.com/gin-gonic/gin"
)

// TeamPackageHandler 团队包 API 处理器
type TeamPackageHandler struct {
	svc *teampackage.Service
}

// NewTeamPackageHandler 创建 TeamPackageHandler
func NewTeamPackageHandler(svc *teampackage.Service) *TeamPackageHandler {
	return &TeamPackageHandler{svc: svc}
}

// Import 预览导入的团队包
func (h *TeamPackageHandler) Import(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的文件"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".zip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 .zip 格式的文件"})
		return
	}

	zipData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	preview, err := h.svc.ImportPreview(c.Request.Context(), zipData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, preview)
}

// ImportConfirm 确认导入团队包
func (h *TeamPackageHandler) ImportConfirm(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的文件"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".zip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 .zip 格式的文件"})
		return
	}

	zipData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	var confirm model.TeamPackageImportConfirm
	if err := c.ShouldBindJSON(&confirm); err != nil {
		// 如果没有 JSON body，使用默认策略
		confirm = model.TeamPackageImportConfirm{
			Mode:           "overwrite",
			WorkflowAction: "overwrite",
		}
	}

	result, err := h.svc.ImportConfirm(c.Request.Context(), zipData, &confirm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Export 导出团队包
func (h *TeamPackageHandler) Export(c *gin.Context) {
	var req model.ExportTeamPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	zipData, filename, err := h.svc.Export(c.Request.Context(), req.WorkflowID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")
	c.Header("Content-Length", strconv.Itoa(len(zipData)))
	c.Data(http.StatusOK, "application/zip", zipData)
}

// RegisterRoutes 注册路由
func (h *TeamPackageHandler) RegisterRoutes(r *gin.RouterGroup) {
	teamPackages := r.Group("/team-packages")
	{
		teamPackages.POST("/import", h.Import)
		teamPackages.POST("/import/confirm", h.ImportConfirm)
		teamPackages.POST("/export", h.Export)
	}
}