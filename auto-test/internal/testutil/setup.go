// auto-test/internal/testutil/setup.go
// Package testutil provides common test setup utilities
package testutil

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// SetupTestDB creates an in-memory SQLite database for testing
func SetupTestDB() (*sql.DB, repo.Dialect, error) {
	cfg := repo.DBConfig{
		Type: repo.DBTypeSQLite,
		Path: ":memory:",
	}
	db, dialect, err := repo.NewDB(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create test database: %w", err)
	}

	// Create tables
	if err := createTables(db); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, dialect, nil
}

// createTables creates all necessary tables for testing
func createTables(db *sql.DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS base_agents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			api_url TEXT DEFAULT NULL,
			api_token TEXT,
			default_model TEXT DEFAULT NULL,
			cli_path TEXT DEFAULT 'claude',
			git_bash_path TEXT DEFAULT NULL,
			max_tokens INTEGER DEFAULT NULL,
			timeout_minutes INTEGER DEFAULT 30,
			is_default INTEGER DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS agent_configs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			role TEXT NOT NULL,
			base_agent_id TEXT DEFAULT NULL,
			description TEXT,
			system_prompt TEXT,
			max_tokens INTEGER DEFAULT 4096,
			temperature REAL DEFAULT 0.7,
			is_default INTEGER DEFAULT 0,
			is_system INTEGER DEFAULT 0,
			requires_human INTEGER DEFAULT 0,
			mention_patterns TEXT DEFAULT NULL,
			config_generated_at TEXT DEFAULT NULL,
			config_path TEXT DEFAULT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS workflow_templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			agent_ids TEXT DEFAULT NULL,
			transitions TEXT DEFAULT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			mode TEXT NOT NULL,
			status TEXT DEFAULT 'draft',
			local_path TEXT NOT NULL,
			git_repo TEXT DEFAULT NULL,
			config TEXT DEFAULT NULL,
			description TEXT DEFAULT NULL,
			workflow_template_id TEXT DEFAULT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS threads (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			title TEXT,
			status TEXT DEFAULT 'active',
			workflow_template_id TEXT DEFAULT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			name TEXT DEFAULT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS agent_skill_bindings (
			id TEXT PRIMARY KEY,
			agent_role_id TEXT NOT NULL,
			skill_id TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS agent_subagent_bindings (
			id TEXT PRIMARY KEY,
			agent_role_id TEXT NOT NULL,
			subagent_id TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS agent_command_bindings (
			id TEXT PRIMARY KEY,
			agent_role_id TEXT NOT NULL,
			command_id TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS agent_rule_bindings (
			id TEXT PRIMARY KEY,
			agent_role_id TEXT NOT NULL,
			rule_id TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS agent_settings_bindings (
			id TEXT PRIMARY KEY,
			agent_role_id TEXT NOT NULL,
			settings_id TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

// InsertTestBaseAgent inserts a test base agent
func InsertTestBaseAgent(db *sql.DB, id, name, agentType string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	query := `INSERT INTO base_agents (id, name, type, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	_, err := db.Exec(query, id, name, agentType, now, now)
	return err
}

// InsertTestAgentConfig inserts a test agent config
func InsertTestAgentConfig(db *sql.DB, config *model.AgentRoleConfig) error {
	query := `INSERT INTO agent_configs (id, name, role, base_agent_id, description, system_prompt, max_tokens, temperature, is_default, is_system, requires_human, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now().Format("2006-01-02 15:04:05")
	var baseAgentID interface{}
	if config.BaseAgentID != uuid.Nil {
		baseAgentID = config.BaseAgentID.String()
	}
	_, err := db.Exec(query,
		config.ID.String(),
		config.Name,
		config.Role,
		baseAgentID,
		config.Description,
		config.SystemPrompt,
		config.MaxTokens,
		config.Temperature,
		config.IsDefault,
		config.IsSystem,
		config.RequiresHuman,
		now,
		now,
	)
	return err
}

// InsertTestWorkflowTemplate inserts a test workflow template
func InsertTestWorkflowTemplate(db *sql.DB, id, name string, agentIDs []string) error {
	query := `INSERT INTO workflow_templates (id, name, agent_ids, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	now := time.Now().Format("2006-01-02 15:04:05")
	agentIDsJSON, _ := json.Marshal(agentIDs)
	_, err := db.Exec(query, id, name, string(agentIDsJSON), now, now)
	return err
}

// NewTestAgentConfig creates a test agent config with defaults
func NewTestAgentConfig(name string) *model.AgentRoleConfig {
	return &model.AgentRoleConfig{
		ID:           uuid.New(),
		Name:         name,
		Role:         model.AgentRoleAgent,
		SystemPrompt: "Test system prompt",
		MaxTokens:    4096,
		Temperature:  0.7,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// CleanupTestDB closes the test database
func CleanupTestDB(db *sql.DB) {
	if db != nil {
		db.Close()
	}
}

// TestContext returns a standard context for tests
func TestContext() context.Context {
	return context.Background()
}