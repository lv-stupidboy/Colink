package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/configgen"
	"github.com/anthropic/isdp/internal/service/rule"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RuleHandler Rule API处理器
type RuleHandler struct {
	svc           *rule.Service
	storagePath   string
	maxSize       int64
	autoGenerator *configgen.AutoGenerator // 自动配置生成器
	agentRepo     *repo.AgentConfigRepository // 用于获取关联角色信息
}

// NewRuleHandler 创建RuleHandler
func NewRuleHandler(svc *rule.Service, storagePath string, maxSize int64, autoGenerator *configgen.AutoGenerator, agentRepo *repo.AgentConfigRepository) *RuleHandler {
	return &RuleHandler{
		svc:           svc,
		storagePath:   storagePath,
		maxSize:       maxSize,
		autoGenerator: autoGenerator,
		agentRepo:     agentRepo,
	}
}

// List 列出所有Rules
func (h *RuleHandler) List(c *gin.Context) {
	var query model.RuleListQuery
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

	rules, total, err := h.svc.List(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      rules,
		"total":     total,
		"page":      query.Page,
		"page_size": query.PageSize,
	})
}

// Get 获取Rule详情
func (h *RuleHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	r, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
		return
	}

	// 读取文件内容
	if h.storagePath != "" && r.Name != "" {
		filePath := filepath.Join(h.storagePath, r.Name+".md")
		if content, err := os.ReadFile(filePath); err == nil {
			r.Content = string(content)
		}
	}

	c.JSON(http.StatusOK, r)
}

// Create 创建Rule
func (h *RuleHandler) Create(c *gin.Context) {
	var req model.CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 校验名称格式
	if !isValidRuleName(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称只能包含小写字母、数字和中划线，且必须以字母开头"})
		return
	}

	r, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		if err == rule.ErrRuleNameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "规约名称已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 如果提供了内容，保存到文件
	if req.Content != "" && h.storagePath != "" {
		if err := os.MkdirAll(h.storagePath, 0755); err == nil {
			filePath := filepath.Join(h.storagePath, r.Name+".md")
			os.WriteFile(filePath, []byte(req.Content), 0644)
		}
	}

	c.JSON(http.StatusCreated, r)
}

// Update 更新Rule
func (h *RuleHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.UpdateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	r, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取受影响的Agent角色列表
	var affectedAgents []gin.H
	var affectedCount int
	if h.autoGenerator != nil {
		affectedIDs, err := h.autoGenerator.GetAffectedAgentsByRule(c.Request.Context(), id)
		if err == nil && len(affectedIDs) > 0 {
			affectedCount = len(affectedIDs)
			for _, agentID := range affectedIDs {
				agent, err := h.agentRepo.FindByID(c.Request.Context(), agentID)
				if err == nil {
					affectedAgents = append(affectedAgents, gin.H{
						"id":   agent.ID.String(),
						"name": agent.Name,
					})
				}
			}
			// 批量生成配置（后台执行）
			go func() {
				ctx := context.Background()
				h.autoGenerator.GenerateMultiple(ctx, affectedIDs)
			}()
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":              r.ID,
		"name":            r.Name,
		"description":     r.Description,
		"supportedAgents": r.SupportedAgents,
		"updatedAt":       r.UpdatedAt,
		"affectedAgents":  affectedAgents,
		"affectedCount":   affectedCount,
	})
}

// Delete 删除Rule
func (h *RuleHandler) Delete(c *gin.Context) {
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

// isValidRuleName 校验规约名称格式
// 规则：只能包含小写字母、数字和中划线，且必须以字母开头
func isValidRuleName(name string) bool {
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

// GetAgentRules 获取Agent绑定的Rules
func (h *RuleHandler) GetAgentRules(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	rules, err := h.svc.GetAgentRules(c.Request.Context(), agentRoleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"count": len(rules),
	})
}

// BindRules 绑定Rules到Agent
func (h *RuleHandler) BindRules(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var req model.BindRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.BindRulesToAgent(c.Request.Context(), agentRoleID, req.RuleIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 不在此处自动生成，前端批量操作后统一调用 generateConfig

	c.Status(http.StatusNoContent)
}

// UnbindRule 解绑Rule
func (h *RuleHandler) UnbindRule(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	ruleID, err := uuid.Parse(c.Param("rule_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}

	if err := h.svc.UnbindRuleFromAgent(c.Request.Context(), agentRoleID, ruleID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 不在此处自动生成，前端批量操作后统一调用 generateConfig

	c.Status(http.StatusNoContent)
}

// RegisterRoutes 注册路由
func (h *RuleHandler) RegisterRoutes(r *gin.RouterGroup) {
	// Rule CRUD 路由
	rules := r.Group("/rules")
	{
		rules.GET("", h.List)
		rules.POST("", h.Create)
		rules.GET("/:id", h.Get)
		rules.PUT("/:id", h.Update)
		rules.DELETE("/:id", h.Delete)
		rules.GET("/:id/agents", h.GetBoundAgents) // 获取绑定的角色列表
	}

	// Agent-Rule 绑定路由
	agents := r.Group("/agents")
	{
		agents.GET("/:id/rules", h.GetAgentRules)
		agents.POST("/:id/rules", h.BindRules)
		agents.DELETE("/:id/rules/:rule_id", h.UnbindRule)
	}
}

// GetBoundAgents 获取Rule绑定的所有Agent角色
func (h *RuleHandler) GetBoundAgents(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if h.autoGenerator == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "autoGenerator not initialized"})
		return
	}

	affectedIDs, err := h.autoGenerator.GetAffectedAgentsByRule(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	agents := make([]gin.H, 0)
	for _, agentID := range affectedIDs {
		agent, err := h.agentRepo.FindByID(c.Request.Context(), agentID)
		if err == nil {
			agents = append(agents, gin.H{
				"id":   agent.ID.String(),
				"name": agent.Name,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"count":  len(agents),
	})
}