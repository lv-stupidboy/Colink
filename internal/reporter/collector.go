// internal/reporter/collector.go
package reporter

import (
	"context"
	"database/sql"
	"time"
)

// Collector queries database to aggregate usage statistics.
type Collector struct {
	db *sql.DB
}

// NewCollector creates a new Collector instance.
func NewCollector(db *sql.DB) *Collector {
	return &Collector{db: db}
}

// CollectStats queries database and returns aggregated statistics.
// Uses 10 second timeout context to avoid blocking main service.
func (c *Collector) CollectStats(ctx context.Context) (StatsData, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	stats := StatsData{}

	// Query projects count
	if err := c.queryCount(ctx, "SELECT COUNT(*) FROM projects", &stats.ProjectsCount); err != nil {
		return stats, err
	}

	// Query threads count
	if err := c.queryCount(ctx, "SELECT COUNT(*) FROM threads", &stats.TasksCount); err != nil {
		return stats, err
	}

	// Query workflow templates count
	if err := c.queryCount(ctx, "SELECT COUNT(*) FROM workflow_templates", &stats.TeamsCount); err != nil {
		return stats, err
	}

	// Query base agents with all statistics grouped by type
	baseAgents, err := c.queryBaseAgentStats(ctx)
	if err != nil {
		return stats, err
	}
	stats.BaseAgents = baseAgents

	return stats, nil
}

// queryCount executes a COUNT query and stores result.
func (c *Collector) queryCount(ctx context.Context, query string, result *int) error {
	row := c.db.QueryRowContext(ctx, query)
	return row.Scan(result)
}

// queryBaseAgentStats returns comprehensive statistics for each base agent type.
func (c *Collector) queryBaseAgentStats(ctx context.Context) ([]BaseAgentStats, error) {
	// 查询所有 base_agents 的统计数据
	// 1. 处理 base_agent_id 为空的角色，归入默认 base_agent
	// 2. user_messages_count：统计 agent_invocations 的调用次数（每次调用代表一次用户请求）
	// 3. agent_messages_count：统计 messages 中 role='agent' 的消息数
	query := `
		SELECT
			ba.type,
			COUNT(DISTINCT ba.id) as base_count,
			COUNT(DISTINCT ac.id) as agents_count,
			COUNT(DISTINCT ai.id) as user_messages_count,
			SUM(CASE WHEN m.role = 'agent' THEN 1 ELSE 0 END) as agent_messages_count
		FROM base_agents ba
		LEFT JOIN agent_configs ac ON ac.base_agent_id = ba.id OR (ac.base_agent_id IS NULL AND ba.is_default = true)
		LEFT JOIN agent_invocations ai ON ai.agent_config_id = ac.id
		LEFT JOIN messages m ON m.role = 'agent' AND CAST(m.agent_id AS TEXT) = CAST(ac.id AS TEXT)
		GROUP BY ba.type
	`

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []BaseAgentStats
	for rows.Next() {
		var stat BaseAgentStats
		if err := rows.Scan(&stat.Type, &stat.Count, &stat.AgentsCount, &stat.UserMessagesCount, &stat.AgentMessagesCount); err != nil {
			return nil, err
		}
		results = append(results, stat)
	}

	return results, rows.Err()
}