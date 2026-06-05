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

// List 列出消息（默认返回最新的N条）
func (h *MessageHandler) List(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	messages, err := h.service.GetByThreadID(c.Request.Context(), threadID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取消息总数，用于判断是否还有更多历史
	total, err := h.service.GetMessageCount(c.Request.Context(), threadID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages":  messages,
		"total":     total,
		"hasMore":   len(messages) < total,
	})
}

// ListBeforeCursor 列出指定cursor之前的消息（用于向上滚动加载历史）
func (h *MessageHandler) ListBeforeCursor(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	cursor := c.Query("cursor")
	if cursor == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cursor is required"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	messages, err := h.service.GetByThreadIDBeforeCursor(c.Request.Context(), threadID, cursor, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"hasMore":  len(messages) >= limit, // 如果返回数量等于limit，可能还有更多
	})
}

// ImageInput 图片输入结构
type ImageInput struct {
	MimeType string `json:"mimeType" binding:"required"` // MIME类型：image/png, image/jpeg
	Data     string `json:"data" binding:"required"`     // base64数据（不含前缀）
}

// Create 创建消息
func (h *MessageHandler) Create(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	var req struct {
		Content          string       `json:"content" binding:"required"`
		Images           []ImageInput `json:"images"`           // 图片附件（多模态输入）
		SkipAgentTrigger bool         `json:"skipAgentTrigger"` // 前端已处理Agent触发时设为true
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

	// 转换图片输入为模型格式
	var images []model.ImageContent
	for _, img := range req.Images {
		images = append(images, model.ImageContent{
			MimeType: img.MimeType,
			Data:     img.Data,
		})
	}

	msg, err := h.service.CreateWithImages(c.Request.Context(), threadID, model.MessageRoleUser, userID, req.Content, images, req.SkipAgentTrigger)
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
		messages.GET("/thread/:threadId/history", h.ListBeforeCursor)
		messages.POST("/thread/:threadId", h.Create)
	}
}