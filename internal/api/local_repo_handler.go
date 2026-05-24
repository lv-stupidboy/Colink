package api

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/local_repo"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// LocalRepoHandler 本地代码仓API处理器
type LocalRepoHandler struct {
	service *local_repo.Service
}

// NewLocalRepoHandler 创建处理器
func NewLocalRepoHandler(service *local_repo.Service) *LocalRepoHandler {
	return &LocalRepoHandler{service: service}
}

func isLocalRepoValidationError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "不能为空") ||
		strings.Contains(errMsg, "不能包含路径分隔符") ||
		strings.Contains(errMsg, "路径必须位于工作空间内") ||
		strings.Contains(errMsg, "仅支持 SSH 格式")
}

// List 列出所有本地代码仓
func (h *LocalRepoHandler) List(c *gin.Context) {
	repos, err := h.service.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, repos)
}

// Get 获取单个本地代码仓
func (h *LocalRepoHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	repo, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}
	c.JSON(http.StatusOK, repo)
}

// Upload 上传ZIP创建本地代码仓
func (h *LocalRepoHandler) Upload(c *gin.Context) {
	name := c.PostForm("name")
	targetPath := c.PostForm("targetPath")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的文件"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".zip" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "只支持 .zip 格式的文件"})
		return
	}

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}

	req := &model.UploadRepoRequest{
		Name:       name,
		TargetPath: targetPath,
	}

	repo, err := h.service.Upload(c.Request.Context(), fileBytes, header.Filename, req)
	if err != nil {
		if isLocalRepoValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, repo)
}

// Clone 从远程URL克隆代码仓
func (h *LocalRepoHandler) Clone(c *gin.Context) {
	var req model.CloneRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	repo, err := h.service.Clone(c.Request.Context(), &req)
	if err != nil {
		if isLocalRepoValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, repo)
}

// GetRemoteBranches 获取远程仓库分支列表
func (h *LocalRepoHandler) GetRemoteBranches(c *gin.Context) {
	var req model.RemoteBranchesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	branches, err := h.service.GetRemoteBranches(req.GitUrl)
	if err != nil {
		if isLocalRepoValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, branches)
}

// Sync 同步本地代码仓
func (h *LocalRepoHandler) Sync(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	repo, err := h.service.Sync(c.Request.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "GIT地址未配置") || isLocalRepoValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, repo)
}

// ConfigureGit 配置代码仓的git URL
func (h *LocalRepoHandler) ConfigureGit(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req model.GitConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	repo, err := h.service.ConfigureGit(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, repo)
}

// Delete 删除本地代码仓
func (h *LocalRepoHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		if isLocalRepoValidationError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// BrowsePath 浏览文件系统路径
func (h *LocalRepoHandler) BrowsePath(c *gin.Context) {
	path := c.Query("path")

	result, err := h.service.BrowsePath(c.Request.Context(), path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *LocalRepoHandler) CreateFolder(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.CreateFolder(c.Request.Context(), req.Path, req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RegisterRoutes 注册路由
func (h *LocalRepoHandler) RegisterRoutes(r *gin.RouterGroup) {
	repos := r.Group("/repos")
	{
		repos.GET("", h.List)
		repos.GET("/browse", h.BrowsePath)                  // BEFORE /:id
		repos.POST("/folder", h.CreateFolder)               // BEFORE /:id
		repos.POST("/upload", h.Upload)                     // BEFORE /:id
		repos.POST("/clone", h.Clone)                       // BEFORE /:id
		repos.POST("/remote-branches", h.GetRemoteBranches) // BEFORE /:id
		repos.GET("/:id", h.Get)
		repos.DELETE("/:id", h.Delete)
		repos.POST("/:id/sync", h.Sync)
		repos.PUT("/:id/git-config", h.ConfigureGit)
	}
}
