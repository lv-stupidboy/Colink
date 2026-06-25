package api_test

import (
	"net/http"
	"testing"

	"github.com/anthropic/isdp/internal/api"
	agentservice "github.com/anthropic/isdp/internal/service/agent"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupInvocationRouter() *gin.Engine {
	return setupStandaloneRouter(func(group *gin.RouterGroup) {
		handler := api.NewInvocationHandler(nil, nil, nil)
		handler.RegisterRoutes(group)
		group.POST("/mcp/callback", handler.MCPCallback)
	})
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-29
func TestInvocationHandler_RejectsInvalidIDsBeforeOrchestration(t *testing.T) {
	router := setupInvocationRouter()

	spawnW := performJSON(router, http.MethodPost, "/api/v1/threads/not-a-uuid/invocations", map[string]any{
		"role":  "agent",
		"input": "run task",
	})
	assert.Equal(t, http.StatusBadRequest, spawnW.Code)

	listW := performJSON(router, http.MethodGet, "/api/v1/threads/not-a-uuid/invocations", nil)
	assert.Equal(t, http.StatusBadRequest, listW.Code)

	getW := performJSON(router, http.MethodGet, "/api/v1/invocations/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, getW.Code)

	cancelW := performJSON(router, http.MethodPost, "/api/v1/invocations/not-a-uuid/cancel", nil)
	assert.Equal(t, http.StatusBadRequest, cancelW.Code)
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-30
func TestInvocationHandler_SpawnValidationAndMCPCallback(t *testing.T) {
	router := setupInvocationRouter()
	threadID := uuid.New()

	missingBodyW := performJSON(router, http.MethodPost, "/api/v1/threads/"+threadID.String()+"/invocations", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, missingBodyW.Code)

	badConfigW := performJSON(router, http.MethodPost, "/api/v1/threads/"+threadID.String()+"/invocations", map[string]any{
		"configId": "not-a-uuid",
		"role":     "agent",
		"input":    "run task",
	})
	assert.Equal(t, http.StatusBadRequest, badConfigW.Code)

	callbackW := performJSON(router, http.MethodPost, "/api/v1/mcp/callback", map[string]any{
		"invocationId": uuid.New().String(),
		"status":       "completed",
	})
	require.Equal(t, http.StatusOK, callbackW.Code)
	assert.Contains(t, callbackW.Body.String(), `"status":"ok"`)
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-59
func TestInvocationHandler_ListRunningEmptyState(t *testing.T) {
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {
		handler := api.NewInvocationHandler(
			agentservice.NewOrchestrator(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, nil),
			nil,
			nil,
		)
		handler.RegisterRoutes(group)
	})

	w := performJSON(router, http.MethodGet, "/api/v1/invocations/running", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"instances":[]}`, w.Body.String())
}
