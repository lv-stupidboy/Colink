package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

func TestExecutionServiceConstructionAdapterAndCancelFlow(t *testing.T) {
	ctx := context.Background()
	db := openExecutionPersistenceDB(t)
	invocationRepo := repo.NewAgentInvocationRepository(db, repo.DBTypeSQLite)
	defaultAdapter := &mockAgentAdapter{}
	es := NewExecutionService(
		invocationRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		defaultAdapter,
		nil,
		nil,
		nil,
		false,
		nil,
	)
	es.SetAPIURL("http://127.0.0.1:18080")
	es.SetMemoryManager(nil)
	es.SetSessionManager(nil)
	es.SetMCPBindingRepository(nil)
	if es.apiURL != "http://127.0.0.1:18080" || es.tokenBudgetManager == nil || es.runningAgents == nil || es.cliSessions == nil {
		t.Fatalf("NewExecutionService did not initialize expected fields")
	}
	if got, err := es.getAdapter(ctx, &model.AgentRoleConfig{}, nil); err != nil || got != defaultAdapter {
		t.Fatalf("getAdapter default = %#v err=%v", got, err)
	}
	if _, err := es.getAdapter(ctx, &model.AgentRoleConfig{}, &model.BaseAgent{Type: model.BaseAgentType("unsupported")}); err == nil || !strings.Contains(err.Error(), "不支持") {
		t.Fatalf("getAdapter unsupported error = %v", err)
	}

	completedID := uuid.New()
	now := time.Now()
	if err := invocationRepo.Create(ctx, &model.AgentInvocation{
		ID:            completedID,
		ThreadID:      uuid.New(),
		AgentConfigID: uuid.New(),
		Role:          model.AgentRoleAgent,
		AgentName:     "Done",
		Status:        model.InvocationStatusCompleted,
		Input:         "done",
		StartedAt:     &now,
		CompletedAt:   &now,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create completed invocation: %v", err)
	}
	if err := es.CancelAgent(ctx, completedID); err != nil {
		t.Fatalf("CancelAgent completed returned error: %v", err)
	}
	completed, err := invocationRepo.FindByID(ctx, completedID)
	if err != nil || completed.Status != model.InvocationStatusCompleted {
		t.Fatalf("completed invocation changed: %#v err=%v", completed, err)
	}

	threadID := uuid.New()
	configID := uuid.New()
	runningID := uuid.New()
	if err := invocationRepo.Create(ctx, &model.AgentInvocation{
		ID:            runningID,
		ThreadID:      threadID,
		AgentConfigID: configID,
		Role:          model.AgentRoleDeveloper,
		AgentName:     "Coder",
		Status:        model.InvocationStatusRunning,
		Input:         "work",
		StartedAt:     &now,
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create running invocation: %v", err)
	}
	es.csMu.Lock()
	es.cliSessions[threadID.String()+":"+configID.String()] = "cached-session"
	es.csMu.Unlock()
	if err := es.CancelAgent(ctx, runningID); err != nil {
		t.Fatalf("CancelAgent running returned error: %v", err)
	}
	cancelled, err := invocationRepo.FindByID(ctx, runningID)
	if err != nil {
		t.Fatalf("find cancelled invocation: %v", err)
	}
	if cancelled.Status != model.InvocationStatusCancelled || cancelled.SessionID != "cached-session" || cancelled.CompletedAt == nil {
		t.Fatalf("cancelled invocation = %#v", cancelled)
	}
	if err := es.CancelAgent(ctx, uuid.New()); err == nil || !strings.Contains(err.Error(), "failed to find invocation") {
		t.Fatalf("CancelAgent missing error = %v", err)
	}
}

func TestExecutionServiceResumeStrategyDecisions(t *testing.T) {
	ctx := context.Background()
	db := openExecutionPersistenceDB(t)
	invocationRepo := repo.NewAgentInvocationRepository(db, repo.DBTypeSQLite)
	es := &ExecutionService{invocationRepo: invocationRepo}
	threadID := uuid.New()
	targetID := uuid.New()
	otherID := uuid.New()
	now := time.Now()
	completedAt := now.Add(-time.Minute)

	if got, session := es.shouldUseResumeStrategy(ctx, threadID, targetID, nil); got != "" || session != "" {
		t.Fatalf("shouldUseResumeStrategy without mention = %q %q", got, session)
	}
	if got, session := es.shouldUseResumeStrategy(ctx, threadID, targetID, []string{otherID.String()}); got != "" || session != "" {
		t.Fatalf("shouldUseResumeStrategy unmentioned = %q %q", got, session)
	}

	if err := invocationRepo.Create(ctx, &model.AgentInvocation{
		ID:            uuid.New(),
		ThreadID:      threadID,
		AgentConfigID: targetID,
		Role:          model.AgentRoleDeveloper,
		AgentName:     "Coder",
		Status:        model.InvocationStatusCompleted,
		Input:         "done",
		StartedAt:     &completedAt,
		CompletedAt:   &completedAt,
		CreatedAt:     completedAt,
		SessionID:     "session-123",
	}); err != nil {
		t.Fatalf("create completed invocation: %v", err)
	}
	if got, session := es.shouldUseResumeStrategy(ctx, threadID, targetID, []string{targetID.String()}); got != SessionStrategyResume || session != "session-123" {
		t.Fatalf("shouldUseResumeStrategy = %q %q", got, session)
	}
	if got, session := es.shouldAutoResume(ctx, threadID, targetID); got != SessionStrategyResume || session != "session-123" {
		t.Fatalf("shouldAutoResume = %q %q", got, session)
	}
	if got, session := es.shouldAutoResume(ctx, threadID, otherID); got != "" || session != "" {
		t.Fatalf("shouldAutoResume other = %q %q", got, session)
	}

	if isResumeFallbackError(nil) || isResumeFallbackError(context.Canceled) || !isResumeFallbackError(assertionError("session not found")) || !isResumeFallbackError(assertionError("broken pipe")) {
		t.Fatalf("isResumeFallbackError behavior changed")
	}
	if got := timePtr(now); got == nil || !got.Equal(now) {
		t.Fatalf("timePtr = %#v", got)
	}
}

type assertionError string

func (e assertionError) Error() string {
	return string(e)
}
