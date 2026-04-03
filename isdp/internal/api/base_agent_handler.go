package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BaseAgentHandler 基础Agent API处理器
type BaseAgentHandler struct {
	baseAgentSvc *agent.BaseAgentService
}

// NewBaseAgentHandler 创建处理器
func NewBaseAgentHandler(baseAgentSvc *agent.BaseAgentService) *BaseAgentHandler {
	return &BaseAgentHandler{baseAgentSvc: baseAgentSvc}
}

// List 列出所有基础Agent
func (h *BaseAgentHandler) List(c *gin.Context) {
	agents, err := h.baseAgentSvc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, agents)
}

// Get 获取基础Agent详情
func (h *BaseAgentHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	agent, err := h.baseAgentSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "base agent not found"})
		return
	}
	c.JSON(http.StatusOK, agent)
}

// GetTypes 获取支持的基础Agent类型
func (h *BaseAgentHandler) GetTypes(c *gin.Context) {
	types := h.baseAgentSvc.GetTypes()
	c.JSON(http.StatusOK, types)
}

// Create 创建基础Agent
func (h *BaseAgentHandler) Create(c *gin.Context) {
	var req model.CreateBaseAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agent, err := h.baseAgentSvc.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, agent)
}

// Update 更新基础Agent
func (h *BaseAgentHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.UpdateBaseAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	agent, err := h.baseAgentSvc.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, agent)
}

// Delete 删除基础Agent
func (h *BaseAgentHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.baseAgentSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// Test 测试基础Agent连接
func (h *BaseAgentHandler) Test(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.baseAgentSvc.TestConnection(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Connection successful",
	})
}

// SetDefault 设置默认基础Agent
func (h *BaseAgentHandler) SetDefault(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.baseAgentSvc.SetDefault(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回更新后的列表
	agents, err := h.baseAgentSvc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "设置成功"})
		return
	}
	c.JSON(http.StatusOK, agents)
}

// RegisterRoutes 注册路由
func (h *BaseAgentHandler) RegisterRoutes(r *gin.RouterGroup) {
	baseAgents := r.Group("/base-agents")
	{
		baseAgents.GET("", h.List)
		baseAgents.POST("", h.Create)
		baseAgents.GET("/types", h.GetTypes)
		baseAgents.GET("/:id", h.Get)
		baseAgents.PUT("/:id", h.Update)
		baseAgents.DELETE("/:id", h.Delete)
		baseAgents.POST("/:id/test", h.Test)
		baseAgents.PUT("/:id/default", h.SetDefault)
	}
}