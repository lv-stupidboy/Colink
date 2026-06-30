package a2a

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

func TestWorklistOrderingDelayA2AAndMutation(t *testing.T) {
	w := NewWorklist()
	ctx := context.Background()
	threadID := uuid.New()
	future := time.Now().Add(150 * time.Millisecond)

	if err := w.Enqueue(ctx, WorklistItem{ThreadID: threadID, TargetRole: model.AgentRole("low"), Priority: 1, Payload: "low"}); err != nil {
		t.Fatalf("enqueue low: %v", err)
	}
	if err := w.Enqueue(ctx, WorklistItem{ThreadID: threadID, TargetRole: model.AgentRole("high"), Priority: 10, Payload: "high"}); err != nil {
		t.Fatalf("enqueue high: %v", err)
	}
	if err := w.Enqueue(ctx, WorklistItem{ThreadID: threadID, TargetRole: model.AgentRole("later"), Priority: 100, Payload: "later", ProcessAfter: &future}); err != nil {
		t.Fatalf("enqueue delayed: %v", err)
	}
	if w.Size() != 3 {
		t.Fatalf("size = %d", w.Size())
	}
	if peek := w.Peek(); peek == nil || peek.Payload != "high" {
		t.Fatalf("peek = %#v", peek)
	}
	if !w.HasPendingAgent(threadID, "high") {
		t.Fatalf("expected pending high agent")
	}
	byThread := w.GetByThread(threadID)
	if len(byThread) != 3 {
		t.Fatalf("GetByThread = %#v", byThread)
	}
	if !w.Prioritize(byThread[0].ID, 50) {
		t.Fatalf("Prioritize should find item")
	}
	if !w.Remove(byThread[1].ID) {
		t.Fatalf("Remove should find item")
	}
	if w.Remove(uuid.New()) {
		t.Fatalf("Remove missing should be false")
	}

	item, err := w.Dequeue(ctx)
	if err != nil || item == nil {
		t.Fatalf("Dequeue returned %#v err=%v", item, err)
	}
	if item.ProcessAfter != nil && item.ProcessAfter.After(time.Now()) {
		t.Fatalf("dequeued delayed item too early: %#v", item)
	}

	if err := w.EnqueueA2A(WorklistItem{ThreadID: threadID, TargetRole: model.AgentRole("reviewer"), A2ADepth: 1}, "planner"); err != nil {
		t.Fatalf("EnqueueA2A: %v", err)
	}
	if err := w.EnqueueA2A(WorklistItem{ThreadID: threadID, TargetRole: model.AgentRole("too-deep"), A2ADepth: 2, MaxDepth: 2}, "planner"); err != nil {
		t.Fatalf("EnqueueA2A depth limit should be silent: %v", err)
	}
	if got := w.GetA2ADepth(threadID); got != 1 {
		t.Fatalf("A2A depth = %d", got)
	}
	if items := w.GetA2AItems(threadID); len(items) != 1 || items[0].A2AFrom != "planner" {
		t.Fatalf("A2A items = %#v", items)
	}
	if got := w.CountA2ABySource(threadID, "planner"); got != 1 {
		t.Fatalf("CountA2ABySource = %d", got)
	}

	w.Clear()
	deadlineCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if item, err := w.Dequeue(deadlineCtx); item != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Dequeue canceled item=%#v err=%v", item, err)
	}
}

func TestWorklistRegistryPushAndOwnership(t *testing.T) {
	registry := NewWorklistRegistry()
	threadID := uuid.New().String()
	parentID := uuid.New().String()

	entry := registry.Register(threadID, []string{"planner", "coder"}, 2, parentID)
	if entry.OriginalCount != 2 || !registry.Has(threadID) || registry.Get(threadID, parentID) != entry {
		t.Fatalf("registered entry = %#v", entry)
	}
	if key := registryKey(threadID, parentID); key != parentID {
		t.Fatalf("registry key = %s", key)
	}
	if key := registryKey(threadID, ""); key != threadID {
		t.Fatalf("thread registry key = %s", key)
	}

	// Only the currently executing cat can push when callerCatID is provided.
	if result := registry.Push(threadID, []string{"reviewer"}, "coder", parentID, "msg-0"); result.Reason != PushReasonCallerMismatch {
		t.Fatalf("caller mismatch result = %#v", result)
	}
	if result := registry.Push(threadID, []string{"coder"}, "planner", parentID, "msg-3"); result.Reason != PushReasonAllDuplicate {
		t.Fatalf("duplicate result = %#v", result)
	}
	result := registry.Push(threadID, []string{"reviewer", "tester"}, "planner", parentID, "msg-1")
	if len(result.Added) != 2 || result.Added[0] != "reviewer" {
		t.Fatalf("push result = %#v entry=%#v", result, entry)
	}
	if registry.GetA2AFrom(threadID, parentID, "reviewer") != "planner" || registry.GetA2ATriggerMessageID(threadID, parentID, "reviewer") != "msg-1" {
		t.Fatalf("A2A metadata from=%q trigger=%q", registry.GetA2AFrom(threadID, parentID, "reviewer"), registry.GetA2ATriggerMessageID(threadID, parentID, "reviewer"))
	}
	if result := registry.Push(threadID, []string{"ops"}, "planner", parentID, "msg-2"); result.Reason != PushReasonDepthLimit {
		t.Fatalf("depth result = %#v", result)
	}

	registry.UpdateExecutedIndex(threadID, parentID, 2)
	result = registry.Push(threadID, []string{"planner"}, "reviewer", parentID, "msg-4")
	if len(result.Added) != 0 && result.Reason != "" {
		t.Fatalf("unexpected push after depth consumed: %#v", result)
	}
	if registry.GetA2AFrom("missing", "", "x") != "" || registry.GetA2ATriggerMessageID("missing", "", "x") != "" {
		t.Fatalf("missing metadata should be empty")
	}

	other := registry.Register(threadID, []string{"new"}, 1, parentID)
	registry.Unregister(threadID, entry, parentID)
	if registry.Get(threadID, parentID) != other {
		t.Fatalf("stale unregister should not remove current entry")
	}
	registry.Unregister(threadID, other, parentID)
	if registry.Has(threadID) || registry.Get(threadID, parentID) != nil {
		t.Fatalf("entry should be unregistered")
	}
	if result := registry.Push("missing", []string{"x"}, "", "", ""); result.Reason != PushReasonNotFound {
		t.Fatalf("missing push = %#v", result)
	}
	if GetWorklistRegistry() == nil {
		t.Fatalf("global registry should exist")
	}
	if uuidToString(uuid.Nil) != "" || uuidToString(uuid.MustParse("11111111-1111-1111-1111-111111111111")) == "" {
		t.Fatalf("uuidToString behavior changed")
	}
}
