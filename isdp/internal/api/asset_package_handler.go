package api

import (
	"io"
	"net/http"
	"strconv"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/assetpackage"
	"github.com/gin-gonic/gin"
)

// AssetPackageHandler 资产包 API 处理器
type AssetPackageHandler struct {
	svc *assetpackage.Service
}

// NewAssetPackageHandler 创建 AssetPackageHandler
func NewAssetPackageHandler(svc *assetpackage.Service) *AssetPackageHandler {
	return &AssetPackageHandler{
		svc: svc,
	}
}

// Import 导入资产包（ZIP 文件上传）
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

// RegisterRoutes 注册路由
func (h *AssetPackageHandler) RegisterRoutes(r *gin.RouterGroup) {
	packages := r.Group("/asset-packages")
	{
		packages.POST("/import", h.Import)
		packages.POST("/export", h.Export)
	}
}