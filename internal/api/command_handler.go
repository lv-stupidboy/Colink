package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/command"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CommandHandler Command API处理器
type CommandHandler struct {
	svc         *command.Service
	storagePath string
	maxSize     int64
}

// NewCommandHandler 创建CommandHandler
func NewCommandHandler(svc *command.Service, storagePath string, maxSize int64) *CommandHandler {
	return &CommandHandler{
		svc:         svc,
		storagePath: storagePath,
		maxSize:     maxSize,
	}
}

// List 列出所有Commands
func (h *CommandHandler) List(c *gin.Context) {
	var query model.CommandListQuery
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

	commands, total, err := h.svc.List(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       commands,
		"total":      total,
		"page":       query.Page,
		"page_size":  query.PageSize,
	})
}

// Get 获取Command详情
func (h *CommandHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	cmd, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "command not found"})
		return
	}

	// 读取文件内容
	if h.storagePath != "" && cmd.Name != "" {
		filePath := filepath.Join(h.storagePath, cmd.Name+".md")
		if content, err := os.ReadFile(filePath); err == nil {
			cmd.Content = string(content)
		}
	}

	c.JSON(http.StatusOK, cmd)
}

// Create 创建Command
func (h *CommandHandler) Create(c *gin.Context) {
	var req model.CreateCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 校验名称格式
	if !isValidCommandName(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称只能包含小写字母、数字和中划线，且必须以字母开头"})
		return
	}

	cmd, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		if err == command.ErrCommandNameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "命令名称已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 如果提供了内容，保存到文件
	if req.Content != "" && h.storagePath != "" {
		if err := os.MkdirAll(h.storagePath, 0755); err == nil {
			filePath := filepath.Join(h.storagePath, cmd.Name+".md")
			os.WriteFile(filePath, []byte(req.Content), 0644)
		}
	}

	c.JSON(http.StatusCreated, cmd)
}

// Update 更新Command
func (h *CommandHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.UpdateCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cmd, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cmd)
}

// Delete 删除Command
func (h *CommandHandler) Delete(c *gin.Context) {
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

// isValidCommandName 校验命令名称格式
// 规则：只能包含小写字母、数字和中划线，且必须以字母开头
func isValidCommandName(name string) bool {
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

// GetSkills 获取Command绑定的技能
func (h *CommandHandler) GetSkills(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	skills, err := h.svc.GetSkills(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"skills": skills,
		"count":  len(skills),
	})
}

// BindSkills 绑定技能到Command
func (h *CommandHandler) BindSkills(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.BindSkillsToCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.BindSkills(c.Request.Context(), id, req.SkillIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// UnbindSkill 解绑技能
func (h *CommandHandler) UnbindSkill(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	skillID, err := uuid.Parse(c.Param("skill_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid skill id"})
		return
	}

	if err := h.svc.UnbindSkill(c.Request.Context(), id, skillID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetAgentCommands 获取Agent绑定的Commands
func (h *CommandHandler) GetAgentCommands(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	commands, err := h.svc.GetAgentCommands(c.Request.Context(), agentRoleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"commands": commands,
		"count":    len(commands),
	})
}

// BindCommands 绑定Commands到Agent
func (h *CommandHandler) BindCommands(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var req model.BindCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.BindCommandsToAgent(c.Request.Context(), agentRoleID, req.CommandIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// UnbindCommand 解绑Command
func (h *CommandHandler) UnbindCommand(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	commandID, err := uuid.Parse(c.Param("command_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid command id"})
		return
	}

	if err := h.svc.UnbindCommandFromAgent(c.Request.Context(), agentRoleID, commandID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// RegisterRoutes 注册路由
func (h *CommandHandler) RegisterRoutes(r *gin.RouterGroup) {
	// Command CRUD 路由
	commands := r.Group("/commands")
	{
		commands.GET("", h.List)
		commands.POST("", h.Create)
		commands.GET("/:id", h.Get)
		commands.PUT("/:id", h.Update)
		commands.DELETE("/:id", h.Delete)
		commands.GET("/:id/skills", h.GetSkills)
		commands.POST("/:id/skills", h.BindSkills)
		commands.DELETE("/:id/skills/:skill_id", h.UnbindSkill)
	}

	// Agent-Command 绑定路由
	agents := r.Group("/agents")
	{
		agents.GET("/:id/commands", h.GetAgentCommands)
		agents.POST("/:id/commands", h.BindCommands)
		agents.DELETE("/:id/commands/:command_id", h.UnbindCommand)
	}
}