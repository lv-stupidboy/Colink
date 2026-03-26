package a2a

import (
	"testing"

	"github.com/google/uuid"
)

func TestInvocationRegistry_Register(t *testing.T) {
	registry := NewInvocationRegistry()

	threadID := uuid.New()
	record := &InvocationRecord{
		ID:       uuid.New(),
		ThreadID: threadID,
		CatID:    "developer",
		UserID:   "user1",
	}

	token, err := registry.Register(record)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if token == "" {
		t.Error("Register() returned empty token")
	}

	// Verify the record was stored
	got := registry.Verify(record.ID, token)
	if got == nil {
		t.Error("Verify() returned nil for valid token")
	}
	if got.ID != record.ID {
		t.Errorf("Verify() returned wrong record, got ID = %v, want %v", got.ID, record.ID)
	}
}

func TestInvocationRegistry_Verify_Invalid(t *testing.T) {
	registry := NewInvocationRegistry()

	// Test with non-existent invocation
	got := registry.Verify(uuid.New(), "invalid-token")
	if got != nil {
		t.Error("Verify() should return nil for invalid invocation")
	}
}

func TestInvocationRegistry_IsLatest(t *testing.T) {
	registry := NewInvocationRegistry()

	threadID := uuid.New()

	// Register first invocation
	record1 := &InvocationRecord{
		ID:       uuid.New(),
		ThreadID: threadID,
		CatID:    "developer",
		UserID:   "user1",
	}
	registry.Register(record1)

	// First invocation should be latest
	if !registry.IsLatest(record1.ID) {
		t.Error("First invocation should be latest")
	}

	// Register second invocation for same thread
	record2 := &InvocationRecord{
		ID:       uuid.New(),
		ThreadID: threadID,
		CatID:    "architect",
		UserID:   "user1",
	}
	registry.Register(record2)

	// Now second should be latest, first should be stale
	if registry.IsLatest(record1.ID) {
		t.Error("First invocation should be stale after second registration")
	}
	if !registry.IsLatest(record2.ID) {
		t.Error("Second invocation should be latest")
	}
}

func TestInvocationRegistry_IsLatestForSlot(t *testing.T) {
	registry := NewInvocationRegistry()

	threadID := uuid.New()

	// Register first invocation for developer slot
	record1 := &InvocationRecord{
		ID:       uuid.New(),
		ThreadID: threadID,
		CatID:    "developer",
		UserID:   "user1",
	}
	registry.Register(record1)

	// Register invocation for different slot (architect)
	record2 := &InvocationRecord{
		ID:       uuid.New(),
		ThreadID: threadID,
		CatID:    "architect",
		UserID:   "user1",
	}
	registry.Register(record2)

	// Both should be latest for their respective slots
	if !registry.IsLatestForSlot(record1.ID) {
		t.Error("First invocation should be latest for developer slot")
	}
	if !registry.IsLatestForSlot(record2.ID) {
		t.Error("Second invocation should be latest for architect slot")
	}

	// Register new invocation for developer slot
	record3 := &InvocationRecord{
		ID:       uuid.New(),
		ThreadID: threadID,
		CatID:    "developer",
		UserID:   "user1",
	}
	registry.Register(record3)

	// Now record1 should be stale for developer slot
	if registry.IsLatestForSlot(record1.ID) {
		t.Error("First developer invocation should be stale for slot")
	}
	if !registry.IsLatestForSlot(record3.ID) {
		t.Error("Third invocation should be latest for developer slot")
	}
}

func TestInvocationRegistry_Complete(t *testing.T) {
	registry := NewInvocationRegistry()

	record := &InvocationRecord{
		ID:       uuid.New(),
		ThreadID: uuid.New(),
		CatID:    "developer",
		UserID:   "user1",
	}
	registry.Register(record)

	// Complete the invocation
	registry.Complete(record.ID)

	// After completion, the record should be marked as completed
	completed := registry.Get(record.ID)
	if completed == nil {
		t.Error("Get() returned nil for completed invocation")
	}
	if completed.Status != "completed" {
		t.Errorf("Status = %v, want 'completed'", completed.Status)
	}
}

func TestInvocationRegistry_GetActiveByThread(t *testing.T) {
	registry := NewInvocationRegistry()

	threadID := uuid.New()
	otherThreadID := uuid.New()

	// Register multiple invocations
	record1 := &InvocationRecord{
		ID:       uuid.New(),
		ThreadID: threadID,
		CatID:    "developer",
		UserID:   "user1",
	}
	registry.Register(record1)

	record2 := &InvocationRecord{
		ID:       uuid.New(),
		ThreadID: threadID,
		CatID:    "architect",
		UserID:   "user1",
	}
	registry.Register(record2)

	record3 := &InvocationRecord{
		ID:       uuid.New(),
		ThreadID: otherThreadID,
		CatID:    "tester",
		UserID:   "user1",
	}
	registry.Register(record3)

	// Get active invocations for threadID
	active := registry.GetActiveByThread(threadID)
	if len(active) != 2 {
		t.Errorf("GetActiveByThread() returned %d records, want 2", len(active))
	}

	// Complete one and verify it's no longer in active list
	registry.Complete(record1.ID)
	active = registry.GetActiveByThread(threadID)
	if len(active) != 1 {
		t.Errorf("GetActiveByThread() after Complete returned %d records, want 1", len(active))
	}
}

func TestInvocationRegistry_TokenExpiry(t *testing.T) {
	registry := NewInvocationRegistry()

	record := &InvocationRecord{
		ID:       uuid.New(),
		ThreadID: uuid.New(),
		CatID:    "developer",
		UserID:   "user1",
	}
	token, _ := registry.Register(record)

	// Token should be valid initially
	got := registry.Verify(record.ID, token)
	if got == nil {
		t.Error("Verify() returned nil for valid token")
	}

	// Wrong token should fail
	got = registry.Verify(record.ID, "wrong-token")
	if got != nil {
		t.Error("Verify() should return nil for wrong token")
	}

	// Wrong invocation ID should fail
	got = registry.Verify(uuid.New(), token)
	if got != nil {
		t.Error("Verify() should return nil for wrong invocation ID")
	}
}