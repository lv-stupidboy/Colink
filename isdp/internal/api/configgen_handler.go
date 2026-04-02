package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/service/configgen"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ConfigGenHandler 配置生成 API 处理器
type ConfigGenHandler struct {
	configGenSvc *configgen.Service
}

// NewConfigGenHandler 创建处理器
func NewConfigGenHandler(configGenSvc *configgen.Service) *ConfigGenHandler {
	return &ConfigGenHandler{
		configGenSvc: configGenSvc,
	}
}

// SyncConfigRequest 同步配置请求
type SyncConfigRequest struct {
	BaseAgentType string `json:"baseAgentType" binding:"required"` // claude_code | open_code
	CleanExisting bool   `json:"cleanExisting"`                    // 是否清理现有配置
}

// GenerateAgentConfigRequest 生成Agent配置请求
type GenerateAgentConfigRequest struct {
	BaseAgentType string `json:"baseAgentType" binding:"required"` // claude_code | open_code
	CleanExisting bool   `json:"cleanExisting"`
}

// SyncConfig 同步配置到项目
// POST /projects/:id/config/sync
func (h *ConfigGenHandler) SyncConfig(c *gin.Context) {
	projectID := c.Param("id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少项目 ID"})
		return
	}

	var req SyncConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 验证 baseAgentType
	if req.BaseAgentType != "claude_code" && req.BaseAgentType != "open_code" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "baseAgentType 必须是 claude_code 或 open_code"})
		return
	}

	result, err := h.configGenSvc.GenerateConfig(c.Request.Context(), &configgen.GenerateConfigRequest{
		ProjectID:     projectID,
		BaseAgentType: req.BaseAgentType,
		CleanExisting: req.CleanExisting,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "配置同步成功",
		"projectId":   result.ProjectID,
		"targetDir":   result.TargetDir,
		"skillsCount": result.SkillsCount,
		"agentRoles":  result.AgentRoles,
		"details":     result.Results,
	})
}

// GenerateAgentConfig 生成Agent角色配置
// POST /agents/:id/config/generate
func (h *ConfigGenHandler) GenerateAgentConfig(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 Agent ID"})
		return
	}

	// 解析 agentID 为 uuid
	agentUUID, err := uuid.Parse(agentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 Agent ID 格式"})
		return
	}

	var req GenerateAgentConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 验证 baseAgentType
	if req.BaseAgentType != "claude_code" && req.BaseAgentType != "open_code" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "baseAgentType 必须是 claude_code 或 open_code"})
		return
	}

	result, err := h.configGenSvc.GenerateAgentConfig(c.Request.Context(), &configgen.GenerateAgentConfigRequest{
		AgentRoleID:   agentUUID,
		BaseAgentType: req.BaseAgentType,
		CleanExisting: req.CleanExisting,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Agent配置生成成功",
		"agentId":         result.AgentID,
		"configPath":      result.ConfigPath,
		"skillsCount":     result.SkillsCount,
		"subagentsCount":  result.SubagentsCount,
		"commandsCount":   result.CommandsCount,
		"rulesCount":      result.RulesCount,
		"settingsCount":   result.SettingsCount,
		"generatedAt":     result.GeneratedAt,
	})
}

// PreviewAgentConfig 预览Agent角色配置（生成前）
// GET /agents/:id/config/preview
func (h *ConfigGenHandler) PreviewAgentConfig(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 Agent ID"})
		return
	}

	// 解析 agentID 为 uuid
	agentUUID, err := uuid.Parse(agentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 Agent ID 格式"})
		return
	}

	result, err := h.configGenSvc.PreviewAgentConfig(c.Request.Context(), agentUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// RegisterRoutes 注册路由
func (h *ConfigGenHandler) RegisterRoutes(r *gin.RouterGroup) {
	// 项目级配置同步（保留兼容）
	projects := r.Group("/projects")
	{
		projects.POST("/:id/config/sync", h.SyncConfig)
	}

	// Agent级配置生成（新增）
	agents := r.Group("/agents")
	{
		agents.GET("/:id/config/preview", h.PreviewAgentConfig)
		agents.POST("/:id/config/generate", h.GenerateAgentConfig)
	}
}