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
	Type               string `json:"type"`
	Count              int    `json:"count"`                // 基础Agent数量（base_agents表中该类型的记录数）
	AgentsCount        int    `json:"agents_count"`         // 该类型下的角色配置数量（agent_configs表中关联到该类型的记录数，包含base_agent_id为空归入默认的）
	UserMessagesCount  int    `json:"user_messages_count"`  // 用户请求次数（agent_invocations表中该类型角色的调用次数）
	AgentMessagesCount int    `json:"agent_messages_count"` // Agent响应消息数（messages表中role='agent'且agent_id匹配该类型角色的消息数）
}