package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/configgen"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// configGenAdapter wraps configgen.Service to implement agent.ConfigGenerator
type configGenAdapter struct {
	service *configgen.Service
}

// newConfigGenAdapter creates a new adapter
func newConfigGenAdapter(service *configgen.Service) *configGenAdapter {
	return &configGenAdapter{service: service}
}

// GenerateAgentConfig implements agent.ConfigGenerator interface
func (a *configGenAdapter) GenerateAgentConfig(ctx context.Context, agentId uuid.UUID, cliType string) (
	skillsCount, commandsCount, subagentsCount, rulesCount, settingsCount int, err error) {

	req := &configgen.GenerateAgentConfigRequest{
		AgentRoleID:   agentId,
		BaseAgentType: cliType,
		CleanExisting: true,
	}
	result, err := a.service.GenerateAgentConfig(ctx, req)
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	return result.SkillsCount, result.CommandsCount, result.SubagentsCount,
		result.RulesCount, result.SettingsCount, nil
}

// AgentHandler Agent配置API处理器
type AgentHandler struct {
	configSvc        *agent.ConfigService
	baseAgentSvc     *agent.BaseAgentService
	orchestrator     *agent.Orchestrator
	threadRepo       *repo.ThreadRepository
	workflowRepo     *repo.WorkflowTemplateRepository
	configGenService *configgen.Service       // 配置生成服务
	autoGenerator    *configgen.AutoGenerator // 自动配置生成器
	// 绑定关系 repository
	agentSkillBindingRepo    *repo.AgentSkillBindingRepository
	agentSubagentBindingRepo *repo.AgentSubagentBindingRepository
	agentCommandBindingRepo  *repo.AgentCommandBindingRepository
	agentRuleBindingRepo     *repo.AgentRuleBindingRepository
	agentSettingsBindingRepo *repo.AgentSettingsBindingRepository
}

// NewAgentHandler 创建处理器
func NewAgentHandler(
	configSvc *agent.ConfigService,
	baseAgentSvc *agent.BaseAgentService,
	orchestrator *agent.Orchestrator,
	threadRepo *repo.ThreadRepository,
	workflowRepo *repo.WorkflowTemplateRepository,
	configGenService *configgen.Service, // 配置生成服务
	autoGenerator *configgen.AutoGenerator, // 自动配置生成器
	agentSkillBindingRepo *repo.AgentSkillBindingRepository,
	agentSubagentBindingRepo *repo.AgentSubagentBindingRepository,
	agentCommandBindingRepo *repo.AgentCommandBindingRepository,
	agentRuleBindingRepo *repo.AgentRuleBindingRepository,
	agentSettingsBindingRepo *repo.AgentSettingsBindingRepository,
) *AgentHandler {
	return &AgentHandler{
		configSvc:                configSvc,
		baseAgentSvc:             baseAgentSvc,
		orchestrator:             orchestrator,
		threadRepo:               threadRepo,
		workflowRepo:             workflowRepo,
		configGenService:         configGenService,
		autoGenerator:            autoGenerator,
		agentSkillBindingRepo:    agentSkillBindingRepo,
		agentSubagentBindingRepo: agentSubagentBindingRepo,
		agentCommandBindingRepo:  agentCommandBindingRepo,
		agentRuleBindingRepo:     agentRuleBindingRepo,
		agentSettingsBindingRepo: agentSettingsBindingRepo,
	}
}

// List 列出所有配置
func (h *AgentHandler) List(c *gin.Context) {
	configs, err := h.configSvc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, configs)
}

// Get 获取配置
func (h *AgentHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	config, err := h.configSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
		return
	}
	c.JSON(http.StatusOK, config)
}

// GetByRole 按角色获取配置
func (h *AgentHandler) GetByRole(c *gin.Context) {
	role := model.AgentRole(c.Param("role"))
	configs, err := h.configSvc.GetByRole(c.Request.Context(), role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, configs)
}

// Create 创建配置
func (h *AgentHandler) Create(c *gin.Context) {
	var req model.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.configSvc.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 前端会紧接着调用绑定API，由绑定API触发配置生成
	// 避免重复生成（创建 + 多个绑定 = N次生成）

	c.JSON(http.StatusCreated, config)
}

// Update 更新配置
func (h *AgentHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config, err := h.configSvc.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 不在此处自动生成配置，因为前端会紧接着调用绑定API
	// 绑定API会触发配置生成，避免重复生成

	c.JSON(http.StatusOK, config)
}

// Delete 删除配置
func (h *AgentHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// 检查是否被工作流引用
	templates, err := h.workflowRepo.FindByAgentID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check references"})
		return
	}

	if len(templates) > 0 {
		// 提取模板名称
		var names []string
		for _, t := range templates {
			names = append(names, t.Name)
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          "agent is referenced by workflow templates",
			"referenced":     true,
			"referenceNames": names,
		})
		return
	}

	if err := h.configSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// BatchDeleteRequest 批量删除请求
type BatchDeleteRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

