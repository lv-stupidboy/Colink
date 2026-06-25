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

// @feature F011 - 技能市场
// @priority P2
// @id API-02-25
func TestRegistryHandler_CRUDLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	createW := performJSON(f.router, http.MethodPost, "/api/v1/registries", map[string]any{
		"name":         "team-skills",
		"displayName":  "Team Skills",
		"type":         "github",
		"url":          "https://github.com/example/team-skills",
		"authConfig":   map[string]string{"token": "secret"},
		"syncInterval": 60,
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.SkillRegistry
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	assert.Equal(t, "team-skills", created.Name)
	assert.Equal(t, model.RegistryStatusActive, created.Status)
	assert.Equal(t, model.RegistrySyncPending, created.SyncStatus)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/registries?search=team&type=github&status=active&page=1&size=5", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), `"total":1`)
	assert.Contains(t, listW.Body.String(), "Team Skills")

	getW := performJSON(f.router, http.MethodGet, "/api/v1/registries/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)
	assert.Contains(t, getW.Body.String(), "team-skills")

	updateW := performJSON(f.router, http.MethodPut, "/api/v1/registries/"+created.ID.String(), map[string]any{
		"displayName":  "Updated Team Skills",
		"url":          "https://github.com/example/updated-skills",
		"syncInterval": 120,
		"status":       "inactive",
	})
	require.Equal(t, http.StatusOK, updateW.Code)
	assert.Contains(t, updateW.Body.String(), "Updated Team Skills")
	assert.Contains(t, updateW.Body.String(), "inactive")

	deleteW := performJSON(f.router, http.MethodDelete, "/api/v1/registries/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, deleteW.Code)

	missingW := performJSON(f.router, http.MethodGet, "/api/v1/registries/"+created.ID.String(), nil)
	assert.Equal(t, http.StatusNotFound, missingW.Code)
}

// @feature F011 - 技能市场
// @priority P2
// @id API-02-26
func TestRegistryHandler_RejectsInvalidRequests(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	missingFieldsW := performJSON(f.router, http.MethodPost, "/api/v1/registries", map[string]any{
		"name": "missing-url",
	})
	assert.Equal(t, http.StatusBadRequest, missingFieldsW.Code)

	invalidGetW := performJSON(f.router, http.MethodGet, "/api/v1/registries/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, invalidGetW.Code)

	invalidUpdateW := performJSON(f.router, http.MethodPut, "/api/v1/registries/not-a-uuid", map[string]any{
		"displayName": "bad",
	})
	assert.Equal(t, http.StatusBadRequest, invalidUpdateW.Code)

	invalidDeleteW := performJSON(f.router, http.MethodDelete, "/api/v1/registries/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, invalidDeleteW.Code)

	invalidSyncW := performJSON(f.router, http.MethodPost, "/api/v1/registries/not-a-uuid/sync", nil)
	assert.Equal(t, http.StatusBadRequest, invalidSyncW.Code)

	invalidPreviewW := performJSON(f.router, http.MethodPost, "/api/v1/registries/not-a-uuid/sync-preview", nil)
	assert.Equal(t, http.StatusBadRequest, invalidPreviewW.Code)

	invalidConfirmW := performJSON(f.router, http.MethodPost, "/api/v1/registries/not-a-uuid/sync-confirm", map[string]any{
		"registryId": uuid.New().String(),
	})
	assert.Equal(t, http.StatusBadRequest, invalidConfirmW.Code)
}
