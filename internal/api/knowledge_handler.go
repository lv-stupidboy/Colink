package api

import (
	"net/http"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/knowledge"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// KnowledgeHandler 知识库 API 处理器
type KnowledgeHandler struct {
	knowledgeSvc *knowledge.Service
}

// NewKnowledgeHandler 创建 KnowledgeHandler
func NewKnowledgeHandler(knowledgeSvc *knowledge.Service) *KnowledgeHandler {
	return &KnowledgeHandler{
		knowledgeSvc: knowledgeSvc,
	}
}

// List 列出所有知识库
func (h *KnowledgeHandler) List(c *gin.Context) {
	var query model.KnowledgeBaseListQuery
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

	kbs, total, err := h.knowledgeSvc.List(c.Request.Context(), &query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      kbs,
		"total":     total,
		"page":      query.Page,
		"page_size": query.Size,
	})
}

// Get 获取单个知识库
func (h *KnowledgeHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识库 ID"})
		return
	}

	kb, err := h.knowledgeSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "知识库不存在"})
		return
	}

	c.JSON(http.StatusOK, kb)
}

// Create 创建知识库
func (h *KnowledgeHandler) Create(c *gin.Context) {
	var req model.CreateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	kb, err := h.knowledgeSvc.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, kb)
}

// Update 更新知识库
func (h *KnowledgeHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识库 ID"})
		return
	}

	var req model.UpdateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	kb, err := h.knowledgeSvc.Update(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, kb)
}

// Delete 删除知识库
func (h *KnowledgeHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识库 ID"})
		return
	}

	if err := h.knowledgeSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// Query 查询知识库
func (h *KnowledgeHandler) Query(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的知识库 ID"})
		return
	}

	var req model.KnowledgeQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置默认限制
	if req.Limit <= 0 {
		req.Limit = 10
	}

	result, err := h.knowledgeSvc.Query(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// QueryAll 查询所有知识库
func (h *KnowledgeHandler) QueryAll(c *gin.Context) {
	var req model.KnowledgeQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 设置默认限制
	if req.Limit <= 0 {
		req.Limit = 10
	}

	results, err := h.knowledgeSvc.QueryAll(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   req.Query,
		"results": results,
	})
}

// RegisterRoutes 注册路由
func (h *KnowledgeHandler) RegisterRoutes(r *gin.RouterGroup) {
	knowledge := r.Group("/knowledge")
	{
		knowledge.GET("", h.List)
		knowledge.POST("", h.Create)
		knowledge.GET("/:id", h.Get)
		knowledge.PUT("/:id", h.Update)
		knowledge.DELETE("/:id", h.Delete)
		knowledge.POST("/:id/query", h.Query)
		knowledge.POST("/query", h.QueryAll)
	}
}