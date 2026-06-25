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

// @feature F017 - 知识库
// @priority P2
// @id API-02-45
func TestKnowledgeHandler_CRUDLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	createW := performJSON(f.router, http.MethodPost, "/api/v1/knowledge", map[string]any{
		"name":          "api-docs",
		"displayName":   "API Docs",
		"description":   "Knowledge for API contracts",
		"type":          "git",
		"config":        map[string]string{"path": "/tmp/docs"},
		"queryEndpoint": "",
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.KnowledgeBase
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	assert.Equal(t, "api-docs", created.Name)
	assert.Equal(t, model.KnowledgeBaseStatusActive, created.Status)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/knowledge?search=api&type=git&status=active&page=1&size=5", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), `"total":1`)
	assert.Contains(t, listW.Body.String(), "API Docs")

	getW := performJSON(f.router, http.MethodGet, "/api/v1/knowledge/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)
	assert.Contains(t, getW.Body.String(), "Knowledge for API contracts")

	updateW := performJSON(f.router, http.MethodPut, "/api/v1/knowledge/"+created.ID.String(), map[string]any{
		"displayName": "Updated API Docs",
		"description": "Updated knowledge",
		"status":      "inactive",
	})
	require.Equal(t, http.StatusOK, updateW.Code)
	assert.Contains(t, updateW.Body.String(), "Updated API Docs")
	assert.Contains(t, updateW.Body.String(), "inactive")

	deleteW := performJSON(f.router, http.MethodDelete, "/api/v1/knowledge/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, deleteW.Code)

	missingW := performJSON(f.router, http.MethodGet, "/api/v1/knowledge/"+created.ID.String(), nil)
	assert.Equal(t, http.StatusNotFound, missingW.Code)
}

// @feature F017 - 知识库
// @priority P2
// @id API-02-46
func TestKnowledgeHandler_RejectsInvalidRequests(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	createW := performJSON(f.router, http.MethodPost, "/api/v1/knowledge", map[string]any{
		"name": "missing-type",
	})
	assert.Equal(t, http.StatusBadRequest, createW.Code)

	invalidGetW := performJSON(f.router, http.MethodGet, "/api/v1/knowledge/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, invalidGetW.Code)

	invalidUpdateW := performJSON(f.router, http.MethodPut, "/api/v1/knowledge/not-a-uuid", map[string]any{
		"displayName": "bad",
	})
	assert.Equal(t, http.StatusBadRequest, invalidUpdateW.Code)

	invalidDeleteW := performJSON(f.router, http.MethodDelete, "/api/v1/knowledge/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, invalidDeleteW.Code)

	invalidQueryW := performJSON(f.router, http.MethodPost, "/api/v1/knowledge/not-a-uuid/query", map[string]any{
		"query": "hello",
	})
	assert.Equal(t, http.StatusBadRequest, invalidQueryW.Code)

	missingQueryAllW := performJSON(f.router, http.MethodPost, "/api/v1/knowledge/query", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, missingQueryAllW.Code)

	_ = uuid.Nil
}
