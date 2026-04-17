package api

import (
	"net/http"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AgentHandler Agent配置API处理器
type AgentHandler struct {
	configSvc       *agent.ConfigService
	baseAgentSvc    *agent.BaseAgentService
	orchestrator    *agent.Orchestrator
	threadRepo      *repo.ThreadRepository
	debugThreadMgr  *agent.DebugThreadManager // 调试线程管理器
	workflowRepo    *repo.WorkflowTemplateRepository
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
	debugThreadMgr *agent.DebugThreadManager, // 新增
	workflowRepo *repo.WorkflowTemplateRepository,
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
		debugThreadMgr:           debugThreadMgr,
		workflowRepo:             workflowRepo,
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
				"error":            "系统预置角色不可删除",
				"hasSystemAgent":   true,
				"systemAgentName":  config.Name,
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
		Name:           original.Name + " (副本)",
		Role:           original.Role,
		BaseAgentID:    original.BaseAgentID,
		Description:    original.Description,
		SystemPrompt:   original.SystemPrompt,
		MaxTokens:      original.MaxTokens,
		Temperature:    original.Temperature,
		IsDefault:      false, // 副本不设为默认
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

	c.JSON(http.StatusCreated, copy)
}

// DebugRequest 调试请求
type DebugRequest struct {
	Input       string `json:"input" binding:"required"`
	ProjectPath string `json:"projectPath"`
	ThreadID    string `json:"threadId"` // 前端传入的预创建threadId，用于WebSocket已连接的场景
}

// DebugResponse 调试响应
type DebugResponse struct {
	InvocationID string `json:"invocationId"`
	ThreadID     string `json:"threadId"` // 添加 threadId，前端用这个订阅 WebSocket
	Output       string `json:"output"`
	SandboxURL   string `json:"sandboxUrl,omitempty"`
}

// CreateDebugThreadRequest 创建调试Thread请求
type CreateDebugThreadRequest struct {
	ProjectPath string `json:"projectPath"`
}

// CreateDebugThreadResponse 创建调试Thread响应
type CreateDebugThreadResponse struct {
	ThreadID string `json:"threadId"`
}

// CreateDebugThread 预创建调试Thread - 完全内存操作
func (h *AgentHandler) CreateDebugThread(c *gin.Context) {
	var req CreateDebugThreadRequest
	projectPath := ""
	if err := c.ShouldBindJSON(&req); err == nil {
		projectPath = req.ProjectPath
	}

	thread := h.debugThreadMgr.CreateThread(projectPath)
	c.JSON(http.StatusOK, &CreateDebugThreadResponse{
		ThreadID: thread.ID.String(),
	})
}

// Debug 调试Agent - 启动交互式会话
func (h *AgentHandler) Debug(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req DebugRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取Agent配置
	config, err := h.configSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "config not found"})
		return
	}

	// 解析或创建调试线程
	var debugThreadID uuid.UUID
	if req.ThreadID != "" {
		debugThreadID, err = uuid.Parse(req.ThreadID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid threadId"})
			return
		}
		// 验证线程存在
		if h.debugThreadMgr.GetThread(debugThreadID) == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "debug thread not found"})
			return
		}
		// 如果传递了新的 ProjectPath，更新线程的工作目录
		if req.ProjectPath != "" {
			h.debugThreadMgr.SetProjectPath(debugThreadID, req.ProjectPath)
		}
	} else {
		thread := h.debugThreadMgr.CreateThread(req.ProjectPath)
		debugThreadID = thread.ID
	}

	// 启动Agent执行
	invocation, err := h.orchestrator.SpawnDebugAgent(c.Request.Context(), &agent.SpawnRequest{
		ThreadID:    debugThreadID,
		ConfigID:    config.ID,
		Role:        config.Role,
		Input:       req.Input,
		ProjectPath: req.ProjectPath,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, &DebugResponse{
		InvocationID: invocation.ID.String(),
		ThreadID:     debugThreadID.String(),
	})
}

// ContinueDebugRequest 继续调试请求
type ContinueDebugRequest struct {
	Message string `json:"message" binding:"required"`
}

// ContinueDebug 继续调试会话 - 发送消息到正在运行的会话
func (h *AgentHandler) ContinueDebug(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	// 验证是调试线程
	if h.debugThreadMgr.GetThread(threadID) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "debug thread not found"})
		return
	}

	var req ContinueDebugRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.orchestrator.ContinueDebugAgent(c.Request.Context(), threadID, req.Message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "sent"})
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

// RegisterRoutes 注册路由
func (h *AgentHandler) RegisterRoutes(r *gin.RouterGroup) {
	agents := r.Group("/agents")
	{
		agents.GET("", h.List)
		agents.POST("", h.Create)
		// 注意：具体路由必须在参数化路由之前注册
		agents.GET("/role/:role", h.GetByRole)
		agents.POST("/debug/thread", h.CreateDebugThread) // 预创建调试Thread
		agents.POST("/debug/:threadId/continue", h.ContinueDebug)
		agents.POST("/question/:threadId/answer", h.SubmitQuestionAnswer) // AskUserQuestion 答案提交
		agents.GET("/:id", h.Get)
		agents.PUT("/:id", h.Update)
		agents.DELETE("/:id", h.Delete)
		agents.POST("/:id/refs", h.CheckReferences)
		agents.POST("/batch-delete", h.BatchDelete)
		agents.POST("/:id/copy", h.Copy)
		agents.POST("/:id/debug", h.Debug)
	}
}