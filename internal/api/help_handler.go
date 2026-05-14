package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
)

// HelpHandler 帮助入口API处理器
type HelpHandler struct {
	supportGroup    string
	officialWebsite string
	docLink         string
	feedbackAPI     string
	client          *http.Client
	clientOnce      sync.Once
}

// NewHelpHandler 创建HelpHandler
func NewHelpHandler(supportGroup, officialWebsite, docLink, feedbackAPI string) *HelpHandler {
	return &HelpHandler{
		supportGroup:    supportGroup,
		officialWebsite: officialWebsite,
		docLink:         docLink,
		feedbackAPI:     feedbackAPI,
	}
}

// GetConfig 获取帮助配置
func (h *HelpHandler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"support_group":    h.supportGroup,
		"official_website": h.officialWebsite,
		"doc_link":         h.docLink,
		"feedback_enabled": h.feedbackAPI != "",
	})
}

// FeedbackRequest 问题反馈请求
type FeedbackRequest struct {
	Type        string   `json:"type" binding:"required"` // 问题类型
	Description string   `json:"description"`            // 问题描述（可选，如果有图片）
	Images      []string `json:"images"`                 // 图片列表(base64数组)
}

// SubmitFeedback 提交问题反馈
func (h *HelpHandler) SubmitFeedback(c *gin.Context) {
	var req FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查是否有内容
	if req.Description == "" && len(req.Images) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写问题描述或添加图片"})
		return
	}

	// 检查是否配置了反馈API
	if h.feedbackAPI == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "反馈功能未开通"})
		return
	}

	// 获取当前用户名（与 Reporter 保持一致）
	username := "unknown"
	if u := os.Getenv("USER"); u != "" {
		username = u
	} else if u := os.Getenv("USERNAME"); u != "" {
		username = u
	}

	// 发送到配置的反馈API
	h.initClient()
	payload := map[string]interface{}{
		"type":        req.Type,
		"user":        username,
		"description": req.Description,
		"images":      req.Images,
	}
	jsonPayload, _ := json.Marshal(payload)

	resp, err := h.client.Post(h.feedbackAPI, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发送反馈失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "反馈服务返回错误"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "反馈已提交"})
}

// initClient 初始化HTTP客户端（懒加载）
func (h *HelpHandler) initClient() {
	h.clientOnce.Do(func() {
		h.client = &http.Client{}
	})
}

// RegisterRoutes 注册路由
func (h *HelpHandler) RegisterRoutes(r *gin.RouterGroup) {
	helpGroup := r.Group("/help")
	{
		helpGroup.GET("/config", h.GetConfig)
		helpGroup.POST("/feedback", h.SubmitFeedback)
	}
}