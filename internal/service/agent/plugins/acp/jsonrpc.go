// internal/service/agent/plugins/acp/jsonrpc.go
// ACP JSON-RPC transport
package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

type acpTransport struct {
	stdin           io.WriteCloser
	stdout          io.ReadCloser
	nextID          atomic.Uint64
	pending         map[uint64]chan *jsonrpcResponse
	pendingMu       sync.Mutex
	onNotification  func(method string, params json.RawMessage)
	onServerRequest func(id interface{}, method string, params json.RawMessage)
	ctx             context.Context
	cancel          context.CancelFunc
	done            chan struct{}
	writeMu         sync.Mutex
}

func newACPTransport(stdin io.WriteCloser, stdout io.ReadCloser, onNotification func(method string, params json.RawMessage)) *acpTransport {
	ctx, cancel := context.WithCancel(context.Background())

	return &acpTransport{
		stdin:          stdin,
		stdout:         stdout,
		pending:        make(map[uint64]chan *jsonrpcResponse),
		onNotification: onNotification,
		ctx:            ctx,
		cancel:         cancel,
		done:           make(chan struct{}),
	}
}

func (t *acpTransport) Start() error {
	go t.readLoop()
	return nil
}

func (t *acpTransport) SendRequest(method string, params interface{}) (json.RawMessage, error) {
	t.writeMu.Lock()

	id := t.nextID.Add(1)
	ch := make(chan *jsonrpcResponse, 1)

	t.pendingMu.Lock()
	t.pending[id] = ch
	t.pendingMu.Unlock()

	request := struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      uint64      `json:"id"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		t.pendingMu.Lock()
		delete(t.pending, id)
		t.pendingMu.Unlock()
		t.writeMu.Unlock()
		return nil, fmt.Errorf("ACP marshal request %s: %w", method, err)
	}

	if _, err := t.stdin.Write(append(payload, '\n')); err != nil {
		t.pendingMu.Lock()
		delete(t.pending, id)
		t.pendingMu.Unlock()
		t.writeMu.Unlock()
		return nil, fmt.Errorf("ACP write request %s: %w", method, err)
	}

	t.writeMu.Unlock()

	select {
	case resp := <-ch:
		t.pendingMu.Lock()
		delete(t.pending, id)
		t.pendingMu.Unlock()

		if resp.Error != nil {
			return nil, fmt.Errorf("ACP RPC error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-t.ctx.Done():
		t.pendingMu.Lock()
		delete(t.pending, id)
		t.pendingMu.Unlock()
		return nil, fmt.Errorf("ACP request %s timed out: %w", method, t.ctx.Err())
	}
}

func (t *acpTransport) SendResponse(id interface{}, result interface{}) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	response := struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      interface{} `json:"id"`
		Result  interface{} `json:"result,omitempty"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	payload, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("ACP marshal response: %w", err)
	}

	if _, err := t.stdin.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("ACP write response: %w", err)
	}

	return nil
}

func (t *acpTransport) SendNotification(method string, params interface{}) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	notification := struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	payload, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("ACP marshal notification %s: %w", method, err)
	}

	if _, err := t.stdin.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("ACP write notification %s: %w", method, err)
	}

	return nil
}

func (t *acpTransport) Close() error {
	t.cancel()

	var closeErr error
	if t.stdin != nil {
		closeErr = t.stdin.Close()
	}

	// Wait for readLoop to finish, but with timeout
	// OpenClaw ACP bridge may not exit after stdin close, so we need timeout
	select {
	case <-t.done:
		// readLoop finished normally
	case <-time.After(500 * time.Millisecond):
		// Timeout: force close stdout to break scanner.Scan() block
		if t.stdout != nil {
			t.stdout.Close()
		}
		// Wait a bit more for readLoop to finish after stdout close
		select {
		case <-t.done:
		case <-time.After(200 * time.Millisecond):
			// Still not finished, but we've done our best
		}
	}

	t.pendingMu.Lock()
	t.pending = make(map[uint64]chan *jsonrpcResponse)
	t.pendingMu.Unlock()

	return closeErr
}

// SetNotificationHandler 设置 notification handler
// 用于长连接模式下动态更新 handler
func (t *acpTransport) SetNotificationHandler(handler func(method string, params json.RawMessage)) {
	t.pendingMu.Lock()
	t.onNotification = handler
	t.pendingMu.Unlock()
}

// SetServerRequestHandler 设置服务端 request handler
// 用于处理服务端发起的 request（如 session/request_permission）
func (t *acpTransport) SetServerRequestHandler(handler func(id interface{}, method string, params json.RawMessage)) {
	t.pendingMu.Lock()
	t.onServerRequest = handler
	t.pendingMu.Unlock()
}

func (t *acpTransport) readLoop() {
	defer close(t.done)

	scanner := bufio.NewScanner(t.stdout)
	scanner.Buffer(make([]byte, 0, 10*1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		var envelope struct {
			ID     *json.RawMessage `json:"id"`
			Method string           `json:"method"`
			Params json.RawMessage  `json:"params,omitempty"`
		}

		if err := json.Unmarshal(line, &envelope); err != nil {
			continue
		}

		// 有 ID + 有 method：服务端发起的 request，需要回复 response
		if envelope.ID != nil && envelope.Method != "" {
			// 解析 ID 为具体类型
			var idValue interface{}
			if err := json.Unmarshal(*envelope.ID, &idValue); err != nil {
				continue
			}
			// 调用服务端 request handler
			if t.onServerRequest != nil {
				t.onServerRequest(idValue, envelope.Method, envelope.Params)
			}
			continue
		}

		// 有 ID + 无 method：客户端 request 的 response
		if envelope.ID != nil {
			var msg jsonrpcResponse
			if err := json.Unmarshal(line, &msg); err != nil {
				continue
			}

			if msg.ID > 0 {
				t.pendingMu.Lock()
				if ch, ok := t.pending[msg.ID]; ok {
					ch <- &msg
				}
				t.pendingMu.Unlock()
			}
			continue
		}

		// 无 ID + 有 method：notification
		if envelope.Method != "" && t.onNotification != nil {
			t.onNotification(envelope.Method, envelope.Params)
		}
	}
}
