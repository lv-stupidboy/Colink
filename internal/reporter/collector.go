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
	// Single query to get all stats grouped by base agent type
	query := `
		SELECT
			ba.type,
			COUNT(DISTINCT ba.id) as base_count,
			COUNT(DISTINCT ac.id) as agents_count,
			SUM(CASE WHEN m.role = 'user' AND m.agent_id IS NOT NULL THEN 1 ELSE 0 END) as user_messages_count,
			SUM(CASE WHEN m.role = 'agent' THEN 1 ELSE 0 END) as agent_messages_count
		FROM base_agents ba
		LEFT JOIN agent_configs ac ON ac.base_agent_id = ba.id
		LEFT JOIN messages m ON m.agent_id = ac.id
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