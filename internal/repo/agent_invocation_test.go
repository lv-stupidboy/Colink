package repo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestAgentInvocationRepositoryLifecycleQueries(t *testing.T) {
	ctx := context.Background()
	db := openAgentInvocationRepoTestDB(t)
	repository := NewAgentInvocationRepository(db, DBTypeSQLite)

	threadID := uuid.New()
	configID := uuid.New()
	triggeredBy := uuid.New()
	startedAt := time.Now().Add(-2 * time.Minute).Truncate(time.Second)
	completedAt := time.Now().Add(-1 * time.Minute).Truncate(time.Second)
	invocation := &model.AgentInvocation{
		ID:            uuid.New(),
		ThreadID:      threadID,
		AgentConfigID: configID,
		Role:          model.AgentRole("reviewer"),
		AgentName:     "Review Agent",
		Status:        model.InvocationStatusRunning,
		Input:         "please review",
		FullPrompt:    "system + user",
		Output:        "thinking",
		StartedAt:     &startedAt,
		CompletedAt:   &completedAt,
		CreatedAt:     time.Now().Add(-3 * time.Minute),
		SessionID:     "session-1",
		CallbackToken: "callback-token",
		TriggeredBy:   triggeredBy,
	}
	if err := repository.Create(ctx, invocation); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := repository.FindByID(ctx, invocation.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if got.ThreadID != threadID || got.AgentConfigID != configID || got.AgentName != "Review Agent" || got.SessionID != "session-1" || got.TriggeredBy != triggeredBy {
		t.Fatalf("FindByID got unexpected invocation: %#v", got)
	}

	invocation.Status = model.InvocationStatusCompleted
	invocation.Output = "done"
	invocation.InputTokens = 11
	invocation.OutputTokens = 7
	invocation.CacheReadTokens = 3
	invocation.CacheCreationTokens = 2
	invocation.CostUsd = 0.42
	invocation.DurationMs = 1200
	invocation.DurationApiMs = 900
	invocation.FullPrompt = "updated prompt"
	invocation.CompletedAt = &completedAt
	if err := repository.Update(ctx, invocation); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	updated, err := repository.FindByID(ctx, invocation.ID)
	if err != nil {
		t.Fatalf("FindByID updated returned error: %v", err)
	}
	if updated.Status != model.InvocationStatusCompleted || updated.Output != "done" || updated.InputTokens != 11 || updated.DurationApiMs != 900 {
		t.Fatalf("updated invocation = %#v", updated)
	}

	byThread, err := repository.FindByThreadID(ctx, threadID)
	if err != nil {
		t.Fatalf("FindByThreadID returned error: %v", err)
	}
	if len(byThread) != 1 || byThread[0].ID != invocation.ID {
		t.Fatalf("FindByThreadID = %#v", byThread)
	}
	emptyThread, err := repository.FindByThreadID(ctx, uuid.New())
	if err != nil || len(emptyThread) != 0 {
		t.Fatalf("FindByThreadID empty = %#v err=%v", emptyThread, err)
	}

	byStatus, err := repository.FindByStatus(ctx, model.InvocationStatusCompleted)
	if err != nil {
		t.Fatalf("FindByStatus returned error: %v", err)
	}
	if len(byStatus) != 1 || byStatus[0].ID != invocation.ID {
		t.Fatalf("FindByStatus = %#v", byStatus)
	}

	recent, err := repository.FindRecentlyCompletedByThread(ctx, threadID, 10)
	if err != nil {
		t.Fatalf("FindRecentlyCompletedByThread returned error: %v", err)
	}
	if len(recent) != 1 || recent[0].ID != invocation.ID {
		t.Fatalf("FindRecentlyCompletedByThread = %#v", recent)
	}
	oldRecent, err := repository.FindRecentlyCompletedByThread(ctx, threadID, 0)
	if err != nil {
		t.Fatalf("FindRecentlyCompletedByThread old returned error: %v", err)
	}
	if len(oldRecent) != 0 {
		t.Fatalf("expected no recent invocations with zero-minute window, got %#v", oldRecent)
	}

	if err := repository.Delete(ctx, invocation.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := repository.FindByID(ctx, invocation.ID); err == nil {
		t.Fatalf("FindByID should fail after delete")
	}
}

func openAgentInvocationRepoTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE agent_invocations (
		id TEXT PRIMARY KEY,
		thread_id TEXT,
		agent_config_id TEXT,
		role TEXT,
		agent_name TEXT,
		status TEXT,
		input TEXT,
		full_prompt TEXT,
		output TEXT,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		created_at TIMESTAMP,
		process_id TEXT,
		session_id TEXT,
		input_tokens INTEGER DEFAULT 0,
		output_tokens INTEGER DEFAULT 0,
		cache_read_tokens INTEGER DEFAULT 0,
		cache_creation_tokens INTEGER DEFAULT 0,
		cost_usd REAL DEFAULT 0,
		duration_ms INTEGER DEFAULT 0,
		duration_api_ms INTEGER DEFAULT 0,
		callback_token TEXT,
		triggered_by TEXT
	)`)
	if err != nil {
		t.Fatalf("exec schema: %v", err)
	}
	return db
}
