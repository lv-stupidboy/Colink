package model

import "time"

// InvocationContentBlock 增量持久化的内容块
// 用于 Agent 后台执行时实时保存输出内容
type InvocationContentBlock struct {
	ID           string                 `json:"id" gorm:"primaryKey"`
	InvocationID string                 `json:"invocationId" gorm:"index;not null"`
	Type         string                 `json:"type" gorm:"not null"` // thinking, text, tool_use, tool_result
	Content      string                 `json:"content,omitempty"`
	ToolName     string                 `json:"toolName,omitempty"`
	ToolID       string                 `json:"toolId,omitempty"`
	Input        map[string]interface{} `json:"input,omitempty" gorm:"type:json"`
	Output       string                 `json:"output,omitempty"`
	IsError      bool                   `json:"isError,omitempty"`
	Status       string                 `json:"status,omitempty"` // streaming, completed
	Timestamp    int64                  `json:"timestamp" gorm:"not null"`
	StartedAt    int64                  `json:"startedAt,omitempty"`
	CompletedAt  int64                  `json:"completedAt,omitempty"`
	CreatedAt    time.Time              `json:"createdAt" gorm:"autoCreateTime"`
}

// TableName 指定表名
func (InvocationContentBlock) TableName() string {
	return "invocation_content_blocks"
}