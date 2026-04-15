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
	ProjectsCount int              `json:"projects_count"`
	TasksCount    int              `json:"tasks_count"`
	TeamsCount    int              `json:"teams_count"`
	AgentsCount   int              `json:"agents_count"`
	BaseAgents    []BaseAgentStats `json:"base_agents"`
}

// BaseAgentStats represents count of each base agent type.
type BaseAgentStats struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}