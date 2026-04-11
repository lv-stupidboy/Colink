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
	CWD        string        `json:"cwd"`
	MCPServers []interface{} `json:"mcpServers"`
}

// session/new response: configOptions (newer) and/or legacy models/modes
type acpNewSessionResult struct {
	SessionID     string                 `json:"sessionId"`
	Models        *acpSessionModels      `json:"models,omitempty"`
	Modes         *acpSessionModes       `json:"modes,omitempty"`
	ConfigOptions []acpSessionConfigOpt  `json:"configOptions,omitempty"`
	Meta          map[string]interface{} `json:"_meta,omitempty"`
}

// acpSessionModels holds available and current model info (legacy API).
type acpSessionModels struct {
	AvailableModels []acpModelInfo `json:"availableModels"`
	DefaultModelID  string         `json:"defaultModelId,omitempty"`
	CurrentModelID  string         `json:"currentModelId,omitempty"`
}

type acpModelInfo struct {
	ModelID string `json:"modelId"`
	Name    string `json:"name"`
}

// acpSessionModes holds available and current mode/agent info (legacy API).
type acpSessionModes struct {
	AvailableModes []acpModeInfo `json:"availableModes"`
	DefaultModeID  string        `json:"defaultModeId,omitempty"`
	CurrentModeID  string        `json:"currentModeId,omitempty"`
}

type acpModeInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type acpSessionConfigOpt struct {
	ConfigID string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	Type     string `json:"type"`
	Options  []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"options,omitempty"`
	CurrentValue string `json:"currentValue,omitempty"`
	Default      string `json:"default,omitempty"`
}

// acpSetModelParams sets the model for an existing session.
// Maps to ACP method "session/set_model" (legacy, widely supported).
type acpSetModelParams struct {
	SessionID string `json:"sessionId"`
	ModelID   string `json:"modelId"`
}

// acpSetModeParams sets the mode (agent) for an existing session.
// Maps to ACP method "session/set_mode" (legacy, widely supported).
type acpSetModeParams struct {
	SessionID string `json:"sessionId"`
	ModeID    string `json:"modeId"`
}

// acpSetConfigOptionParams sets a config option for an existing session.
// Maps to ACP method "session/set_config_option" (newer API).
type acpSetConfigOptionParams struct {
	SessionID string `json:"sessionId"`
	ConfigID  string `json:"configId"`
	Value     string `json:"value"`
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
	SessionUpdate string          `json:"sessionUpdate"`
	InputTokens   int64           `json:"inputTokens,omitempty"`
	OutputTokens  int64           `json:"outputTokens,omitempty"`
	Cost          json.RawMessage `json:"cost,omitempty"`
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
