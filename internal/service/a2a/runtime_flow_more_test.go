package a2a

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMCPAuthServiceTokenLifecycleAndCallbackHandler(t *testing.T) {
	ctx := context.Background()
	auth := NewMCPAuthService(time.Hour)
	threadID := uuid.New()
	invocationID := uuid.New()

	token, err := auth.GenerateToken(ctx, threadID, invocationID)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("GenerateToken() returned empty token")
	}
	if err := auth.ValidateToken(ctx, token, threadID, invocationID); err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if err := auth.ValidateToken(ctx, token, uuid.New(), invocationID); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("ValidateToken(wrong thread) error = %v, want ErrInvalidToken", err)
	}
	info, err := auth.GetTokenInfo(token)
	if err != nil || info.ThreadID != threadID || info.InvocationID != invocationID {
		t.Fatalf("GetTokenInfo() info=%+v error=%v", info, err)
	}

	handler := NewMCPCallbackHandler(auth)
	called := false
	handler.RegisterHandler("append", func(ctx context.Context, req *MCPCallbackRequest) error {
		called = true
		if req.Action != "append" {
			t.Fatalf("action = %q, want append", req.Action)
		}
		return nil
	})
	err = handler.HandleCallback(ctx, &MCPCallbackRequest{
		TokenID:      token,
		ThreadID:     threadID.String(),
		InvocationID: invocationID.String(),
		Action:       "append",
	})
	if err != nil {
		t.Fatalf("HandleCallback() error = %v", err)
	}
	if !called {
		t.Fatal("registered callback was not called")
	}
	if err := auth.ValidateToken(ctx, token, threadID, invocationID); !errors.Is(err, ErrTokenUsed) {
		t.Fatalf("ValidateToken(used) error = %v, want ErrTokenUsed", err)
	}

	expiring := NewMCPAuthService(time.Nanosecond)
	expiredToken, err := expiring.GenerateToken(ctx, threadID, invocationID)
	if err != nil {
		t.Fatalf("GenerateToken(expiring) error = %v", err)
	}
	time.Sleep(time.Millisecond)
	if err := expiring.ValidateToken(ctx, expiredToken, threadID, invocationID); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("ValidateToken(expired) error = %v, want ErrTokenExpired", err)
	}
	if err := auth.RevokeToken(ctx, "missing"); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("RevokeToken(missing) error = %v, want ErrTokenNotFound", err)
	}
	if err := handler.HandleCallback(ctx, &MCPCallbackRequest{ThreadID: "bad"}); err == nil {
		t.Fatal("HandleCallback(invalid uuid) error = nil, want error")
	}
}

func TestQueueProcessorAutoExecuteAndPauseState(t *testing.T) {
	queue := NewInvocationQueue()
	threadID := uuid.New()
	messageID := uuid.New()
	spawned := make(chan string, 1)
	updated := make(chan string, 1)
	processor := NewQueueProcessor(QueueProcessorDeps{
		Queue: queue,
		SpawnAgent: func(ctx context.Context, gotThreadID uuid.UUID, catID string, content string) error {
			if gotThreadID != threadID {
				t.Fatalf("threadID = %s, want %s", gotThreadID, threadID)
			}
			spawned <- catID + ":" + content
			return nil
		},
		MessageUpdater: func(ctx context.Context, gotMessageID string, deliveredAt int64) error {
			updated <- gotMessageID
			return nil
		},
	})

	entry := &QueueEntry{
		ThreadID:     threadID,
		UserID:       "user-1",
		Content:      "please review",
		MessageID:    &messageID,
		Source:       "agent",
		TargetAgents: []string{"reviewer"},
		Intent:       "execute",
		AutoExecute:  true,
	}
	if result, err := queue.Enqueue(entry); err != nil || result.Outcome != "enqueued" {
		t.Fatalf("Enqueue() result=%+v error=%v", result, err)
	}

	if !processor.HasQueuedForThread(threadID) || !processor.HasQueuedAgentForCat(threadID, "reviewer") {
		t.Fatal("processor should report queued work before auto execution")
	}
	if err := processor.TryAutoExecute(context.Background(), threadID); err != nil {
		t.Fatalf("TryAutoExecute() error = %v", err)
	}
	select {
	case got := <-spawned:
		if got != "reviewer:please review" {
			t.Fatalf("spawned = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("SpawnAgent was not called")
	}
	select {
	case got := <-updated:
		if got != messageID.String() {
			t.Fatalf("MessageUpdater messageID = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("MessageUpdater was not called")
	}
	waitForCondition(t, func() bool { return !processor.HasQueuedForThread(threadID) })

	_, _ = queue.Enqueue(&QueueEntry{
		ThreadID:     threadID,
		UserID:       "user-2",
		Content:      "again",
		Source:       "agent",
		TargetAgents: []string{"reviewer"},
		Intent:       "execute",
	})
	if err := processor.OnInvocationComplete(context.Background(), threadID, "reviewer", "failed"); err != nil {
		t.Fatalf("OnInvocationComplete(failed) error = %v", err)
	}
	if !processor.IsPaused(threadID, "reviewer") || processor.GetPauseReason(threadID, "reviewer") != "failed" {
		t.Fatalf("processor pause state not set")
	}
	processor.ClearPause(threadID, "reviewer")
	if processor.IsPaused(threadID, "reviewer") {
		t.Fatal("processor should not be paused after ClearPause")
	}
}

func TestEnqueueA2ATargetsAndEventBus(t *testing.T) {
	ctx := context.Background()
	queue := NewInvocationQueue()
	threadID := uuid.New()
	target := uuid.New().String()

	if _, err := EnqueueA2ATargets(ctx, nil, &A2ATriggerOptions{}); err == nil {
		t.Fatal("EnqueueA2ATargets(nil deps) error = nil, want error")
	}
	result, err := EnqueueA2ATargets(ctx, &A2ATriggerDeps{Queue: queue}, &A2ATriggerOptions{
		TargetCats: []string{"bad-id", target},
		Content:    "handoff",
		UserID:     "user-1",
		ThreadID:   threadID,
		CallerCatID: "caller",
	})
	if err != nil {
		t.Fatalf("EnqueueA2ATargets() error = %v", err)
	}
	if result.Fallback || len(result.Enqueued) != 1 || result.Enqueued[0] != target {
		t.Fatalf("EnqueueA2ATargets() result = %+v", result)
	}

	duplicate, err := EnqueueA2ATargets(ctx, &A2ATriggerDeps{Queue: queue}, &A2ATriggerOptions{
		TargetCats: []string{target},
		Content:    "duplicate",
		UserID:     "user-1",
		ThreadID:   threadID,
	})
	if err != nil {
		t.Fatalf("EnqueueA2ATargets(duplicate) error = %v", err)
	}
	if len(duplicate.Enqueued) != 0 {
		t.Fatalf("duplicate enqueue should be skipped, got %+v", duplicate)
	}

	bus := NewA2AEventBus()
	ch := bus.Subscribe()
	event := A2AHandoffEvent{FromCat: "planner", ToCat: "coder", ThreadID: threadID, Depth: 2}
	bus.Publish(event)
	select {
	case got := <-ch:
		if got != event {
			t.Fatalf("published event = %+v, want %+v", got, event)
		}
	case <-time.After(time.Second):
		t.Fatal("event was not published")
	}
	bus.Unsubscribe(ch)
	if GetA2AEventBus() == nil {
		t.Fatal("global event bus is nil")
	}
}

func waitForCondition(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not satisfied before timeout")
}
