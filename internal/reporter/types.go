// internal/reporter/types.go
// Package reporter implements usage statistics reporting to external service.
package reporter

// ReportData is the top-level structure sent to the reporting endpoint.
type ReportData struct {
	Username   string    `json:"username"`
	Version    string    `json:"version"`
	ReportTime string    `json:"report_time"`
	Stats      StatsData `json:"stats"`
}

// StatsData contains aggregated usage statistics.
type StatsData struct {
	ProjectsCount          int              `json:"projects_count"`
	ThreadsCount           int              `json:"threads_count"`
	WorkflowTemplatesCount int              `json:"workflow_templates_count"`
	AgentConfigsCount      int              `json:"agent_configs_count"`
	BaseAgents             []BaseAgentStats `json:"base_agents"`
}

// BaseAgentStats represents count of each base agent type.
type BaseAgentStats struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}