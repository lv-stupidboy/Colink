package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-18
func TestBaseAgentHandler_ListAndConnectionErrors(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	createW := performJSON(f.router, http.MethodPost, "/api/v1/base-agents", map[string]any{
		"name":         "Unsupported Runtime",
		"type":         "unknown_runtime",
		"defaultModel": "unknown-model",
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.BaseAgent
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))

	listW := performJSON(f.router, http.MethodGet, "/api/v1/base-agents", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), "Unsupported Runtime")
	assert.NotContains(t, listW.Body.String(), "secret-token")

	testW := performJSON(f.router, http.MethodPost, "/api/v1/base-agents/"+created.ID.String()+"/test", nil)
	require.Equal(t, http.StatusBadRequest, testW.Code)
	assert.Contains(t, testW.Body.String(), `"success":false`)

	invalidGetW := performJSON(f.router, http.MethodGet, "/api/v1/base-agents/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, invalidGetW.Code)

	missingGetW := performJSON(f.router, http.MethodGet, "/api/v1/base-agents/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, missingGetW.Code)
}

// @feature F005 - 线程管理
// @priority P1
// @id API-02-19
func TestThreadHandler_UpdateWorkflowAndRejectInvalidInputs(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	threadW := performJSON(f.router, http.MethodPost, "/api/v1/threads/project/"+f.projectID.String(), map[string]any{
		"name": "Thread update task",
	})
	require.Equal(t, http.StatusCreated, threadW.Code)

	var thread model.Thread
	require.NoError(t, json.Unmarshal(threadW.Body.Bytes(), &thread))

	updateW := performJSON(f.router, http.MethodPut, "/api/v1/threads/"+thread.ID.String(), map[string]any{
		"workflowTemplateId": f.workflowID.String(),
	})
	require.Equal(t, http.StatusOK, updateW.Code)
	assert.Contains(t, updateW.Body.String(), f.workflowID.String())

	invalidCreateW := performJSON(f.router, http.MethodPost, "/api/v1/threads/project/not-a-uuid", map[string]any{
		"name": "bad",
	})
	assert.Equal(t, http.StatusBadRequest, invalidCreateW.Code)

	invalidStatusW := performJSON(f.router, http.MethodPut, "/api/v1/threads/not-a-uuid/status", map[string]any{
		"status": "running",
	})
	assert.Equal(t, http.StatusBadRequest, invalidStatusW.Code)

	missingStatusW := performJSON(f.router, http.MethodPut, "/api/v1/threads/"+thread.ID.String()+"/status", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, missingStatusW.Code)

	invalidPhaseW := performJSON(f.router, http.MethodPut, "/api/v1/threads/not-a-uuid/phase", map[string]any{
		"phase": "review",
	})
	assert.Equal(t, http.StatusBadRequest, invalidPhaseW.Code)
}

// @feature F005 - 线程管理
// @priority P1
// @id API-02-20
func TestArtifactHandler_RejectsInvalidRequests(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	invalidListW := performJSON(f.router, http.MethodGet, "/api/v1/threads/not-a-uuid/artifacts", nil)
	assert.Equal(t, http.StatusBadRequest, invalidListW.Code)

	invalidCreateW := performJSON(f.router, http.MethodPost, "/api/v1/threads/not-a-uuid/artifacts", map[string]any{
		"type": "document",
	})
	assert.Equal(t, http.StatusBadRequest, invalidCreateW.Code)

	missingTypeW := performJSON(f.router, http.MethodPost, "/api/v1/threads/"+uuid.New().String()+"/artifacts", map[string]any{
		"name": "Untyped",
	})
	assert.Equal(t, http.StatusBadRequest, missingTypeW.Code)

	invalidGetW := performJSON(f.router, http.MethodGet, "/api/v1/artifacts/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, invalidGetW.Code)

	missingGetW := performJSON(f.router, http.MethodGet, "/api/v1/artifacts/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, missingGetW.Code)
}
