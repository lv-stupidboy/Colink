package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/humantask"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// HumanTaskHandler 人工任务 API 处理器
type HumanTaskHandler struct {
	svc *humantask.Service
}

// NewHumanTaskHandler 创建处理器
func NewHumanTaskHandler(svc *humantask.Service) *HumanTaskHandler {
	return &HumanTaskHandler{svc: svc}
}

// List 列出任务（可选按状态筛选）
// GET /api/v1/human-tasks?status=pending
func (h *HumanTaskHandler) List(c *gin.Context) {
	statusStr := c.Query("status")
	if statusStr == "" {
		// 默认列出所有待处理的任务
		statusStr = string(model.HumanTaskStatusPending)
	}

	status := model.HumanTaskStatus(statusStr)
	// 验证状态值是否有效
	validStatuses := []model.HumanTaskStatus{
		model.HumanTaskStatusPending,
		model.HumanTaskStatusInProgress,
		model.HumanTaskStatusCompleted,
		model.HumanTaskStatusRejected,
		model.HumanTaskStatusFailed,
	}
	isValid := false
	for _, s := range validStatuses {
		if s == status {
			isValid = true
			break
		}
	}
	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status value"})
		return
	}

	tasks, err := h.svc.List(c.Request.Context(), status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tasks)
}

// Get 获取任务详情
// GET /api/v1/human-tasks/:id
func (h *HumanTaskHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	task, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	c.JSON(http.StatusOK, task)
}

// Submit 提交交付物
// POST /api/v1/human-tasks/:id/submit
func (h *HumanTaskHandler) Submit(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.SubmitHumanTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证必填字段
	if req.OutputContent == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outputContent is required"})
		return
	}

	resp, err := h.svc.Submit(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Start 开始执行任务（更改状态为 in_progress）
// PUT /api/v1/human-tasks/:id/start
func (h *HumanTaskHandler) Start(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.svc.Start(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "in_progress"})
}

// Reject 拒绝任务
// PUT /api/v1/human-tasks/:id/reject
func (h *HumanTaskHandler) Reject(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.svc.Reject(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "rejected"})
}

// RegisterRoutes 注册路由
func (h *HumanTaskHandler) RegisterRoutes(r *gin.RouterGroup) {
	humanTasks := r.Group("/human-tasks")
	{
		humanTasks.GET("", h.List)
		humanTasks.GET("/:id", h.Get)
		humanTasks.POST("/:id/submit", h.Submit)
		humanTasks.PUT("/:id/start", h.Start)
		humanTasks.PUT("/:id/reject", h.Reject)
	}
}