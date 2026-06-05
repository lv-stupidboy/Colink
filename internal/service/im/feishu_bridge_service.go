// Package im provides Feishu (Lark) IM integration services
package im

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/internal/ws"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// FeishuBridgeService bridges Feishu IM messages to ISDP agent execution.
// Deprecated: use IMBridgeService with FeishuAdapter + DeliveryService.
type FeishuBridgeService struct {
	sessionRepo  *repo.IMSessionRepository
	threadRepo   *repo.ThreadRepository
	projectRepo  *repo.ProjectRepository
	orchestrator *agent.Orchestrator
	larkClient   *LarkCLIClient
	wsHub        *ws.Hub
	cfg          config.FeishuConfig
	logger       *zap.Logger

	// pendingChats tracks in-flight requests to prevent duplicate processing
	pendingMu    sync.Mutex
	pendingChats map[string]bool

	// buffers accumulates text chunks before sending to Feishu
	bufferMu    sync.Mutex
	buffers     map[string]*chunkBuffer
	larkHealthy bool

	// dedupCache deduplicates incoming messages by message ID
	dedupCache *DedupCache

	// rateLimiter enforces outbound message rate limits per chat
	rateLimiter *RateLimiter
}

// chunkBuffer buffers text chunks with debounce timer
type chunkBuffer struct {
	text         strings.Builder
	timer        *time.Timer
	chatID       string
	invocationID string
}

// NewFeishuBridgeService creates a new Feishu bridge service
func NewFeishuBridgeService(
	sessionRepo *repo.IMSessionRepository,
	threadRepo *repo.ThreadRepository,
	projectRepo *repo.ProjectRepository,
	orchestrator *agent.Orchestrator,
	larkClient *LarkCLIClient,
	wsHub *ws.Hub,
	cfg config.FeishuConfig,
	logger *zap.Logger,
) *FeishuBridgeService {
	return &FeishuBridgeService{
		sessionRepo:  sessionRepo,
		threadRepo:   threadRepo,
		projectRepo:  projectRepo,
		orchestrator: orchestrator,
		larkClient:   larkClient,
		wsHub:        wsHub,
		cfg:          cfg,
		logger:       logger,
		pendingChats: make(map[string]bool),
		buffers:      make(map[string]*chunkBuffer),
		larkHealthy:  true,
		dedupCache:   NewDedupCache(1000),
		rateLimiter:  NewRateLimiter(20, 60*time.Second),
	}
}

// SetLarkHealthy sets the lark-cli health status
func (s *FeishuBridgeService) SetLarkHealthy(healthy bool) {
	s.larkHealthy = healthy
}

// HandleMessageEvent processes incoming Feishu message events
func (s *FeishuBridgeService) HandleMessageEvent(ctx context.Context, event FeishuMessageReceivedEvent) {
	// Parse message text
	text := event.Message.ParseTextContent()
	if text == "" {
		s.logger.Debug("empty message text, ignoring")
		return
	}

	// Check for duplicate message
	messageID := event.Message.MessageID
	if s.dedupCache.IsDuplicate(messageID) {
		s.logger.Debug("duplicate message, skipping", zap.String("messageID", messageID))
		return
	}

	// Get user ID from sender
	userID := ""
	if event.Sender.SenderID.OpenID != "" {
		userID = event.Sender.SenderID.OpenID
	} else if event.Sender.SenderID.UserID != "" {
		userID = event.Sender.SenderID.UserID
	}
	userName := "" // Feishu doesn't always provide name in this event

	session, err := s.getOrCreateSession(ctx, event.Message.ChatID, event.Message.ChatType, userID, userName)
	if err != nil {
		s.logger.Error("failed to get or create session",
			zap.String("chatID", event.Message.ChatID),
			zap.Error(err))
		return
	}

	// Trigger agent execution
	s.triggerAgent(ctx, session, text)
}

