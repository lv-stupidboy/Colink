package acp

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestACPTransportRequestNotificationAndServerRequest(t *testing.T) {
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()
	defer clientToServerR.Close()
	defer serverToClientW.Close()

	notifications := make(chan string, 1)
	serverRequests := make(chan string, 1)
	transport := newACPTransport(clientToServerW, serverToClientR, func(method string, params json.RawMessage) {
		notifications <- method + ":" + string(params)
	})
	transport.SetServerRequestHandler(func(id interface{}, method string, params json.RawMessage) {
		serverRequests <- method + ":" + string(params)
		_ = transport.SendResponse(id, map[string]string{"accepted": "yes"})
	})
	if err := transport.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		scanner := bufio.NewScanner(clientToServerR)
		for scanner.Scan() {
			var msg struct {
				ID     uint64          `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				continue
			}
			switch msg.Method {
			case "initialize":
				writeRPCLine(t, serverToClientW, map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": map[string]any{"protocolVersion": 2025}})
				writeRPCLine(t, serverToClientW, map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"ok": true}})
				writeRPCLine(t, serverToClientW, map[string]any{"jsonrpc": "2.0", "id": "srv-1", "method": "elicitation/create", "params": map[string]any{"message": "pick"}})
			case "fail":
				writeRPCLine(t, serverToClientW, map[string]any{"jsonrpc": "2.0", "id": msg.ID, "error": map[string]any{"code": -32602, "message": "bad params"}})
			}
		}
	}()

	result, err := transport.SendRequest("initialize", map[string]string{"client": "test"})
	if err != nil {
		t.Fatalf("SendRequest initialize: %v", err)
	}
	if !strings.Contains(string(result), "protocolVersion") {
		t.Fatalf("initialize result = %s", result)
	}
	select {
	case got := <-notifications:
		if !strings.Contains(got, "session/update") {
			t.Fatalf("notification = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for notification")
	}
	select {
	case got := <-serverRequests:
		if !strings.Contains(got, "elicitation/create") {
			t.Fatalf("server request = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for server request")
	}

	if _, err := transport.SendRequest("fail", nil); err == nil || !strings.Contains(err.Error(), "bad params") {
		t.Fatalf("SendRequest fail err = %v", err)
	}
	if err := transport.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatalf("server reader did not stop")
	}
}

func TestACPTransportWritesNotificationResponseAndHandlesClose(t *testing.T) {
	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()
	defer clientToServerR.Close()
	defer serverToClientW.Close()

	transport := newACPTransport(clientToServerW, serverToClientR, nil)
	transport.SetNotificationHandler(func(method string, params json.RawMessage) {})
	transport.SetServerRequestHandler(func(id interface{}, method string, params json.RawMessage) {})
	if err := transport.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	lines := make(chan string, 2)
	go func() {
		scanner := bufio.NewScanner(clientToServerR)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()

	if err := transport.SendNotification("ready", map[string]bool{"ok": true}); err != nil {
		t.Fatalf("SendNotification: %v", err)
	}
	if err := transport.SendResponse("request-1", map[string]string{"answer": "yes"}); err != nil {
		t.Fatalf("SendResponse: %v", err)
	}

	assertJSONRPCLine(t, <-lines, `"method":"ready"`)
	assertJSONRPCLine(t, <-lines, `"id":"request-1"`)

	if err := transport.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := transport.SendRequest("after-close", nil); err == nil {
		t.Fatalf("SendRequest after close should fail")
	}
}

func writeRPCLine(t *testing.T, writer *io.PipeWriter, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal rpc line: %v", err)
	}
	if _, err := writer.Write(append(payload, '\n')); err != nil {
		t.Fatalf("write rpc line: %v", err)
	}
}

func assertJSONRPCLine(t *testing.T, line string, want string) {
	t.Helper()
	if !strings.Contains(line, `"jsonrpc":"2.0"`) || !strings.Contains(line, want) {
		t.Fatalf("line = %s, want %s", line, want)
	}
}
