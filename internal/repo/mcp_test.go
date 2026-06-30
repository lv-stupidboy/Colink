package repo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestMCPRepositoriesServerLifecycleAndBindings(t *testing.T) {
	ctx := context.Background()
	db := openMCPRepoTestDB(t)
	serverRepo := NewMCPServerRepository(db, DBTypeSQLite)
	bindingRepo := NewAgentMCPBindingRepository(db, DBTypeSQLite)

	active := &model.MCPServer{
		ID:          uuid.New(),
		Name:        "github-tools",
		DisplayName: "GitHub Tools",
		Description: "GitHub MCP",
		Transport:   model.MCPTransportStdio,
		Command:     "npx",
		Args:        []string{"-y", "server"},
		Env:         map[string]string{"TOKEN": "value"},
		SourceType:  model.MCPSourcePersonal,
		Status:      model.MCPStatusActive,
		CreatedAt:   time.Now().Add(-time.Minute),
		UpdatedAt:   time.Now().Add(-time.Minute),
	}
	if err := serverRepo.Create(ctx, active); err != nil {
		t.Fatalf("Create active returned error: %v", err)
	}
	disabled := &model.MCPServer{
		ID:         uuid.New(),
		Name:       "wiki-tools",
		Transport:  model.MCPTransportHTTP,
		URL:        "http://127.0.0.1:9100/mcp",
		Headers:    map[string]string{"Authorization": "Bearer token"},
		SourceType: model.MCPSourceTeam,
		Status:     model.MCPStatusDisabled,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := serverRepo.Create(ctx, disabled); err != nil {
		t.Fatalf("Create disabled returned error: %v", err)
	}

	byID, err := serverRepo.FindByID(ctx, active.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if byID.Name != active.Name || byID.Args[1] != "server" || byID.Env["TOKEN"] != "value" {
		t.Fatalf("FindByID = %#v", byID)
	}
	byName, err := serverRepo.FindByName(ctx, "wiki-tools")
	if err != nil {
		t.Fatalf("FindByName returned error: %v", err)
	}
	if byName.URL != disabled.URL || byName.Headers["Authorization"] == "" {
		t.Fatalf("FindByName = %#v", byName)
	}

	list, total, err := serverRepo.List(ctx, &model.MCPServerListQuery{Search: "tools", Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if total != 2 || len(list) != 1 {
		t.Fatalf("List page total=%d len=%d", total, len(list))
	}
	activeList, total, err := serverRepo.List(ctx, &model.MCPServerListQuery{Status: string(model.MCPStatusActive), PageSize: 200})
	if err != nil {
		t.Fatalf("List active returned error: %v", err)
	}
	if total != 1 || len(activeList) != 1 || activeList[0].Name != "github-tools" {
		t.Fatalf("List active total=%d list=%#v", total, activeList)
	}

	active.DisplayName = "GitHub Updated"
	active.Transport = model.MCPTransportSSE
	active.Command = ""
	active.URL = "http://127.0.0.1:9200/sse"
	active.Args = []string{"ignored"}
	active.Headers = map[string]string{"X-Test": "1"}
	if err := serverRepo.Update(ctx, active); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	updated, err := serverRepo.FindByID(ctx, active.ID)
	if err != nil {
		t.Fatalf("FindByID updated returned error: %v", err)
	}
	if updated.DisplayName != "GitHub Updated" || updated.Transport != model.MCPTransportSSE || updated.Headers["X-Test"] != "1" {
		t.Fatalf("updated = %#v", updated)
	}

	agentID := uuid.New()
	if err := bindingRepo.ReplaceBindings(ctx, agentID, []uuid.UUID{active.ID, disabled.ID}); err != nil {
		t.Fatalf("ReplaceBindings returned error: %v", err)
	}
	ids, err := bindingRepo.FindServerIDsByAgentRoleID(ctx, agentID)
	if err != nil {
		t.Fatalf("FindServerIDsByAgentRoleID returned error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("FindServerIDsByAgentRoleID = %#v", ids)
	}
	exists, err := bindingRepo.ExistsBinding(ctx, agentID, active.ID)
	if err != nil || !exists {
		t.Fatalf("ExistsBinding exists=%v err=%v", exists, err)
	}
	boundServers, err := bindingRepo.FindServersByAgentRoleID(ctx, agentID)
	if err != nil {
		t.Fatalf("FindServersByAgentRoleID returned error: %v", err)
	}
	if len(boundServers) != 1 || boundServers[0].ID != active.ID {
		t.Fatalf("FindServersByAgentRoleID = %#v", boundServers)
	}
	if err := bindingRepo.ReplaceBindings(ctx, agentID, []uuid.UUID{active.ID}); err != nil {
		t.Fatalf("ReplaceBindings second returned error: %v", err)
	}
	ids, err = bindingRepo.FindServerIDsByAgentRoleID(ctx, agentID)
	if err != nil || len(ids) != 1 || ids[0] != active.ID {
		t.Fatalf("FindServerIDsByAgentRoleID after replace = %#v err=%v", ids, err)
	}

	if err := serverRepo.Delete(ctx, active.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := serverRepo.FindByID(ctx, active.ID); err == nil {
		t.Fatalf("FindByID should fail after delete")
	}
	if _, err := serverRepo.FindByName(ctx, "missing"); err == nil {
		t.Fatalf("FindByName should fail for missing server")
	}
}

func openMCPRepoTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE mcp_servers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			display_name TEXT,
			description TEXT,
			transport TEXT NOT NULL DEFAULT 'stdio',
			command TEXT,
			args BLOB NOT NULL DEFAULT '[]',
			env BLOB NOT NULL DEFAULT '{}',
			url TEXT,
			headers BLOB NOT NULL DEFAULT '{}',
			source_type TEXT NOT NULL DEFAULT 'personal',
			status TEXT NOT NULL DEFAULT 'active',
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		)`,
		`CREATE TABLE agent_mcp_bindings (
			id TEXT PRIMARY KEY,
			agent_role_id TEXT NOT NULL,
			mcp_server_id TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(agent_role_id, mcp_server_id)
		)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
}
