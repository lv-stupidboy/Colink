package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	agentservice "github.com/anthropic/isdp/internal/service/agent"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestInvocationHandlerListGetRunningAndMCPCallback(t *testing.T) {
	ctx := context.Background()
	db := openInvocationHandlerTestDB(t)
	invocationRepo := repo.NewAgentInvocationRepository(db, repo.DBTypeSQLite)
	orch := agentservice.NewOrchestrator(invocationRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, nil)
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		NewInvocationHandler(orch, nil, nil).RegisterRoutes(group)
	})

	threadID := uuid.New()
	invocationID := uuid.New()
	startedAt := time.Now().Add(-time.Minute).Truncate(time.Second)
	invocation := &model.AgentInvocation{
		ID:            invocationID,
		ThreadID:      threadID,
		AgentConfigID: uuid.New(),
		Role:          model.AgentRoleReviewer,
		AgentName:     "Reviewer",
		Status:        model.InvocationStatusRunning,
		Input:         "review this",
		Output:        "working",
		StartedAt:     &startedAt,
		CreatedAt:     startedAt,
	}
	if err := invocationRepo.Create(ctx, invocation); err != nil {
		t.Fatalf("Create invocation returned error: %v", err)
	}

	list := performAPILightJSON(router, http.MethodGet, "/api/v1/threads/"+threadID.String()+"/invocations", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("ListByThread code=%d body=%s", list.Code, list.Body.String())
	}
	var listed []model.AgentInvocation
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal ListByThread: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != invocationID || listed[0].Status != model.InvocationStatusRunning {
		t.Fatalf("ListByThread = %#v", listed)
	}

	get := performAPILightJSON(router, http.MethodGet, "/api/v1/invocations/"+invocationID.String(), nil)
	if get.Code != http.StatusOK {
		t.Fatalf("Get code=%d body=%s", get.Code, get.Body.String())
	}
	var got model.AgentInvocation
	if err := json.Unmarshal(get.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal Get: %v", err)
	}
	if got.ID != invocationID || got.AgentName != "Reviewer" {
		t.Fatalf("Get = %#v", got)
	}

	running := performAPILightJSON(router, http.MethodGet, "/api/v1/invocations/running", nil)
	if running.Code != http.StatusOK {
		t.Fatalf("ListRunning code=%d body=%s", running.Code, running.Body.String())
	}

	callback := performAPILightJSON(router, http.MethodPost, "/api/v1/invocations/"+invocationID.String()+"/callback", map[string]any{"action": "done"})
	if callback.Code != http.StatusNotFound {
		t.Fatalf("unregistered callback route should be 404, got code=%d body=%s", callback.Code, callback.Body.String())
	}
	w := performAPILightJSON(router, http.MethodPost, "/api/v1/threads/"+threadID.String()+"/invocations/callback", map[string]any{"action": "done"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("thread callback route should be unregistered, got code=%d body=%s", w.Code, w.Body.String())
	}

	handler := NewInvocationHandler(orch, nil, nil)
	mcpRouter := setupAPILightRouter(func(group *gin.RouterGroup) {
		group.POST("/mcp-callback", handler.MCPCallback)
	})
	mcp := performAPILightJSON(mcpRouter, http.MethodPost, "/api/v1/mcp-callback", map[string]any{"action": "done"})
	if mcp.Code != http.StatusOK {
		t.Fatalf("MCPCallback code=%d body=%s", mcp.Code, mcp.Body.String())
	}
	badMCP := performAPILightJSON(mcpRouter, http.MethodPost, "/api/v1/mcp-callback", "{bad")
	if badMCP.Code != http.StatusBadRequest {
		t.Fatalf("bad MCPCallback code=%d body=%s", badMCP.Code, badMCP.Body.String())
	}

	missing := performAPILightJSON(router, http.MethodGet, "/api/v1/invocations/"+uuid.New().String(), nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing Get code=%d body=%s", missing.Code, missing.Body.String())
	}
}

func openInvocationHandlerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE agent_invocations (
		id TEXT PRIMARY KEY,
		thread_id TEXT,
		agent_config_id TEXT,
		role TEXT,
		agent_name TEXT,
		status TEXT,
		input TEXT,
		full_prompt TEXT,
		output TEXT,
		started_at TIMESTAMP,
		completed_at TIMESTAMP,
		created_at TIMESTAMP,
		process_id TEXT,
		session_id TEXT,
		input_tokens INTEGER DEFAULT 0,
		output_tokens INTEGER DEFAULT 0,
		cache_read_tokens INTEGER DEFAULT 0,
		cache_creation_tokens INTEGER DEFAULT 0,
		cost_usd REAL DEFAULT 0,
		duration_ms INTEGER DEFAULT 0,
		duration_api_ms INTEGER DEFAULT 0,
		callback_token TEXT,
		triggered_by TEXT
	)`)
	if err != nil {
		t.Fatalf("exec schema: %v", err)
	}
	return db
}
