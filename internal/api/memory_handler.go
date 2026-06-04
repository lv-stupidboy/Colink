package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/service/memory"
	"github.com/gin-gonic/gin"
)

type MemoryHandler struct {
	memoryManager *memory.MemoryManager
}

type rawMemoryResponse struct {
	Scope   memory.MemoryScopeIdentity `json:"scope"`
	Team    memory.RawMarkdownGroup    `json:"team"`
	Project memory.RawMarkdownGroup    `json:"project"`
}

func NewMemoryHandler(memoryManager *memory.MemoryManager) *MemoryHandler {
	return &MemoryHandler{
		memoryManager: memoryManager,
	}
}

func (h *MemoryHandler) Raw(c *gin.Context) {
	if h.memoryManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory manager is not initialized"})
		return
	}

	var scope memory.MemoryScopeIdentity
	h.applyScopeQuery(c, &scope)
	h.writeRawMemoryResponse(c, scope)
}

func (h *MemoryHandler) writeRawMemoryResponse(c *gin.Context, scope memory.MemoryScopeIdentity) {
	memoryType := c.DefaultQuery("type", "all")

	response := rawMemoryResponse{Scope: scope}
	if memoryType == "all" || memoryType == "team" {
		team, err := h.memoryManager.ReadRawMarkdown(memory.MemoryTypeTeam, scope)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		response.Team = team
	}
	if memoryType == "all" || memoryType == "project" {
		project, err := h.memoryManager.ReadRawMarkdown(memory.MemoryTypeProject, scope)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		response.Project = project
	}
	if memoryType != "all" && memoryType != "team" && memoryType != "project" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be all, team, or project"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *MemoryHandler) RegisterRoutes(r *gin.RouterGroup) {
	memoryGroup := r.Group("/memory")
	{
		memoryGroup.GET("/raw", h.Raw)
	}
}

func (h *MemoryHandler) applyScopeQuery(c *gin.Context, scope *memory.MemoryScopeIdentity) {
	if value := c.Query("teamId"); value != "" {
		scope.TeamID = value
	}
	if value := c.Query("teamName"); value != "" {
		scope.TeamName = value
	}
	if value := c.Query("projectId"); value != "" {
		scope.ProjectID = value
	}
	if value := c.Query("projectName"); value != "" {
		scope.ProjectName = value
	}
	if value := c.Query("workspacePath"); value != "" {
		scope.WorkspacePath = value
	}
}
