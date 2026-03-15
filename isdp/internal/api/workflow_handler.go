package api

import (
	"net/http"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/workflow"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// WorkflowHandler 工作流模板API处理器
type WorkflowHandler struct {
	service *workflow.Service
}

// NewWorkflowHandler 创建处理器
func NewWorkflowHandler(service *workflow.Service) *WorkflowHandler {
	return &WorkflowHandler{service: service}
}

// List 列出工作流模板
func (h *WorkflowHandler) List(c *gin.Context) {
	templates, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, templates)
}

// Get 获取工作流模板
func (h *WorkflowHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	template, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow template not found"})
		return
	}
	c.JSON(http.StatusOK, template)
}

// Create 创建工作流模板
func (h *WorkflowHandler) Create(c *gin.Context) {
	var req model.CreateWorkflowTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, template)
}

// Update 更新工作流模板
func (h *WorkflowHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.UpdateWorkflowTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template, err := h.service.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, template)
}

// Delete 删除工作流模板
func (h *WorkflowHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		// 区分业务错误和系统错误
		if strings.Contains(err.Error(), "已被") || strings.Contains(err.Error(), "默认工作流") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// SetDefault 设置默认工作流模板
func (h *WorkflowHandler) SetDefault(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	template, err := h.service.SetDefault(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, template)
}

// RegisterRoutes 注册路由
func (h *WorkflowHandler) RegisterRoutes(r *gin.RouterGroup) {
	workflows := r.Group("/workflows")
	{
		workflows.GET("", h.List)
		workflows.POST("", h.Create)
		workflows.GET("/:id", h.Get)
		workflows.PUT("/:id", h.Update)
		workflows.PUT("/:id/default", h.SetDefault)
		workflows.DELETE("/:id", h.Delete)
	}
}