package agent

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestOrchestratorThinWrapperHelpers(t *testing.T) {
	orch := NewOrchestrator(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, nil)
	if orch.GetExecutionService() == nil {
		t.Fatalf("NewOrchestrator did not create execution service")
	}
	if orch.runningAgents == nil || orch.executionService.runningAgents == nil {
		t.Fatalf("running agent maps should be initialized")
	}

	orch.SetExecutionServiceAPIURL("http://127.0.0.1:18080")
	if orch.executionService.apiURL != "http://127.0.0.1:18080" {
		t.Fatalf("apiURL = %q", orch.executionService.apiURL)
	}
	sessionManager := NewSessionManager(nil, SessionManagerConfig{})
	orch.SetSessionManager(sessionManager)
	if orch.executionService.sessionManager != sessionManager {
		t.Fatalf("session manager was not forwarded")
	}
	orch.SetMCPBindingRepository(nil)
	if orch.executionService.mcpBindingRepo != nil {
		t.Fatalf("nil MCP binding repo should be forwarded")
	}

	config := &model.AgentRoleConfig{
		ID:        uuid.New(),
		Name:      "Reviewer",
		Role:      model.AgentRoleReviewer,
		MaxTokens: 0,
	}
	merged := orch.mergeConfig(config, &model.BaseAgent{MaxTokens: 4096})
	if merged == config || merged.MaxTokens != 4096 {
		t.Fatalf("mergeConfig result = %#v", merged)
	}
	if config.MaxTokens != 0 {
		t.Fatalf("mergeConfig should not mutate source config")
	}
	config.MaxTokens = 1024
	merged = orch.mergeConfig(config, &model.BaseAgent{MaxTokens: 4096})
	if merged.MaxTokens != 1024 {
		t.Fatalf("explicit config MaxTokens should win, got %d", merged.MaxTokens)
	}
	merged = orch.mergeConfig(config, nil)
	if merged.MaxTokens != 1024 {
		t.Fatalf("nil base agent merge = %#v", merged)
	}

	threadID := uuid.New()
	got := orch.formatMessages([]*model.Message{
		{ThreadID: threadID, Role: model.MessageRoleUser, Content: "hello"},
		{ThreadID: threadID, Role: model.MessageRoleAgent, AgentID: "reviewer", Content: "done"},
	})
	if !strings.Contains(got, "[用户] hello") || !strings.Contains(got, "[reviewer] done") {
		t.Fatalf("formatMessages = %q", got)
	}

	thread := &model.Thread{ID: threadID, CurrentPhase: model.PhaseReview, Status: model.ThreadStatusRunning}
	env := orch.getEnvironmentInfo(thread)
	if !strings.Contains(env, threadID.String()) || !strings.Contains(env, string(model.PhaseReview)) || !strings.Contains(env, string(model.ThreadStatusRunning)) {
		t.Fatalf("getEnvironmentInfo = %q", env)
	}
	if artifacts := orch.getArtifacts(thread); artifacts != "" {
		t.Fatalf("getArtifacts currently returns %q", artifacts)
	}
}

func TestStartupReconcilerMarksOrphanInvocationsInterrupted(t *testing.T) {
	ctx := context.Background()
	db := openAgentReconcilerTestDB(t)
	invocationRepo := repo.NewAgentInvocationRepository(db, repo.DBTypeSQLite)
	reconciler := NewStartupReconciler(invocationRepo, nil)

	withoutPID := newRunningInvocation("")
	invalidPID := newRunningInvocation("not-a-pid")
	completed := newRunningInvocation("")
	completed.Status = model.InvocationStatusCompleted
	for _, inv := range []*model.AgentInvocation{withoutPID, invalidPID, completed} {
		if err := invocationRepo.Create(ctx, inv); err != nil {
			t.Fatalf("Create invocation returned error: %v", err)
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE agent_invocations SET process_id = ? WHERE id = ?`, "not-a-pid", invalidPID.ID.String()); err != nil {
		t.Fatalf("set process id returned error: %v", err)
	}

	reconciler.Reconcile(ctx)

	gotWithoutPID, err := invocationRepo.FindByID(ctx, withoutPID.ID)
	if err != nil {
		t.Fatalf("FindByID withoutPID returned error: %v", err)
	}
	if gotWithoutPID.Status != model.InvocationStatusInterrupted || gotWithoutPID.CompletedAt == nil || !strings.Contains(gotWithoutPID.Output, "server restart") {
		t.Fatalf("withoutPID after reconcile = %#v", gotWithoutPID)
	}
	gotInvalidPID, err := invocationRepo.FindByID(ctx, invalidPID.ID)
	if err != nil {
		t.Fatalf("FindByID invalidPID returned error: %v", err)
	}
	if gotInvalidPID.Status != model.InvocationStatusInterrupted || gotInvalidPID.CompletedAt == nil || !strings.Contains(gotInvalidPID.Output, "terminated unexpectedly") {
		t.Fatalf("invalidPID after reconcile = %#v", gotInvalidPID)
	}
	gotCompleted, err := invocationRepo.FindByID(ctx, completed.ID)
	if err != nil {
		t.Fatalf("FindByID completed returned error: %v", err)
	}
	if gotCompleted.Status != model.InvocationStatusCompleted {
		t.Fatalf("completed invocation should not be reconciled: %#v", gotCompleted)
	}
}

func TestStartupReconcilerNoopsOnRepositoryErrorOrEmptySet(t *testing.T) {
	reconciler := NewStartupReconciler(repo.NewAgentInvocationRepository(openAgentReconcilerTestDB(t), repo.DBTypeSQLite), nil)
	reconciler.Reconcile(context.Background())

	db := openAgentReconcilerTestDB(t)
	db.Close()
	reconciler = NewStartupReconciler(repo.NewAgentInvocationRepository(db, repo.DBTypeSQLite), nil)
	reconciler.Reconcile(context.Background())
}

func TestStartupReconcilerProcessProbe(t *testing.T) {
	reconciler := &StartupReconciler{}
	if reconciler.isProcessAlive("not-a-pid") {
		t.Fatalf("invalid pid should not be alive")
	}
	if reconciler.isProcessAlive("-1") {
		t.Fatalf("negative pid should not be alive")
	}
}

func newRunningInvocation(processID string) *model.AgentInvocation {
	var processIDPtr *string
	if processID != "" {
		processIDPtr = &processID
	}
	now := time.Now().Add(-time.Minute).Truncate(time.Second)
	return &model.AgentInvocation{
		ID:            uuid.New(),
		ThreadID:      uuid.New(),
		AgentConfigID: uuid.New(),
		Role:          model.AgentRoleReviewer,
		AgentName:     "Review Agent",
		Status:        model.InvocationStatusRunning,
		Input:         "input",
		CreatedAt:     now,
		StartedAt:     &now,
		ProcessID:     processIDPtr,
	}
}

func openAgentReconcilerTestDB(t *testing.T) *sql.DB {
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
