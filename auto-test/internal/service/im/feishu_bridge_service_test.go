package im_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestFeishuBridgeService_DedupCache_Integration(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	cache := NewDedupCache(1000)

	messageID1 := "msg_123"
	messageID2 := "msg_456"

	if cache.IsDuplicate(messageID1) {
		t.Error("first message should not be duplicate")
	}

	if !cache.IsDuplicate(messageID1) {
		t.Error("second occurrence of same message should be duplicate")
	}

	if cache.IsDuplicate(messageID2) {
		t.Error("different message should not be duplicate")
	}

	if cache.Size() != 2 {
		t.Errorf("cache size = %d, want 2", cache.Size())
	}
}

func TestFeishuBridgeService_NewFeishuBridgeService_InitializesDedupCache(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	service := NewFeishuBridgeService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		config.FeishuConfig{},
		logger,
	)

	if service.dedupCache == nil {
		t.Error("dedupCache should be initialized, got nil")
	}

	if service.dedupCache.Size() != 0 {
		t.Errorf("dedupCache initial size = %d, want 0", service.dedupCache.Size())
	}
}

func TestFeishuBridgeService_NewFeishuBridgeService_InitializesRateLimiter(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	service := NewFeishuBridgeService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		config.FeishuConfig{},
		logger,
	)

	if service.rateLimiter == nil {
		t.Error("rateLimiter should be initialized, got nil")
	}
}

func TestFeishuBridgeService_RateLimiter_EnforcesLimit(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	service := NewFeishuBridgeService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		config.FeishuConfig{},
		logger,
	)

	chatID := "chat_123"

	for i := 0; i < 20; i++ {
		if !service.rateLimiter.TryAcquire(chatID) {
			t.Errorf("message %d should be allowed (under limit of 20)", i+1)
		}
	}

	if service.rateLimiter.TryAcquire(chatID) {
		t.Error("message 21 should be rate limited (exceeds limit of 20)")
	}
}

