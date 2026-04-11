package im

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// integrationTestAdapter records all sent messages for verification.
type integrationTestAdapter struct {
	mu        sync.Mutex
	platform  string
	textSent  []string
	cardSent  []string
	maxMsgLen int
}

func newIntegrationTestAdapter(platform string, maxLen int) *integrationTestAdapter {
	return &integrationTestAdapter{
		platform:  platform,
		maxMsgLen: maxLen,
	}
}

func (f *integrationTestAdapter) Platform() string { return f.platform }

func (f *integrationTestAdapter) SendText(ctx context.Context, chatID, text string) SendResult {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.textSent = append(f.textSent, text)
	return SendResult{OK: true}
}

func (f *integrationTestAdapter) SendCard(ctx context.Context, chatID, cardJSON string) SendResult {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cardSent = append(f.cardSent, cardJSON)
	return SendResult{OK: true}
}

func (f *integrationTestAdapter) ReplyText(ctx context.Context, chatID, messageID, text string) SendResult {
	return SendResult{OK: true}
}

func (f *integrationTestAdapter) CreateStreamingCard(ctx context.Context, chatID string) (string, error) {
	return "card-id-1", nil
}

func (f *integrationTestAdapter) UpdateStreamingCard(ctx context.Context, cardID string, content string, sequence int) error {
	return nil
}

func (f *integrationTestAdapter) FinalizeStreamingCard(ctx context.Context, cardID string, content string, sequence int) error {
	return nil
}

func (f *integrationTestAdapter) CheckHealth(ctx context.Context) error {
	return nil
}

func (f *integrationTestAdapter) MaxMessageLength() int {
	if f.maxMsgLen > 0 {
		return f.maxMsgLen
	}
	return 4000
}

func (f *integrationTestAdapter) getSentText() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.textSent...)
}

func (f *integrationTestAdapter) getSentCards() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.cardSent...)
}

// fakeOrchestrator records spawned agents and triggers chunk callbacks.
type fakeOrchestrator struct {
	mu         sync.Mutex
	spawned    []spawnRecord
	onChunk    func(threadID, invocationID uuid.UUID, chunk agent.Chunk, agentID, agentName string)
	shouldFail bool
}

type spawnRecord struct {
	threadID uuid.UUID
	message  string
}

func (f *fakeOrchestrator) SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string) error {
	f.mu.Lock()
	f.spawned = append(f.spawned, spawnRecord{threadID: threadID, message: userMessage})
	onChunk := f.onChunk
	fail := f.shouldFail
	f.mu.Unlock()

	if fail {
		return nil // don't trigger chunks on failure
	}

	if onChunk != nil {
		invocationID := uuid.New()

		// Simulate agent execution with chunks
		onChunk(threadID, invocationID, agent.Chunk{
			Type:    agent.ChunkTypeText,
			Content: "Agent response to: " + userMessage,
		}, "agent-1", "TestAgent")

		onChunk(threadID, invocationID, agent.Chunk{
			Type:     agent.ChunkTypeToolUse,
			ToolName: "read",
			ToolID:   "tool-1",
		}, "agent-1", "TestAgent")

		onChunk(threadID, invocationID, agent.Chunk{
			Type:     agent.ChunkTypeToolResult,
			ToolName: "read",
			IsError:  false,
		}, "agent-1", "TestAgent")

		onChunk(threadID, invocationID, agent.Chunk{
			Type:    agent.ChunkTypeStatus,
			Content: "completed",
		}, "agent-1", "TestAgent")
	}

	return nil
}

func (f *fakeOrchestrator) getSpawnedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.spawned)
}

func (f *fakeOrchestrator) getLastSpawned() *spawnRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.spawned) == 0 {
		return nil
	}
	return &f.spawned[len(f.spawned)-1]
}