// ReferencedAgentInfo 被引用的Agent信息
type ReferencedAgentInfo struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	WorkflowNames []string `json:"workflowNames"`
}

// BatchDelete 批量删除配置
func (h *AgentHandler) BatchDelete(c *gin.Context) {
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids is required"})
		return
	}

	ctx := c.Request.Context()
	var referencedAgents []ReferencedAgentInfo
	var validIDs []uuid.UUID

	// 1. 解析并验证所有 ID，检查系统角色和引用状态
	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue // 忽略无效 ID
		}

		// 获取配置检查是否为系统角色
		config, err := h.configSvc.GetByID(ctx, id)
		if err != nil {
			continue // 忽略不存在的 ID
		}

		if config.IsSystem {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":           "系统预置角色不可删除",
				"hasSystemAgent":  true,
				"systemAgentName": config.Name,
			})
			return
		}

		// 检查工作流引用
		templates, err := h.workflowRepo.FindByAgentID(ctx, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check references"})
			return
		}

		if len(templates) > 0 {
			var names []string
			for _, t := range templates {
				names = append(names, t.Name)
			}
			referencedAgents = append(referencedAgents, ReferencedAgentInfo{
				ID:            idStr,
				Name:          config.Name,
				WorkflowNames: names,
			})
		} else {
			validIDs = append(validIDs, id)
		}
	}

	// 2. 任一有引用则拒绝整个操作
	if len(referencedAgents) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":            "部分Agent被工作流引用，无法删除",
			"referencedAgents": referencedAgents,
		})
		return
	}

	// 3. 执行批量删除
	for _, id := range validIDs {
		if err := h.configSvc.Delete(ctx, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// 清理绑定关系
		if h.agentSkillBindingRepo != nil {
			h.agentSkillBindingRepo.DeleteByAgentRoleID(ctx, id)
		}
		if h.agentSubagentBindingRepo != nil {
			h.agentSubagentBindingRepo.DeleteByAgentRoleID(ctx, id)
		}
		if h.agentCommandBindingRepo != nil {
			h.agentCommandBindingRepo.DeleteByAgentRoleID(ctx, id)
		}
		if h.agentRuleBindingRepo != nil {
			h.agentRuleBindingRepo.DeleteByAgentRoleID(ctx, id)
		}
		if h.agentSettingsBindingRepo != nil {
			h.agentSettingsBindingRepo.DeleteByAgentRoleID(ctx, id)
		}
	}

	c.Status(http.StatusNoContent)
}

// CheckReferences 检查Agent是否被工作流引用
func (h *AgentHandler) CheckReferences(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	templates, err := h.workflowRepo.FindByAgentID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 提取模板名称列表
	var refNames []string
	for _, t := range templates {
		refNames = append(refNames, t.Name)
	}

	c.JSON(http.StatusOK, gin.H{
		"referenced":       len(templates) > 0,
		"referenceCount":   len(templates),
		"referenceNames":   refNames,
		"referenceDetails": templates,
	})
}

// Copy 复制角色
func (h *AgentHandler) Copy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// 获取原始配置
	original, err := h.configSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
		return
	}

	// 创建副本
	copyReq := &model.CreateAgentRequest{
		Name:            original.Name + " (副本)",
		Role:            original.Role,
		BaseAgentID:     original.BaseAgentID,
		Description:     original.Description,
		SystemPrompt:    original.SystemPrompt,
		MaxTokens:       original.MaxTokens,
		Temperature:     original.Temperature,
		IsDefault:       false, // 副本不设为默认
		MentionPatterns: original.MentionPatterns,
	}

	copy, err := h.configSvc.Create(c.Request.Context(), copyReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 复制绑定关系
	ctx := c.Request.Context()
	newID := copy.ID
	now := time.Now()

	// 1. 复制 Skill 绑定
	if h.agentSkillBindingRepo != nil {
		skillIDs, err := h.agentSkillBindingRepo.FindByAgentRoleID(ctx, id)
		if err == nil {
			for _, skillID := range skillIDs {
				binding := &model.AgentSkillBinding{
					ID:          uuid.New(),
					AgentRoleID: newID,
					SkillID:     skillID,
					CreatedAt:   now,
				}
				h.agentSkillBindingRepo.Create(ctx, binding)
			}
		}
	}

	// 2. 复制 Subagent 绑定
	if h.agentSubagentBindingRepo != nil {
		subagentIDs, err := h.agentSubagentBindingRepo.FindByAgentRoleID(ctx, id)
		if err == nil {
			for _, subagentID := range subagentIDs {
				binding := &model.AgentSubagentBinding{
					ID:          uuid.New(),
					AgentRoleID: newID,
					SubagentID:  subagentID,
					CreatedAt:   now,
				}
				h.agentSubagentBindingRepo.Create(ctx, binding)
			}
		}
	}

	// 3. 复制 Command 绑定
	if h.agentCommandBindingRepo != nil {
		commandIDs, err := h.agentCommandBindingRepo.FindByAgentRoleID(ctx, id)
		if err == nil {
			for _, commandID := range commandIDs {
				binding := &model.AgentCommandBinding{
					ID:          uuid.New(),
					AgentRoleID: newID,
					CommandID:   commandID,
					CreatedAt:   now,
				}
				h.agentCommandBindingRepo.Create(ctx, binding)
			}
		}
	}

	// 4. 复制 Rule 绑定
	if h.agentRuleBindingRepo != nil {
		ruleIDs, err := h.agentRuleBindingRepo.FindByAgentRoleID(ctx, id)
		if err == nil {
			for _, ruleID := range ruleIDs {
				binding := &model.AgentRuleBinding{
					ID:          uuid.New(),
					AgentRoleID: newID,
					RuleID:      ruleID,
					CreatedAt:   now,
				}
				h.agentRuleBindingRepo.Create(ctx, binding)
			}
		}
	}

	// 自动生成配置（复制角色后）
	if h.autoGenerator != nil {
		if err := h.autoGenerator.GenerateSync(ctx, newID); err != nil {
			// 生成失败不影响角色复制，仅记录日志
		}
	}

	c.JSON(http.StatusCreated, copy)
}

