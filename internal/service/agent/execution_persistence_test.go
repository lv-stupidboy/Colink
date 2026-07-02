package agent

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestExecutionServicePersistenceAndRecoveryHelpers(t *testing.T) {
	ctx := context.Background()
	db := openExecutionPersistenceDB(t)
	msgRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	invocationRepo := repo.NewAgentInvocationRepository(db, repo.DBTypeSQLite)
	blockRepo := repo.NewContentBlockRepository(db, repo.DBTypeSQLite)
	threadID := uuid.New()
	agentID := uuid.New()
	invocationID := uuid.New()
	now := time.Now()

	es := &ExecutionService{
		msgRepo:            msgRepo,
		invocationRepo:     invocationRepo,
		contentBlockRepo:   blockRepo,
		contentBlockBuffer: make([]model.InvocationContentBlock, 0, 20),
		lastFlush:          time.Now(),
		runningAgents: map[uuid.UUID]*RunningAgent{
			invocationID: {
				InvocationID: invocationID,
				ThreadID:     threadID,
				AgentConfig:  &model.AgentRoleConfig{ID: agentID, Name: "Planner"},
			},
		},
	}
	config := &model.AgentRoleConfig{ID: agentID, Name: "Planner", Role: model.AgentRoleAgent}
	base := &model.BaseAgent{Type: model.BaseAgentType("hermes"), DefaultModel: "qwen"}

	es.saveAgentMessage(ctx, threadID, config, base, "first output", []ContentBlockData{{ID: "b1", Type: "text", Content: "first output"}})
	msg := es.saveAgentMessageWithReturn(ctx, threadID, invocationID, config, base, "second output", nil)
	if msg == nil || msg.ID == uuid.Nil {
		t.Fatalf("saveAgentMessageWithReturn returned %#v", msg)
	}
	messages, err := msgRepo.FindByThreadID(ctx, threadID, 10)
	if err != nil {
		t.Fatalf("FindByThreadID messages returned error: %v", err)
	}
	if len(messages) != 2 || messages[0].Content != "first output" || messages[1].Content != "second output" {
		t.Fatalf("messages = %#v", messages)
	}

	completed := now
	inv := &model.AgentInvocation{
		ID:            invocationID,
		ThreadID:      threadID,
		AgentConfigID: agentID,
		Role:          model.AgentRoleAgent,
		AgentName:     "Planner",
		Status:        model.InvocationStatusCompleted,
		Input:         "input",
		Output:        "output",
		StartedAt:     &now,
		CompletedAt:   &completed,
		CreatedAt:     now,
		SessionID:     "session-1",
	}
	if err := invocationRepo.Create(ctx, inv); err != nil {
		t.Fatalf("create invocation: %v", err)
	}
	got, err := es.GetInvocationStatus(ctx, invocationID)
	if err != nil || got.ID != invocationID || got.SessionID != "session-1" {
		t.Fatalf("GetInvocationStatus = %#v err=%v", got, err)
	}
	byThread, err := es.GetInvocationsByThread(ctx, threadID)
	if err != nil || len(byThread) != 1 || byThread[0].ID != invocationID {
		t.Fatalf("GetInvocationsByThread = %#v err=%v", byThread, err)
	}
	recent := es.GetRecentlyCompletedInvocations(ctx, threadID, 30)
	if len(recent) != 1 || recent[0].InvocationID != invocationID.String() || recent[0].Status != string(model.InvocationStatusCompleted) {
		t.Fatalf("recent invocations = %#v", recent)
	}
	if got := (&ExecutionService{}).GetRecentlyCompletedInvocations(ctx, threadID, 30); got != nil {
		t.Fatalf("nil repo recent invocations = %#v", got)
	}

	if err := blockRepo.Upsert(ctx, &model.InvocationContentBlock{
		ID:           "persisted",
		InvocationID: invocationID.String(),
		Type:         "text",
		Content:      "from db",
		Status:       "done",
		Timestamp:    1,
	}); err != nil {
		t.Fatalf("upsert content block: %v", err)
	}
	recovery := es.GetRunningInvocationsWithContentBlocks(ctx, threadID)
	if len(recovery) != 1 {
		t.Fatalf("recovery = %#v", recovery)
	}
	recoveredBlocks, ok := recovery[0].ContentBlocks.([]ContentBlockData)
	if !ok || len(recoveredBlocks) != 1 || recoveredBlocks[0].Content != "from db" {
		t.Fatalf("recovered blocks = %#v", recovery[0].ContentBlocks)
	}

	for i := 0; i < 10; i++ {
		es.addToContentBlockBuffer(model.InvocationContentBlock{
			ID:           "buffered-" + string(rune('a'+i)),
			InvocationID: invocationID.String(),
			Type:         "text",
			Content:      "buffered",
			Timestamp:    int64(i + 2),
		}, invocationID)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		blocks, err := blockRepo.FindByInvocation(ctx, invocationID)
		if err == nil && len(blocks) >= 11 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	blocks, err := blockRepo.FindByInvocation(ctx, invocationID)
	t.Fatalf("buffered content blocks were not flushed, blocks=%#v err=%v", blocks, err)
}

func openExecutionPersistenceDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE messages (id TEXT PRIMARY KEY, thread_id TEXT NOT NULL, role TEXT NOT NULL, agent_id TEXT, content TEXT, content_blocks BLOB, message_type TEXT, metadata BLOB, created_at TEXT NOT NULL, sortable_id TEXT, reported_at TIMESTAMP NULL, mentions BLOB, origin TEXT, reply_to TEXT)`,
		`CREATE TABLE agent_invocations (id TEXT PRIMARY KEY, thread_id TEXT, agent_config_id TEXT, role TEXT, agent_name TEXT, status TEXT, input TEXT, full_prompt TEXT, output TEXT, started_at TIMESTAMP, completed_at TIMESTAMP, created_at TIMESTAMP, process_id TEXT, session_id TEXT, input_tokens INTEGER DEFAULT 0, output_tokens INTEGER DEFAULT 0, cache_read_tokens INTEGER DEFAULT 0, cache_creation_tokens INTEGER DEFAULT 0, cost_usd REAL DEFAULT 0, duration_ms INTEGER DEFAULT 0, duration_api_ms INTEGER DEFAULT 0, callback_token TEXT, triggered_by TEXT)`,
		`CREATE TABLE invocation_content_blocks (id TEXT PRIMARY KEY, invocation_id TEXT NOT NULL, type TEXT NOT NULL, content TEXT, tool_name TEXT, tool_id TEXT, input BLOB, output TEXT, is_error BOOLEAN, status TEXT, timestamp INTEGER NOT NULL, started_at INTEGER, completed_at INTEGER, created_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create execution persistence schema: %v", err)
		}
	}
	return db
}