// getOrCreateSession gets or creates an IM session for the chat
func (s *FeishuBridgeService) getOrCreateSession(ctx context.Context, chatID, chatType, userID, userName string) (*model.IMSession, error) {
	// Try to find existing session
	session, err := s.sessionRepo.FindByChatID(ctx, string(model.IMPlatformFeishu), chatID)
	if err == nil && session.IsActive {
		// Update last message time
		if err := s.sessionRepo.UpdateLastMessageAt(ctx, session.ID); err != nil {
			s.logger.Warn("failed to update last message time", zap.Error(err))
		}
		return session, nil
	}

	// Create new session with new thread
	threadID := uuid.New()
	projectID := uuid.Nil

	// Parse default project ID if configured
	if s.cfg.DefaultProjectID != "" {
		if parsed, err := uuid.Parse(s.cfg.DefaultProjectID); err == nil {
			projectID = parsed
		}
	}

	// Create thread
	thread := &model.Thread{
		ID:           threadID,
		ProjectID:    projectID,
		Name:         fmt.Sprintf("飞书会话 %s", chatID),
		Status:       "idle",
		CurrentPhase: "requirement",
		CurrentAgent: "",
		Depth:        0,
	}

	if err := s.threadRepo.Create(ctx, thread); err != nil {
		return nil, fmt.Errorf("failed to create thread: %w", err)
	}

	// Create IM session
	session = &model.IMSession{
		ID:        uuid.New(),
		Platform:  model.IMPlatformFeishu,
		ChatID:    chatID,
		ChatType:  chatType,
		ThreadID:  threadID,
		ProjectID: projectID,
		UserID:    userID,
		UserName:  userName,
		IsActive:  true,
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create im_session: %w", err)
	}

	return session, nil
}

// triggerAgent triggers agent execution for the session
func (s *FeishuBridgeService) triggerAgent(ctx context.Context, session *model.IMSession, message string) {
	chatID := session.ChatID

	// Check for duplicate request
	s.pendingMu.Lock()
	if s.pendingChats[chatID] {
		s.pendingMu.Unlock()
		// If lark-cli is healthy, send "in progress" message
		if s.larkHealthy {
			_ = s.larkClient.SendTextMessage(ctx, chatID, "Agent 正在执行中，请稍候...")
		}
		return
	}
	s.pendingChats[chatID] = true
	s.pendingMu.Unlock()

	// Ensure cleanup
	defer func() {
		s.pendingMu.Lock()
		delete(s.pendingChats, chatID)
		s.pendingMu.Unlock()
	}()

	// Trigger agent execution
	s.orchestrator.SpawnAgentForUserMessage(ctx, session.ThreadID, message, nil)
}

// OnAgentChunk handles agent chunk output - implements ChunkListener
func (s *FeishuBridgeService) OnAgentChunk(threadID, invocationID uuid.UUID, chunk agent.Chunk, agentID, agentName string) {
	// Skip if lark-cli not healthy
	if !s.larkHealthy {
		return
	}

	// Find session by thread ID
	session, err := s.sessionRepo.FindByThreadID(context.Background(), threadID)
	if err != nil || session == nil {
		// Not an IM session, skip
		return
	}

	chatID := session.ChatID
	ctx := context.Background()

	// Handle different chunk types
	switch chunk.Type {
	case agent.ChunkTypeText:
		s.accumulateAndFlush(chatID, invocationID.String(), chunk.Content)

	case agent.ChunkTypeToolUse:
		if !s.rateLimiter.TryAcquire(chatID) {
			s.logger.Warn("rate limited, dropping message", zap.String("chatID", chatID))
			return
		}
		msg := fmt.Sprintf("🔧 Using tool: %s", chunk.ToolName)
		_ = s.larkClient.SendCardMessage(ctx, chatID, s.buildSimpleCard(msg, "blue"))

	case agent.ChunkTypeToolResult:
		if !s.rateLimiter.TryAcquire(chatID) {
			s.logger.Warn("rate limited, dropping message", zap.String("chatID", chatID))
			return
		}
		statusIcon := "✅"
		if chunk.IsError {
			statusIcon = "❌"
		}
		msg := fmt.Sprintf("%s %s completed", statusIcon, chunk.ToolName)
		_ = s.larkClient.SendCardMessage(ctx, chatID, s.buildSimpleCard(msg, "green"))

	case agent.ChunkTypeError:
		if !s.rateLimiter.TryAcquire(chatID) {
			s.logger.Warn("rate limited, dropping message", zap.String("chatID", chatID))
			return
		}
		msg := fmt.Sprintf("⚠️ Error: %s", chunk.Content)
		_ = s.larkClient.SendCardMessage(ctx, chatID, s.buildSimpleCard(msg, "red"))

	case agent.ChunkTypeStatus:
		if chunk.Content == "completed" || chunk.Content == "failed" || chunk.Content == "stopped" {
			if !s.rateLimiter.TryAcquire(chatID) {
				s.logger.Warn("rate limited, dropping message", zap.String("chatID", chatID))
				return
			}
			s.sendCompletionCard(ctx, chatID, invocationID.String(), chunk.Content)
		}

	case agent.ChunkTypeThinking:
		// Skip thinking chunks for IM

	case agent.ChunkTypeUsage:
		// Could track usage here for completion card
	}
}

