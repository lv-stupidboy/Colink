package api

import (
	"io"
	"net/http"
	"path/filepath"

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

// Create 创建Settings（支持目录上传）
func (h *SettingsHandler) Create(c *gin.Context) {
	// 获取元数据
	name := c.PostForm("name")
	description := c.PostForm("description")
	version := c.PostForm("version")

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	// 构建文件列表
	files := make([]settings.FileData, 0)

	// 处理多文件上传（前端上传目录时，每个文件作为单独的 form file）
	form, err := c.MultipartForm()
	if err == nil && form != nil {
		for _, fileHeaders := range form.File {
			for _, fileHeader := range fileHeaders {
				// 获取相对路径（前端需要传递 relativePath 参数或在文件名中包含路径）
				relativePath := fileHeader.Filename

				// 打开文件
				file, err := fileHeader.Open()
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open uploaded file"})
					return
				}

				files = append(files, settings.FileData{
					RelativePath: relativePath,
					Content:      file,
				})
			}
		}
	}

	// 创建 Settings
	req := &settings.CreateFromFileRequest{
		Name:        name,
		Description: description,
		Version:     version,
		Files:       files,
	}

	settingsRecord, err := h.svc.CreateFromFile(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 关闭所有打开的文件
	for _, f := range files {
		if closer, ok := f.Content.(io.Closer); ok {
			closer.Close()
		}
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