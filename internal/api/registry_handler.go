package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/skill"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RegistryHandler 联邦技能源 API 处理器
type RegistryHandler struct {
	registrySvc *skill.RegistryService
}

// NewRegistryHandler 创建 RegistryHandler
func NewRegistryHandler(registrySvc *skill.RegistryService) *RegistryHandler {
	return &RegistryHandler{
		registrySvc: registrySvc,
	}
}

// List 列出所有注册表
func (h *RegistryHandler) List(c *gin.Context) {
	var query repo.RegistryListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置默认分页
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Size <= 0 {
		query.Size = 20
	}

	registries, total, err := h.registrySvc.List(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      registries,
		"total":     total,
		"page":      query.Page,
		"page_size": query.Size,
	})
}

// Get 获取单个注册表
func (h *RegistryHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的注册表 ID"})
		return
	}

	registry, err := h.registrySvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "注册表不存在"})
		return
	}

	c.JSON(http.StatusOK, registry)
}

// Create 创建注册表
func (h *RegistryHandler) Create(c *gin.Context) {
	var req model.CreateRegistryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	registry, err := h.registrySvc.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, registry)
}

// Update 更新注册表
func (h *RegistryHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的注册表 ID"})
		return
	}

	var req model.UpdateRegistryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	registry, err := h.registrySvc.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, registry)
}

// Delete 删除注册表
func (h *RegistryHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的注册表 ID"})
		return
	}

	if err := h.registrySvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// Sync 同步注册表技能
func (h *RegistryHandler) Sync(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的注册表 ID"})
		return
	}

	result, err := h.registrySvc.Sync(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  err.Error(),
			"result": result,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// SyncAll 同步所有注册表
func (h *RegistryHandler) SyncAll(c *gin.Context) {
	results, err := h.registrySvc.SyncAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "同步完成",
		"results": results,
	})
}

// RegisterRoutes 注册路由
func (h *RegistryHandler) RegisterRoutes(r *gin.RouterGroup) {
	registries := r.Group("/registries")
	{
		registries.GET("", h.List)
		registries.POST("", h.Create)
		registries.GET("/:id", h.Get)
		registries.PUT("/:id", h.Update)
		registries.DELETE("/:id", h.Delete)
		registries.POST("/:id/sync", h.Sync)
		registries.POST("/sync", h.SyncAll)
	}
}