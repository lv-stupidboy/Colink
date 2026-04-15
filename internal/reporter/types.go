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
	BaseAgents    []BaseAgentStats `json:"base_agents"`
}

// BaseAgentStats represents statistics for each base agent type.
type BaseAgentStats struct {
	Type              string `json:"type"`
	Count             int    `json:"count"`               // 基础Agent数量
	AgentsCount       int    `json:"agents_count"`        // 该类型下的角色配置数量
	UserMessagesCount int    `json:"user_messages_count"` // 用户发给该类型Agent的消息数
	AgentMessagesCount int   `json:"agent_messages_count"` // 该类型Agent的响应消息数
}