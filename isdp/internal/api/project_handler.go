package api

import (
	"net/http"

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
	// 文件浏览 API（调试模式）
	r.GET("/files/browse", h.ListFilesByPath)
}