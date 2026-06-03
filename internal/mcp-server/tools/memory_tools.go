package tools

import (
	"encoding/json"
	"fmt"
	"github.com/anthropic/isdp/internal/mcp-server/client"
)

type MemoryAddTool struct {
	APIURL        string
	InvocationID  string
	CallbackToken string
}

func (t *MemoryAddTool) Name() string {
	return "memory.add"
}

func (t *MemoryAddTool) Description() string {
	return "Add a shared Colink memory candidate. Prefer passing topic/facts/usage after you compact the memory yourself; Colink validates, merges by topic, and falls back to server-side compaction when structured fields are absent."
}

func (t *MemoryAddTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Reusable fact, constraint, convention, command, or verified conclusion to remember.",
			},
			"workspacePath": map[string]interface{}{
				"type":        "string",
				"description": "Optional workspace path. If omitted, Colink resolves it from the current invocation thread.",
			},
			"tags": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Optional search tags.",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"team", "project"},
				"description": "Optional memory type. Use team for user preferences, agent roles, collaboration rules; project for workspace-specific facts.",
			},
			"topic": map[string]interface{}{
				"type":        "string",
				"description": "Optional normalized topic, e.g. user_preferences, port_constraints, memory_test_rules.",
			},
			"facts": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Compact long-term facts extracted by the current agent. Prefer this over raw chat text.",
			},
			"usage": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Optional concise application guidance for the facts.",
			},
		},
	}
}

func (t *MemoryAddTool) Execute(args map[string]interface{}) (interface{}, error) {
	content, _ := args["content"].(string)
	if content == "" {
		if facts, ok := args["facts"]; !ok || facts == nil {
			return nil, fmt.Errorf("content or facts argument must be provided")
		}
	}
	reqBody := map[string]interface{}{
		"action":  "add",
		"content": content,
	}
	copyOptionalMemoryArgs(reqBody, args)
	return callMemoryAPI(t.APIURL, t.InvocationID, t.CallbackToken, reqBody)
}

type MemorySearchTool struct {
	APIURL        string
	InvocationID  string
	CallbackToken string
}

func (t *MemorySearchTool) Name() string {
	return "memory.search"
}

func (t *MemorySearchTool) Description() string {
	return "Search shared Colink Markdown memory across team and project memory."
}

func (t *MemorySearchTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query. Empty returns recent memories.",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"team", "project"},
				"description": "Optional memory type filter.",
			},
			"workspacePath": map[string]interface{}{
				"type":        "string",
				"description": "Optional workspace path. If omitted, Colink resolves it from the current invocation thread.",
			},
		},
	}
}

func (t *MemorySearchTool) Execute(args map[string]interface{}) (interface{}, error) {
	reqBody := map[string]interface{}{
		"action": "search",
	}
	copyOptionalMemoryArgs(reqBody, args)
	return callMemoryAPI(t.APIURL, t.InvocationID, t.CallbackToken, reqBody)
}

func copyOptionalMemoryArgs(reqBody, args map[string]interface{}) {
	for _, key := range []string{"workspacePath", "query", "type", "tags", "topic", "facts", "usage"} {
		if value, ok := args[key]; ok {
			reqBody[key] = value
		}
	}
}

func copyStructuredMemoryArgs(reqBody, args map[string]interface{}) {
	for _, key := range []string{"tags", "topic", "facts", "usage"} {
		if value, ok := args[key]; ok {
			reqBody[key] = value
		}
	}
}

func callMemoryAPI(apiURL, invocationID, callbackToken string, reqBody map[string]interface{}) (interface{}, error) {
	authClient := client.NewAuthClient(apiURL, invocationID, callbackToken)
	respBody, err := authClient.CallAPI("POST", "/memory", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to execute memory operation: %w", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result, nil
}
