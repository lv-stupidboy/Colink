package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	mcpservice "github.com/anthropic/isdp/internal/service/mcp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MCPHandler MCP server asset API handler.
type MCPHandler struct {
	svc *mcpservice.Service
}

func NewMCPHandler(svc *mcpservice.Service) *MCPHandler {
	return &MCPHandler{svc: svc}
}

func (h *MCPHandler) List(c *gin.Context) {
	var query model.MCPServerListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}

	servers, total, err := h.svc.List(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":     servers,
		"total":    total,
		"page":     query.Page,
		"pageSize": query.PageSize,
	})
}

func (h *MCPHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	server, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "mcp server not found"})
		return
	}
	c.JSON(http.StatusOK, server)
}

func (h *MCPHandler) Create(c *gin.Context) {
	var req model.CreateMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	server, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		if err == mcpservice.ErrMCPServerNameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "MCP Server 名称已存在"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, server)
}

func (h *MCPHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req model.UpdateMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	server, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, server)
}

func (h *MCPHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MCPHandler) GetAgentMCPServers(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}
	servers, err := h.svc.GetAgentBindings(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": servers})
}

func (h *MCPHandler) BindAgentMCPServers(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}
	var req model.BindMCPServersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ReplaceAgentBindings(c.Request.Context(), agentID, req.MCPServerIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "绑定成功"})
}

func (h *MCPHandler) RegisterRoutes(r *gin.RouterGroup) {
	servers := r.Group("/mcp-servers")
	{
		servers.GET("", h.List)
		servers.POST("", h.Create)
		servers.GET("/:id", h.Get)
		servers.PUT("/:id", h.Update)
		servers.DELETE("/:id", h.Delete)
	}

	agents := r.Group("/agents")
	{
		agents.GET("/:id/mcp-servers", h.GetAgentMCPServers)
		agents.PUT("/:id/mcp-servers", h.BindAgentMCPServers)
	}
}
