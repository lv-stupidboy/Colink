package im

import (
	"context"
	"fmt"
	"sync"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// agentSpawner defines the orchestrator capability required by IM bridge.
type agentSpawner interface {
	SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string) error
}

// IMBridgeService provides platform-agnostic IM inbound/outbound bridge.
// It handles:
// 1) inbound IM messages -> thread/session mapping -> agent spawning
// 2) agent chunks -> platform adapter/delivery routing
type IMBridgeService struct {
	adapters map[string]IMAdapter
	delivery map[string]*DeliveryService

	sessionRepo *repo.IMSessionRepository
	threadRepo  *repo.ThreadRepository
	projectRepo *repo.ProjectRepository

	orchestrator agentSpawner
	wsHub        *ws.Hub
	sessionLock  *SessionLock
	logger       *zap.Logger

	mu sync.RWMutex
}

// NewIMBridgeService creates a platform-agnostic IM bridge service.
func NewIMBridgeService(
	sessionRepo *repo.IMSessionRepository,
	threadRepo *repo.ThreadRepository,
	projectRepo *repo.ProjectRepository,
	orchestrator agentSpawner,
	wsHub *ws.Hub,
	sessionLock *SessionLock,
	logger *zap.Logger,
) *IMBridgeService {
	if logger == nil {
		logger = zap.NewNop()
	}
	if sessionLock == nil {
		sessionLock = NewSessionLock()
	}

	return &IMBridgeService{
		adapters:     make(map[string]IMAdapter),
		delivery:     make(map[string]*DeliveryService),
		sessionRepo:  sessionRepo,
		threadRepo:   threadRepo,
		projectRepo:  projectRepo,
		orchestrator: orchestrator,
		wsHub:        wsHub,
		sessionLock:  sessionLock,
		logger:       logger,
	}
}

// RegisterAdapter registers adapter and delivery for a platform.
func (s *IMBridgeService) RegisterAdapter(adapter IMAdapter, delivery *DeliveryService) {
	if adapter == nil || delivery == nil {
		return
	}

	platform := adapter.Platform()
	s.mu.Lock()
	s.adapters[platform] = adapter
	s.delivery[platform] = delivery
	s.mu.Unlock()
}

// HandleInboundMessage processes an inbound IM message and triggers an agent run.
func (s *IMBridgeService) HandleInboundMessage(
	ctx context.Context,
	platform, chatID, chatType, userID, userName, messageID, text string,
) error {
	if text == "" {
		return nil
	}
	if s.orchestrator == nil {
		return fmt.Errorf("orchestrator is nil")
	}

	release := s.sessionLock.Acquire(chatID)
	defer release()

	session, err := s.getOrCreateSession(ctx, platform, chatID, chatType, userID, userName)
	if err != nil {
		return err
	}

	if err := s.orchestrator.SpawnAgentForUserMessage(ctx, session.ThreadID, text); err != nil {
		return fmt.Errorf("failed to spawn agent for inbound message %s: %w", messageID, err)
	}

	return nil
}

func (s *IMBridgeService) getOrCreateSession(ctx context.Context, platform, chatID, chatType, userID, userName string) (*model.IMSession, error) {
	if s.sessionRepo == nil || s.threadRepo == nil {
		return nil, fmt.Errorf("repositories are not initialized")
	}

	existing, err := s.sessionRepo.FindByChatID(ctx, platform, chatID)
	if err == nil && existing != nil && existing.IsActive {
		if updateErr := s.sessionRepo.UpdateLastMessageAt(ctx, existing.ID); updateErr != nil {
			s.logger.Warn("failed to update im session lastMessageAt", zap.Error(updateErr), zap.String("chatID", chatID))
		}
		return existing, nil
	}

	thread := &model.Thread{
		ID:           uuid.New(),
		ProjectID:    uuid.Nil,
		Name:         fmt.Sprintf("IM Session %s", chatID),
		Status:       model.ThreadStatusIdle,
		CurrentPhase: model.PhaseRequirement,
		CurrentAgent: "",
		Depth:        0,
	}
	if err := s.threadRepo.Create(ctx, thread); err != nil {
		return nil, fmt.Errorf("failed to create thread: %w", err)
	}

	session := &model.IMSession{
		ID:        uuid.New(),
		Platform:  model.IMPlatform(platform),
		ChatID:    chatID,
		ChatType:  chatType,
		ThreadID:  thread.ID,
		ProjectID: thread.ProjectID,
		UserID:    userID,
		UserName:  userName,
		IsActive:  true,
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create im session: %w", err)
	}

	return session, nil
}

