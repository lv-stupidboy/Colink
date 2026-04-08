package api

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/settings"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SettingsHandler Settings API处理器
type SettingsHandler struct {
	svc         *settings.Service
	storagePath string
}

// NewSettingsHandler 创建SettingsHandler
func NewSettingsHandler(svc *settings.Service, storagePath string) *SettingsHandler {
	return &SettingsHandler{
		svc:         svc,
		storagePath: storagePath,
	}
}

// List 列出所有Settings
func (h *SettingsHandler) List(c *gin.Context) {
	var query model.SettingsListQuery
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

	settingsList, total, err := h.svc.List(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      settingsList,
		"total":     total,
		"page":      query.Page,
		"pageSize":  query.PageSize,
	})
}

// GetByID 获取Settings详情
func (h *SettingsHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	settingsRecord, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "settings not found"})
		return
	}

	c.JSON(http.StatusOK, settingsRecord)
}

// Create 创建Settings（支持目录上传，zip格式）
func (h *SettingsHandler) Create(c *gin.Context) {
	// 获取元数据
	name := c.PostForm("name")
	description := c.PostForm("description")

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的文件"})
		return
	}
	defer file.Close()

	// 检查文件扩展名 - 只支持 zip
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".zip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 .zip 格式的文件"})
		return
	}

	// 读取文件内容
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	// 解析 zip 文件
	zipReader, err := zip.NewReader(bytes.NewReader(fileBytes), int64(len(fileBytes)))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "解压文件失败: " + err.Error()})
		return
	}

	// 构建文件列表（前端已处理路径，直接使用 zip 中的路径）
	files := make([]settings.FileData, 0)
	for _, f := range zipReader.File {
		// 跳过目录
		if f.FileInfo().IsDir() {
			continue
		}

		// 获取文件名（直接使用 zip 中的路径）
		fileName := f.Name
		if fileName == "" {
			continue
		}

		// 打开文件
		rc, err := f.Open()
		if err != nil {
			continue
		}

		// 读取文件内容
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		files = append(files, settings.FileData{
			RelativePath: fileName,
			Content:      bytes.NewReader(content),
		})
	}

	// 创建 Settings
	req := &settings.CreateFromFileRequest{
		Name:        name,
		Description: description,
		Files:       files,
	}

	settingsRecord, err := h.svc.CreateFromFile(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, settingsRecord)
}

// Delete 删除Settings
func (h *SettingsHandler) Delete(c *gin.Context) {
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

// BindToAgent 绑定Settings到Agent角色
func (h *SettingsHandler) BindToAgent(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("agentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	var req model.BindSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.BindSettings(c.Request.Context(), agentRoleID, req.SettingsIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetAgentSettings 获取Agent绑定的Settings
func (h *SettingsHandler) GetAgentSettings(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("agentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	settingsList, err := h.svc.GetBoundSettings(c.Request.Context(), agentRoleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"settings": settingsList,
		"count":    len(settingsList),
	})
}

// UnbindSettings 解绑Settings
func (h *SettingsHandler) UnbindSettings(c *gin.Context) {
	agentRoleID, err := uuid.Parse(c.Param("agentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent id"})
		return
	}

	settingsID, err := uuid.Parse(c.Param("settingsId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid settings id"})
		return
	}

	if err := h.svc.UnbindSettings(c.Request.Context(), agentRoleID, settingsID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetBoundAgents 获取Settings绑定的所有Agents
func (h *SettingsHandler) GetBoundAgents(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	agents, err := h.svc.GetBoundAgents(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, agents)
}

// ReadDirectory 读取Settings目录内容
func (h *SettingsHandler) ReadDirectory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	subPath := c.Query("path") // 子目录路径

	content, err := h.svc.ReadDirectoryContent(c.Request.Context(), id, subPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, content)
}

// ReadFile 读取Settings目录中的文件内容
func (h *SettingsHandler) ReadFile(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	filePath := c.Query("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	content, err := h.svc.ReadFileContent(c.Request.Context(), id, filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 根据文件扩展名设置 Content-Type
	ext := filepath.Ext(filePath)
	contentType := "text/plain"
	switch ext {
	case ".json":
		contentType = "application/json"
	case ".yaml", ".yml":
		contentType = "text/yaml"
	case ".md":
		contentType = "text/markdown"
	case ".txt":
		contentType = "text/plain"
	}

	c.Data(http.StatusOK, contentType, content)
}

// RegisterRoutes 注册路由
func (h *SettingsHandler) RegisterRoutes(r *gin.RouterGroup) {
	// Settings CRUD 路由
	settingsGroup := r.Group("/settings")
	{
		settingsGroup.GET("", h.List)
		settingsGroup.POST("", h.Create)
		settingsGroup.GET("/:id", h.GetByID)
		settingsGroup.DELETE("/:id", h.Delete)
		settingsGroup.GET("/:id/agents", h.GetBoundAgents)
		settingsGroup.GET("/:id/directory", h.ReadDirectory)
		settingsGroup.GET("/:id/file", h.ReadFile)
	}

	// Agent-Settings 绑定路由
	agentSettings := r.Group("/agent-roles")
	{
		agentSettings.GET("/:agentId/settings", h.GetAgentSettings)
		agentSettings.POST("/:agentId/settings", h.BindToAgent)
		agentSettings.DELETE("/:agentId/settings/:settingsId", h.UnbindSettings)
	}
}