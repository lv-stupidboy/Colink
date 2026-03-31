package model

import (
	"time"

	"github.com/google/uuid"
)

// ========== MultiMention Models ==========

// MultiMentionStatus 多讨论请求状态
type MultiMentionStatus string

const (
	MultiMentionStatusPending   MultiMentionStatus = "pending"   // 待执行
	MultiMentionStatusRunning   MultiMentionStatus = "running"   // 正在执行
	MultiMentionStatusPartial   MultiMentionStatus = "partial"   // 部分响应（等待其他）
	MultiMentionStatusDone      MultiMentionStatus = "done"      // 全部响应完成
	MultiMentionStatusTimeout   MultiMentionStatus = "timeout"   // 超时
	MultiMentionStatusFailed    MultiMentionStatus = "failed"    // 执行失败
)

// MultiMentionRequest 多Agent讨论请求
type MultiMentionRequest struct {
	ID             uuid.UUID          `json:"id"`                         // 请求唯一标识符
	ThreadID       uuid.UUID          `json:"threadId"`                   // 关联会话ID
	Initiator      string             `json:"initiator"`                  // 发起者Agent ID
	CallbackTo     string             `json:"callbackTo"`                 // 回调Agent ID
	Targets        []string           `json:"targets"`                    // 目标Agent ID列表 (1-3个)
	Question       string             `json:"question"`                   // 问题内容
	Context        string             `json:"context,omitempty"`          // 上下文信息
	Status         MultiMentionStatus `json:"status"`                     // 请求状态
	TimeoutMinutes int                `json:"timeoutMinutes"`             // 超时时间(分钟)
	SearchEvidence []string           `json:"searchEvidence,omitempty"`   // 搜索证据引用列表
	OverrideReason string             `json:"overrideReason,omitempty"`   // 跳过搜索的理由
	CreatedAt      time.Time          `json:"createdAt"`                  // 创建时间
	UpdatedAt      time.Time          `json:"updatedAt"`                  // 更新时间
}

func (r *MultiMentionRequest) TableName() string {
	return "multi_mention_requests"
}

// MultiMentionResponse 多Agent讨论响应
type MultiMentionResponse struct {
	ID        uuid.UUID `json:"id"`          // 响应唯一标识符
	RequestID uuid.UUID `json:"requestId"`   // 关联请求ID
	AgentID   string    `json:"agentId"`     // 响应Agent ID
	Content   string    `json:"content"`     // 响应内容
	CreatedAt time.Time `json:"createdAt"`   // 创建时间
}

func (r *MultiMentionResponse) TableName() string {
	return "multi_mention_responses"
}

// CreateMultiMentionRequestParams 创建多讨论请求参数
type CreateMultiMentionRequestParams struct {
	ThreadID       uuid.UUID   `json:"threadId"`
	Initiator      string      `json:"initiator"`
	CallbackTo     string      `json:"callbackTo"`
	Targets        []string    `json:"targets"`                    // 1-3个
	Question       string      `json:"question"`
	Context        string      `json:"context,omitempty"`
	TimeoutMinutes int         `json:"timeoutMinutes,omitempty"`   // 默认8
	SearchEvidence []string    `json:"searchEvidence,omitempty"`   // 必须提供或提供OverrideReason
	OverrideReason string      `json:"overrideReason,omitempty"`   // 跳过搜索的理由
}

// AggregatedMultiMentionResult 聚合的多讨论结果
type AggregatedMultiMentionResult struct {
	RequestID uuid.UUID              `json:"requestId"`
	Status    MultiMentionStatus     `json:"status"`
	Responses []*MultiMentionResponse `json:"responses"`
	Timeout   bool                   `json:"timeout"` // 是否超时
}