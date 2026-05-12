package api

import (
	"fmt"
	"net/http"

	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/configgen"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ConfigGenHandler 配置生成 API 处理器
type ConfigGenHandler struct {
	configGenSvc  *configgen.Service
	autoGenerator *configgen.AutoGenerator // 自动配置生成器
	agentRepo     *repo.AgentConfigRepository
	baseAgentRepo *repo.BaseAgentRepository
}

// NewConfigGenHandler 创建处理器
func NewConfigGenHandler(configGenSvc *configgen.Service, autoGenerator *configgen.AutoGenerator, agentRepo *repo.AgentConfigRepository, baseAgentRepo *repo.BaseAgentRepository) *ConfigGenHandler {
	return &ConfigGenHandler{
		configGenSvc:  configGenSvc,
		autoGenerator: autoGenerator,
		agentRepo:     agentRepo,
		baseAgentRepo: baseAgentRepo,
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

	// 动态验证 baseAgentType（从插件注册中心获取支持的类型）
	validTypes := agent.GetTypes()
	typeValid := false
	for _, t := range validTypes {
		if string(t.Type) == req.BaseAgentType {
			typeValid = true
			break
		}
	}
	if !typeValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("不支持的 baseAgentType: %s", req.BaseAgentType)})
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

	// 动态验证 baseAgentType（从插件注册中心获取支持的类型）
	validTypes := agent.GetTypes()
	typeValid := false
	for _, t := range validTypes {
		if string(t.Type) == req.BaseAgentType {
			typeValid = true
			break
		}
	}
	if !typeValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("不支持的 baseAgentType: %s", req.BaseAgentType)})
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

// RefreshAgentConfig 刷新Agent角色配置（自动检测类型）
// POST /agents/:id/config/refresh
// 无需传递 baseAgentType，自动从角色配置中获取
func (h *ConfigGenHandler) RefreshAgentConfig(c *gin.Context) {
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

	// 使用 AutoGenerator 自动生成（无需指定类型）
	if h.autoGenerator == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AutoGenerator 未初始化"})
		return
	}

	// 获取角色配置以返回详细信息
	agentRole, err := h.agentRepo.FindByID(c.Request.Context(), agentUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取角色配置失败"})
		return
	}

	// 执行生成
	if err := h.autoGenerator.GenerateSync(c.Request.Context(), agentUUID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取生成后的配置路径
	agentRole, _ = h.agentRepo.FindByID(c.Request.Context(), agentUUID)

	c.JSON(http.StatusOK, gin.H{
		"message":    "配置生成成功",
		"agentId":    agentID,
		"agentName":  agentRole.Name,
		"configPath": agentRole.ConfigPath,
	})
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
		agents.POST("/:id/config/refresh", h.RefreshAgentConfig) // 新增：自动检测类型
	}
}