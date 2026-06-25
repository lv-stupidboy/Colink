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
			checkpoints TEXT DEFAULT NULL,
			estimated_time INTEGER DEFAULT 0,
			is_system INTEGER DEFAULT 0,
			is_default INTEGER DEFAULT 0,
			routable_teams TEXT DEFAULT NULL,
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
			current_phase TEXT DEFAULT '',
			current_agent TEXT DEFAULT '',
			depth INTEGER DEFAULT 0,
			abort_token TEXT DEFAULT NULL,
			workflow_template_id TEXT DEFAULT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			name TEXT DEFAULT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS agent_invocations (
			id TEXT PRIMARY KEY,
			thread_id TEXT NOT NULL,
			agent_config_id TEXT DEFAULT NULL,
			role TEXT NOT NULL,
			agent_name TEXT DEFAULT NULL,
			status TEXT DEFAULT 'running',
			input TEXT,
			full_prompt TEXT,
			output TEXT,
			started_at TEXT DEFAULT NULL,
			completed_at TEXT DEFAULT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			process_id TEXT DEFAULT NULL,
			session_id TEXT DEFAULT NULL,
			input_tokens INTEGER DEFAULT 0,
			output_tokens INTEGER DEFAULT 0,
			cache_read_tokens INTEGER DEFAULT 0,
			cache_creation_tokens INTEGER DEFAULT 0,
			cost_usd REAL DEFAULT 0.0,
			duration_ms INTEGER DEFAULT 0,
			duration_api_ms INTEGER DEFAULT 0,
			callback_token TEXT DEFAULT NULL,
			triggered_by TEXT DEFAULT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS human_tasks (
			id TEXT PRIMARY KEY,
			thread_id TEXT NOT NULL,
			invocation_id TEXT NOT NULL,
			agent_config_id TEXT NOT NULL,
			agent_name TEXT DEFAULT NULL,
			wait_reason TEXT DEFAULT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at TEXT DEFAULT NULL,
			project_id TEXT DEFAULT NULL,
			project_name TEXT DEFAULT NULL,
			thread_name TEXT DEFAULT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			thread_id TEXT NOT NULL,
			role TEXT NOT NULL,
			agent_id TEXT DEFAULT NULL,
			content TEXT,
			content_blocks TEXT DEFAULT NULL,
			message_type TEXT DEFAULT 'text',
			metadata TEXT DEFAULT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			mentions TEXT DEFAULT NULL,
			origin TEXT DEFAULT NULL,
			reply_to TEXT DEFAULT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS artifacts (
			id TEXT PRIMARY KEY,
			thread_id TEXT NOT NULL,
			type TEXT DEFAULT NULL,
			name TEXT DEFAULT NULL,
			path TEXT DEFAULT NULL,
			content TEXT,
			metadata TEXT DEFAULT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS subagents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT DEFAULT NULL,
			supported_agents TEXT DEFAULT '["claude_code"]',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS skills (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT DEFAULT NULL,
			tags TEXT DEFAULT '[]',
			source_type TEXT NOT NULL DEFAULT 'personal',
			source_registry_id TEXT DEFAULT NULL,
			source_path TEXT DEFAULT NULL,
			author_id TEXT DEFAULT NULL,
			project_id TEXT DEFAULT NULL,
			supported_agents TEXT DEFAULT '["claude_code"]',
			use_count INTEGER DEFAULT 0,
			status TEXT DEFAULT 'active',
			is_public INTEGER DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT DEFAULT NULL,
			directory_path TEXT DEFAULT NULL,
			supported_agents TEXT DEFAULT '["claude_code"]',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS skill_registries (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			display_name TEXT DEFAULT NULL,
			type TEXT NOT NULL,
			url TEXT NOT NULL,
			auth_config TEXT DEFAULT '{}',
			sync_interval INTEGER DEFAULT 0,
			last_sync_at TEXT DEFAULT NULL,
			sync_status TEXT DEFAULT 'pending',
			skill_count INTEGER DEFAULT 0,
			status TEXT DEFAULT 'active',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS knowledge_bases (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			display_name TEXT DEFAULT NULL,
			description TEXT DEFAULT NULL,
			type TEXT NOT NULL,
			config TEXT DEFAULT '{}',
			query_endpoint TEXT DEFAULT NULL,
			status TEXT DEFAULT 'active',
			query_count INTEGER DEFAULT 0,
			last_query_at TEXT DEFAULT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS local_repos (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			git_url TEXT DEFAULT '',
			local_path TEXT NOT NULL,
			branch TEXT DEFAULT NULL,
			last_commit TEXT DEFAULT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			error_message TEXT DEFAULT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS commands (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT DEFAULT NULL,
			supported_agents TEXT DEFAULT '["claude_code"]',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS mcp_servers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			display_name TEXT DEFAULT NULL,
			description TEXT DEFAULT NULL,
			transport TEXT NOT NULL,
			command TEXT DEFAULT NULL,
			args TEXT DEFAULT '[]',
			env TEXT DEFAULT '{}',
			url TEXT DEFAULT NULL,
			headers TEXT DEFAULT '{}',
			source_type TEXT NOT NULL DEFAULT 'personal',
			supported_agents TEXT DEFAULT '["claude_code"]',
			status TEXT NOT NULL DEFAULT 'active',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS command_skill_bindings (
			id TEXT PRIMARY KEY,
			command_id TEXT NOT NULL,
			skill_id TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS subagent_skill_bindings (
			id TEXT PRIMARY KEY,
			subagent_id TEXT NOT NULL,
			skill_id TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
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
		`CREATE TABLE IF NOT EXISTS agent_mcp_bindings (
			id TEXT PRIMARY KEY,
			agent_role_id TEXT NOT NULL,
			mcp_server_id TEXT NOT NULL,
			enabled INTEGER DEFAULT 1,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS agent_rule_bindings (
			id TEXT PRIMARY KEY,
			agent_role_id TEXT NOT NULL,
			rule_id TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS rules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT DEFAULT NULL,
			supported_agents TEXT DEFAULT '["claude_code"]',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
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
