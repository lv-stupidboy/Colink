package api

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/subagent"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SubagentHandler Subagent API处理器
type SubagentHandler struct {
	svc    *subagent.Service
	config *config.SubagentConfig
}

// NewSubagentHandler 创建SubagentHandler
func NewSubagentHandler(svc *subagent.Service, cfg *config.SubagentConfig) *SubagentHandler {
	return &SubagentHandler{
		svc:    svc,
		config: cfg,
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

// Upload 上传子代理文件
func (h *SubagentHandler) Upload(c *gin.Context) {
	// 获取配置
	maxSize := h.config.GetUploadMaxSize()
	storagePath := h.config.GetStoragePath()

	// 检查文件大小（在读取前检查）
	if c.Request.ContentLength > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("文件大小超过限制，最大允许 %dMB", maxSize/1024/1024)})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的文件"})
		return
	}
	defer file.Close()

	// 再次检查文件大小（以实际大小为准）
	if header.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("文件大小超过限制，最大允许 %dMB", maxSize/1024/1024)})
		return
	}

	// 检查文件扩展名 - 支持 .md 和 .zip
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".md" && ext != ".zip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 .md 和 .zip 格式的文件"})
		return
	}

	// 读取文件内容
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	var name, description, content string

	if ext == ".md" {
		// 直接解析 Markdown 文件
		content = string(fileBytes)
		metadata := parseSubagentMD(content)
		name = metadata.Name
		description = metadata.Description
	} else {
		// 解析 zip 文件
		reader, err := zip.NewReader(bytes.NewReader(fileBytes), int64(len(fileBytes)))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "解压文件失败: " + err.Error()})
			return
		}

		// 查找 agent.md 文件
		var agentMDContent string
		for _, f := range reader.File {
			if strings.HasSuffix(f.Name, "agent.md") || f.Name == "subagent.md" {
				rc, err := f.Open()
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "读取配置文件失败"})
					return
				}
				contentBytes, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "读取配置内容失败"})
					return
				}
				agentMDContent = string(contentBytes)
				break
			}
		}

		if agentMDContent == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "未找到 agent.md 或 subagent.md 文件"})
			return
		}

		content = agentMDContent
		metadata := parseSubagentMD(content)
		name = metadata.Name
		description = metadata.Description
	}

	// 如果没有从 markdown 中提取到名称，从文件名提取
	if name == "" {
		name = strings.TrimSuffix(header.Filename, ext)
	}

	// 统一清理名称：只保留小写字母、数字和中划线
	re := regexp.MustCompile(`[^a-z0-9-]`)
	name = strings.ToLower(re.ReplaceAllString(name, "-"))
	name = strings.Trim(name, "-")
	if name == "" {
		name = "subagent-" + uuid.New().String()[:8]
	}

	// 确保名称以字母开头
	if len(name) > 0 && (name[0] < 'a' || name[0] > 'z') {
		name = "s-" + name
	}

	// 校验名称格式
	if !isValidSubagentName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称只能包含小写字母、数字和中划线，且必须以字母开头"})
		return
	}

	// 创建子代理记录
	req := &model.CreateSubagentRequest{
		Name:        name,
		Description: description,
		Content:     content,
	}

	subagentRecord, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		if err.Error() == "subagent name already exists" {
			c.JSON(http.StatusConflict, gin.H{"error": "子代理名称已存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 保存文件到本地
	// 确保存储目录存在
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建存储目录失败"})
		return
	}

	// 使用子代理名称作为文件名，保存为 .md 文件
	savePath := filepath.Join(storagePath, subagentRecord.Name+".md")
	if err := os.WriteFile(savePath, []byte(content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}

	c.JSON(http.StatusCreated, subagentRecord)
}

// SubagentMetadata 子代理元数据
type SubagentMetadata struct {
	Name        string
	Description string
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

// parseSubagentMD 解析子代理 Markdown 文件提取元数据
func parseSubagentMD(content string) SubagentMetadata {
	metadata := SubagentMetadata{}

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

// RegisterRoutes 注册路由
func (h *SubagentHandler) RegisterRoutes(r *gin.RouterGroup) {
	// Subagent CRUD 路由
	subagents := r.Group("/subagents")
	{
		subagents.GET("", h.List)
		subagents.POST("", h.Create)
		subagents.POST("/upload", h.Upload) // 上传路由
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