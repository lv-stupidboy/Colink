package acp

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/anthropic/isdp/internal/service/agent"
	"github.com/google/uuid"
)

type recordingWriteCloser struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	closed bool
}

func (w *recordingWriteCloser) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.Write(p)
}

func (w *recordingWriteCloser) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return nil
}

func (w *recordingWriteCloser) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

func testACPTransport(writer *recordingWriteCloser) *acpTransport {
	return newACPTransport(writer, io.NopCloser(strings.NewReader("")), nil)
}

func TestBaseACPAdapterSessionStatusStopAndToolResultErrors(t *testing.T) {
	adapter := NewBaseACPAdapter(AcpAdapterConfig{}, nil)
	if got := adapter.GetSessionStatus("missing"); got != agent.SessionStatusIdle {
		t.Fatalf("missing status = %s", got)
	}

	sessionID := "session-1"
	adapter.sessions[sessionID] = &acpSession{id: "acp-1", status: agent.SessionStatusRunning}
	if got := adapter.GetSessionStatus(sessionID); got != agent.SessionStatusRunning {
		t.Fatalf("running status = %s", got)
	}
	if err := adapter.StopSession(sessionID); err != nil {
		t.Fatalf("StopSession returned error: %v", err)
	}
	if got := adapter.GetSessionStatus(sessionID); got != agent.SessionStatusIdle {
		t.Fatalf("stopped status = %s", got)
	}
	if err := adapter.StopSession("missing"); err != nil {
		t.Fatalf("missing StopSession returned error: %v", err)
	}

	invocationID := uuid.New()
	if err := adapter.SendToolResult(invocationID, "tool", "answer"); err == nil || !strings.Contains(err.Error(), "session not found") {
		t.Fatalf("missing session SendToolResult err = %v", err)
	}
	adapter.sessions[invocationID.String()] = &acpSession{id: "acp-2"}
	if err := adapter.SendToolResult(invocationID, "tool", "answer"); err == nil || !strings.Contains(err.Error(), "transport not available") {
		t.Fatalf("missing transport SendToolResult err = %v", err)
	}
}

func TestBaseACPAdapterSendToolResultElicitationPath(t *testing.T) {
	writer := &recordingWriteCloser{}
	adapter := NewBaseACPAdapter(AcpAdapterConfig{}, nil)
	invocationID := uuid.New()
	adapter.sessions[invocationID.String()] = &acpSession{
		id:                   "acp-elicitation",
		transport:            testACPTransport(writer),
		pendingElicitationID: "rpc-1",
		pendingElicitationQuestions: []agent.QuestionItem{
			{Header: "Mode", Question: "Choose", Options: []agent.QuestionOption{{Label: "fast"}}},
		},
		pendingQuestion: &agent.Chunk{Type: agent.ChunkTypeQuestion},
	}

	if err := adapter.SendToolResult(invocationID, "tool", `{"question_0":"fast"}`); err != nil {
		t.Fatalf("SendToolResult elicitation returned error: %v", err)
	}
	written := writer.String()
	if !strings.Contains(written, `"id":"rpc-1"`) || !strings.Contains(written, `"action":"accept"`) || !strings.Contains(written, `"question_0":"fast"`) {
		t.Fatalf("unexpected elicitation response: %s", written)
	}
	session := adapter.sessions[invocationID.String()]
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.pendingElicitationID != nil || session.pendingQuestion != nil || session.pendingElicitationQuestions != nil {
		t.Fatalf("pending elicitation state not cleared: %#v", session)
	}
}

