package tools

import (
	"encoding/json"
	"fmt"

	"github.com/anthropic/isdp/internal/mcp-server/client"
)

// TeamListAgentsTool lists Colink-managed team agents available to the workspace.
type TeamListAgentsTool struct {
	APIURL        string
	InvocationID  string
	CallbackToken string
}

func (t *TeamListAgentsTool) Name() string {
	return "team.list_agents"
}

func (t *TeamListAgentsTool) Description() string {
	return "List Colink-managed team agents available for the current workspace, including id, name, role, description, capabilities, and source."
}

func (t *TeamListAgentsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"workspacePath": map[string]interface{}{
				"type":        "string",
				"description": "Optional workspace path. If omitted, Colink resolves it from the current invocation thread.",
			},
		},
	}
}

func (t *TeamListAgentsTool) Execute(args map[string]interface{}) (interface{}, error) {
	authClient := client.NewAuthClient(t.APIURL, t.InvocationID, t.CallbackToken)
	reqBody := map[string]interface{}{}
	if workspacePath, ok := args["workspacePath"].(string); ok {
		reqBody["workspacePath"] = workspacePath
	}

	respBody, err := authClient.CallAPI("POST", "/team/list-agents", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to list team agents: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return result, nil
}
