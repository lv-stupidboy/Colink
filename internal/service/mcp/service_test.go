package mcp

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func setupMCPServiceTest(t *testing.T) (*Service, *sql.DB) {
	t.Helper()

	db, _, err := repo.NewDB(repo.DBConfig{Type: repo.DBTypeSQLite, Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	schema := []string{
		`CREATE TABLE agent_configs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			role TEXT NOT NULL,
			description TEXT,
			system_prompt TEXT,
			max_tokens INTEGER,
			temperature REAL,
			base_agent_id TEXT,
			is_default BOOLEAN DEFAULT FALSE,
			is_system BOOLEAN DEFAULT FALSE,
			requires_human BOOLEAN DEFAULT FALSE,
			mention_patterns TEXT DEFAULT '[]',
			config_generated_at TEXT,
			config_path TEXT,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE TABLE mcp_servers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			display_name TEXT,
			description TEXT,
			transport TEXT NOT NULL DEFAULT 'stdio',
			command TEXT,
			args TEXT NOT NULL DEFAULT '[]',
			env TEXT NOT NULL DEFAULT '{}',
			url TEXT,
			headers TEXT NOT NULL DEFAULT '{}',
			source_type TEXT NOT NULL DEFAULT 'personal',
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE agent_mcp_bindings (
			id TEXT PRIMARY KEY,
			agent_role_id TEXT NOT NULL,
			mcp_server_id TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(agent_role_id, mcp_server_id),
			FOREIGN KEY(agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
			FOREIGN KEY(mcp_server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
		)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("failed to create test schema: %v", err)
		}
	}

	serverRepo := repo.NewMCPServerRepository(db, repo.DBTypeSQLite)
	bindingRepo := repo.NewAgentMCPBindingRepository(db, repo.DBTypeSQLite)
	agentRepo := repo.NewAgentConfigRepository(db, repo.DBTypeSQLite)
	return NewService(serverRepo, bindingRepo, agentRepo, zap.NewNop()), db
}

func TestMCPServiceFunctionalFlow(t *testing.T) {
	svc, db := setupMCPServiceTest(t)
	defer db.Close()

	ctx := context.Background()
	agentRepo := repo.NewAgentConfigRepository(db, repo.DBTypeSQLite)
	agentID := uuid.New()
	now := time.Now()
	if err := agentRepo.Create(ctx, &model.AgentRoleConfig{
		ID:              agentID,
		Name:            "Review Agent",
		Role:            model.AgentRole("custom"),
		SystemPrompt:    "review",
		MaxTokens:       4096,
		Temperature:     0.2,
		MentionPatterns: []string{"@review"},
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatalf("failed to create agent role: %v", err)
	}

	stdioServer, err := svc.Create(ctx, &model.CreateMCPServerRequest{
		Name:        "github-tools",
		DisplayName: "GitHub Tools",
		Transport:   model.MCPTransportStdio,
		Command:     "npx",
		Args:        []string{"-y", "@modelcontextprotocol/server-github"},
		Env:         map[string]string{"GITHUB_TOKEN": "${env:GITHUB_TOKEN}"},
		Status:      model.MCPStatusActive,
	})
	if err != nil {
		t.Fatalf("failed to create stdio server: %v", err)
	}

	httpServer, err := svc.Create(ctx, &model.CreateMCPServerRequest{
		Name:      "docs-search",
		Transport: model.MCPTransportHTTP,
		URL:       "https://example.test/mcp",
		Headers:   map[string]string{"Authorization": "Bearer ${env:DOCS_TOKEN}"},
		Status:    model.MCPStatusActive,
	})
	if err != nil {
		t.Fatalf("failed to create http server: %v", err)
	}

	if _, err := svc.Create(ctx, &model.CreateMCPServerRequest{
		Name:      "missing-command",
		Transport: model.MCPTransportStdio,
	}); err == nil {
		t.Fatal("expected stdio server without command to fail")
	}

	allServers, total, err := svc.List(ctx, &model.MCPServerListQuery{
		Status:   "active",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("failed to list servers: %v", err)
	}
	if total != 2 || len(allServers) != 2 {
		t.Fatalf("expected 2 active servers, total=%d servers=%#v", total, allServers)
	}

	disabled := model.MCPStatusDisabled
	if _, err := svc.Update(ctx, httpServer.ID, &model.UpdateMCPServerRequest{Status: &disabled}); err != nil {
		t.Fatalf("failed to disable http server: %v", err)
	}

	if err := svc.ReplaceAgentBindings(ctx, agentID, []uuid.UUID{stdioServer.ID, httpServer.ID}); err != nil {
		t.Fatalf("failed to bind servers to agent: %v", err)
	}
	bound, err := svc.GetAgentBindings(ctx, agentID)
	if err != nil {
		t.Fatalf("failed to get bindings: %v", err)
	}
	if len(bound) != 1 || bound[0].ID != stdioServer.ID {
		t.Fatalf("expected only active stdio server binding, got %#v", bound)
	}
}
