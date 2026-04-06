package agent

import "encoding/json"

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *jsonrpcError   `json:"error"`
}

type jsonrpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type acpInitializeParams struct {
	ProtocolVersion    int                   `json:"protocolVersion"`
	ClientCapabilities acpClientCapabilities `json:"clientCapabilities"`
}

type acpClientCapabilities struct {
	PromptCapabilities acpPromptCapabilities `json:"promptCapabilities"`
}

type acpPromptCapabilities struct {
	Image           bool `json:"image"`
	EmbeddedContext bool `json:"embeddedContext"`
}

type acpInitializeResult struct {
	ProtocolVersion   int                    `json:"protocolVersion"`
	AgentCapabilities map[string]interface{} `json:"agentCapabilities"`
}

type acpNewSessionParams struct {
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	Model     string `json:"model"`
}

type acpNewSessionResult struct {
	SessionID string `json:"sessionId"`
}

type acpCancelParams struct {
	SessionID string `json:"sessionId"`
}

type acpPermissionRequest struct {
	SessionID  string      `json:"sessionId"`
	Permission interface{} `json:"permission"`
}

type acpPermissionResponse struct {
	Allow string `json:"allow"`
}

type acpPromptParams struct {
	SessionID string            `json:"sessionId"`
	Prompt    []acpContentBlock `json:"prompt"`
}

type acpContentBlock struct {
	Type     string `json:"type"` // ACP: "text", "resource", "image"
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
}

type acpPromptResult struct {
	StopReason string `json:"stopReason"` // ACP: "end_turn", "cancelled", "max_tokens", "refusal"
}

type acpSessionUpdateParams struct {
	SessionID string          `json:"sessionId"`
	Update    json.RawMessage `json:"update"`
}

type acpSessionUpdateHeader struct {
	SessionUpdate string `json:"sessionUpdate"`
}

type acpAgentMessageChunk struct {
	SessionUpdate string          `json:"sessionUpdate"`
	Content       acpContentBlock `json:"content"`
}

type acpAgentThoughtChunk struct {
	SessionUpdate string          `json:"sessionUpdate"`
	Content       acpContentBlock `json:"content"`
}

type acpToolCall struct {
	SessionUpdate string            `json:"sessionUpdate"`
	ToolCallID    string            `json:"toolCallId"`
	Status        string            `json:"status"`
	Title         string            `json:"title"`
	RawInput      interface{}       `json:"rawInput,omitempty"`
	Kind          string            `json:"kind,omitempty"`
	Content       []acpContentBlock `json:"content,omitempty"`
}

type acpToolCallUpdate struct {
	SessionUpdate string            `json:"sessionUpdate"`
	ToolCallID    string            `json:"toolCallId"`
	Status        string            `json:"status"`
	Content       []acpContentBlock `json:"content,omitempty"`
}

type acpUsageUpdate struct {
	SessionUpdate string  `json:"sessionUpdate"`
	InputTokens   int64   `json:"inputTokens,omitempty"`
	OutputTokens  int64   `json:"outputTokens,omitempty"`
	Cost          float64 `json:"cost,omitempty"`
}

type acpPlanUpdate struct {
	SessionUpdate string         `json:"sessionUpdate"`
	Entries       []acpPlanEntry `json:"entries,omitempty"`
}

type acpPlanEntry struct {
	Content  string `json:"content"`
	Status   string `json:"status"`
	Priority int    `json:"priority,omitempty"`
}
