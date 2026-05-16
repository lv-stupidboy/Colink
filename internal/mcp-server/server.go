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

// Server MCP 服务器实现
type Server struct {
	apiURL        string
	invocationID  string
	callbackToken string
	tools         map[string]Tool
	mu            sync.Mutex
}

// Tool MCP 工具接口
type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]interface{}
	Execute(args map[string]interface{}) (interface{}, error)
}

// JSONRPCRequest JSON-RPC 请求
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// JSONRPCResponse JSON-RPC 响应
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError RPC 错误
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewServer 创建 MCP Server
func NewServer(apiURL, invocationID, callbackToken string) *Server {
	s := &Server{
		apiURL:        apiURL,
		invocationID:  invocationID,
		callbackToken: callbackToken,
		tools:         make(map[string]Tool),
	}

	// 注册工具 - 分层设计：team_memory + project_memory（替代原 memory 工具）
	s.registerTool(&tools.TeamMemoryTool{
		APIURL:        apiURL,
		InvocationID:  invocationID,
		CallbackToken: callbackToken,
	})
	s.registerTool(&tools.ProjectMemoryTool{
		APIURL:        apiURL,
		InvocationID:  invocationID,
		CallbackToken: callbackToken,
	})
	// 保留其他工具
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

// Run 启动 MCP Server（stdio 模式）
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

		// 解析请求
		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(encoder, nil, -32700, "Parse error")
			continue
		}

		// 处理请求
		resp := s.handleRequest(&req)
		if err := encoder.Encode(resp); err != nil {
			return fmt.Errorf("encode error: %w", err)
		}
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
		JSONRPC: "2.0",
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
						"type": "text",
						"text": fmt.Sprintf("Error: %v", err),
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
	encoder.Encode(&JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	})
}