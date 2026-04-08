// isdp/internal/service/agent/debug_thread_manager_test.go
package agent

import (
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

func TestNewDebugThreadManager(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	if mgr == nil {
		t.Error("Expected non-nil manager")
	}
	if mgr.threads == nil {
		t.Error("Expected initialized threads map")
	}
	if mgr.maxAge != 2*time.Hour {
		t.Errorf("Expected maxAge 2h, got %v", mgr.maxAge)
	}
}

func TestDebugThreadManager_CreateThread(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread("")

	if thread == nil {
		t.Error("Expected non-nil thread")
	}
	if thread.ID == uuid.Nil {
		t.Error("Expected non-nil thread ID")
	}
	if thread.Status != "idle" {
		t.Errorf("Expected status 'idle', got %s", thread.Status)
	}
	if len(thread.Messages) != 0 {
		t.Error("Expected empty messages slice")
	}
	if thread.ProjectPath != "" {
		t.Errorf("Expected empty project path, got %s", thread.ProjectPath)
	}

	// Verify thread is stored
	retrieved := mgr.GetThread(thread.ID)
	if retrieved == nil || retrieved.ID != thread.ID {
		t.Error("Thread not stored correctly")
	}
}

func TestDebugThreadManager_AddMessage(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread("")

	msg := &model.Message{
		ID:      uuid.New(),
		Role:    "user",
		Content: "test message",
	}
	mgr.AddMessage(thread.ID, msg)

	messages := mgr.GetMessages(thread.ID)
	if len(messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(messages))
	}
	if messages[0].Content != "test message" {
		t.Errorf("Expected 'test message', got %s", messages[0].Content)
	}
}

func TestDebugThreadManager_SetStatus(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread("")

	mgr.SetStatus(thread.ID, "running")
	if thread.Status != "running" {
		t.Errorf("Expected status 'running', got %s", thread.Status)
	}
}

func TestDebugThreadManager_DeleteThread(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread("")

	mgr.DeleteThread(thread.ID)
	if mgr.GetThread(thread.ID) != nil {
		t.Error("Expected thread to be deleted")
	}
}

func TestDebugThreadManager_ConcurrentAccess(t *testing.T) {
	mgr := NewDebugThreadManager(nil)

	// Concurrent thread creation
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			thread := mgr.CreateThread("")
			if thread == nil {
				t.Error("Failed to create thread concurrently")
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestDebugThreadManager_TryStartExecution(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread("")

	// Should succeed from idle state
	if !mgr.TryStartExecution(thread.ID) {
		t.Error("Expected TryStartExecution to succeed from idle state")
	}
	if thread.Status != DebugThreadStatusRunning {
		t.Errorf("Expected status 'running', got %s", thread.Status)
	}

	// Should fail from running state
	if mgr.TryStartExecution(thread.ID) {
		t.Error("Expected TryStartExecution to fail from running state")
	}

	// Set to completed and try again
	mgr.SetStatus(thread.ID, DebugThreadStatusCompleted)
	if !mgr.TryStartExecution(thread.ID) {
		t.Error("Expected TryStartExecution to succeed from completed state")
	}
	if thread.Status != DebugThreadStatusRunning {
		t.Errorf("Expected status 'running', got %s", thread.Status)
	}
}

func TestDebugThreadManager_CompareAndSwapStatus(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread("")

	// Should succeed when status matches
	if !mgr.CompareAndSwapStatus(thread.ID, DebugThreadStatusIdle, DebugThreadStatusRunning) {
		t.Error("Expected CompareAndSwapStatus to succeed")
	}
	if thread.Status != DebugThreadStatusRunning {
		t.Errorf("Expected status 'running', got %s", thread.Status)
	}

	// Should fail when status doesn't match
	if mgr.CompareAndSwapStatus(thread.ID, DebugThreadStatusIdle, DebugThreadStatusRunning) {
		t.Error("Expected CompareAndSwapStatus to fail when status doesn't match")
	}
}

func TestDebugThreadManager_GetProjectPath(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread("/path/to/project")

	path := mgr.GetProjectPath(thread.ID)
	if path != "/path/to/project" {
		t.Errorf("Expected '/path/to/project', got %s", path)
	}

	// Test non-existent thread
	path = mgr.GetProjectPath(uuid.New())
	if path != "" {
		t.Errorf("Expected empty path for non-existent thread, got %s", path)
	}
}