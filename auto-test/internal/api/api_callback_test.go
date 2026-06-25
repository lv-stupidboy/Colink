package api_test

import (
	"net/http"
	"testing"

	"github.com/anthropic/isdp/internal/api"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCallbackRouter() *gin.Engine {
	return setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewCallbackHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).RegisterRoutes(group)
	})
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-38
func TestCallbackHandler_PostMessageValidation(t *testing.T) {
	router := setupCallbackRouter()

	missingFieldsW := performJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{
		"content": "hello",
	})
	assert.Equal(t, http.StatusBadRequest, missingFieldsW.Code)

	invalidInvocationW := performJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{
		"invocationId":  "not-a-uuid",
		"callbackToken": "token",
		"content":       "hello",
	})
	assert.Equal(t, http.StatusBadRequest, invalidInvocationW.Code)
}

// @feature F014 - Agent 记忆
// @priority P2
// @id API-02-39
func TestCallbackHandler_MemoryAndTeamAgentsUnavailable(t *testing.T) {
	router := setupCallbackRouter()

	memoryW := performJSON(router, http.MethodPost, "/api/v1/callbacks/memory", map[string]any{
		"invocationId":  uuid.New().String(),
		"callbackToken": "token",
		"action":        "add",
	})
	require.Equal(t, http.StatusServiceUnavailable, memoryW.Code)
	assert.Contains(t, memoryW.Body.String(), "memory manager is not initialized")

	agentsW := performJSON(router, http.MethodPost, "/api/v1/callbacks/team/list-agents", map[string]any{
		"invocationId":  uuid.New().String(),
		"callbackToken": "token",
	})
	require.Equal(t, http.StatusServiceUnavailable, agentsW.Code)
	assert.Contains(t, agentsW.Body.String(), "memory manager is not initialized")
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-40
func TestCallbackHandler_QueryValidation(t *testing.T) {
	router := setupCallbackRouter()

	pendingMissingW := performJSON(router, http.MethodGet, "/api/v1/callbacks/pending-mentions", nil)
	assert.Equal(t, http.StatusBadRequest, pendingMissingW.Code)

	pendingInvalidW := performJSON(router, http.MethodGet, "/api/v1/callbacks/pending-mentions?invocationId=not-a-uuid&callbackToken=token", nil)
	assert.Equal(t, http.StatusBadRequest, pendingInvalidW.Code)

	contextMissingW := performJSON(router, http.MethodGet, "/api/v1/callbacks/thread-context", nil)
	assert.Equal(t, http.StatusBadRequest, contextMissingW.Code)

	contextInvalidW := performJSON(router, http.MethodGet, "/api/v1/callbacks/thread-context?invocationId=not-a-uuid&callbackToken=token", nil)
	assert.Equal(t, http.StatusBadRequest, contextInvalidW.Code)
}
