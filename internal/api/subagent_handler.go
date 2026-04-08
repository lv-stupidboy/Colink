package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/subagent"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SubagentHandler Subagent API处理器
type SubagentHandler struct {
	svc         *subagent.Service
	storagePath string
	uploadMax   int64
}

// NewSubagentHandler 创建SubagentHandler
func NewSubagentHandler(svc *subagent.Service, storagePath string, uploadMax int64) *SubagentHandler {
	return &SubagentHandler{
		svc:         svc,
		storagePath: storagePath,
		uploadMax:   uploadMax,
	}
}

// List 列出所有Subagents
func (h *SubagentHandler) List(c *gin.Context) {
	var query model.SubagentListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置默认分页
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}

	subagents, total, err := h.svc.List(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       subagents,
		"total":      total,
		"page":       query.Page,
		"page_size":  query.PageSize,
	})
}

// Get 获取Subagent详情
func (h *SubagentHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	subagent, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "subagent not found"})
		return
	}

	c.JSON(http.StatusOK, subagent)
}

// Create 创建Subagent
func (h *SubagentHandler) Create(c *gin.Context) {
	var req model.CreateSubagentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 校验名称格式
	if !isValidSubagentName(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称只能包含小写字母、数字和中划线，且必须以字母开头"})
		return
	}

	subagent, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		if err.Error() == "subagent name already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": "subagent name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, subagent)
}

// Update 更新Subagent
func (h *SubagentHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.UpdateSubagentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	subagent, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, subagent)
}

// Delete 删除Subagent
func (h *SubagentHandler) Delete(c *gin.Context) {
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

// GetAgentSubagents 获取Agent绑定的Subagents
func (h *SubagentHandler) GetAgentSubagents(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	subagents, err := h.svc.GetAgentSubagents(c.Request.Context(), agentRoleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subagents": subagents,
		"count":     len(subagents),
	})
}

// BindSubagents 绑定Subagents到Agent
func (h *SubagentHandler) BindSubagents(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var req model.BindSubagentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.BindSubagents(c.Request.Context(), agentRoleID, req.SubagentIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// UnbindSubagent 解绑Subagent
func (h *SubagentHandler) UnbindSubagent(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	subagentID, err := uuid.Parse(c.Param("subagent_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subagent id"})
		return
	}

	if err := h.svc.UnbindSubagent(c.Request.Context(), agentRoleID, subagentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetSkills 获取Subagent绑定的Skills
func (h *SubagentHandler) GetSkills(c *gin.Context) {
	subagentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subagent id"})
		return
	}

	skills, err := h.svc.GetSkills(c.Request.Context(), subagentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"skills": skills,
		"count":  len(skills),
	})
}

// BindSkills 绑定Skills到Subagent
func (h *SubagentHandler) BindSkills(c *gin.Context) {
	subagentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subagent id"})
		return
	}

	var req model.BindSkillsToSubagentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.BindSkills(c.Request.Context(), subagentID, req.SkillIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// UnbindSkill 解绑Subagent的Skill
func (h *SubagentHandler) UnbindSkill(c *gin.Context) {
	subagentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subagent id"})
		return
	}

	skillID, err := uuid.Parse(c.Param("skill_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid skill id"})
		return
	}

	if err := h.svc.UnbindSkill(c.Request.Context(), subagentID, skillID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// isValidSubagentName 校验子代理名称格式
// 规则：只能包含小写字母、数字和中划线，且必须以字母开头
func isValidSubagentName(name string) bool {
	if len(name) == 0 {
		return false
	}
	// 必须以字母开头
	if name[0] < 'a' || name[0] > 'z' {
		return false
	}
	// 只能包含小写字母、数字和中划线
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return true
}

// RegisterRoutes 注册路由
func (h *SubagentHandler) RegisterRoutes(r *gin.RouterGroup) {
	// Subagent CRUD 路由
	subagents := r.Group("/subagents")
	{
		subagents.GET("", h.List)
		subagents.POST("", h.Create)
		subagents.GET("/:id", h.Get)
		subagents.PUT("/:id", h.Update)
		subagents.DELETE("/:id", h.Delete)
		// Subagent-Skill 绑定路由
		subagents.GET("/:id/skills", h.GetSkills)
		subagents.POST("/:id/skills", h.BindSkills)
		subagents.DELETE("/:id/skills/:skill_id", h.UnbindSkill)
	}

	// Agent-Subagent 绑定路由
	agents := r.Group("/agents")
	{
		agents.GET("/:id/subagents", h.GetAgentSubagents)
		agents.POST("/:id/subagents", h.BindSubagents)
		agents.DELETE("/:id/subagents/:subagent_id", h.UnbindSubagent)
	}
}