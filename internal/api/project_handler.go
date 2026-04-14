package api

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/project"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ProjectHandler 项目API处理器
type ProjectHandler struct {
	service *project.Service
}

// NewProjectHandler 创建处理器
func NewProjectHandler(service *project.Service) *ProjectHandler {
	return &ProjectHandler{service: service}
}

// List 列出项目
func (h *ProjectHandler) List(c *gin.Context) {
	projects, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, projects)
}

// Get 获取项目
func (h *ProjectHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	project, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}
	c.JSON(http.StatusOK, project)
}

// Create 创建项目
func (h *ProjectHandler) Create(c *gin.Context) {
	var req model.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, project)
}

// Update 更新项目
func (h *ProjectHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project, err := h.service.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, project)
}

// Delete 删除项目
func (h *ProjectHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// ListFiles 列出项目文件
func (h *ProjectHandler) ListFiles(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	subPath := c.Query("path")
	result, err := h.service.ListFiles(c.Request.Context(), id, subPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ListFilesByPath 根据路径列出文件（用于调试模式）
func (h *ProjectHandler) ListFilesByPath(c *gin.Context) {
	basePath := c.Query("basePath")
	if basePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "basePath is required"})
		return
	}

	subPath := c.Query("path")
	result, err := h.service.ListFilesByPath(c.Request.Context(), basePath, subPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// BrowsePath 浏览文件系统路径
func (h *ProjectHandler) BrowsePath(c *gin.Context) {
	path := c.Query("path")
	result, err := h.service.BrowsePath(c.Request.Context(), path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ValidatePath 验证路径是否可用于项目
func (h *ProjectHandler) ValidatePath(c *gin.Context) {
	path := c.Query("path")
	result, err := h.service.ValidatePath(c.Request.Context(), path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// CreateFolder 创建文件夹
func (h *ProjectHandler) CreateFolder(c *gin.Context) {
	var req model.CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.CreateFolder(c.Request.Context(), req.Path, req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetFileContent 获取文件内容
func (h *ProjectHandler) GetFileContent(c *gin.Context) {
	basePath := c.Query("basePath")
	if basePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "basePath is required"})
		return
	}

	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	result, err := h.service.GetFileContent(c.Request.Context(), basePath, filePath)
	if err != nil {
		// 根据错误类型返回不同的 HTTP 状态码
		errMsg := err.Error()
		if strings.Contains(errMsg, "不存在") {
			c.JSON(http.StatusNotFound, gin.H{"error": errMsg})
		} else if strings.Contains(errMsg, "无效") || strings.Contains(errMsg, "目录") || strings.Contains(errMsg, "不能为空") {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
		}
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetFileImage 获取图片文件（直接返回文件内容用于预览）
func (h *ProjectHandler) GetFileImage(c *gin.Context) {
	basePath := c.Query("basePath")
	if basePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "basePath is required"})
		return
	}

	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	// 拼接完整路径
	fullPath := filepath.Join(basePath, filePath)

	// 安全检查：防止路径穿越
	basePathClean := filepath.Clean(basePath)
	if !strings.HasPrefix(fullPath, basePathClean) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}

	// 直接返回文件
	c.File(fullPath)
}

// RegisterRoutes 注册路由
func (h *ProjectHandler) RegisterRoutes(r *gin.RouterGroup) {
	projects := r.Group("/projects")
	{
		projects.GET("", h.List)
		projects.POST("", h.Create)
		// Note: /:id/files must be registered BEFORE /:id to ensure proper matching
		projects.GET("/:id/files", h.ListFiles)
		projects.GET("/:id", h.Get)
		projects.PUT("/:id", h.Update)
		projects.DELETE("/:id", h.Delete)
	}
	// 文件浏览 API
	files := r.Group("/files")
	{
		files.GET("/browse", h.BrowsePath)
		files.GET("/validate", h.ValidatePath)
		files.GET("/content", h.GetFileContent)
		files.GET("/image", h.GetFileImage)
		files.POST("/folder", h.CreateFolder)
	}
}
