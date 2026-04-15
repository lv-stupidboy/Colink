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

	// Query agent configs count
	if err := c.queryCount(ctx, "SELECT COUNT(*) FROM agent_configs", &stats.AgentsCount); err != nil {
		return stats, err
	}

	// Query base agents grouped by type
	baseAgents, err := c.queryBaseAgents(ctx)
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

// queryBaseAgents returns count of each base agent type.
func (c *Collector) queryBaseAgents(ctx context.Context) ([]BaseAgentStats, error) {
	query := "SELECT type, COUNT(*) FROM base_agents GROUP BY type"
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []BaseAgentStats
	for rows.Next() {
		var stat BaseAgentStats
		if err := rows.Scan(&stat.Type, &stat.Count); err != nil {
			return nil, err
		}
		results = append(results, stat)
	}

	return results, rows.Err()
}