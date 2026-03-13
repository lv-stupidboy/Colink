package api

import (
	"net/http"
	"strconv"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/message"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MessageHandler 消息API处理器
type MessageHandler struct {
	service *message.Service
}

// NewMessageHandler 创建处理器
func NewMessageHandler(service *message.Service) *MessageHandler {
	return &MessageHandler{service: service}
}

// List 列出消息
func (h *MessageHandler) List(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	messages, err := h.service.GetByThreadID(c.Request.Context(), threadID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, messages)
}

// Create 创建消息
func (h *MessageHandler) Create(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户ID（暂时使用匿名）
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		userID = "anonymous"
	}

	msg, err := h.service.Create(c.Request.Context(), threadID, model.MessageRoleUser, userID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, msg)
}

// RegisterRoutes 注册路由
func (h *MessageHandler) RegisterRoutes(r *gin.RouterGroup) {
	messages := r.Group("/messages")
	{
		messages.GET("/thread/:threadId", h.List)
		messages.POST("/thread/:threadId", h.Create)
	}
}