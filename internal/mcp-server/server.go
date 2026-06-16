package mcpserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/anthropic/isdp/internal/mcp-server/tools"
)

type Server struct {
	apiURL        string
	invocationID  string
	callbackToken string
	tools         map[string]Tool
	mu            sync.Mutex
}

type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]interface{}
	Execute(args map[string]interface{}) (interface{}, error)
}

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewServer(apiURL, invocationID, callbackToken string) *Server {
	s := &Server{
		apiURL:        apiURL,
		invocationID:  invocationID,
		callbackToken: callbackToken,
		tools:         make(map[string]Tool),
	}

	// Team and project memory are handled through memory.add with type=team/project.
	s.registerTool(&tools.TeamListAgentsTool{
		APIURL:        apiURL,
		InvocationID:  invocationID,
		CallbackToken: callbackToken,
	})
	s.registerTool(&tools.MemoryAddTool{
		APIURL:        apiURL,
		InvocationID:  invocationID,
		CallbackToken: callbackToken,
	})
	s.registerTool(&tools.MemorySearchTool{
		APIURL:        apiURL,
		InvocationID:  invocationID,
		CallbackToken: callbackToken,
	})
	s.registerTool(&tools.PostMessageTool{
		APIURL:        apiURL,
		InvocationID:  invocationID,
		CallbackToken: callbackToken,
	})
	s.registerTool(&tools.ThreadContextTool{
		APIURL:        apiURL,
		InvocationID:  invocationID,
		CallbackToken: callbackToken,
	})

	return s
}

func (s *Server) registerTool(tool Tool) {
	s.mu.Lock()
	s.tools[tool.Name()] = tool
	s.mu.Unlock()
}

func (s *Server) Run() error {
	reader := bufio.NewReader(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(encoder, nil, -32700, "Parse error")
			os.Stdout.Sync() // 确保错误响应立即发送
			continue
		}

		resp := s.handleRequest(&req)
		if err := encoder.Encode(resp); err != nil {
			return fmt.Errorf("encode error: %w", err)
		}
		os.Stdout.Sync() // 确保响应立即发送给 CLI
		fmt.Fprintf(os.Stderr, "MCP: response sent for request %v\n", req.ID)
	}
}

func (s *Server) handleRequest(req *JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32601, Message: "Method not found"},
		}
	}
}

func (s *Server) handleInitialize(req *JSONRPCRequest) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "isdp-callback",
				"version": "1.0.0",
			},
		},
	}
}

func (s *Server) handleToolsList(req *JSONRPCRequest) *JSONRPCResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	toolList := make([]map[string]interface{}, 0, len(s.tools))
	for _, tool := range s.tools {
		toolList = append(toolList, map[string]interface{}{
			"name":        tool.Name(),
			"description": tool.Description(),
			"inputSchema": tool.InputSchema(),
		})
	}

	return &JSONRPCResponse{
		JSONRPC: req.JSONRPC,
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": toolList,
		},
	}
}

func (s *Server) handleToolsCall(req *JSONRPCRequest) *JSONRPCResponse {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: "Invalid params"},
		}
	}

	s.mu.Lock()
	tool, exists := s.tools[params.Name]
	s.mu.Unlock()

	if !exists {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: -32602, Message: "Tool not found"},
		}
	}

	result, err := tool.Execute(params.Arguments)
	if err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type":    "text",
						"text":    fmt.Sprintf("Error: %v", err),
						"isError": true,
					},
				},
			},
		}
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("%v", result),
				},
			},
		},
	}
}

func (s *Server) sendError(encoder *json.Encoder, id interface{}, code int, message string) {
	_ = encoder.Encode(&JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	})
}
