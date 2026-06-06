package message

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
)

// AgentSpawner Agent触发接口（避免循环依赖）
type AgentSpawner interface {
	SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string, images []model.ImageContent) error
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

// Create 创建消息（纯文本）
// skipAgentTrigger: 当前端已处理Agent触发时设为true，避免重复触发
func (s *Service) Create(ctx context.Context, threadID uuid.UUID, role model.MessageRole, agentID, content string, skipAgentTrigger bool) (*model.Message, error) {
	return s.CreateWithImages(ctx, threadID, role, agentID, content, nil, skipAgentTrigger)
}

// CreateWithImages 创建消息（支持图片）
// images: 图片附件列表（多模态输入）
func (s *Service) CreateWithImages(ctx context.Context, threadID uuid.UUID, role model.MessageRole, agentID, content string, images []model.ImageContent, skipAgentTrigger bool) (*model.Message, error) {
	msg := &model.Message{
		ThreadID:    threadID,
		Role:        role,
		AgentID:     agentID,
		Content:     content,
		MessageType: model.MessageTypeText,
	}

	// 如果有图片，将图片信息存储到 contentBlocks（前端渲染格式）
	if len(images) > 0 {
		// 构建图片列表（用于 media_gallery rich block）
		mediaItems := make([]map[string]interface{}, len(images))
		for i, img := range images {
			dataURI := fmt.Sprintf("data:%s;base64,%s", img.MimeType, img.Data)
			mediaItems[i] = map[string]interface{}{
				"id":           fmt.Sprintf("img-%d", i),
				"url":          dataURI,
				"thumbnailUrl": dataURI,
			}
		}

		// 构建内容块数组：文本块 + 图片画廊块
		ts := time.Now().UnixMilli()
		contentBlocksArray := []map[string]interface{}{
			{
				"id":        fmt.Sprintf("text-%d", ts),
				"type":      "text",
				"content":   content,
				"timestamp": ts,
			},
			{
				"id":        fmt.Sprintf("img-%d", ts),
				"type":      "rich",
				"richType":  "media_gallery",
				"timestamp": ts,
				"images":    mediaItems,
			},
		}
		msg.ContentBlocks, _ = jsonMarshal(contentBlocksArray)
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
			if err := s.agentSpawner.SpawnAgentForUserMessage(context.Background(), threadID, content, images); err != nil {
				// 记录错误但不影响用户消息保存
				println("[WARN] Failed to spawn agent for user message:", err.Error())
			}
		}()
	}

	return msg, nil
}

// jsonMarshal 辅助函数
func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// GetByThreadID 根据ThreadID获取消息列表（最新的N条）
func (s *Service) GetByThreadID(ctx context.Context, threadID uuid.UUID, limit int) ([]*model.Message, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.repo.FindByThreadID(ctx, threadID, limit)
}

// GetByThreadIDBeforeCursor 根据ThreadID获取指定cursor之前的消息（用于向上滚动加载历史）
func (s *Service) GetByThreadIDBeforeCursor(ctx context.Context, threadID uuid.UUID, cursor string, limit int) ([]*model.Message, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.repo.FindByThreadIDBeforeCursor(ctx, threadID, cursor, limit)
}

// GetMessageCount 获取消息总数
func (s *Service) GetMessageCount(ctx context.Context, threadID uuid.UUID) (int, error) {
	return s.repo.CountByThreadID(ctx, threadID)
}