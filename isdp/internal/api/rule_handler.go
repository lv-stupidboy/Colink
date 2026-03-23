package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/rule"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RuleHandler Rule API处理器
type RuleHandler struct {
	svc         *rule.Service
	storagePath string
	maxSize     int64
}

// NewRuleHandler 创建RuleHandler
func NewRuleHandler(svc *rule.Service, storagePath string, maxSize int64) *RuleHandler {
	return &RuleHandler{
		svc:         svc,
		storagePath: storagePath,
		maxSize:     maxSize,
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
		"data":       rules,
		"total":      total,
		"page":       query.Page,
		"page_size":  query.PageSize,
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

	// 设置默认 scope
	if req.Scope == "" {
		req.Scope = model.RuleScopeInstance
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

	c.JSON(http.StatusOK, r)
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

// Upload 上传规约文件
func (h *RuleHandler) Upload(c *gin.Context) {
	// 获取 scope 参数（默认为 instance）
	scopeStr := c.PostForm("scope")
	scope := model.RuleScopeInstance
	if scopeStr != "" {
		scope = model.RuleScope(scopeStr)
		// 验证 scope
		if scope != model.RuleScopePublic && scope != model.RuleScopeInstance {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scope 必须是 public 或 instance"})
			return
		}
	}

	// 检查文件大小（在读取前检查）
	if c.Request.ContentLength > h.maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("文件大小超过限制，最大允许 %dMB", h.maxSize/1024/1024)})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的文件"})
		return
	}
	defer file.Close()

	// 再次检查文件大小（以实际大小为准）
	if header.Size > h.maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("文件大小超过限制，最大允许 %dMB", h.maxSize/1024/1024)})
		return
	}

	// 检查文件扩展名 - 只支持 .md
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".md" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 .md 格式的文件"})
		return
	}

	// 读取文件内容
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	content := string(fileBytes)
	metadata := parseRuleMD(content)

	// 如果没有从 markdown 中提取到名称，从文件名提取
	name := metadata.Name
	if name == "" {
		name = strings.TrimSuffix(header.Filename, ext)
	}

	// 统一清理名称：只保留小写字母、数字和中划线
	re := regexp.MustCompile(`[^a-z0-9-]`)
	name = strings.ToLower(re.ReplaceAllString(name, "-"))
	name = strings.Trim(name, "-")
	if name == "" {
		name = "rule-" + uuid.New().String()[:8]
	}

	// 确保名称以字母开头
	if len(name) > 0 && (name[0] < 'a' || name[0] > 'z') {
		name = "r-" + name
	}

	// 校验名称格式
	if !isValidRuleName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称只能包含小写字母、数字和中划线，且必须以字母开头"})
		return
	}

	// 创建规约记录
	req := &model.CreateRuleRequest{
		Name:        name,
		Description: metadata.Description,
		Scope:       scope,
	}

	r, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		if err == rule.ErrRuleNameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "规约名称已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 保存文件到本地
	// 确保存储目录存在
	if err := os.MkdirAll(h.storagePath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建存储目录失败"})
		return
	}

	// 使用规约名称作为文件名，保存为 .md 文件
	savePath := filepath.Join(h.storagePath, r.Name+".md")
	if err := os.WriteFile(savePath, fileBytes, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}

	c.JSON(http.StatusCreated, r)
}

// RuleMetadata 规约元数据
type RuleMetadata struct {
	Name        string
	Description string
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

// parseRuleMD 解析规约 Markdown 文件提取元数据
func parseRuleMD(content string) RuleMetadata {
	metadata := RuleMetadata{}

	// 提取标题 (第一个 # 标题)
	titleRegex := regexp.MustCompile(`(?m)^#\s+(.+)$`)
	if matches := titleRegex.FindStringSubmatch(content); len(matches) > 1 {
		metadata.Name = strings.TrimSpace(matches[1])
	}

	// 提取描述 (## Description 或 ## 描述 下的内容，直到下一个 ## 标题)
	descRegex := regexp.MustCompile(`(?s)##\s*(?:Description|描述)\s*\n+(.+?)(?:\n##|$)`)
	if matches := descRegex.FindStringSubmatch(content); len(matches) > 1 {
		desc := strings.TrimSpace(matches[1])
		// 移除末尾的空行
		desc = strings.TrimRight(desc, "\n")
		metadata.Description = desc
	}

	return metadata
}

// GetPublicRules 获取所有公共规约
func (h *RuleHandler) GetPublicRules(c *gin.Context) {
	rules, err := h.svc.GetPublicRules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"count": len(rules),
	})
}

// GetInstanceRules 获取所有实例规约
func (h *RuleHandler) GetInstanceRules(c *gin.Context) {
	rules, err := h.svc.GetInstanceRules(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"count": len(rules),
	})
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

	c.Status(http.StatusNoContent)
}

// RegisterRoutes 注册路由
func (h *RuleHandler) RegisterRoutes(r *gin.RouterGroup) {
	// Rule CRUD 路由
	rules := r.Group("/rules")
	{
		rules.GET("", h.List)
		rules.GET("/public", h.GetPublicRules)
		rules.GET("/instance", h.GetInstanceRules)
		rules.POST("", h.Create)
		rules.POST("/upload", h.Upload)
		rules.GET("/:id", h.Get)
		rules.PUT("/:id", h.Update)
		rules.DELETE("/:id", h.Delete)
	}

	// Agent-Rule 绑定路由
	agents := r.Group("/agents")
	{
		agents.GET("/:id/rules", h.GetAgentRules)
		agents.POST("/:id/rules", h.BindRules)
		agents.DELETE("/:id/rules/:rule_id", h.UnbindRule)
	}
}