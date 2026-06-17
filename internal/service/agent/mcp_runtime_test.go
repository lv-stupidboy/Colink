package agent

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

func setupMCPRuntimeTest(t *testing.T) (*ExecutionService, *repo.MCPServerRepository, *repo.AgentMCPBindingRepository, *sql.DB) {
	t.Helper()

	db, _, err := repo.NewDB(repo.DBConfig{Type: repo.DBTypeSQLite, Path: ":memory:"})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	schema := []string{
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
			supported_agents TEXT NOT NULL DEFAULT '["claude_code"]',
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
			UNIQUE(agent_role_id, mcp_server_id)
		)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("failed to create test schema: %v", err)
		}
	}

	bindingRepo := repo.NewAgentMCPBindingRepository(db, repo.DBTypeSQLite)
	return &ExecutionService{mcpBindingRepo: bindingRepo}, repo.NewMCPServerRepository(db, repo.DBTypeSQLite), bindingRepo, db
}

func TestLoadBoundMCPServersForBaseAgentTypes(t *testing.T) {
	es, serverRepo, bindingRepo, db := setupMCPRuntimeTest(t)
	defer db.Close()

	ctx := context.Background()
	now := time.Now()
	agentConfig := &model.AgentRoleConfig{ID: uuid.New()}

	servers := []*model.MCPServer{
		{
			ID:              uuid.New(),
			Name:            "shared-tools",
			Transport:       model.MCPTransportStdio,
			Command:         "shared",
			SourceType:      model.MCPSourcePersonal,
			SupportedAgents: []string{"claude_code", "open_code", "hermes", "open_claw"},
			Status:          model.MCPStatusActive,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              uuid.New(),
			Name:            "open-code-only",
			Transport:       model.MCPTransportHTTP,
			URL:             "https://example.test/mcp",
			Headers:         map[string]string{},
			SourceType:      model.MCPSourcePersonal,
			SupportedAgents: []string{"open_code"},
			Status:          model.MCPStatusActive,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              uuid.New(),
			Name:            "hermes-only",
			Transport:       model.MCPTransportSSE,
			URL:             "https://example.test/sse",
			Headers:         map[string]string{},
			SourceType:      model.MCPSourcePersonal,
			SupportedAgents: []string{"hermes"},
			Status:          model.MCPStatusActive,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:              uuid.New(),
			Name:            "open-claw-only",
			Transport:       model.MCPTransportStdio,
			Command:         "openclaw-tool",
			SourceType:      model.MCPSourcePersonal,
			SupportedAgents: []string{"open_claw"},
			Status:          model.MCPStatusActive,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			ID:         uuid.New(),
			Name:       "legacy-claude-default",
			Transport:  model.MCPTransportStdio,
			Command:    "legacy",
			SourceType: model.MCPSourcePersonal,
			Status:     model.MCPStatusActive,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}

	serverIDs := make([]uuid.UUID, 0, len(servers))
	for _, server := range servers {
		if server.Args == nil {
			server.Args = []string{}
		}
		if server.Env == nil {
			server.Env = map[string]string{}
		}
		if server.Headers == nil {
			server.Headers = map[string]string{}
		}
		if err := serverRepo.Create(ctx, server); err != nil {
			t.Fatalf("failed to create MCP server %s: %v", server.Name, err)
		}
		serverIDs = append(serverIDs, server.ID)
	}
	if err := bindingRepo.ReplaceBindings(ctx, agentConfig.ID, serverIDs); err != nil {
		t.Fatalf("failed to bind MCP servers: %v", err)
	}

	tests := []struct {
		agentType model.BaseAgentType
		wantNames []string
	}{
		{"claude_code", []string{"legacy-claude-default", "shared-tools"}},
		{"open_code", []string{"open-code-only", "shared-tools"}},
		{"hermes", []string{"hermes-only", "shared-tools"}},
		{"open_claw", []string{"open-claw-only", "shared-tools"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.agentType), func(t *testing.T) {
			got := es.loadBoundMCPServers(ctx, agentConfig, &model.BaseAgent{Type: tt.agentType})
			gotNames := make([]string, 0, len(got))
			for _, server := range got {
				gotNames = append(gotNames, server.Name)
			}
			if !sameStringSet(gotNames, tt.wantNames) {
				t.Fatalf("unexpected MCP servers for %s: got %v want %v", tt.agentType, gotNames, tt.wantNames)
			}
		})
	}
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, item := range a {
		counts[item]++
	}
	for _, item := range b {
		counts[item]--
		if counts[item] < 0 {
			return false
		}
	}
	return true
}