// accumulateAndFlush buffers text and flushes on threshold or timer
func (s *FeishuBridgeService) accumulateAndFlush(chatID, invocationID, text string) {
	s.bufferMu.Lock()
	key := chatID + ":" + invocationID

	// Get or create buffer
	buf, exists := s.buffers[key]
	if !exists {
		buf = &chunkBuffer{
			chatID:       chatID,
			invocationID: invocationID,
		}
		s.buffers[key] = buf
	}

	// Append text
	buf.text.WriteString(text)
	currentLen := buf.text.Len()

	// Cancel existing timer if any
	if buf.timer != nil {
		buf.timer.Stop()
	}

	// Flush immediately if >= 200 chars (copy content under lock)
	if currentLen >= 200 {
		flushed := buf.text.String()
		buf.text.Reset()
		s.bufferMu.Unlock()
		s.sendText(chatID, flushed)
		return
	}

	// Set debounce timer - capture content snapshot in closure
	buf.timer = time.AfterFunc(500*time.Millisecond, func() {
		s.bufferMu.Lock()
		b, ok := s.buffers[key]
		if !ok || b.text.Len() == 0 {
			s.bufferMu.Unlock()
			return
		}
		flushed := b.text.String()
		b.text.Reset()
		if b.timer != nil {
			b.timer.Stop()
		}
		s.bufferMu.Unlock()
		s.sendText(chatID, flushed)
	})
	s.bufferMu.Unlock()
}

// sendText sends text to Feishu asynchronously
func (s *FeishuBridgeService) sendText(chatID, text string) {
	go func() {
		if !s.rateLimiter.TryAcquire(chatID) {
			s.logger.Warn("rate limited, dropping message", zap.String("chatID", chatID))
			return
		}
		ctx := context.Background()
		_ = s.larkClient.SendTextMessage(ctx, chatID, text)
	}()
}

// flushBufferLocked sends buffered text to Feishu (caller must hold bufferMu)
func (s *FeishuBridgeService) flushBufferLocked(key string, buf *chunkBuffer) {
	if buf.timer != nil {
		buf.timer.Stop()
		buf.timer = nil
	}

	if buf.text.Len() == 0 {
		return
	}

	text := buf.text.String()
	chatID := buf.chatID

	// Reset buffer
	buf.text.Reset()

	// Send asynchronously to not block
	go func() {
		if !s.rateLimiter.TryAcquire(chatID) {
			s.logger.Warn("rate limited, dropping message", zap.String("chatID", chatID))
			return
		}
		ctx := context.Background()
		_ = s.larkClient.SendTextMessage(ctx, chatID, text)
	}()

	// Remove from map
	delete(s.buffers, key)
}

// sendCompletionCard sends a completion summary card
func (s *FeishuBridgeService) sendCompletionCard(ctx context.Context, chatID, invocationID, status string) {
	headerColor := "green"
	if status == "failed" || status == "stopped" {
		headerColor = "red"
	}

	title := "✅ 执行完成"
	if status == "failed" {
		title = "❌ 执行失败"
	} else if status == "stopped" {
		title = "⏹️ 执行终止"
	}

	card := fmt.Sprintf(`{
		"config": {
			"wide_screen_mode": true
		},
		"header": {
			"title": {
				"tag": "plain_text",
				"content": "%s"
			},
			"template": "%s"
		},
		"elements": [
			{
				"tag": "div",
				"text": {
					"tag": "plain_text",
					"content": "状态: %s"
				}
			}
		]
	}`, title, headerColor, status)

	_ = s.larkClient.SendCardMessage(ctx, chatID, card)
}

// buildSimpleCard builds a simple Feishu card message
func (s *FeishuBridgeService) buildSimpleCard(content, color string) string {
	return fmt.Sprintf(`{
		"config": {
			"wide_screen_mode": true
		},
		"header": {
			"title": {
				"tag": "plain_text",
				"content": "%s"
			},
			"template": "%s"
		}
	}`, content, color)
}
