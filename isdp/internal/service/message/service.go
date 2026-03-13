package message

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
)

// Service Message服务
type Service struct {
	repo  *repo.MessageRepository
	wsHub *ws.Hub
}

// NewService 创建Message服务
func NewService(repo *repo.MessageRepository, wsHub *ws.Hub) *Service {
	return &Service{repo: repo, wsHub: wsHub}
}

// Create 创建消息
func (s *Service) Create(ctx context.Context, threadID uuid.UUID, role model.MessageRole, agentID, content string) (*model.Message, error) {
	msg := &model.Message{
		ThreadID:    threadID,
		Role:        role,
		AgentID:     agentID,
		Content:     content,
		MessageType: model.MessageTypeText,
	}

	if err := s.repo.Create(ctx, msg); err != nil {
		return nil, err
	}

	// 通过WebSocket广播消息
	if s.wsHub != nil {
		s.wsHub.BroadcastToThread(threadID.String(), ws.WSMessage{
			Type:      "agent_message",
			ThreadID:  threadID.String(),
			Timestamp: msg.CreatedAt.UnixMilli(),
			Payload: map[string]interface{}{
				"messageId": msg.ID.String(),
				"agentId":   agentID,
				"content":   content,
			},
		})
	}

	return msg, nil
}

// GetByThreadID 根据ThreadID获取消息列表
func (s *Service) GetByThreadID(ctx context.Context, threadID uuid.UUID, limit int) ([]*model.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.FindByThreadID(ctx, threadID, limit)
}