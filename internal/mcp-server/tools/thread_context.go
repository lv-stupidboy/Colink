package tools

import (
	"encoding/json"
	"fmt"

	"github.com/anthropic/isdp/internal/mcp-server/client"
)

// ThreadContextTool 获取对话上下文工具
type ThreadContextTool struct {
	APIURL        string
	InvocationID  string
	CallbackToken string
}

func (t *ThreadContextTool) Name() string {
	return "thread_context"
}

func (t *ThreadContextTool) Description() string {
	return "Get the current thread conversation context. Returns recent messages and metadata."
}

func (t *ThreadContextTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of messages to retrieve (default: 10)",
				"default":     10,
			},
		},
	}
}

func (t *ThreadContextTool) Execute(args map[string]interface{}) (interface{}, error) {
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	// 调用 ISDP callback API
	client := client.NewAuthClient(t.APIURL, t.InvocationID, t.CallbackToken)

	reqBody := map[string]interface{}{
		"limit": limit,
	}

	respBody, err := client.CallAPI("GET", "/thread-context", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread context: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}