func TestHandleMessageEvent_NewSession(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	called := make(chan string, 1)
	service := &mockBridgeService{
		onSpawnAgent: func(threadID uuid.UUID, message string) {
			called <- message
		},
		logger: logger,
	}

	ctx := context.Background()
	event := FeishuMessageReceivedEvent{
		Message: FeishuMessage{
			MessageID:   "msg_001",
			ChatID:      "chat_test",
			ChatType:    "group",
			MessageType: "text",
			Content:     `{"text":"Hello Agent"}`,
		},
		Sender: FeishuSender{
			SenderID: struct {
				OpenID  string `json:"open_id"`
				UserID  string `json:"user_id"`
				UnionID string `json:"union_id"`
			}{
				OpenID: "ou_test_user",
			},
		},
	}

	service.HandleMessageEvent(ctx, event)

	select {
	case msg := <-called:
		if msg != "Hello Agent" {
			t.Errorf("spawned message = %q, want %q", msg, "Hello Agent")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("agent spawn not called within timeout")
	}

	if service.sessionCreated != 1 {
		t.Errorf("session created %d times, want 1", service.sessionCreated)
	}
	if service.threadCreated != 1 {
		t.Errorf("thread created %d times, want 1", service.threadCreated)
	}
}

func TestHandleMessageEvent_ExistingSession(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	existingThreadID := uuid.New()
	called := make(chan uuid.UUID, 1)
	service := &mockBridgeService{
		existingSession: &model.IMSession{
			ID:       uuid.New(),
			Platform: model.IMPlatformFeishu,
			ChatID:   "chat_existing",
			ThreadID: existingThreadID,
			IsActive: true,
		},
		onSpawnAgent: func(threadID uuid.UUID, message string) {
			called <- threadID
		},
		logger: logger,
	}

	ctx := context.Background()
	event := FeishuMessageReceivedEvent{
		Message: FeishuMessage{
			MessageID:   "msg_002",
			ChatID:      "chat_existing",
			ChatType:    "group",
			MessageType: "text",
			Content:     `{"text":"Follow up message"}`,
		},
		Sender: FeishuSender{},
	}

	service.HandleMessageEvent(ctx, event)

	select {
	case threadID := <-called:
		if threadID != existingThreadID {
			t.Errorf("spawned for thread %v, want %v", threadID, existingThreadID)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("agent spawn not called within timeout")
	}

	if service.sessionCreated != 0 {
		t.Errorf("session created %d times, want 0 (should reuse)", service.sessionCreated)
	}
	if service.lastMessageUpdated != 1 {
		t.Errorf("last message updated %d times, want 1", service.lastMessageUpdated)
	}
}

func TestHandleMessageEvent_EmptyText(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	service := &mockBridgeService{
		onSpawnAgent: func(threadID uuid.UUID, message string) {
			t.Error("should not spawn agent for empty message")
		},
		logger: logger,
	}

	ctx := context.Background()
	event := FeishuMessageReceivedEvent{
		Message: FeishuMessage{
			MessageID:   "msg_empty",
			ChatID:      "chat_test",
			ChatType:    "group",
			MessageType: "text",
			Content:     `{"text":""}`,
		},
	}

	service.HandleMessageEvent(ctx, event)

	time.Sleep(50 * time.Millisecond)

	if service.sessionCreated > 0 {
		t.Error("should not create session for empty message")
	}
}

func TestOnAgentChunk_TextBuffering(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	sentMessages := make(chan string, 10)
	service := &mockBridgeService{
		onSendText: func(chatID, text string) {
			sentMessages <- text
		},
		logger: logger,
	}

	threadID := uuid.New()
	chatID := "chat_buffering"
	service.existingSession = &model.IMSession{
		ID:       uuid.New(),
		Platform: model.IMPlatformFeishu,
		ChatID:   chatID,
		ThreadID: threadID,
		IsActive: true,
	}

	service.initBridgeService()
	invocationID := uuid.New()

	chunk1 := agent.Chunk{Type: agent.ChunkTypeText, Content: "Hello "}
	chunk2 := agent.Chunk{Type: agent.ChunkTypeText, Content: "world!"}

	service.OnAgentChunk(threadID, invocationID, chunk1, "agent1", "TestAgent")
	service.OnAgentChunk(threadID, invocationID, chunk2, "agent1", "TestAgent")

	select {
	case msg := <-sentMessages:
		if msg != "Hello world!" {
			t.Errorf("sent message = %q, want %q", msg, "Hello world!")
		}
	case <-time.After(700 * time.Millisecond):
		t.Error("text message not sent after debounce timeout")
	}
}

func TestOnAgentChunk_ToolUse(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	sentCards := make(chan string, 10)
	service := &mockBridgeService{
		onSendCard: func(chatID, card string) {
			sentCards <- card
		},
		logger: logger,
	}

	threadID := uuid.New()
	chatID := "chat_tooluse"
	service.existingSession = &model.IMSession{
		ID:       uuid.New(),
		Platform: model.IMPlatformFeishu,
		ChatID:   chatID,
		ThreadID: threadID,
		IsActive: true,
	}

	service.initBridgeService()
	invocationID := uuid.New()

	chunk := agent.Chunk{
		Type:     agent.ChunkTypeToolUse,
		ToolName: "bash",
	}

	service.OnAgentChunk(threadID, invocationID, chunk, "agent1", "TestAgent")

	select {
	case card := <-sentCards:
		if !strings.Contains(card, "bash") {
			t.Errorf("card should contain tool name 'bash', got: %s", card)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("tool use card not sent")
	}
}

func TestOnAgentChunk_Error(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	sentCards := make(chan string, 10)
	service := &mockBridgeService{
		onSendCard: func(chatID, card string) {
			sentCards <- card
		},
		logger: logger,
	}

	threadID := uuid.New()
	chatID := "chat_error"
	service.existingSession = &model.IMSession{
		ID:       uuid.New(),
		Platform: model.IMPlatformFeishu,
		ChatID:   chatID,
		ThreadID: threadID,
		IsActive: true,
	}

	service.initBridgeService()
	invocationID := uuid.New()

	chunk := agent.Chunk{
		Type:    agent.ChunkTypeError,
		Content: "Something went wrong",
	}

	service.OnAgentChunk(threadID, invocationID, chunk, "agent1", "TestAgent")

	select {
	case card := <-sentCards:
		if !strings.Contains(card, "red") {
			t.Error("error card should have red template")
		}
		if !strings.Contains(card, "Something went wrong") {
			t.Errorf("card should contain error message, got: %s", card)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("error card not sent")
	}
}

func TestOnAgentChunk_Status(t *testing.T) {
	logger := zap.NewNop()
	defer logger.Sync()

	sentCards := make(chan string, 10)
	service := &mockBridgeService{
		onSendCard: func(chatID, card string) {
			sentCards <- card
		},
		logger: logger,
	}

	threadID := uuid.New()
	chatID := "chat_status"
	service.existingSession = &model.IMSession{
		ID:       uuid.New(),
		Platform: model.IMPlatformFeishu,
		ChatID:   chatID,
		ThreadID: threadID,
		IsActive: true,
	}

	service.initBridgeService()
	invocationID := uuid.New()

	chunk := agent.Chunk{
		Type:    agent.ChunkTypeStatus,
		Content: "completed",
	}

	service.OnAgentChunk(threadID, invocationID, chunk, "agent1", "TestAgent")

	select {
	case card := <-sentCards:
		if !strings.Contains(card, "completed") {
			t.Errorf("completion card should contain status, got: %s", card)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("completion card not sent")
	}
}

type mockBridgeService struct {
	FeishuBridgeService
	existingSession    *model.IMSession
	sessionCreated     int
	threadCreated      int
	lastMessageUpdated int
	onSpawnAgent       func(threadID uuid.UUID, message string)
	onSendText         func(chatID, text string)
	onSendCard         func(chatID, card string)
	logger             *zap.Logger
}

func (m *mockBridgeService) initBridgeService() {
	m.pendingChats = make(map[string]bool)
	m.buffers = make(map[string]*chunkBuffer)
	m.larkHealthy = true
	m.dedupCache = NewDedupCache(1000)
	m.rateLimiter = NewRateLimiter(20, 60*time.Second)
}

func (m *mockBridgeService) HandleMessageEvent(ctx context.Context, event FeishuMessageReceivedEvent) {
	text := event.Message.ParseTextContent()
	if text == "" {
		return
	}

	messageID := event.Message.MessageID
	if m.dedupCache == nil {
		m.dedupCache = NewDedupCache(1000)
	}
	if m.dedupCache.IsDuplicate(messageID) {
		return
	}

	var session *model.IMSession
	if m.existingSession != nil && m.existingSession.ChatID == event.Message.ChatID {
		session = m.existingSession
		m.lastMessageUpdated++
	} else {
		threadID := uuid.New()
		session = &model.IMSession{
			ID:       uuid.New(),
			Platform: model.IMPlatformFeishu,
			ChatID:   event.Message.ChatID,
			ThreadID: threadID,
			IsActive: true,
		}
		m.sessionCreated++
		m.threadCreated++
	}

	if m.onSpawnAgent != nil {
		m.onSpawnAgent(session.ThreadID, text)
	}
}

func (m *mockBridgeService) OnAgentChunk(threadID, invocationID uuid.UUID, chunk agent.Chunk, agentID, agentName string) {
	if m.existingSession == nil || m.existingSession.ThreadID != threadID {
		return
	}

	chatID := m.existingSession.ChatID
	ctx := context.Background()

	switch chunk.Type {
	case agent.ChunkTypeText:
		m.accumulateAndFlush(chatID, invocationID.String(), chunk.Content)

	case agent.ChunkTypeToolUse:
		if m.onSendCard != nil {
			m.onSendCard(chatID, m.buildSimpleCard("🔧 Using tool: "+chunk.ToolName, "blue"))
		}

	case agent.ChunkTypeError:
		if m.onSendCard != nil {
			m.onSendCard(chatID, m.buildSimpleCard("⚠️ Error: "+chunk.Content, "red"))
		}

	case agent.ChunkTypeStatus:
		if chunk.Content == "completed" || chunk.Content == "failed" || chunk.Content == "stopped" {
			if m.onSendCard != nil {
				m.sendCompletionCard(ctx, chatID, invocationID.String(), chunk.Content)
			}
		}
	}
}

func (m *mockBridgeService) accumulateAndFlush(chatID, invocationID, text string) {
	m.bufferMu.Lock()
	key := chatID + ":" + invocationID

	buf, exists := m.buffers[key]
	if !exists {
		buf = &chunkBuffer{
			chatID:       chatID,
			invocationID: invocationID,
		}
		m.buffers[key] = buf
	}

	buf.text.WriteString(text)
	currentLen := buf.text.Len()

	if buf.timer != nil {
		buf.timer.Stop()
	}

	if currentLen >= 200 {
		flushed := buf.text.String()
		buf.text.Reset()
		m.bufferMu.Unlock()
		if m.onSendText != nil {
			m.onSendText(chatID, flushed)
		}
		return
	}

	buf.timer = time.AfterFunc(500*time.Millisecond, func() {
		m.bufferMu.Lock()
		b, ok := m.buffers[key]
		if !ok || b.text.Len() == 0 {
			m.bufferMu.Unlock()
			return
		}
		flushed := b.text.String()
		b.text.Reset()
		if b.timer != nil {
			b.timer.Stop()
		}
		m.bufferMu.Unlock()
		if m.onSendText != nil {
			m.onSendText(chatID, flushed)
		}
	})
	m.bufferMu.Unlock()
}

func (m *mockBridgeService) sendCompletionCard(ctx context.Context, chatID, invocationID, status string) {
	if m.onSendCard != nil {
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

		card := `{"header":{"title":"` + title + `","template":"` + headerColor + `"},"elements":[{"text":"状态: ` + status + `"}]}`
		m.onSendCard(chatID, card)
	}
}

func (m *mockBridgeService) buildSimpleCard(content, color string) string {
	return `{"header":{"title":"` + content + `","template":"` + color + `"}}`
}
