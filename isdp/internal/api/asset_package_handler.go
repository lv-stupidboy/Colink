package api

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

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

// Import 预览资产包（ZIP 文件上传）
func (h *AssetPackageHandler) Import(c *gin.Context) {
	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的文件"})
		return
	}
	defer file.Close()

	// 检查文件扩展名
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".zip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 .zip 格式的文件"})
		return
	}

	// 读取文件内容
	zipData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	// 预览资产包
	result, err := h.svc.ImportPreview(c.Request.Context(), zipData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ImportConfirm 确认导入资产包
func (h *AssetPackageHandler) ImportConfirm(c *gin.Context) {
	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的文件"})
		return
	}
	defer file.Close()

	// 检查文件扩展名
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".zip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 .zip 格式的文件"})
		return
	}

	// 读取文件内容
	zipData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	// 解析确认参数 - 从 form 中读取 confirm 字段
	var confirm model.AssetPackageImportConfirm
	confirmStr := c.PostForm("confirm")
	if confirmStr != "" {
		if err := json.Unmarshal([]byte(confirmStr), &confirm); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "解析确认参数失败: " + err.Error()})
			return
		}
	} else {
		// 使用默认策略
		confirm = model.AssetPackageImportConfirm{
			Mode: "overwrite",
		}
	}

	// 确认导入
	result, err := h.svc.ImportConfirm(c.Request.Context(), zipData, &confirm)
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
		packages.POST("/import/confirm", h.ImportConfirm)
		packages.POST("/export", h.Export)
	}
}