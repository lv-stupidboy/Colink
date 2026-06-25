package api_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/anthropic/isdp/auto-test/internal/testutil"
	"github.com/anthropic/isdp/internal/api"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/a2a"
	"github.com/anthropic/isdp/internal/service/memory"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-49
func TestCallbackHandler_PostMessagePendingMentionsAndThreadContext(t *testing.T) {
	f := setupAPISurfaceFixture(t)
	registry := a2a.NewInvocationRegistry()
	msgRepo := repo.NewMessageRepository(f.db, repo.DBTypeSQLite)
	api.NewCallbackHandler(registry, nil, nil, msgRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).RegisterRoutes(f.router.Group("/api/v1"))

	threadID := uuid.New()
	invocationID := uuid.New()
	record := &a2a.InvocationRecord{
		ID:       invocationID,
		ThreadID: threadID,
		CatID:    "agent-a",
		UserID:   "user-1",
	}
	token, err := registry.Register(record)
	require.NoError(t, err)

	_, err = f.db.Exec(
		`INSERT INTO messages (id, thread_id, role, agent_id, content, created_at, mentions, origin)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(),
		threadID.String(),
		"user",
		"",
		"please review this",
		time.Now().Format("2006-01-02 15:04:05"),
		`["agent-a"]`,
		"user",
	)
	require.NoError(t, err)

	postW := performJSON(f.router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{
		"invocationId":    invocationID.String(),
		"callbackToken":   token,
		"content":         "callback response",
		"targetCats":      []string{"agent-b"},
		"clientMessageId": "client-1",
	})
	require.Equal(t, http.StatusOK, postW.Code)
	assert.Contains(t, postW.Body.String(), `"status":"ok"`)
	assert.Contains(t, postW.Body.String(), `"threadId":"`+threadID.String()+`"`)
	assert.Contains(t, postW.Body.String(), `"clientMessageId":"client-1"`)

	pendingW := performJSON(f.router, http.MethodGet, "/api/v1/callbacks/pending-mentions?invocationId="+invocationID.String()+"&callbackToken="+token, nil)
	require.Equal(t, http.StatusOK, pendingW.Code)
	assert.Contains(t, pendingW.Body.String(), "please review this")
	assert.Contains(t, pendingW.Body.String(), `"from":"user"`)

	contextW := performJSON(f.router, http.MethodGet, "/api/v1/callbacks/thread-context?invocationId="+invocationID.String()+"&callbackToken="+token+"&keyword=callback&catId=agent-a", nil)
	require.Equal(t, http.StatusOK, contextW.Code)
	assert.Contains(t, contextW.Body.String(), "callback response")
	assert.NotContains(t, contextW.Body.String(), "please review this")
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-50
func TestCallbackHandler_StaleInvocationIsIgnored(t *testing.T) {
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {})
	registry := a2a.NewInvocationRegistry()
	msgRepo := repo.NewMessageRepository(mustSetupCallbackDB(t), repo.DBTypeSQLite)
	api.NewCallbackHandler(registry, nil, nil, msgRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).RegisterRoutes(router.Group("/api/v1"))

	threadID := uuid.New()
	oldInvocationID := uuid.New()
	oldRecord := &a2a.InvocationRecord{ID: oldInvocationID, ThreadID: threadID, CatID: "agent-a"}
	oldToken, err := registry.Register(oldRecord)
	require.NoError(t, err)
	_, err = registry.Register(&a2a.InvocationRecord{ID: uuid.New(), ThreadID: threadID, CatID: "agent-b"})
	require.NoError(t, err)

	w := performJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{
		"invocationId":  oldInvocationID.String(),
		"callbackToken": oldToken,
		"content":       "late response",
	})
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"stale_ignored"`)
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-56
func TestCallbackHandler_MemoryAndTeamAgentsLifecycle(t *testing.T) {
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {})
	registry := a2a.NewInvocationRegistry()
	manager := memory.NewMemoryManagerWithTeamPath(nil, t.TempDir())
	api.NewCallbackHandler(registry, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, manager).RegisterRoutes(router.Group("/api/v1"))

	threadID := uuid.New()
	invocationID := uuid.New()
	token, err := registry.Register(&a2a.InvocationRecord{
		ID:       invocationID,
		ThreadID: threadID,
		CatID:    "memory-agent",
	})
	require.NoError(t, err)
	workspacePath := t.TempDir()

	addW := performJSON(router, http.MethodPost, "/api/v1/callbacks/memory", map[string]any{
		"invocationId":  invocationID.String(),
		"callbackToken": token,
		"action":        "add",
		"type":          "project",
		"workspacePath": workspacePath,
		"content":       "Callback memory keeps API contracts visible.",
		"topic":         "callback-memory",
		"facts":         []string{"memory callback accepts valid invocation credentials"},
	})
	require.Equal(t, http.StatusOK, addW.Code)
	assert.Contains(t, addW.Body.String(), `"success":true`)

	searchW := performJSON(router, http.MethodPost, "/api/v1/callbacks/memory", map[string]any{
		"invocationId":  invocationID.String(),
		"callbackToken": token,
		"action":        "search",
		"type":          "project",
		"workspacePath": workspacePath,
		"query":         "API contracts",
	})
	require.Equal(t, http.StatusOK, searchW.Code)
	assert.Contains(t, searchW.Body.String(), `"success":true`)

	agentsW := performJSON(router, http.MethodPost, "/api/v1/callbacks/team/list-agents", map[string]any{
		"invocationId":  invocationID.String(),
		"callbackToken": token,
	})
	require.Equal(t, http.StatusOK, agentsW.Code)
	assert.Contains(t, agentsW.Body.String(), `"agents":[]`)
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-57
func TestCallbackHandler_MemoryRejectsUnavailableAndInvalidIdentity(t *testing.T) {
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {})
	api.NewCallbackHandler(a2a.NewInvocationRegistry(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).RegisterRoutes(router.Group("/api/v1"))

	unavailableMemoryW := performJSON(router, http.MethodPost, "/api/v1/callbacks/memory", map[string]any{
		"action": "search",
	})
	assert.Equal(t, http.StatusServiceUnavailable, unavailableMemoryW.Code)

	unavailableAgentsW := performJSON(router, http.MethodPost, "/api/v1/callbacks/team/list-agents", map[string]any{})
	assert.Equal(t, http.StatusServiceUnavailable, unavailableAgentsW.Code)

	memoryRouter := setupStandaloneRouter(func(group *gin.RouterGroup) {})
	api.NewCallbackHandler(a2a.NewInvocationRegistry(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, memory.NewMemoryManagerWithTeamPath(nil, t.TempDir())).RegisterRoutes(memoryRouter.Group("/api/v1"))

	badIdentityW := performJSON(memoryRouter, http.MethodPost, "/api/v1/callbacks/memory", map[string]any{
		"invocationId":  "not-a-uuid",
		"callbackToken": "bad-token",
		"action":        "search",
	})
	assert.Equal(t, http.StatusUnauthorized, badIdentityW.Code)

	badAgentsIdentityW := performJSON(memoryRouter, http.MethodPost, "/api/v1/callbacks/team/list-agents", map[string]any{
		"invocationId":  uuid.New().String(),
		"callbackToken": "bad-token",
	})
	assert.Equal(t, http.StatusUnauthorized, badAgentsIdentityW.Code)
}

func mustSetupCallbackDB(t *testing.T) *sql.DB {
	t.Helper()
	db, _, err := testutil.SetupTestDB()
	require.NoError(t, err)
	t.Cleanup(func() { testutil.CleanupTestDB(db) })
	return db
}
