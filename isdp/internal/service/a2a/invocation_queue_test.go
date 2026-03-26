package a2a

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestInvocationQueue_Enqueue(t *testing.T) {
	queue := NewInvocationQueue()

	threadID := uuid.New()
	entry := &QueueEntry{
		ThreadID: threadID,
		UserID:   "user1",
		Content:  "test message",
		Source:   "user",
		Intent:   "execute",
	}

	result, err := queue.Enqueue(entry)
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if result.Outcome != "enqueued" {
		t.Errorf("Enqueue() outcome = %v, want 'enqueued'", result.Outcome)
	}
	if result.Entry == nil {
		t.Error("Enqueue() entry is nil")
	}
}

func TestInvocationQueue_Enqueue_Merge(t *testing.T) {
	queue := NewInvocationQueue()

	threadID := uuid.New()

	// Enqueue first entry
	entry1 := &QueueEntry{
		ThreadID:     threadID,
		UserID:       "user1",
		Content:      "first message",
		Source:       "user",
		Intent:       "execute",
		TargetAgents: []string{"developer"},
	}
	queue.Enqueue(entry1)

	// Enqueue similar entry - should merge
	entry2 := &QueueEntry{
		ThreadID:     threadID,
		UserID:       "user1",
		Content:      "second message",
		Source:       "user",
		Intent:       "execute",
		TargetAgents: []string{"developer"},
	}
	result, _ := queue.Enqueue(entry2)

	if result.Outcome != "merged" {
		t.Errorf("Enqueue() should merge similar entries, outcome = %v", result.Outcome)
	}
	if result.Entry.Content != "first message\nsecond message" {
		t.Errorf("Merged content = %v, want 'first message\\nsecond message'", result.Entry.Content)
	}
}

func TestInvocationQueue_Enqueue_Full(t *testing.T) {
	queue := NewInvocationQueue()

	threadID := uuid.New()

	// Fill the queue to max capacity
	for i := 0; i < MaxQueueDepth+1; i++ {
		// Use different target agents to prevent merging
		entry := &QueueEntry{
			ThreadID:     threadID,
			UserID:       "user1",
			Content:      "message",
			Source:       "user",
			Intent:       "execute",
			TargetAgents: []string{uuid.New().String()}, // Different each time
		}
		result, _ := queue.Enqueue(entry)

		if i < MaxQueueDepth {
			if result.Outcome != "enqueued" {
				t.Errorf("Entry %d should be enqueued, got %v", i, result.Outcome)
			}
		} else {
			if result.Outcome != "full" {
				t.Errorf("Entry %d should get 'full', got %v", i, result.Outcome)
			}
		}
	}
}

func TestInvocationQueue_Dequeue(t *testing.T) {
	queue := NewInvocationQueue()

	threadID := uuid.New()

	// Dequeue from empty queue
	got := queue.Dequeue(threadID, "user1")
	if got != nil {
		t.Error("Dequeue() on empty queue should return nil")
	}

	// Enqueue entry
	entry := &QueueEntry{
		ThreadID: threadID,
		UserID:   "user1",
		Content:  "test message",
		Source:   "user",
	}
	queue.Enqueue(entry)

	// Dequeue
	got = queue.Dequeue(threadID, "user1")
	if got == nil {
		t.Fatal("Dequeue() returned nil")
	}
	if got.Content != "test message" {
		t.Errorf("Dequeue() content = %v, want 'test message'", got.Content)
	}

	// Queue should be empty now
	got = queue.Dequeue(threadID, "user1")
	if got != nil {
		t.Error("Dequeue() on empty queue should return nil after dequeue")
	}
}

