package message

import (
	"context"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
)

// AgentSpawner Agent触发接口（避免循环依赖）
type AgentSpawner interface {
	SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string) error
}

// Service Message服务
type Service struct {
	repo         *repo.MessageRepository
	wsHub        *ws.Hub
	agentSpawner AgentSpawner
}

// NewService 创建Message服务
func NewService(repo *repo.MessageRepository, wsHub *ws.Hub) *Service {
	return &Service{repo: repo, wsHub: wsHub}
}

// SetAgentSpawner 设置Agent触发器（解决循环依赖）
func (s *Service) SetAgentSpawner(spawner AgentSpawner) {
	s.agentSpawner = spawner
}

// Create 创建消息
// skipAgentTrigger: 当前端已处理Agent触发时设为true，避免重复触发
func (s *Service) Create(ctx context.Context, threadID uuid.UUID, role model.MessageRole, agentID, content string, skipAgentTrigger bool) (*model.Message, error) {
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

	// 只有Agent消息才需要通过WebSocket广播
	// 用户消息前端已经乐观更新，不需要再次广播
	if s.wsHub != nil && role == model.MessageRoleAgent {
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

	// 用户消息触发Agent响应（除非前端已处理）
	if role == model.MessageRoleUser && s.agentSpawner != nil && !skipAgentTrigger {
		go func() {
			// 异步触发Agent，不阻塞用户请求
			if err := s.agentSpawner.SpawnAgentForUserMessage(context.Background(), threadID, content); err != nil {
				// 记录错误但不影响用户消息保存
				println("[WARN] Failed to spawn agent for user message:", err.Error())
			}
		}()
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