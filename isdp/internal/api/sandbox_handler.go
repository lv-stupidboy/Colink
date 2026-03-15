package api

import (
	"fmt"
	"net/http"

	"github.com/anthropic/isdp/internal/service/sandbox"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SandboxHandler 沙箱API处理器
type SandboxHandler struct {
	sandboxSvc *sandbox.SandboxService
}

// NewSandboxHandler 创建处理器
func NewSandboxHandler(sandboxSvc *sandbox.SandboxService) *SandboxHandler {
	return &SandboxHandler{sandboxSvc: sandboxSvc}
}

// RunProject 运行项目到沙箱
func (h *SandboxHandler) RunProject(c *gin.Context) {
	var req struct {
		ThreadID    string `json:"thread_id"`
		ProjectPath string `json:"project_path"`
		Mode        string `json:"mode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fmt.Printf("[RunProject] Request: thread_id=%s, project_path=%s, mode=%s\n", req.ThreadID, req.ProjectPath, req.Mode)

	// threadId 是可选的，没有时生成一个新的
	var threadID uuid.UUID
	var err error
	if req.ThreadID != "" {
		threadID, err = uuid.Parse(req.ThreadID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
			return
		}
	} else {
		threadID = uuid.New()
	}

	mode := sandbox.RunMode(req.Mode)
	if mode == "" {
		mode = sandbox.RunModeLocal
	}

	// 如果没有指定项目路径，使用threadId作为路径
	projectPath := req.ProjectPath
	if projectPath == "" {
		projectPath = threadID.String()
	}

	result, err := h.sandboxSvc.RunProject(c.Request.Context(), &sandbox.RunProjectRequest{
		ThreadID:    threadID,
		ProjectPath: projectPath,
		Mode:        mode,
	})
	if err != nil {
		// 打印详细错误日志
		fmt.Printf("[RunProject] Error: %v\n", err)
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetServer 获取沙箱服务状态
func (h *SandboxHandler) GetServer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	server, err := h.sandboxSvc.GetProjectServer(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          server.ID,
		"threadId":    server.ThreadID,
		"projectPath": server.ProjectPath,
		"mode":        server.Mode,
		"port":        server.Port,
		"url":         server.URL,
		"status":      server.Status,
		"startedAt":   server.StartedAt,
		"containerId": server.ContainerID,
	})
}

// StopServer 停止沙箱服务
func (h *SandboxHandler) StopServer(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.sandboxSvc.StopProject(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetPreviewByThread 按Thread获取预览URL
func (h *SandboxHandler) GetPreviewByThread(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	server, err := h.sandboxSvc.GetProjectServerByThread(c.Request.Context(), threadID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active server for this thread"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          server.ID,
		"threadId":    server.ThreadID,
		"projectPath": server.ProjectPath,
		"port":        server.Port,
		"url":         server.URL,
		"status":      server.Status,
	})
}

// ListServers 列出所有活跃的服务
func (h *SandboxHandler) ListServers(c *gin.Context) {
	servers := h.sandboxSvc.ListProjectServers()

	result := make([]gin.H, len(servers))
	for i, server := range servers {
		result[i] = gin.H{
			"id":          server.ID,
			"threadId":    server.ThreadID,
			"projectPath": server.ProjectPath,
			"mode":        server.Mode,
			"port":        server.Port,
			"url":         server.URL,
			"status":      server.Status,
			"startedAt":   server.StartedAt,
			"containerId": server.ContainerID,
		}
	}

	c.JSON(http.StatusOK, result)
}

// GetLogs 获取沙箱日志
func (h *SandboxHandler) GetLogs(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	logs, err := h.sandboxSvc.GetServerLogs(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// CheckDocker 检查Docker可用性
func (h *SandboxHandler) CheckDocker(c *gin.Context) {
	available := h.sandboxSvc.IsDockerAvailable()
	c.JSON(http.StatusOK, gin.H{"available": available})
}

// RegisterRoutes 注册路由
func (h *SandboxHandler) RegisterRoutes(r *gin.RouterGroup) {
	sandbox := r.Group("/sandbox")
	{
		sandbox.GET("", h.ListServers)
		sandbox.POST("/run", h.RunProject)
		sandbox.GET("/docker/status", h.CheckDocker)
		sandbox.GET("/:id", h.GetServer)
		sandbox.GET("/:id/logs", h.GetLogs)
		sandbox.POST("/:id/stop", h.StopServer)
		sandbox.GET("/preview/:threadId", h.GetPreviewByThread)
	}
}