func TestInvocationQueue_HasQueuedAgent(t *testing.T) {
	queue := NewInvocationQueue()

	threadID := uuid.New()
	agentID := "developer"

	// No queued entries initially
	if queue.HasQueuedAgent(threadID.String(), agentID) {
		t.Error("HasQueuedAgent() should return false for empty queue")
	}

	// Enqueue entry targeting developer
	entry := &QueueEntry{
		ThreadID:     threadID,
		UserID:       "user1",
		Content:      "test",
		Source:       "user",
		TargetAgents: []string{agentID},
	}
	queue.Enqueue(entry)

	// Should find the agent
	if !queue.HasQueuedAgent(threadID.String(), agentID) {
		t.Error("HasQueuedAgent() should return true for queued agent")
	}

	// Different agent should not be found
	if queue.HasQueuedAgent(threadID.String(), "architect") {
		t.Error("HasQueuedAgent() should return false for non-queued agent")
	}
}

func TestInvocationQueue_CountAgentEntriesForThread(t *testing.T) {
	queue := NewInvocationQueue()

	threadID := uuid.New()
	otherThreadID := uuid.New()

	// Enqueue entries from different sources
	entry1 := &QueueEntry{
		ThreadID: threadID,
		UserID:   "user1",
		Content:  "user message",
		Source:   "user",
	}
	queue.Enqueue(entry1)

	entry2 := &QueueEntry{
		ThreadID: threadID,
		UserID:   "user1",
		Content:  "agent message",
		Source:   "agent",
		TargetAgents: []string{uuid.New().String()}, // Prevent merge
	}
	queue.Enqueue(entry2)

	entry3 := &QueueEntry{
		ThreadID: otherThreadID,
		UserID:   "user1",
		Content:  "other thread agent",
		Source:   "agent",
	}
	queue.Enqueue(entry3)

	// Count agent entries for threadID
	count := queue.CountAgentEntriesForThread(threadID.String())
	if count != 1 {
		t.Errorf("CountAgentEntriesForThread() = %d, want 1", count)
	}

	// Count agent entries for otherThreadID
	count = queue.CountAgentEntriesForThread(otherThreadID.String())
	if count != 1 {
		t.Errorf("CountAgentEntriesForThread() = %d, want 1", count)
	}
}

func TestInvocationQueue_Size(t *testing.T) {
	queue := NewInvocationQueue()

	threadID := uuid.New()

	// Empty queue
	if queue.Size(threadID, "user1") != 0 {
		t.Error("Size() should be 0 for empty queue")
	}

	// Add entries with different targets to prevent merging
	for i := 0; i < 3; i++ {
		entry := &QueueEntry{
			ThreadID:     threadID,
			UserID:       "user1",
			Content:      "message",
			TargetAgents: []string{uuid.New().String()},
		}
		queue.Enqueue(entry)
	}

	if queue.Size(threadID, "user1") != 3 {
		t.Errorf("Size() = %d, want 3", queue.Size(threadID, "user1"))
	}
}

func TestInvocationQueue_PeekOldestAcrossUsers(t *testing.T) {
	queue := NewInvocationQueue()

	threadID := uuid.New()

	// No entries initially
	got := queue.PeekOldestAcrossUsers(threadID.String())
	if got != nil {
		t.Error("PeekOldestAcrossUsers() should return nil for empty queue")
	}

	// Add entries from different users with explicit timestamps to ensure order
	now := time.Now()
	entry1 := &QueueEntry{
		ThreadID:     threadID,
		UserID:       "user1",
		Content:      "first",
		TargetAgents: []string{uuid.New().String()},
		CreatedAt:    now.Add(-1 * time.Second), // 1 second earlier
	}
	queue.Enqueue(entry1)

	entry2 := &QueueEntry{
		ThreadID:     threadID,
		UserID:       "user2",
		Content:      "second",
		TargetAgents: []string{uuid.New().String()},
		CreatedAt:    now, // later
	}
	queue.Enqueue(entry2)

	// Should return the oldest entry (by CreatedAt)
	got = queue.PeekOldestAcrossUsers(threadID.String())
	if got == nil {
		t.Fatal("PeekOldestAcrossUsers() returned nil")
	}
	if got.Content != "first" {
		t.Errorf("PeekOldestAcrossUsers() content = %v, want 'first'", got.Content)
	}
}