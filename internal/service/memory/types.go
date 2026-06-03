package memory

import "time"

type MemoryType string

const (
	MemoryTypeProject MemoryType = "project"
	MemoryTypeTeam    MemoryType = "team"
)

type MemorySource string

const (
	MemorySourceUserMessage      MemorySource = "user_message"
	MemorySourceAgentObservation MemorySource = "agent_observation"
	MemorySourceCommandResult    MemorySource = "command_result"
	MemorySourceManual           MemorySource = "manual"
)

type MemoryConfidence string

const (
	MemoryConfidenceHigh   MemoryConfidence = "high"
	MemoryConfidenceMedium MemoryConfidence = "medium"
	MemoryConfidenceLow    MemoryConfidence = "low"
)

type MemoryStatus string

const (
	MemoryStatusActive     MemoryStatus = "active"
	MemoryStatusOutdated   MemoryStatus = "outdated"
	MemoryStatusSuperseded MemoryStatus = "superseded"
	MemoryStatusUncertain  MemoryStatus = "uncertain"
)

type AgentInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Role         string   `json:"role,omitempty"`
	Description  string   `json:"description,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Source       string   `json:"source,omitempty"`
}

type MemoryEntry struct {
	ID         string           `json:"id"`
	Type       MemoryType       `json:"type"`
	Source     MemorySource     `json:"source"`
	Confidence MemoryConfidence `json:"confidence"`
	Status     MemoryStatus     `json:"status"`
	Tags       []string         `json:"tags,omitempty"`
	Topic      string           `json:"topic,omitempty"`
	Created    time.Time        `json:"created"`
	Updated    time.Time        `json:"updated"`
	Memory     string           `json:"memory"`
	Usage      string           `json:"usage,omitempty"`
}

type MemoryDraft struct {
	Topic string   `json:"topic,omitempty"`
	Facts []string `json:"facts,omitempty"`
	Usage []string `json:"usage,omitempty"`
}

type MemoryScopeIdentity struct {
	TeamID        string `json:"teamId,omitempty"`
	TeamName      string `json:"teamName,omitempty"`
	ProjectID     string `json:"projectId,omitempty"`
	ProjectName   string `json:"projectName,omitempty"`
	WorkspacePath string `json:"workspacePath,omitempty"`
}

type AddMemoryCandidateInput struct {
	Content       string       `json:"content"`
	Source        MemorySource `json:"source"`
	WorkspacePath string       `json:"workspacePath"`
	Scope         MemoryScopeIdentity
	CurrentAgent  string      `json:"currentAgent,omitempty"`
	CurrentTask   string      `json:"currentTask,omitempty"`
	Tags          []string    `json:"tags,omitempty"`
	Type          MemoryType  `json:"type,omitempty"`
	Draft         MemoryDraft `json:"draft,omitempty"`
}

type AddMemoryCandidateResult struct {
	Written    bool         `json:"written"`
	Type       MemoryType   `json:"type,omitempty"`
	TargetFile string       `json:"targetFile,omitempty"`
	EntryID    string       `json:"entryId,omitempty"`
	Status     MemoryStatus `json:"status,omitempty"`
	Reason     string       `json:"reason"`
}

type SearchMemoryInput struct {
	WorkspacePath   string `json:"workspacePath"`
	Scope           MemoryScopeIdentity
	Query           string     `json:"query,omitempty"`
	Type            MemoryType `json:"type,omitempty"`
	AgentID         string     `json:"agentId,omitempty"`
	IncludeInactive bool       `json:"includeInactive,omitempty"`
	Limit           int        `json:"limit,omitempty"`
}

type MemoryToolRequest struct {
	Action        string     `json:"action"`
	Scope         string     `json:"scope,omitempty"`
	Type          MemoryType `json:"type,omitempty"`
	WorkspacePath string     `json:"workspacePath,omitempty"`
	TeamID        string     `json:"teamId,omitempty"`
	TeamName      string     `json:"teamName,omitempty"`
	ProjectID     string     `json:"projectId,omitempty"`
	ProjectName   string     `json:"projectName,omitempty"`
	Content       string     `json:"content,omitempty"`
	OldText       string     `json:"oldText,omitempty"`
	Query         string     `json:"query,omitempty"`
	Status        string     `json:"status,omitempty"`
	Category      string     `json:"category,omitempty"`
	Tags          []string   `json:"tags,omitempty"`
	Topic         string     `json:"topic,omitempty"`
	Facts         []string   `json:"facts,omitempty"`
	Usage         []string   `json:"usage,omitempty"`
}

type MemoryToolResponse struct {
	Success bool          `json:"success"`
	Message string        `json:"message,omitempty"`
	Error   string        `json:"error,omitempty"`
	Entries []string      `json:"entries,omitempty"`
	Results []MemoryEntry `json:"results,omitempty"`
}
