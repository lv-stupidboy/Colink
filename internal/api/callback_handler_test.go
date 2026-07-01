package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/message"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestCallbackHandlerPostMessageAuthStaleAndPersistence(t *testing.T) {
	db := openCallbackHandlerTestDB(t)
	msgRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	invocationRepo := repo.NewAgentInvocationRepository(db, repo.DBTypeSQLite)
	handler := NewCallbackHandler(
		nil,
		message.NewService(msgRepo, nil),
		msgRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		invocationRepo,
		nil,
		nil,
		nil,
		nil,
	)
	router := setupAPILightRouter(handler.RegisterRoutes)

	threadID := uuid.New()
	invocationID := uuid.New()
	agentConfigID := uuid.New()
	token := "test-callback-token"
	now := time.Now()
	if err := invocationRepo.Create(context.Background(), &model.AgentInvocation{
		ID:            invocationID,
		ThreadID:      threadID,
		AgentConfigID: agentConfigID,
		Role:          model.AgentRoleAgent,
		AgentName:     "Planner",
		Status:        model.InvocationStatusRunning,
		CallbackToken: token,
		CreatedAt:     now,
		StartedAt:     &now,
	}); err != nil {
		t.Fatalf("create invocation: %v", err)
	}
	replyTo := uuid.New()
	okW := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{
		"invocationId":    invocationID.String(),
		"callbackToken":   token,
		"content":         "hello @coder",
		"replyTo":         replyTo.String(),
		"clientMessageId": "client-1",
		"targetCats":      []string{"coder", "reviewer"},
	})
	if okW.Code != http.StatusOK || !bytes.Contains(okW.Body.Bytes(), []byte(`"status":"ok"`)) || !bytes.Contains(okW.Body.Bytes(), []byte("client-1")) {
		t.Fatalf("PostMessage ok code=%d body=%s", okW.Code, okW.Body.String())
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM messages WHERE thread_id = ? AND agent_id = ? AND origin = ?`, threadID.String(), agentConfigID.String(), "callback").Scan(&count); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 1 {
		t.Fatalf("message count = %d", count)
	}
	var mentions []byte
	if err := db.QueryRow(`SELECT mentions FROM messages WHERE thread_id = ?`, threadID.String()).Scan(&mentions); err != nil {
		t.Fatalf("select mentions: %v", err)
	}
	if !json.Valid(mentions) || !bytes.Contains(mentions, []byte("coder")) || !bytes.Contains(mentions, []byte("reviewer")) {
		t.Fatalf("mentions = %s", mentions)
	}

	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing fields code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{"invocationId": "bad", "callbackToken": "x", "content": "x"}); w.Code != http.StatusBadRequest {
		t.Fatalf("bad invocation code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{"invocationId": invocationID.String(), "callbackToken": "wrong", "content": "x"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{"invocationId": invocationID.String(), "callbackToken": token, "content": "x", "threadId": "bad"}); w.Code != http.StatusBadRequest {
		t.Fatalf("bad thread id code=%d", w.Code)
	}

	if _, err := msgRepo.FindByThreadID(context.Background(), threadID, 10); err != nil {
		t.Fatalf("message repo remains usable: %v", err)
	}
}

func TestCallbackHandlerUnavailableMemoryAndInvalidIdentity(t *testing.T) {
	handler := NewCallbackHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := setupAPILightRouter(handler.RegisterRoutes)

	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/memory", map[string]any{}); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("memory unavailable code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/team/list-agents", map[string]any{}); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("list agents unavailable code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/callbacks/pending-mentions?invocationId=bad&callbackToken=x", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("pending mentions bad request code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/callbacks/thread-context?invocationId=bad&callbackToken=x", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("thread context bad request code=%d", w.Code)
	}
}

func TestCallbackHandlerIdentityAndMemoryScopeHelpers(t *testing.T) {
	ctx := context.Background()
	db := openCallbackHandlerTestDB(t)
	projectRepo := repo.NewProjectRepository(db, repo.DBTypeSQLite)
	threadRepo := repo.NewThreadRepository(db, repo.DBTypeSQLite)
	workflowRepo := repo.NewWorkflowTemplateRepository(db, repo.DBTypeSQLite)
	invocationRepo := repo.NewAgentInvocationRepository(db, repo.DBTypeSQLite)
	handler := NewCallbackHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, invocationRepo, projectRepo, threadRepo, workflowRepo, nil)

	dbInvocationID := uuid.New()
	dbThreadID := uuid.New()
	dbAgentID := uuid.New()
	now := time.Now()
	if err := invocationRepo.Create(ctx, &model.AgentInvocation{
		ID:            dbInvocationID,
		ThreadID:      dbThreadID,
		AgentConfigID: dbAgentID,
		Role:          model.AgentRoleAgent,
		AgentName:     "DB Agent",
		Status:        model.InvocationStatusRunning,
		CallbackToken: "db-token",
		CreatedAt:     now,
	}); err != nil {
		t.Fatalf("create invocation: %v", err)
	}
	identity, ok := handler.verifyCallbackIdentity(newCallbackTestContext(http.MethodGet, "/", nil), dbInvocationID.String(), "db-token")
	if !ok || identity.ThreadID != dbThreadID || identity.AgentID != dbAgentID.String() {
		t.Fatalf("db identity = %#v ok=%v", identity, ok)
	}
	if _, ok := handler.verifyCallbackIdentity(newCallbackTestContext(http.MethodGet, "/", nil), "bad", "db-token"); ok {
		t.Fatalf("bad invocation id should not verify")
	}
	if _, ok := handler.verifyCallbackIdentity(newCallbackTestContext(http.MethodGet, "/", nil), dbInvocationID.String(), "wrong"); ok {
		t.Fatalf("wrong token should not verify")
	}

	workflowID := uuid.New()
	projectID := uuid.New()
	if err := workflowRepo.Create(ctx, &model.WorkflowTemplate{ID: workflowID, Name: "Runtime Team", Description: "runtime"}); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	if err := projectRepo.Create(ctx, &model.Project{
		ID:                 projectID,
		Name:               "Helios Project",
		Type:               model.ProjectTypeService,
		Mode:               model.ProjectModeNew,
		Status:             model.ProjectStatusDeveloping,
		LocalPath:          "/workspace/helios",
		WorkflowTemplateID: &workflowID,
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	scopedThreadID := uuid.New()
	if err := threadRepo.Create(ctx, &model.Thread{
		ID:                 scopedThreadID,
		ProjectID:          projectID,
		Name:               "Runtime thread",
		Status:             model.ThreadStatusRunning,
		CurrentPhase:       model.PhaseDevelopment,
		WorkflowTemplateID: &workflowID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}); err != nil {
		t.Fatalf("create thread: %v", err)
	}
	if got := handler.resolveWorkspacePath(ctx, scopedThreadID); got != "/workspace/helios" {
		t.Fatalf("resolveWorkspacePath = %q", got)
	}
	scope := handler.resolveMemoryScope(ctx, scopedThreadID)
	if scope.ProjectID != projectID.String() || scope.ProjectName != "Helios Project" || scope.WorkspacePath != "/workspace/helios" || scope.TeamID != workflowID.String() || scope.TeamName != "Runtime Team" {
		t.Fatalf("resolveMemoryScope = %#v", scope)
	}
	if got := handler.resolveWorkspacePath(ctx, uuid.Nil); got != "" {
		t.Fatalf("nil thread workspace = %q", got)
	}
}

func newCallbackTestContext(method, path string, headers map[string]string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	for key, value := range headers {
		c.Request.Header.Set(key, value)
	}
	return c
}

func openCallbackHandlerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE messages (id TEXT PRIMARY KEY, thread_id TEXT NOT NULL, role TEXT NOT NULL, agent_id TEXT, content TEXT, content_blocks BLOB, message_type TEXT, metadata BLOB, created_at TEXT NOT NULL, reported_at TIMESTAMP NULL, mentions BLOB, origin TEXT, reply_to TEXT)`); err != nil {
		t.Fatalf("create messages: %v", err)
	}
	schema := []string{
		`CREATE TABLE agent_invocations (id TEXT PRIMARY KEY, thread_id TEXT, agent_config_id TEXT, role TEXT, agent_name TEXT, status TEXT, input TEXT, full_prompt TEXT, output TEXT, started_at TIMESTAMP, completed_at TIMESTAMP, created_at TIMESTAMP, process_id TEXT, session_id TEXT, input_tokens INTEGER DEFAULT 0, output_tokens INTEGER DEFAULT 0, cache_read_tokens INTEGER DEFAULT 0, cache_creation_tokens INTEGER DEFAULT 0, cost_usd REAL DEFAULT 0, duration_ms INTEGER DEFAULT 0, duration_api_ms INTEGER DEFAULT 0, callback_token TEXT, triggered_by TEXT)`,
		`CREATE TABLE projects (id TEXT PRIMARY KEY, name TEXT, description TEXT, type TEXT, mode TEXT, status TEXT, local_path TEXT, git_repo TEXT, config BLOB, workflow_template_id TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE threads (id TEXT PRIMARY KEY, project_id TEXT, name TEXT, status TEXT, current_phase TEXT, current_agent TEXT, depth INTEGER, abort_token TEXT, workflow_template_id TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE workflow_templates (id TEXT PRIMARY KEY, name TEXT, description TEXT, agent_ids BLOB, transitions BLOB, checkpoints BLOB, estimated_time TEXT, is_system INTEGER, is_default INTEGER, routable_teams BLOB, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create callback helper table: %v", err)
		}
	}
	return db
}
