package im

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type mockSpawner struct {
	mu       sync.Mutex
	threadID uuid.UUID
	message  string
	calls    int
}

func (m *mockSpawner) SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.threadID = threadID
	m.message = userMessage
	m.calls++
	return nil
}

type mockIMAdapter struct {
	mu                   sync.Mutex
	textCalls            []string
	cardCalls            []string
	maxMsgLen            int
	platformVal          string
	streamingCardCreated bool
	streamingUpdates     []string
	streamingFinalized   bool
	lastStreamingCardID  string
}

func (m *mockIMAdapter) Platform() string { return m.platformVal }
func (m *mockIMAdapter) SendText(ctx context.Context, chatID, text string) SendResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.textCalls = append(m.textCalls, text)
	return SendResult{OK: true}
}
func (m *mockIMAdapter) SendCard(ctx context.Context, chatID, cardJSON string) SendResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cardCalls = append(m.cardCalls, cardJSON)
	return SendResult{OK: true}
}
func (m *mockIMAdapter) ReplyText(ctx context.Context, chatID, messageID, text string) SendResult {
	return SendResult{OK: true}
}
func (m *mockIMAdapter) CreateStreamingCard(ctx context.Context, chatID string, agentName string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streamingCardCreated = true
	m.lastStreamingCardID = "mock-card-" + chatID
	return m.lastStreamingCardID, nil
}
func (m *mockIMAdapter) UpdateStreamingCard(ctx context.Context, cardID string, content string, sequence int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streamingUpdates = append(m.streamingUpdates, content)
	return nil
}
func (m *mockIMAdapter) FinalizeStreamingCard(ctx context.Context, cardID string, content string, sequence int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streamingFinalized = true
	return nil
}
func (m *mockIMAdapter) CheckHealth(ctx context.Context) error { return nil }
func (m *mockIMAdapter) MaxMessageLength() int {
	if m.maxMsgLen > 0 {
		return m.maxMsgLen
	}
	return 4000
}

func TestIMBridgeService_HandleInboundMessage_CreatesSessionAndSpawns(t *testing.T) {
	db := setupBridgeTestDB(t)
	defer db.Close()

	sessionRepo := repo.NewIMSessionRepository(db)
	threadRepo := repo.NewThreadRepository(db, config.DBTypeSQLite)
	projectRepo := repo.NewProjectRepository(db, config.DBTypeSQLite)
	spawner := &mockSpawner{}

	svc := NewIMBridgeService(sessionRepo, threadRepo, projectRepo, spawner, nil, NewSessionLock(), zap.NewNop())

	err := svc.HandleInboundMessage(context.Background(), "feishu", "chat-1", "group", "u1", "alice", "m1", "hello")
	if err != nil {
		t.Fatalf("HandleInboundMessage error: %v", err)
	}

	if spawner.calls != 1 {
		t.Fatalf("spawn calls = %d, want 1", spawner.calls)
	}
	if spawner.message != "hello" {
		t.Fatalf("spawn message = %q, want hello", spawner.message)
	}

	session, err := sessionRepo.FindByChatID(context.Background(), "feishu", "chat-1")
	if err != nil {
		t.Fatalf("FindByChatID error: %v", err)
	}
	if session.ThreadID == uuid.Nil {
		t.Fatal("threadID should be set for created session")
	}
	if session.ThreadID != spawner.threadID {
		t.Fatalf("spawned threadID = %s, session threadID = %s", spawner.threadID, session.ThreadID)
	}
}

