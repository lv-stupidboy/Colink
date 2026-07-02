package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestServerHandleInitializeAndToolsList(t *testing.T) {
	server := NewServer("https://colink.test", "inv-1", "token")

	initResp := server.handleRequest(&JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "initialize"})
	if initResp.Error != nil || initResp.JSONRPC != "2.0" {
		t.Fatalf("initialize response = %#v", initResp)
	}
	initResult := initResp.Result.(map[string]interface{})
	if initResult["protocolVersion"] != "2024-11-05" {
		t.Fatalf("initialize result = %#v", initResult)
	}

	listResp := server.handleRequest(&JSONRPCRequest{JSONRPC: "2.0", ID: 2, Method: "tools/list"})
	if listResp.Error != nil {
		t.Fatalf("tools/list error = %#v", listResp.Error)
	}
	tools := listResp.Result.(map[string]interface{})["tools"].([]map[string]interface{})
	if len(tools) < 5 {
		t.Fatalf("tools/list returned %#v", tools)
	}
}

func TestServerHandleToolsCallSuccessAndErrors(t *testing.T) {
	server := &Server{tools: make(map[string]Tool)}
	server.registerTool(fakeMCPTool{name: "ok", result: map[string]interface{}{"done": true}})
	server.registerTool(fakeMCPTool{name: "fail", errText: "boom"})

	params, _ := json.Marshal(map[string]interface{}{
		"name":      "ok",
		"arguments": map[string]interface{}{"message": "hello"},
	})
	resp := server.handleRequest(&JSONRPCRequest{JSONRPC: "2.0", ID: "ok-1", Method: "tools/call", Params: params})
	if resp.Error != nil || !strings.Contains(resp.Result.(map[string]interface{})["content"].([]map[string]interface{})[0]["text"].(string), "done") {
		t.Fatalf("tools/call success response = %#v", resp)
	}

	failParams, _ := json.Marshal(map[string]interface{}{"name": "fail", "arguments": map[string]interface{}{}})
	resp = server.handleRequest(&JSONRPCRequest{JSONRPC: "2.0", ID: "fail-1", Method: "tools/call", Params: failParams})
	content := resp.Result.(map[string]interface{})["content"].([]map[string]interface{})[0]
	if content["isError"] != true || !strings.Contains(content["text"].(string), "boom") {
		t.Fatalf("tools/call failure response = %#v", resp)
	}

	missingParams, _ := json.Marshal(map[string]interface{}{"name": "missing", "arguments": map[string]interface{}{}})
	resp = server.handleRequest(&JSONRPCRequest{JSONRPC: "2.0", ID: "missing-1", Method: "tools/call", Params: missingParams})
	if resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("missing tool response = %#v", resp)
	}

	resp = server.handleRequest(&JSONRPCRequest{JSONRPC: "2.0", ID: "bad-params", Method: "tools/call", Params: json.RawMessage("{bad")})
	if resp.Error == nil || resp.Error.Message != "Invalid params" {
		t.Fatalf("bad params response = %#v", resp)
	}

	resp = server.handleRequest(&JSONRPCRequest{JSONRPC: "2.0", ID: "unknown", Method: "unknown"})
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("unknown method response = %#v", resp)
	}
}

type fakeMCPTool struct {
	name    string
	result  interface{}
	errText string
}

func (f fakeMCPTool) Name() string { return f.name }

func (f fakeMCPTool) Description() string { return "fake tool" }

func (f fakeMCPTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{"type": "object"}
}

func (f fakeMCPTool) Execute(args map[string]interface{}) (interface{}, error) {
	if f.errText != "" {
		return nil, errString(f.errText)
	}
	return f.result, nil
}

type errString string

func (e errString) Error() string { return string(e) }
