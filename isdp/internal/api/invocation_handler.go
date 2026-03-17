package api

import (
	"fmt"
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/service/a2a"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// InvocationHandler Agent调用API处理器
type InvocationHandler struct {
	orchestrator *agent.Orchestrator
	mcpAuth      *a2a.MCPAuthService
	projectRepo  *repo.ProjectRepository
}

// NewInvocationHandler 创建处理器
func NewInvocationHandler(orchestrator *agent.Orchestrator, mcpAuth *a2a.MCPAuthService, projectRepo *repo.ProjectRepository) *InvocationHandler {
	return &InvocationHandler{
		orchestrator: orchestrator,
		mcpAuth:      mcpAuth,
		projectRepo:  projectRepo,
	}
}

// Spawn 启动Agent
func (h *InvocationHandler) Spawn(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	var req SpawnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取绑定的项目路径
	var projectPath string
	if h.projectRepo != nil {
		project, err := h.projectRepo.GetByThreadID(c.Request.Context(), threadID)
		if err == nil && project != nil {
			projectPath = project.LocalPath
		}
	}

	// Debug logging
	fmt.Printf("[DEBUG] Spawn request: threadID=%s, role=%s, configID=%s, input=%s, projectPath=%s\n",
		threadID, req.Role, req.ConfigID, req.Input, projectPath)

	spawnReq := &agent.SpawnRequest{
		ThreadID:    threadID,
		Role:        req.Role,
		Input:       req.Input,
		ProjectPath: projectPath,
	}

	if req.ConfigID != "" {
		configID, err := uuid.Parse(req.ConfigID)
		if err != nil {
			fmt.Printf("[DEBUG] Failed to parse configID: %v\n", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config_id"})
			return
		}
		spawnReq.ConfigID = configID
	}

	invocation, err := h.orchestrator.SpawnAgent(c.Request.Context(), spawnReq)
	if err != nil {
		fmt.Printf("[DEBUG] SpawnAgent failed: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, invocation)
}

// Cancel 取消调用
func (h *InvocationHandler) Cancel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.orchestrator.CancelAgent(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}

// Get 获取单个调用状态
func (h *InvocationHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	invocation, err := h.orchestrator.GetInvocationStatus(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invocation not found"})
		return
	}
	c.JSON(http.StatusOK, invocation)
}

// MCPCallback MCP回调
func (h *InvocationHandler) MCPCallback(c *gin.Context) {
	var req a2a.MCPCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 处理回调
	// 实际实现中需要调用具体的处理器
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ListByThread 列出 Thread 的 Agent 调用
func (h *InvocationHandler) ListByThread(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	// 获取该 Thread 的所有 Agent 调用
	invocations, err := h.orchestrator.GetInvocationsByThread(c.Request.Context(), threadID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if invocations == nil {
		invocations = []model.AgentInvocation{}
	}
	c.JSON(http.StatusOK, invocations)
}

// SpawnRequest 启动请求
type SpawnRequest struct {
	ConfigID string          `json:"config_id"`
	Role     model.AgentRole `json:"role" binding:"required"`
	Input    string          `json:"input" binding:"required"`
}

// RegisterRoutes 注册路由
func (h *InvocationHandler) RegisterRoutes(r *gin.RouterGroup) {
	// 注意：必须先注册具体路径，再注册通配路径，避免路由冲突
	threads := r.Group("/threads")
	{
		// 先注册具体路径（带固定后缀的）
		threads.GET("/:id/invocations", h.ListByThread)
		threads.POST("/:id/invocations", h.Spawn)
	}

	invocations := r.Group("/invocations")
	{
		invocations.GET("/:id", h.Get)
		invocations.POST("/:id/cancel", h.Cancel)
	}

	// Thread 路由必须在最后注册，避免与 /:threadId/invocations 冲突
	// 这部分在 thread_handler.go 中注册
}