// TestFullIMFlow tests the complete end-to-end flow:
// webhook → session → agent → chunks → delivery → adapter
func TestFullIMFlow(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	sessionRepo := repo.NewIMSessionRepository(db)
	threadRepo := repo.NewThreadRepository(db, config.DBTypeSQLite)
	projectRepo := repo.NewProjectRepository(db, config.DBTypeSQLite)

	adapter := newIntegrationTestAdapter("feishu", 4000)
	rateLimiter := NewRateLimiter(20, 60*time.Second)
	dedupCache := NewDedupCache(1000)
	retryConfig := RetryConfig{MaxAttempts: 1}
	delivery := NewDeliveryService(adapter, retryConfig, rateLimiter, dedupCache, zap.NewNop())

	orchestrator := &fakeOrchestrator{}
	bridge := NewIMBridgeService(sessionRepo, threadRepo, projectRepo, orchestrator, nil, NewSessionLock(), zap.NewNop())
	bridge.RegisterAdapter(adapter, delivery)

	// Wire chunk callback from orchestrator to bridge
	orchestrator.onChunk = bridge.OnAgentChunk

	// Scenario 1: New user message → session created → agent spawned → text chunk → message sent to adapter
	t.Run("NewMessage_CreatesSession_SpawnsAgent_DeliversChunks", func(t *testing.T) {
		ctx := context.Background()

		err := bridge.HandleInboundMessage(ctx, "feishu", "chat-1", "group", "user-1", "Alice", "msg-1", "Hello, agent!")
		if err != nil {
			t.Fatalf("HandleInboundMessage failed: %v", err)
		}

		// Verify session created
		session, err := sessionRepo.FindByChatID(ctx, "feishu", "chat-1")
		if err != nil {
			t.Fatalf("FindByChatID failed: %v", err)
		}
		if session == nil {
			t.Fatal("session should be created")
		}
		if session.ThreadID == uuid.Nil {
			t.Fatal("session.ThreadID should be set")
		}

		// Verify agent spawned
		if orchestrator.getSpawnedCount() != 1 {
			t.Fatalf("spawn count = %d, want 1", orchestrator.getSpawnedCount())
		}
		lastSpawn := orchestrator.getLastSpawned()
		if lastSpawn.message != "Hello, agent!" {
			t.Fatalf("spawn message = %q, want %q", lastSpawn.message, "Hello, agent!")
		}

		// Verify chunks delivered to adapter
		textSent := adapter.getSentText()
		if len(textSent) != 1 {
			t.Fatalf("text sent count = %d, want 1", len(textSent))
		}
		if textSent[0] != "Agent response to: Hello, agent!" {
			t.Fatalf("text sent = %q, want %q", textSent[0], "Agent response to: Hello, agent!")
		}

		cardSent := adapter.getSentCards()
		// Should have: tool_use card, tool_result card, status card (3 total)
		if len(cardSent) != 3 {
			t.Fatalf("card sent count = %d, want 3", len(cardSent))
		}
	})

	// Scenario 2: Duplicate message → dedup skips it
	t.Run("DuplicateMessage_SkippedByDedup", func(t *testing.T) {
		ctx := context.Background()

		// Clear previous state
		adapter.textSent = nil
		adapter.cardSent = nil

		// Create session and thread manually
		threadID := uuid.New()
		thread := &model.Thread{
			ID:           threadID,
			ProjectID:    uuid.Nil,
			Name:         "test-thread",
			Status:       model.ThreadStatusIdle,
			CurrentPhase: model.PhaseRequirement,
		}
		if err := threadRepo.Create(ctx, thread); err != nil {
			t.Fatalf("create thread failed: %v", err)
		}

		session := &model.IMSession{
			ID:        uuid.New(),
			Platform:  model.IMPlatformFeishu,
			ChatID:    "chat-dedup",
			ChatType:  "group",
			ThreadID:  threadID,
			ProjectID: uuid.Nil,
			IsActive:  true,
		}
		if err := sessionRepo.Create(ctx, session); err != nil {
			t.Fatalf("create session failed: %v", err)
		}

		// Trigger chunks with same dedup key twice
		invocationID := uuid.New()
		bridge.OnAgentChunk(threadID, invocationID, agent.Chunk{Type: agent.ChunkTypeText, Content: "first"}, "a1", "Agent")
		bridge.OnAgentChunk(threadID, invocationID, agent.Chunk{Type: agent.ChunkTypeText, Content: "first"}, "a1", "Agent")

		textSent := adapter.getSentText()
		// Should only send once (dedup on second call)
		if len(textSent) != 1 {
			t.Fatalf("text sent count = %d, want 1 (deduped)", len(textSent))
		}
	})

	// Scenario 3: Rate limited message → dropped with warning
	t.Run("RateLimitedMessage_Dropped", func(t *testing.T) {
		ctx := context.Background()

		// Clear previous state
		adapter.textSent = nil
		adapter.cardSent = nil

		// Create a strict rate limiter (max 1 message per 10 seconds)
		strictLimiter := NewRateLimiter(1, 10*time.Second)
		strictDelivery := NewDeliveryService(adapter, RetryConfig{MaxAttempts: 1}, strictLimiter, nil, zap.NewNop())

		// Create session and thread
		threadID := uuid.New()
		thread := &model.Thread{
			ID:           threadID,
			ProjectID:    uuid.Nil,
			Name:         "test-thread-rl",
			Status:       model.ThreadStatusIdle,
			CurrentPhase: model.PhaseRequirement,
		}
		if err := threadRepo.Create(ctx, thread); err != nil {
			t.Fatalf("create thread failed: %v", err)
		}

		session := &model.IMSession{
			ID:        uuid.New(),
			Platform:  model.IMPlatformFeishu,
			ChatID:    "chat-ratelimit",
			ChatType:  "group",
			ThreadID:  threadID,
			ProjectID: uuid.Nil,
			IsActive:  true,
		}
		if err := sessionRepo.Create(ctx, session); err != nil {
			t.Fatalf("create session failed: %v", err)
		}

		// First message should succeed
		result1 := strictDelivery.DeliverText(ctx, "chat-ratelimit", "message 1", "key1")
		if !result1.OK {
			t.Fatalf("first message should succeed, got error: %s", result1.FinalError)
		}

		// Second message should be rate limited
		result2 := strictDelivery.DeliverText(ctx, "chat-ratelimit", "message 2", "key2")
		if result2.OK {
			t.Fatal("second message should be rate limited")
		}
		if result2.Category != ErrCategoryRateLimit {
			t.Fatalf("error category = %s, want %s", result2.Category, ErrCategoryRateLimit)
		}

		textSent := adapter.getSentText()
		// Only first message sent
		if len(textSent) != 1 {
			t.Fatalf("text sent count = %d, want 1 (rate limited)", len(textSent))
		}
	})

	// Scenario 4: Error chunk → error card sent
	t.Run("ErrorChunk_SendsErrorCard", func(t *testing.T) {
		ctx := context.Background()

		// Clear previous state
		adapter.textSent = nil
		adapter.cardSent = nil

		// Create session and thread
		threadID := uuid.New()
		thread := &model.Thread{
			ID:           threadID,
			ProjectID:    uuid.Nil,
			Name:         "test-thread-err",
			Status:       model.ThreadStatusIdle,
			CurrentPhase: model.PhaseRequirement,
		}
		if err := threadRepo.Create(ctx, thread); err != nil {
			t.Fatalf("create thread failed: %v", err)
		}

		session := &model.IMSession{
			ID:        uuid.New(),
			Platform:  model.IMPlatformFeishu,
			ChatID:    "chat-error",
			ChatType:  "group",
			ThreadID:  threadID,
			ProjectID: uuid.Nil,
			IsActive:  true,
		}
		if err := sessionRepo.Create(ctx, session); err != nil {
			t.Fatalf("create session failed: %v", err)
		}

		// Send error chunk
		bridge.OnAgentChunk(threadID, uuid.New(), agent.Chunk{
			Type:    agent.ChunkTypeError,
			Content: "Something went wrong",
		}, "a1", "ErrorAgent")

		cardSent := adapter.getSentCards()
		if len(cardSent) != 1 {
			t.Fatalf("card sent count = %d, want 1", len(cardSent))
		}

		// Verify error card contains error message
		if len(cardSent[0]) == 0 || cardSent[0] == "" {
			t.Fatal("error card should not be empty")
		}
	})

	// Scenario 5: Status chunk → completion card sent
	t.Run("StatusChunk_SendsCompletionCard", func(t *testing.T) {
		ctx := context.Background()

		// Clear previous state
		adapter.textSent = nil
		adapter.cardSent = nil

		// Create session and thread
		threadID := uuid.New()
		thread := &model.Thread{
			ID:           threadID,
			ProjectID:    uuid.Nil,
			Name:         "test-thread-status",
			Status:       model.ThreadStatusIdle,
			CurrentPhase: model.PhaseRequirement,
		}
		if err := threadRepo.Create(ctx, thread); err != nil {
			t.Fatalf("create thread failed: %v", err)
		}

		session := &model.IMSession{
			ID:        uuid.New(),
			Platform:  model.IMPlatformFeishu,
			ChatID:    "chat-status",
			ChatType:  "group",
			ThreadID:  threadID,
			ProjectID: uuid.Nil,
			IsActive:  true,
		}
		if err := sessionRepo.Create(ctx, session); err != nil {
			t.Fatalf("create session failed: %v", err)
		}

		// Send completed status
		bridge.OnAgentChunk(threadID, uuid.New(), agent.Chunk{
			Type:    agent.ChunkTypeStatus,
			Content: "completed",
		}, "a1", "StatusAgent")

		cardSent := adapter.getSentCards()
		if len(cardSent) != 1 {
			t.Fatalf("card sent count = %d, want 1", len(cardSent))
		}

		// Send failed status
		bridge.OnAgentChunk(threadID, uuid.New(), agent.Chunk{
			Type:    agent.ChunkTypeStatus,
			Content: "failed",
		}, "a1", "StatusAgent")

		cardSent = adapter.getSentCards()
		if len(cardSent) != 2 {
			t.Fatalf("card sent count = %d, want 2", len(cardSent))
		}

		// Send non-terminal status (should not send card)
		bridge.OnAgentChunk(threadID, uuid.New(), agent.Chunk{
			Type:    agent.ChunkTypeStatus,
			Content: "running",
		}, "a1", "StatusAgent")

		cardSent = adapter.getSentCards()
		if len(cardSent) != 2 {
			t.Fatalf("card sent count = %d, want 2 (non-terminal ignored)", len(cardSent))
		}
	})

	// Scenario 6: Multi-platform routing: Feishu session → Feishu adapter
	t.Run("MultiPlatform_RoutesToCorrectAdapter", func(t *testing.T) {
		ctx := context.Background()

		// Create a second platform adapter
		slackAdapter := newIntegrationTestAdapter("slack", 3000)
		slackDelivery := NewDeliveryService(slackAdapter, retryConfig, nil, nil, zap.NewNop())
		bridge.RegisterAdapter(slackAdapter, slackDelivery)

		// Clear previous state
		adapter.textSent = nil
		slackAdapter.textSent = nil

		// Create Feishu session
		threadIDFeishu := uuid.New()
		threadFeishu := &model.Thread{
			ID:           threadIDFeishu,
			ProjectID:    uuid.Nil,
			Name:         "feishu-thread",
			Status:       model.ThreadStatusIdle,
			CurrentPhase: model.PhaseRequirement,
		}
		if err := threadRepo.Create(ctx, threadFeishu); err != nil {
			t.Fatalf("create feishu thread failed: %v", err)
		}

		sessionFeishu := &model.IMSession{
			ID:        uuid.New(),
			Platform:  model.IMPlatformFeishu,
			ChatID:    "chat-feishu-multi",
			ChatType:  "group",
			ThreadID:  threadIDFeishu,
			ProjectID: uuid.Nil,
			IsActive:  true,
		}
		if err := sessionRepo.Create(ctx, sessionFeishu); err != nil {
			t.Fatalf("create feishu session failed: %v", err)
		}

		// Create Slack session
		threadIDSlack := uuid.New()
		threadSlack := &model.Thread{
			ID:           threadIDSlack,
			ProjectID:    uuid.Nil,
			Name:         "slack-thread",
			Status:       model.ThreadStatusIdle,
			CurrentPhase: model.PhaseRequirement,
		}
		if err := threadRepo.Create(ctx, threadSlack); err != nil {
			t.Fatalf("create slack thread failed: %v", err)
		}

		sessionSlack := &model.IMSession{
			ID:        uuid.New(),
			Platform:  "slack",
			ChatID:    "chat-slack-multi",
			ChatType:  "channel",
			ThreadID:  threadIDSlack,
			ProjectID: uuid.Nil,
			IsActive:  true,
		}
		if err := sessionRepo.Create(ctx, sessionSlack); err != nil {
			t.Fatalf("create slack session failed: %v", err)
		}

		// Send chunk to Feishu thread
		bridge.OnAgentChunk(threadIDFeishu, uuid.New(), agent.Chunk{
			Type:    agent.ChunkTypeText,
			Content: "feishu message",
		}, "a1", "MultiAgent")

		// Send chunk to Slack thread
		bridge.OnAgentChunk(threadIDSlack, uuid.New(), agent.Chunk{
			Type:    agent.ChunkTypeText,
			Content: "slack message",
		}, "a2", "MultiAgent")

		// Verify routing
		feishuText := adapter.getSentText()
		if len(feishuText) != 1 {
			t.Fatalf("feishu text count = %d, want 1", len(feishuText))
		}
		if feishuText[0] != "feishu message" {
			t.Fatalf("feishu text = %q, want %q", feishuText[0], "feishu message")
		}

		slackText := slackAdapter.getSentText()
		if len(slackText) != 1 {
			t.Fatalf("slack text count = %d, want 1", len(slackText))
		}
		if slackText[0] != "slack message" {
			t.Fatalf("slack text = %q, want %q", slackText[0], "slack message")
		}
	})
}

func setupIntegrationTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS threads (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			name TEXT,
			status TEXT,
			current_phase TEXT,
			current_agent TEXT,
			depth INTEGER,
			workflow_template_id TEXT,
			abort_token TEXT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS im_sessions (
			id TEXT PRIMARY KEY,
			platform TEXT NOT NULL,
			chat_id TEXT NOT NULL,
			chat_type TEXT,
			thread_id TEXT NOT NULL,
			project_id TEXT NOT NULL,
			user_id TEXT,
			user_name TEXT,
			last_message_at TIMESTAMP NULL,
			is_active BOOLEAN NOT NULL,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			UNIQUE(platform, chat_id)
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("failed to create test table: %v", err)
		}
	}

	return db
}
