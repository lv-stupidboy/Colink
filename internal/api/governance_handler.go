package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/gin-gonic/gin"
)

// GovernanceHandler 治理规则 API 处理器
type GovernanceHandler struct{}

// NewGovernanceHandler 创建处理器
func NewGovernanceHandler() *GovernanceHandler {
	return &GovernanceHandler{}
}

// GetGovernanceDigest 获取当前治理摘要内容
func (h *GovernanceHandler) GetGovernanceDigest(c *gin.Context) {
	status := agent.GetGovernanceDigestStatus()
	content := agent.BuildGovernanceDigest()

	c.JSON(http.StatusOK, gin.H{
		"content": content,
		"status":  status,
	})
}

// UpdateGovernanceDigest 更新治理摘要内容（热更新）
func (h *GovernanceHandler) UpdateGovernanceDigest(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	// 验证内容不为空
	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content cannot be empty"})
		return
	}

	// 热更新
	if err := agent.UpdateGovernanceDigestContent(req.Content); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 返回更新后的状态
	status := agent.GetGovernanceDigestStatus()

	c.JSON(http.StatusOK, gin.H{
		"message": "Governance digest updated successfully",
		"status":  status,
	})
}

// GetGovernanceDigestStatus 获取治理摘要状态信息
func (h *GovernanceHandler) GetGovernanceDigestStatus(c *gin.Context) {
	status := agent.GetGovernanceDigestStatus()

	c.JSON(http.StatusOK, status)
}

// RegisterRoutes 注册路由
func (h *GovernanceHandler) RegisterRoutes(r *gin.RouterGroup) {
	governance := r.Group("/governance")
	{
		governance.GET("/digest", h.GetGovernanceDigest)
		governance.PUT("/digest", h.UpdateGovernanceDigest)
		governance.GET("/digest/status", h.GetGovernanceDigestStatus)
	}
}