// SubmitQuestionAnswer 提交 AskUserQuestion 的用户答案
func (h *AgentHandler) SubmitQuestionAnswer(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	var req struct {
		ToolCallID string `json:"toolCallId" binding:"required"`
		Answer     string `json:"answer" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.orchestrator.SubmitQuestionAnswer(threadID, req.ToolCallID, req.Answer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// BatchGenerateConfig 批量生成配置
func (h *AgentHandler) BatchGenerateConfig(c *gin.Context) {
	var req model.BatchGenerateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 解析UUID
	agentIds := make([]uuid.UUID, len(req.AgentIds))
	for i, idStr := range req.AgentIds {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的Agent ID: %s", idStr)})
			return
		}
		agentIds[i] = id
	}

	cliType := req.CliType
	if cliType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cliType 不能为空"})
		return
	}

	// 动态验证 cliType（从插件注册中心获取支持的类型）
	validTypes := agent.GetTypes()
	typeValid := false
	for _, t := range validTypes {
		if string(t.Type) == cliType {
			typeValid = true
			break
		}
	}
	if !typeValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("不支持的 cliType: %s", cliType)})
		return
	}

	// 创建adapter
	adapter := newConfigGenAdapter(h.configGenService)

	result, err := h.configSvc.BatchGenerateConfig(c.Request.Context(), agentIds, cliType, adapter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// BatchUpdateBaseAgent 批量修改基础Agent
func (h *AgentHandler) BatchUpdateBaseAgent(c *gin.Context) {
	var req model.BatchUpdateBaseAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 解析UUID
	agentIds := make([]uuid.UUID, len(req.AgentIds))
	for i, idStr := range req.AgentIds {
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的Agent ID: %s", idStr)})
			return
		}
		agentIds[i] = id
	}

	baseAgentId, err := uuid.Parse(req.BaseAgentId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的基础Agent ID"})
		return
	}

	result, err := h.configSvc.BatchUpdateBaseAgent(c.Request.Context(), agentIds, baseAgentId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// RefreshConfig 刷新配置（自动检测类型）
func (h *AgentHandler) RefreshConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if h.autoGenerator == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AutoGenerator 未初始化"})
		return
	}

	// 获取角色配置以返回详细信息
	agentRole, err := h.configSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取角色配置失败"})
		return
	}

	// 执行生成
	if err := h.autoGenerator.GenerateSync(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取生成后的配置路径
	agentRole, _ = h.configSvc.GetByID(c.Request.Context(), id)

	c.JSON(http.StatusOK, gin.H{
		"message":    "配置生成成功",
		"agentId":    id.String(),
		"agentName":  agentRole.Name,
		"configPath": agentRole.ConfigPath,
	})
}

// RegisterRoutes 注册路由
func (h *AgentHandler) RegisterRoutes(r *gin.RouterGroup) {
	agents := r.Group("/agents")
	{
		agents.GET("", h.List)
		agents.POST("", h.Create)
		// 注意：具体路由必须在参数化路由之前注册
		agents.GET("/role/:role", h.GetByRole)
		agents.POST("/question/:threadId/answer", h.SubmitQuestionAnswer) // AskUserQuestion 答案提交
		agents.POST("/batch-generate-config", h.BatchGenerateConfig)      // 批量生成配置
		agents.POST("/batch-update-base-agent", h.BatchUpdateBaseAgent)   // 批量修改基础Agent
		agents.POST("/batch-delete", h.BatchDelete)
		// 注意：带后缀的路由必须在通用 :id 路由之前注册
		agents.POST("/:id/refs", h.CheckReferences)
		agents.POST("/:id/copy", h.Copy)
		agents.POST("/:id/refresh", h.RefreshConfig) // 刷新配置（自动检测类型）
		agents.GET("/:id", h.Get)
		agents.PUT("/:id", h.Update)
		agents.DELETE("/:id", h.Delete)
	}
}