func TestIMBridgeService_HandleInboundMessage_ReusesSession(t *testing.T) {
	db := setupBridgeTestDB(t)
	defer db.Close()

	sessionRepo := repo.NewIMSessionRepository(db)
	threadRepo := repo.NewThreadRepository(db, config.DBTypeSQLite)
	projectRepo := repo.NewProjectRepository(db, config.DBTypeSQLite)
	spawner := &mockSpawner{}

	svc := NewIMBridgeService(sessionRepo, threadRepo, projectRepo, spawner, nil, NewSessionLock(), zap.NewNop())

	if err := svc.HandleInboundMessage(context.Background(), "feishu", "chat-2", "group", "u1", "alice", "m1", "hello"); err != nil {
		t.Fatalf("first HandleInboundMessage error: %v", err)
	}
	firstThread := spawner.threadID

	if err := svc.HandleInboundMessage(context.Background(), "feishu", "chat-2", "group", "u1", "alice", "m2", "follow-up"); err != nil {
		t.Fatalf("second HandleInboundMessage error: %v", err)
	}

	if spawner.calls != 2 {
		t.Fatalf("spawn calls = %d, want 2", spawner.calls)
	}
	if spawner.threadID != firstThread {
		t.Fatalf("thread should be reused, got %s want %s", spawner.threadID, firstThread)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM im_sessions WHERE platform = ? AND chat_id = ?`, "feishu", "chat-2").Scan(&count); err != nil {
		t.Fatalf("count sessions query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("session rows = %d, want 1", count)
	}
}

func TestIMBridgeService_OnAgentChunk_RoutesTextAndCard(t *testing.T) {
	db := setupBridgeTestDB(t)
	defer db.Close()

	sessionRepo := repo.NewIMSessionRepository(db)
	threadRepo := repo.NewThreadRepository(db, config.DBTypeSQLite)

	threadID := uuid.New()
	thread := &model.Thread{ID: threadID, ProjectID: uuid.Nil, Name: "test", Status: model.ThreadStatusIdle, CurrentPhase: model.PhaseRequirement}
	if err := threadRepo.Create(context.Background(), thread); err != nil {
		t.Fatalf("create thread failed: %v", err)
	}

	session := &model.IMSession{ID: uuid.New(), Platform: model.IMPlatformFeishu, ChatID: "chat-3", ChatType: "group", ThreadID: threadID, ProjectID: uuid.Nil, IsActive: true}
	if err := sessionRepo.Create(context.Background(), session); err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	adapter := &mockIMAdapter{platformVal: "feishu"}
	delivery := NewDeliveryService(adapter, RetryConfig{MaxAttempts: 1}, nil, nil, zap.NewNop())
	svc := NewIMBridgeService(sessionRepo, threadRepo, nil, &mockSpawner{}, nil, NewSessionLock(), zap.NewNop())
	svc.RegisterAdapter(adapter, delivery)

	invocationID := uuid.New()
	svc.OnAgentChunk(threadID, invocationID, agent.Chunk{Type: agent.ChunkTypeText, Content: "hello from agent"}, "a1", "AgentA")
	svc.OnAgentChunk(threadID, invocationID, agent.Chunk{Type: agent.ChunkTypeToolUse, ToolName: "bash"}, "a1", "AgentA")

	if !adapter.streamingCardCreated {
		t.Fatal("streaming card should be created for text chunk")
	}
	if len(adapter.streamingUpdates) != 2 {
		t.Fatalf("streaming updates = %d, want 2 (text + tool-use)", len(adapter.streamingUpdates))
	}
	if adapter.streamingUpdates[0] != "hello from agent" {
		t.Fatalf("first streaming content = %q, want %q", adapter.streamingUpdates[0], "hello from agent")
	}
	if !strings.Contains(adapter.streamingUpdates[1], "bash") {
		t.Fatalf("second streaming content should contain tool name, got: %s", adapter.streamingUpdates[1])
	}
	if len(adapter.cardCalls) != 0 {
		t.Fatalf("card sends = %d, want 0 (all content goes through streaming card)", len(adapter.cardCalls))
	}
}

func TestIMBridgeService_OnAgentChunk_StatusIgnoredWhenNotTerminal(t *testing.T) {
	db := setupBridgeTestDB(t)
	defer db.Close()

	sessionRepo := repo.NewIMSessionRepository(db)
	threadRepo := repo.NewThreadRepository(db, config.DBTypeSQLite)

	threadID := uuid.New()
	thread := &model.Thread{ID: threadID, ProjectID: uuid.Nil, Name: "test", Status: model.ThreadStatusIdle, CurrentPhase: model.PhaseRequirement}
	if err := threadRepo.Create(context.Background(), thread); err != nil {
		t.Fatalf("create thread failed: %v", err)
	}

	session := &model.IMSession{ID: uuid.New(), Platform: model.IMPlatformFeishu, ChatID: "chat-4", ChatType: "group", ThreadID: threadID, ProjectID: uuid.Nil, IsActive: true}
	if err := sessionRepo.Create(context.Background(), session); err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	adapter := &mockIMAdapter{platformVal: "feishu"}
	delivery := NewDeliveryService(adapter, RetryConfig{MaxAttempts: 1}, nil, nil, zap.NewNop())
	svc := NewIMBridgeService(sessionRepo, threadRepo, nil, &mockSpawner{}, nil, NewSessionLock(), zap.NewNop())
	svc.RegisterAdapter(adapter, delivery)

	svc.OnAgentChunk(threadID, uuid.New(), agent.Chunk{Type: agent.ChunkTypeStatus, Content: "running"}, "a1", "AgentA")

	if len(adapter.cardCalls) != 0 {
		t.Fatalf("non-terminal status should not send card, got %d cards", len(adapter.cardCalls))
	}
}

func setupBridgeTestDB(t *testing.T) *sql.DB {
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
