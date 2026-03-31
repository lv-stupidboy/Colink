package api

import (
	"io"
	"net/http"
	"strconv"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/assetpackage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AssetPackageHandler AssetPackage API处理器
type AssetPackageHandler struct {
	svc *assetpackage.Service
}

// NewAssetPackageHandler 创建AssetPackageHandler
func NewAssetPackageHandler(svc *assetpackage.Service) *AssetPackageHandler {
	return &AssetPackageHandler{
		svc: svc,
	}
}

// List 列出所有AssetPackages
func (h *AssetPackageHandler) List(c *gin.Context) {
	var query model.AssetPackageListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置默认分页
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}

	packages, total, err := h.svc.List(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      packages,
		"total":     total,
		"page":      query.Page,
		"pageSize":  query.PageSize,
	})
}

// GetByID 获取AssetPackage详情
func (h *AssetPackageHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	pkg, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset package not found"})
		return
	}

	c.JSON(http.StatusOK, pkg)
}

// Import 导入资产包（ZIP文件上传）
func (h *AssetPackageHandler) Import(c *gin.Context) {
	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的文件"})
		return
	}
	defer file.Close()

	// 检查文件扩展名
	ext := header.Filename
	if len(ext) < 4 || ext[len(ext)-4:] != ".zip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 .zip 格式的文件"})
		return
	}

	// 读取文件内容
	zipData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	// 导入资产包
	result, err := h.svc.Import(c.Request.Context(), zipData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Export 导出资产包（返回 ZIP 文件）
func (h *AssetPackageHandler) Export(c *gin.Context) {
	var req model.ExportAssetPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 导出资产包
	zipData, filename, err := h.svc.Export(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 设置响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")
	c.Header("Content-Length", strconv.Itoa(len(zipData)))

	// 返回 ZIP 文件
	c.Data(http.StatusOK, "application/zip", zipData)
}

// Delete 删除AssetPackage
func (h *AssetPackageHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// RegisterRoutes 注册路由
func (h *AssetPackageHandler) RegisterRoutes(r *gin.RouterGroup) {
	// AssetPackage CRUD 路由
	packages := r.Group("/asset-packages")
	{
		packages.GET("", h.List)
		packages.GET("/:id", h.GetByID)
		packages.POST("/import", h.Import)
		packages.POST("/export", h.Export)
		packages.DELETE("/:id", h.Delete)
	}
}