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
// @id API-02-21
func TestSkillHandler_CRUDBindAndTagLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	createW := performJSON(f.router, http.MethodPost, "/api/v1/skills", map[string]any{
		"name":            "api-review",
		"description":     "Reviews API changes",
		"tags":            []string{"Go", "API设计"},
		"sourceType":      "personal",
		"supportedAgents": []string{"claude_code"},
		"isPublic":        false,
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.Skill
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	assert.Equal(t, "api-review", created.Name)
	assert.False(t, created.IsPublic)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/skills?page=1&page_size=5", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), `"total":1`)
	assert.Contains(t, listW.Body.String(), "api-review")

	getW := performJSON(f.router, http.MethodGet, "/api/v1/skills/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)
	assert.Contains(t, getW.Body.String(), "Reviews API changes")

	updateW := performJSON(f.router, http.MethodPut, "/api/v1/skills/"+created.ID.String(), map[string]any{
		"description":     "Reviews API compatibility",
		"tags":            []string{"Go", "REST API"},
		"supportedAgents": []string{"open_code"},
		"status":          "deprecated",
		"isPublic":        true,
	})
	require.Equal(t, http.StatusOK, updateW.Code)
	assert.Contains(t, updateW.Body.String(), "Reviews API compatibility")
	assert.Contains(t, updateW.Body.String(), "open_code")

	tagsW := performJSON(f.router, http.MethodGet, "/api/v1/skills/tags", nil)
	require.Equal(t, http.StatusOK, tagsW.Code)
	assert.Contains(t, tagsW.Body.String(), "REST API")

	builtInTagsW := performJSON(f.router, http.MethodGet, "/api/v1/skills/tags/builtin", nil)
	require.Equal(t, http.StatusOK, builtInTagsW.Code)
	assert.Contains(t, builtInTagsW.Body.String(), "编程语言")

	agentID := insertAPITestAgent(t, f.db, "Skill Owner")
	bindW := performJSON(f.router, http.MethodPost, "/api/v1/agent-skills/"+agentID.String(), map[string]any{
		"skillIds": []string{created.ID.String()},
	})
	require.Equal(t, http.StatusNoContent, bindW.Code)

	agentSkillsW := performJSON(f.router, http.MethodGet, "/api/v1/agent-skills/"+agentID.String(), nil)
	require.Equal(t, http.StatusOK, agentSkillsW.Code)
	assert.Contains(t, agentSkillsW.Body.String(), `"count":1`)
	assert.Contains(t, agentSkillsW.Body.String(), "api-review")

	boundAgentsW := performJSON(f.router, http.MethodGet, "/api/v1/skills/"+created.ID.String()+"/agents", nil)
	require.Equal(t, http.StatusOK, boundAgentsW.Code)
	assert.Contains(t, boundAgentsW.Body.String(), `"count":1`)
	assert.Contains(t, boundAgentsW.Body.String(), "Skill Owner")

	deleteWhileBoundW := performJSON(f.router, http.MethodDelete, "/api/v1/skills/"+created.ID.String(), nil)
	assert.Equal(t, http.StatusInternalServerError, deleteWhileBoundW.Code)

	unbindW := performJSON(f.router, http.MethodDelete, "/api/v1/agent-skills/"+agentID.String()+"/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, unbindW.Code)

	deleteW := performJSON(f.router, http.MethodDelete, "/api/v1/skills/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, deleteW.Code)
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-22
func TestSkillHandler_RejectsInvalidRequests(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	missingFieldsW := performJSON(f.router, http.MethodPost, "/api/v1/skills", map[string]any{
		"name": "missing-supported-agents",
	})
	assert.Equal(t, http.StatusBadRequest, missingFieldsW.Code)

	invalidGetW := performJSON(f.router, http.MethodGet, "/api/v1/skills/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, invalidGetW.Code)

	missingGetW := performJSON(f.router, http.MethodGet, "/api/v1/skills/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, missingGetW.Code)

	invalidBindW := performJSON(f.router, http.MethodPost, "/api/v1/agent-skills/not-a-uuid", map[string]any{
		"skillIds": []string{uuid.New().String()},
	})
	assert.Equal(t, http.StatusBadRequest, invalidBindW.Code)

	invalidUnbindW := performJSON(f.router, http.MethodDelete, "/api/v1/agent-skills/not-a-uuid/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusBadRequest, invalidUnbindW.Code)

	uploadW := performJSON(f.router, http.MethodPost, "/api/v1/skills/upload", nil)
	assert.Equal(t, http.StatusBadRequest, uploadW.Code)

	importRepoMissingW := performJSON(f.router, http.MethodPost, "/api/v1/skills/import/repo", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, importRepoMissingW.Code)

	importRepoInvalidW := performJSON(f.router, http.MethodPost, "/api/v1/skills/import/repo", map[string]any{
		"repoUrl": "https://example.com/not-supported/repo",
	})
	assert.Equal(t, http.StatusBadRequest, importRepoInvalidW.Code)

	federatedScanMissingW := performJSON(f.router, http.MethodPost, "/api/v1/skills/import/federated/scan", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, federatedScanMissingW.Code)

	federatedScanInvalidW := performJSON(f.router, http.MethodPost, "/api/v1/skills/import/federated/scan", map[string]any{
		"registryId": "not-a-uuid",
	})
	assert.Equal(t, http.StatusBadRequest, federatedScanInvalidW.Code)

	federatedImportMissingW := performJSON(f.router, http.MethodPost, "/api/v1/skills/import/federated", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, federatedImportMissingW.Code)
}
