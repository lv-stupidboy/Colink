package tools

import (
	"encoding/json"
	"fmt"

	"github.com/anthropic/isdp/internal/mcp-server/client"
)

// PostMessageTool A2A 消息发送工具
type PostMessageTool struct {
	APIURL        string
	InvocationID  string
	CallbackToken string
}

func (t *PostMessageTool) Name() string {
	return "post_message"
}

func (t *PostMessageTool) Description() string {
	return "Send a message to trigger downstream Agent via A2A. Use @mention to specify target Agent. Example: '@Reviewer please review this code'"
}

func (t *PostMessageTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Message content with @mention to trigger target Agent",
			},
		},
		"required": []string{"message"},
	}
}

func (t *PostMessageTool) Execute(args map[string]interface{}) (interface{}, error) {
	message, ok := args["message"].(string)
	if !ok {
		return nil, fmt.Errorf("message argument must be a string")
	}

	// 调用 ISDP callback API
	client := client.NewAuthClient(t.APIURL, t.InvocationID, t.CallbackToken)

	reqBody := map[string]string{
		"message": message,
	}

	respBody, err := client.CallAPI("POST", "/post-message", reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}
