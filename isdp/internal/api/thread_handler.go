package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/thread"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ThreadHandler Thread API处理器
type ThreadHandler struct {
	service *thread.Service
}

// NewThreadHandler 创建处理器
func NewThreadHandler(service *thread.Service) *ThreadHandler {
	return &ThreadHandler{service: service}
}

// Get 获取Thread
func (h *ThreadHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	thread, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "thread not found"})
		return
	}
	c.JSON(http.StatusOK, thread)
}

// ListByProject 列出项目的Thread
func (h *ThreadHandler) ListByProject(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	threads, err := h.service.GetByProjectID(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, threads)
}

// Create 创建Thread
func (h *ThreadHandler) Create(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}

	var req struct {
		Name            string   `json:"name"`
		Type            string   `json:"type"`            // 可选：workflow 或 free_discussion
		AvailableAgents []string `json:"availableAgents"` // 可选：自由模式下的可用 Agent 列表
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果没有请求体，使用默认名称
		req.Name = "新任务"
	}

	// 如果名称为空，设置默认名称
	if req.Name == "" {
		req.Name = "新任务"
	}

	// 默认为工作流模式
	threadType := model.ThreadTypeWorkflow
	if req.Type == string(model.ThreadTypeFreeDiscussion) {
		threadType = model.ThreadTypeFreeDiscussion
	}

	var thread *model.Thread
	if threadType == model.ThreadTypeFreeDiscussion && len(req.AvailableAgents) > 0 {
		thread, err = h.service.CreateWithType(c.Request.Context(), projectID, req.Name, threadType, req.AvailableAgents)
	} else {
		thread, err = h.service.CreateWithType(c.Request.Context(), projectID, req.Name, threadType, nil)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, thread)
}

// UpdateStatus 更新状态
func (h *ThreadHandler) UpdateStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateStatus(c.Request.Context(), id, model.ThreadStatus(req.Status)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

// SetPhase 设置阶段
func (h *ThreadHandler) SetPhase(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Phase string `json:"phase" binding:"required"`
		Agent string `json:"agent"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.SetPhase(c.Request.Context(), id, model.Phase(req.Phase), req.Agent); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

// RegisterRoutes 注册路由
func (h *ThreadHandler) RegisterRoutes(r *gin.RouterGroup) {
	threads := r.Group("/threads")
	{
		threads.GET("/:id", h.Get)
		threads.PUT("/:id/status", h.UpdateStatus)
		threads.PUT("/:id/phase", h.SetPhase)
		threads.GET("/project/:projectId", h.ListByProject)
		threads.POST("/project/:projectId", h.Create)
	}
}