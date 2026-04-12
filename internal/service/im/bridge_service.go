package im

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type agentSpawner interface {
	SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string) error
}

// invocationCardState tracks an in-flight streaming card for a specific agent invocation.
// Uses sync.Once to guarantee exactly one card creation attempt regardless of concurrent chunks.
type invocationCardState struct {
	createOnce  sync.Once  // guarantees single creation attempt
	cardID      string     // set after successful creation
	failed      bool       // set if creation or a critical update fails
	fallbackMu  sync.Mutex // protects fallbackBuf
	fallbackBuf []string   // accumulated text when streaming card fails
}

// IMBridgeService provides platform-agnostic IM inbound/outbound bridge.
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

	// streamingCards maps invocationID → *streamingCardState.
	// Uses sync.Map for atomic LoadOrStore to prevent race conditions.
	streamingCards sync.Map
}

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
// Text chunks flow into a streaming card (Cardkit v1) with typewriter effect.
// Tool/error/status chunks are sent as separate card messages.
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
		s.handleTextChunk(ctx, adapter, chatID, invocationID, agentName, chunk.Content)

	case agent.ChunkTypeToolUse:
		s.handleTextChunk(ctx, adapter, chatID, invocationID, agentName,
			fmt.Sprintf("\n🔧 **[%s] Using tool: %s**\n", agentName, chunk.ToolName))

	case agent.ChunkTypeToolResult:
		statusIcon := "✅"
		if chunk.IsError {
			statusIcon = "❌"
		}
		s.handleTextChunk(ctx, adapter, chatID, invocationID, agentName,
			fmt.Sprintf("%s **[%s] %s completed**\n", statusIcon, agentName, chunk.ToolName))

	case agent.ChunkTypeError:
		s.handleTextChunk(ctx, adapter, chatID, invocationID, agentName,
			fmt.Sprintf("\n⚠️ **[%s] Error: %s**\n", agentName, chunk.Content))

	case agent.ChunkTypeStatus:
		if chunk.Content == "completed" || chunk.Content == "failed" || chunk.Content == "stopped" {
			s.handleStatusComplete(ctx, adapter, chatID, invocationID, agentName, chunk.Content, delivery, platform)
		}

	case agent.ChunkTypeThinking, agent.ChunkTypeUsage:

	default:
		s.logger.Debug("unsupported chunk type for IM bridge", zap.String("type", string(chunk.Type)), zap.String("platform", platform))
	}

	_ = agentID
}

// handleTextChunk manages the streaming card lifecycle for text content.
// Uses sync.Map.LoadOrStore to atomically reserve a slot per invocation,
// and sync.Once to ensure exactly one card creation attempt.
func (s *IMBridgeService) handleTextChunk(ctx context.Context, adapter IMAdapter, chatID string, invocationID uuid.UUID, agentName, text string) {
	newState := &invocationCardState{}
	actual, _ := s.streamingCards.LoadOrStore(invocationID, newState)
	state := actual.(*invocationCardState)

	state.createOnce.Do(func() {
		cardID, err := adapter.CreateStreamingCard(ctx, chatID, agentName)
		if err != nil {
			s.logger.Error("failed to create streaming card, using buffered fallback",
				zap.String("chatID", chatID), zap.Error(err))
			state.failed = true
			state.fallbackMu.Lock()
			state.fallbackBuf = append(state.fallbackBuf, text)
			state.fallbackMu.Unlock()
			return
		}
		state.cardID = cardID
	})

	if state.failed {
		state.fallbackMu.Lock()
		state.fallbackBuf = append(state.fallbackBuf, text)
		state.fallbackMu.Unlock()
		return
	}

	if err := adapter.UpdateStreamingCard(ctx, state.cardID, text, 0); err != nil {
		s.logger.Error("failed to update streaming card",
			zap.String("cardID", state.cardID), zap.Error(err))
	}
}

// handleStatusComplete finalizes any in-flight streaming card when the agent finishes.
// If streaming card is active, flushes and disables streaming.
// If fallback was active, sends all accumulated text as a single message.
func (s *IMBridgeService) handleStatusComplete(
	ctx context.Context, adapter IMAdapter, chatID string, invocationID uuid.UUID,
	agentName, status string, delivery *DeliveryService, platform string,
) {
	val, loaded := s.streamingCards.LoadAndDelete(invocationID)
	if !loaded {
		return
	}
	state := val.(*invocationCardState)

	if state.failed {
		state.fallbackMu.Lock()
		buf := state.fallbackBuf
		state.fallbackBuf = nil
		state.fallbackMu.Unlock()

		if len(buf) > 0 {
			fullText := strings.Join(buf, "")
			if err := adapter.SendText(ctx, chatID, fullText).Error; err != "" {
				s.logger.Error("failed to send fallback text",
					zap.String("chatID", chatID), zap.String("error", err))
			}
		}
		return
	}

	if err := adapter.FinalizeStreamingCard(ctx, state.cardID, "", 0); err != nil {
		s.logger.Error("failed to finalize streaming card",
			zap.String("cardID", state.cardID), zap.Error(err))
	}
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

var _ agent.ChunkListener = (*IMBridgeService)(nil).OnAgentChunk
