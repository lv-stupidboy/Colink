package agent

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestOrchestrator_SpawnDebugAgent_NilManager(t *testing.T) {
	o := &Orchestrator{}
	_, err := o.SpawnDebugAgent(context.Background(), &SpawnRequest{})
	if err == nil {
		t.Error("Expected error when debugThreadMgr is nil")
	}
}

func TestOrchestrator_SpawnDebugAgent_ThreadNotFound(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	o := &Orchestrator{debugThreadMgr: mgr}

	_, err := o.SpawnDebugAgent(context.Background(), &SpawnRequest{
		ThreadID: uuid.New(),
	})
	if err == nil {
		t.Error("Expected error when thread not found")
	}
}

func TestOrchestrator_ContinueDebugAgent_ThreadNotFound(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	o := &Orchestrator{debugThreadMgr: mgr}

	err := o.ContinueDebugAgent(context.Background(), uuid.New(), "test")
	if err == nil {
		t.Error("Expected error when thread not found")
	}
}

func TestOrchestrator_ContinueDebugAgent_ThreadRunning(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread("")
	mgr.SetStatus(thread.ID, DebugThreadStatusRunning)

	o := &Orchestrator{debugThreadMgr: mgr}

	err := o.ContinueDebugAgent(context.Background(), thread.ID, "test")
	if err == nil {
		t.Error("Expected error when thread is running")
	}
}

func TestOrchestrator_SetDebugThreadManager(t *testing.T) {
	o := &Orchestrator{}
	mgr := NewDebugThreadManager(nil)

	o.SetDebugThreadManager(mgr)

	if o.debugThreadMgr != mgr {
		t.Error("Expected debugThreadMgr to be set")
	}
}

func TestOrchestrator_ContinueDebugAgent_NoPreviousAgentContext(t *testing.T) {
	mgr := NewDebugThreadManager(nil)
	thread := mgr.CreateThread("")

	o := &Orchestrator{debugThreadMgr: mgr}

	err := o.ContinueDebugAgent(context.Background(), thread.ID, "test")
	if err == nil {
		t.Error("Expected error when no previous agent context found")
	}
}