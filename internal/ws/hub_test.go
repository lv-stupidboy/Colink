package ws

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestHubBroadcastToThreadAndGlobal(t *testing.T) {
	hub := NewHub()
	threadClient := &Client{ThreadID: "thread-1", Send: make(chan []byte, 2)}
	otherClient := &Client{ThreadID: "thread-2", Send: make(chan []byte, 2)}
	hub.clients["thread-1"] = map[*Client]bool{threadClient: true}
	hub.clients["thread-2"] = map[*Client]bool{otherClient: true}

	hub.BroadcastGlobal(WSMessage{Type: "global", Payload: map[string]interface{}{"ok": true}})
	assertWSMessage(t, <-threadClient.Send, "global")
	assertWSMessage(t, <-otherClient.Send, "global")

	go func() {
		message := <-hub.broadcast
		for client := range hub.clients[message.ThreadID] {
			client.Send <- message.Message
		}
	}()
	hub.BroadcastToThread("thread-1", WSMessage{Type: "thread", ThreadID: "thread-1"})
	assertWSMessage(t, <-threadClient.Send, "thread")
	if count := hub.GetClientCount("thread-1"); count != 1 {
		t.Fatalf("GetClientCount = %d", count)
	}
}

func TestSessionBroadcasterAdapter(t *testing.T) {
	hub := NewHub()
	client := &Client{ThreadID: "thread-1", Send: make(chan []byte, 1)}
	hub.clients["thread-1"] = map[*Client]bool{client: true}
	adapter := NewSessionBroadcasterAdapter(hub)

	go func() {
		message := <-hub.broadcast
		for c := range hub.clients[message.ThreadID] {
			c.Send <- message.Message
		}
	}()
	adapter.BroadcastToThread("thread-1", "session.updated", map[string]interface{}{"status": "running"})
	assertWSMessage(t, <-client.Send, "session.updated")
}

func TestClientRecoveryAndCancelHandlers(t *testing.T) {
	threadID := uuid.New()
	invocationID := uuid.New()
	recoverer := &fakeInvocationRecoverer{
		running: []InvocationRecoveryData{{
			InvocationID: invocationID.String(),
			AgentID:      uuid.New().String(),
			AgentName:    "Agent",
			Status:       "running",
		}},
	}
	var cancelled uuid.UUID
	handler := NewHandler(NewHub(), nil, recoverer)
	handler.SetCancelAgentFunc(func(ctx context.Context, id uuid.UUID) error {
		cancelled = id
		return nil
	})
	client := &Client{ThreadID: threadID.String(), Send: make(chan []byte, 2), Handler: handler}

	client.handleRecoverInvocationState(threadID.String())
	assertWSMessage(t, <-client.Send, "invocation_recovery")
	client.handleRecoverInvocationState("not-a-uuid")

	client.handleCancelInvocation(threadID.String(), invocationID.String())
	if cancelled != invocationID {
		t.Fatalf("cancelled invocation = %s", cancelled)
	}
	client.handleCancelInvocation(uuid.New().String(), uuid.New().String())
	client.handleCancelInvocation(threadID.String(), "bad-id")
}

func assertWSMessage(t *testing.T, data []byte, wantType string) {
	t.Helper()
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal ws message: %v", err)
	}
	if msg.Type != wantType {
		t.Fatalf("message type = %q, want %q; raw=%s", msg.Type, wantType, data)
	}
}

type fakeInvocationRecoverer struct {
	running   []InvocationRecoveryData
	completed []InvocationRecoveryData
}

func (f *fakeInvocationRecoverer) GetRunningInvocationsWithContentBlocks(ctx context.Context, threadID uuid.UUID) []InvocationRecoveryData {
	return f.running
}

func (f *fakeInvocationRecoverer) GetRecentlyCompletedInvocations(ctx context.Context, threadID uuid.UUID, sinceMinutes int) []InvocationRecoveryData {
	return f.completed
}
