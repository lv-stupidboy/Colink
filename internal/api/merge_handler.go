package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/service/merge"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MergeHandler 合并门禁 API处理器
type MergeHandler struct {
	gatekeeper *merge.Gatekeeper
}

// NewMergeHandler 创建处理器
func NewMergeHandler(gatekeeper *merge.Gatekeeper) *MergeHandler {
	return &MergeHandler{gatekeeper: gatekeeper}
}

// Check 检查是否可以合并
func (h *MergeHandler) Check(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	result, err := h.gatekeeper.CheckMerge(c.Request.Context(), threadID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// Approve 批准合并
func (h *MergeHandler) Approve(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	// TODO: 实现批准合并逻辑
	c.JSON(http.StatusOK, gin.H{"status": "approved", "threadId": threadID})
}

// Handover 交接
func (h *MergeHandler) Handover(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	// TODO: 实现交接逻辑
	c.JSON(http.StatusOK, gin.H{"status": "handover", "threadId": threadID})
}

// RegisterRoutes 注册路由
func (h *MergeHandler) RegisterRoutes(r *gin.RouterGroup) {
	threads := r.Group("/threads")
	{
		threads.GET("/:id/merge/check", h.Check)
		threads.POST("/:id/merge/approve", h.Approve)
		threads.GET("/:id/merge/handover", h.Handover)
	}
}