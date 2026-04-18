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
	// 验证状态值是否有效（简化后只有三种状态）
	validStatuses := []model.HumanTaskStatus{
		model.HumanTaskStatusPending,
		model.HumanTaskStatusCompleted,
		model.HumanTaskStatusCancelled,
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

// Complete 完成任务
// PUT /api/v1/human-tasks/:id/complete
func (h *HumanTaskHandler) Complete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.svc.Complete(c.Request.Context(), id); err != nil {
		errMsg := err.Error()
		if errMsg == "task is not in pending state" {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "completed"})
}

// Cancel 取消任务
// PUT /api/v1/human-tasks/:id/cancel
func (h *HumanTaskHandler) Cancel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.svc.CancelTask(c.Request.Context(), id); err != nil {
		errMsg := err.Error()
		if errMsg == "task is not in pending state" {
			c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// GetStats 获取任务统计
// GET /api/v1/human-tasks/stats
func (h *HumanTaskHandler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// RegisterRoutes 注册路由
func (h *HumanTaskHandler) RegisterRoutes(r *gin.RouterGroup) {
	humanTasks := r.Group("/human-tasks")
	{
		humanTasks.GET("", h.List)
		humanTasks.GET("/stats", h.GetStats)
		humanTasks.GET("/:id", h.Get)
		humanTasks.PUT("/:id/complete", h.Complete)
		humanTasks.PUT("/:id/cancel", h.Cancel)
	}
}