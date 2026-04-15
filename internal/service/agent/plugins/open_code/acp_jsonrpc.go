// internal/service/agent/plugins/open_code/acp_jsonrpc.go
// ACP JSON-RPC transport
package open_code

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

type acpTransport struct {
	stdin          io.WriteCloser
	stdout         io.ReadCloser
	nextID         atomic.Uint64
	pending        map[uint64]chan *jsonrpcResponse
	pendingMu      sync.Mutex
	onNotification func(method string, params json.RawMessage)
	ctx            context.Context
	cancel         context.CancelFunc
	done           chan struct{}
	writeMu        sync.Mutex
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

	<-t.done

	t.pendingMu.Lock()
	t.pending = make(map[uint64]chan *jsonrpcResponse)
	t.pendingMu.Unlock()

	return closeErr
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

		if envelope.Method != "" && t.onNotification != nil {
			t.onNotification(envelope.Method, envelope.Params)
		}
	}
}