// OnAgentChunk routes agent chunks back to the proper IM platform.
func (s *IMBridgeService) OnAgentChunk(threadID, invocationID uuid.UUID, chunk agent.Chunk, agentID, agentName string) {
	if s.sessionRepo == nil {
		return
	}

	ctx := context.Background()
	session, err := s.sessionRepo.FindByThreadID(ctx, threadID)
	if err != nil || session == nil {
		return
	}

	platform := string(session.Platform)
	adapter, delivery, ok := s.getPlatformDelivery(platform)
	if !ok {
		return
	}

	chatID := session.ChatID

	switch chunk.Type {
	case agent.ChunkTypeText:
		delivery.DeliverText(ctx, chatID, chunk.Content, s.makeDedupKey(platform, invocationID, chunk, "text"))

	case agent.ChunkTypeToolUse:
		card := s.buildSimpleCard(fmt.Sprintf("🔧 [%s] Using tool: %s", agentName, chunk.ToolName), "blue")
		delivery.DeliverCard(ctx, chatID, card, s.makeDedupKey(platform, invocationID, chunk, "tool_use"))

	case agent.ChunkTypeToolResult:
		statusIcon := "✅"
		if chunk.IsError {
			statusIcon = "❌"
		}
		card := s.buildSimpleCard(fmt.Sprintf("%s [%s] %s completed", statusIcon, agentName, chunk.ToolName), "green")
		delivery.DeliverCard(ctx, chatID, card, s.makeDedupKey(platform, invocationID, chunk, "tool_result"))

	case agent.ChunkTypeError:
		card := s.buildSimpleCard(fmt.Sprintf("⚠️ [%s] Error: %s", agentName, chunk.Content), "red")
		delivery.DeliverCard(ctx, chatID, card, s.makeDedupKey(platform, invocationID, chunk, "error"))

	case agent.ChunkTypeStatus:
		if chunk.Content == "completed" || chunk.Content == "failed" || chunk.Content == "stopped" {
			card := s.buildSimpleCard(fmt.Sprintf("[%s] Status: %s", agentName, chunk.Content), "green")
			delivery.DeliverCard(ctx, chatID, card, s.makeDedupKey(platform, invocationID, chunk, "status"))
		}

	case agent.ChunkTypeThinking, agent.ChunkTypeUsage:
		// Skip non-user-facing chunk types for IM transports.

	default:
		s.logger.Debug("unsupported chunk type for IM bridge", zap.String("type", string(chunk.Type)), zap.String("platform", platform))
	}

	_ = adapter // adapter is intentionally looked up to assert registration completeness.
	_ = agentID
}

func (s *IMBridgeService) getPlatformDelivery(platform string) (IMAdapter, *DeliveryService, bool) {
	s.mu.RLock()
	adapter, okAdapter := s.adapters[platform]
	delivery, okDelivery := s.delivery[platform]
	s.mu.RUnlock()
	if !okAdapter || !okDelivery {
		return nil, nil, false
	}
	return adapter, delivery, true
}

func (s *IMBridgeService) makeDedupKey(platform string, invocationID uuid.UUID, chunk agent.Chunk, kind string) string {
	return fmt.Sprintf("%s:%s:%s:%s", platform, invocationID.String(), kind, chunk.ToolName)
}

func (s *IMBridgeService) buildSimpleCard(content, color string) string {
	return fmt.Sprintf(`{
		"config": {"wide_screen_mode": true},
		"header": {
			"title": {"tag": "plain_text", "content": %q},
			"template": %q
		}
	}`, content, color)
}

// Compile-time signature check for ChunkListener compatibility.
var _ agent.ChunkListener = (*IMBridgeService)(nil).OnAgentChunk
