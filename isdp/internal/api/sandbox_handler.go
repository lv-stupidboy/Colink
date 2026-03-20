package api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

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

	// 返回代理URL而不是localhost URL，这样前端可以通过后端代理访问沙箱
	proxyURL := fmt.Sprintf("/api/v1/sandbox/%s/proxy/", result.ID.String())

	c.JSON(http.StatusOK, gin.H{
		"id":          result.ID,
		"threadId":    result.ThreadID,
		"projectPath": result.ProjectPath,
		"mode":        result.Mode,
		"port":        result.Port,
		"url":         proxyURL, // 使用代理URL
		"localUrl":    result.URL, // 保留原始localUrl供调试
		"status":      result.Status,
	})
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

	// 返回代理URL
	proxyURL := fmt.Sprintf("/api/v1/sandbox/%s/proxy/", server.ID.String())

	c.JSON(http.StatusOK, gin.H{
		"id":          server.ID,
		"threadId":    server.ThreadID,
		"projectPath": server.ProjectPath,
		"mode":        server.Mode,
		"port":        server.Port,
		"url":         proxyURL,
		"localUrl":    server.URL,
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

	// 返回代理URL
	proxyURL := fmt.Sprintf("/api/v1/sandbox/%s/proxy/", server.ID.String())

	c.JSON(http.StatusOK, gin.H{
		"id":          server.ID,
		"threadId":    server.ThreadID,
		"projectPath": server.ProjectPath,
		"port":        server.Port,
		"url":         proxyURL,
		"localUrl":    server.URL,
		"status":      server.Status,
	})
}

// ListServers 列出所有活跃的服务
func (h *SandboxHandler) ListServers(c *gin.Context) {
	servers := h.sandboxSvc.ListProjectServers()

	result := make([]gin.H, len(servers))
	for i, server := range servers {
		// 返回代理URL
		proxyURL := fmt.Sprintf("/api/v1/sandbox/%s/proxy/", server.ID.String())
		result[i] = gin.H{
			"id":          server.ID,
			"threadId":    server.ThreadID,
			"projectPath": server.ProjectPath,
			"mode":        server.Mode,
			"port":        server.Port,
			"url":         proxyURL,
			"localUrl":    server.URL,
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

// ProxySandbox 代理沙箱请求（用于在iframe中展示本地沙箱内容）
func (h *SandboxHandler) ProxySandbox(c *gin.Context) {
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

	// 构建目标URL
	targetURL := fmt.Sprintf("http://localhost:%d", server.Port)

	// 获取剩余路径
	remainingPath := c.Param("path")
	if remainingPath != "" {
		targetURL += remainingPath
	}

	// 添加查询参数
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	// 创建代理请求
	proxyReq, err := http.NewRequest(c.Request.Method, targetURL, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create proxy request"})
		return
	}

	// 复制请求头
	for key, values := range c.Request.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to connect to sandbox"})
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// 设置状态码
	c.Status(resp.StatusCode)

	// 复制响应体
	io.Copy(c.Writer, resp.Body)
}

// ProxySandboxWebSocket 代理WebSocket连接
func (h *SandboxHandler) ProxySandboxWebSocket(c *gin.Context) {
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

	// 构建目标WebSocket URL
	targetURL := fmt.Sprintf("ws://localhost:%d", server.Port)
	remainingPath := c.Param("path")
	if remainingPath != "" {
		targetURL += remainingPath
	}
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	// 验证URL格式
	if _, err := url.Parse(targetURL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid target URL"})
		return
	}

	// 这里简化处理，返回目标URL让前端直接连接
	// 完整的WebSocket代理需要更复杂的实现
	c.JSON(http.StatusOK, gin.H{
		"targetUrl": targetURL,
		"port":      server.Port,
	})
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
		// 沙箱代理路由 - 用于在iframe中展示本地沙箱
		sandbox.Any("/:id/proxy/*path", h.ProxySandbox)
	}
}

// GetProxyURL 获取沙箱代理URL（供前端使用）
func (h *SandboxHandler) GetProxyURL(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// 返回代理URL
	proxyURL := fmt.Sprintf("/api/v1/sandbox/%s/proxy/", id.String())
	c.JSON(http.StatusOK, gin.H{
		"proxyUrl": proxyURL,
		"id":       id.String(),
	})
}