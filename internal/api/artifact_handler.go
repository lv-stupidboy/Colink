package api

import (
	"net/http"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ArtifactHandler Artifact API处理器
type ArtifactHandler struct {
	repo *repo.ArtifactRepository
}

// NewArtifactHandler 创建处理器
func NewArtifactHandler(repo *repo.ArtifactRepository) *ArtifactHandler {
	return &ArtifactHandler{repo: repo}
}

// List 列出Thread的Artifacts
func (h *ArtifactHandler) List(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	artifacts, err := h.repo.FindByThreadID(c.Request.Context(), threadID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 确保返回空数组而不是 null
	if artifacts == nil {
		artifacts = []*model.Artifact{}
	}
	c.JSON(http.StatusOK, artifacts)
}

// Get 获取单个Artifact
func (h *ArtifactHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	artifact, err := h.repo.FindByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "artifact not found"})
		return
	}
	c.JSON(http.StatusOK, artifact)
}

// Create 创建Artifact
func (h *ArtifactHandler) Create(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}

	var req struct {
		Type     string                 `json:"type" binding:"required"`
		Name     string                 `json:"name"`
		Path     string                 `json:"path"`
		Content  string                 `json:"content"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	artifact := &model.Artifact{
		ID:        uuid.New(),
		ThreadID:  threadID,
		Type:      model.ArtifactType(req.Type),
		Name:      req.Name,
		Path:      req.Path,
		Content:   req.Content,
		Metadata:  req.Metadata,
		CreatedAt: time.Now(),
	}

	if err := h.repo.Create(c.Request.Context(), artifact); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, artifact)
}

// RegisterRoutes 注册路由
func (h *ArtifactHandler) RegisterRoutes(r *gin.RouterGroup) {
	// 注意：必须先注册具体路径（带固定后缀的），再注册通配路径
	threads := r.Group("/threads")
	{
		threads.GET("/:id/artifacts", h.List)
		threads.POST("/:id/artifacts", h.Create)
	}

	// 单个artifact
	r.GET("/artifacts/:id", h.Get)
}