func TestBaseACPAdapterHandleNotificationBranches(t *testing.T) {
	adapter := NewBaseACPAdapter(AcpAdapterConfig{}, nil)
	session := &acpSession{id: "acp-notify", isdpID: uuid.New().String(), toolCallNames: map[string]string{}}
	var chunks []agent.Chunk
	onChunk := func(chunk agent.Chunk) {
		chunks = append(chunks, chunk)
	}

	updateParams := json.RawMessage(`{"sessionId":"acp-notify","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"hello"}}}`)
	adapter.handleNotification(session, "session/update", updateParams, onChunk)
	if len(chunks) != 1 || chunks[0].Type != agent.ChunkTypeText || chunks[0].Content != "hello" {
		t.Fatalf("session/update chunks = %#v", chunks)
	}
	if session.output.String() != "hello" || session.notificationCount != 1 || session.lastUpdateHash == "" {
		t.Fatalf("session state after update output=%q count=%d hash=%q", session.output.String(), session.notificationCount, session.lastUpdateHash)
	}
	adapter.handleNotification(session, "session/update", updateParams, onChunk)
	if session.duplicateUpdateCount == 0 {
		t.Fatalf("expected duplicate update count")
	}

	session.replayPhase = true
	before := len(chunks)
	adapter.handleNotification(session, "session/update", updateParams, onChunk)
	if len(chunks) != before {
		t.Fatalf("replay update should not emit chunks")
	}
	session.replayPhase = false

	adapter.handleNotification(session, "session/request_user_input", json.RawMessage(`{"sessionId":"acp-notify","toolCallId":"ask-1","toolName":"AskUserQuestion","input":{"question":"Ready?"}}`), onChunk)
	if len(chunks) <= before || chunks[len(chunks)-1].Type != agent.ChunkTypeQuestion || session.pendingQuestion == nil {
		t.Fatalf("user input chunks = %#v pending=%#v", chunks, session.pendingQuestion)
	}
	if adapter.sessions[session.isdpID] != session {
		t.Fatalf("question session was not saved by invocation id")
	}

	toolUpdate := json.RawMessage(`{"sessionId":"acp-notify","update":{"sessionUpdate":"tool_call_update","toolCallId":"tool-1","title":"Read","status":"completed","content":[{"type":"text","text":"done"}]}}`)
	adapter.handleNotification(session, "session/tool_call_update", toolUpdate, onChunk)
	if chunks[len(chunks)-1].Type != agent.ChunkTypeToolResult {
		t.Fatalf("tool update chunks = %#v", chunks)
	}

	adapter.handleNotification(session, "rateLimitWarning", json.RawMessage(`{"message":"rate limited"}`), onChunk)
	if chunks[len(chunks)-1].Type != agent.ChunkTypeError || !strings.Contains(chunks[len(chunks)-1].Content, "rate limited") {
		t.Fatalf("error-like notification chunks = %#v", chunks)
	}

	adapter.handleNotification(session, "session/update", json.RawMessage(`{bad`), onChunk)
	adapter.handleNotification(session, "session/request_user_input", json.RawMessage(`{bad`), onChunk)
	adapter.handleNotification(session, "session/tool_call_update", json.RawMessage(`{bad`), onChunk)
}

func TestBaseACPAdapterHandleServerRequestElicitation(t *testing.T) {
	adapter := NewBaseACPAdapter(AcpAdapterConfig{}, nil)
	writer := &recordingWriteCloser{}
	session := &acpSession{id: "acp-server", isdpID: uuid.New().String(), transport: testACPTransport(writer)}
	var chunks []agent.Chunk
	params := json.RawMessage(`{
		"mode":"form",
		"message":"Choose mode",
		"toolCallId":"tool-elicit",
		"requestedSchema":{"properties":{"question_0":{"type":"string","title":"Mode","oneOf":[{"const":"fast","title":"fast"}]}}}
	}`)
	adapter.handleServerRequest(session, "rpc-2", "elicitation/create", params, func(chunk agent.Chunk) {
		chunks = append(chunks, chunk)
	})
	if len(chunks) != 1 || chunks[0].Type != agent.ChunkTypeQuestion || chunks[0].ToolID != "tool-elicit" {
		t.Fatalf("elicitation chunks = %#v", chunks)
	}
	if session.pendingElicitationID != "rpc-2" || len(session.pendingElicitationQuestions) != 1 || session.pendingQuestion == nil {
		t.Fatalf("pending elicitation state = %#v", session)
	}
	if adapter.sessions[session.isdpID] != session {
		t.Fatalf("elicitation session not registered")
	}

	adapter.handleServerRequest(session, "rpc-3", "elicitation/create", json.RawMessage(`{"mode":"freeform"}`), func(chunk agent.Chunk) {
		t.Fatalf("unsupported mode should not emit chunk: %#v", chunk)
	})
	if !strings.Contains(writer.String(), `"action":"decline"`) {
		t.Fatalf("unsupported mode should decline, writes=%s", writer.String())
	}

	writer2 := &recordingWriteCloser{}
	session2 := &acpSession{id: "acp-cancel", transport: testACPTransport(writer2)}
	adapter.handleServerRequest(session2, "rpc-4", "elicitation/create", params, nil)
	if !strings.Contains(writer2.String(), `"action":"cancel"`) {
		t.Fatalf("nil onChunk should cancel, writes=%s", writer2.String())
	}
	adapter.handleServerRequest(&acpSession{id: "no-transport"}, "rpc-5", "session/request_permission", json.RawMessage(`{}`), nil)
	adapter.handleServerRequest(session, "rpc-6", "unknown/method", json.RawMessage(`{}`), nil)
}
