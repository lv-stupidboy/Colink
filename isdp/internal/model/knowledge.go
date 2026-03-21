package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== Knowledge Base Models ==========

// KnowledgeBaseType 知识库类型
type KnowledgeBaseType string

const (
	KnowledgeBaseTypeGit KnowledgeBaseType = "git"
	KnowledgeBaseTypeMCP KnowledgeBaseType = "mcp"
	KnowledgeBaseTypeAPI KnowledgeBaseType = "api"
)

// KnowledgeBaseStatus 知识库状态
type KnowledgeBaseStatus string

const (
	KnowledgeBaseStatusActive   KnowledgeBaseStatus = "active"
	KnowledgeBaseStatusInactive KnowledgeBaseStatus = "inactive"
)

// KnowledgeBase 知识库模型
type KnowledgeBase struct {
	ID            uuid.UUID          `json:"id"`
	Name          string             `json:"name"`
	DisplayName   string             `json:"display_name,omitempty"`
	Description   string             `json:"description,omitempty"`
	Type          KnowledgeBaseType  `json:"type"`
	Config        map[string]string  `json:"config,omitempty"` // 加密存储
	QueryEndpoint string             `json:"query_endpoint,omitempty"`
	Status        KnowledgeBaseStatus `json:"status"`
	LastQueryAt   *time.Time         `json:"last_query_at,omitempty"`
	QueryCount    int                `json:"query_count"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

func (k *KnowledgeBase) TableName() string {
	return "knowledge_bases"
}

// CreateKnowledgeBaseRequest 创建知识库请求
type CreateKnowledgeBaseRequest struct {
	Name          string            `json:"name" binding:"required"`
	DisplayName   string            `json:"display_name"`
	Description   string            `json:"description"`
	Type          KnowledgeBaseType `json:"type" binding:"required"`
	Config        map[string]string `json:"config"`
	QueryEndpoint string            `json:"query_endpoint"`
}

// UpdateKnowledgeBaseRequest 更新知识库请求
type UpdateKnowledgeBaseRequest struct {
	DisplayName   string            `json:"display_name"`
	Description   string            `json:"description"`
	Config        map[string]string `json:"config"`
	QueryEndpoint string            `json:"query_endpoint"`
	Status        KnowledgeBaseStatus `json:"status"`
}

// KnowledgeQueryRequest 知识查询请求
type KnowledgeQueryRequest struct {
	Query string `json:"query" binding:"required"`
	Limit int    `json:"limit"`
}

// KnowledgeQueryResult 知识查询结果
type KnowledgeQueryResult struct {
	Query   string              `json:"query"`
	Results []*KnowledgeSnippet `json:"results"`
	Total   int                 `json:"total"`
	Source  string              `json:"source"`
	Error   string              `json:"error,omitempty"`
}

// KnowledgeSnippet 知识片段
type KnowledgeSnippet struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Source      string   `json:"source"`
	URL         string   `json:"url,omitempty"`
	Relevance   float64  `json:"relevance"`
	Tags        []string `json:"tags,omitempty"`
}

// KnowledgeBaseListQuery 知识库列表查询参数
type KnowledgeBaseListQuery struct {
	Type   string `form:"type"`
	Status string `form:"status"`
	Search string `form:"search"`
	Page   int    `form:"page"`
	Size   int    `form:"size"`
}