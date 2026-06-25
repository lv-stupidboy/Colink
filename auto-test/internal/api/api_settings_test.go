package api_test

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertAPISettings(t *testing.T, db *sql.DB, name, dir string) uuid.UUID {
	t.Helper()

	settingsID := uuid.New()
	_, err := db.Exec(
		`INSERT INTO settings (id, name, description, directory_path, supported_agents, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		settingsID.String(),
		name,
		"Shared runtime settings",
		dir,
		`["claude_code"]`,
		time.Now().Format("2006-01-02 15:04:05"),
		time.Now().Format("2006-01-02 15:04:05"),
	)
	require.NoError(t, err)
	return settingsID
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-23
func TestSettingsHandler_CRUDBindAndReadLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)
	settingsDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "config.json"), []byte(`{"model":"test"}`), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(settingsDir, "nested"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "nested", "notes.md"), []byte("# Notes"), 0644))
	settingsID := insertAPISettings(t, f.db, "runtime-settings", settingsDir)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/settings?search=runtime&page=1&page_size=5", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), `"total":1`)
	assert.Contains(t, listW.Body.String(), "runtime-settings")

	getW := performJSON(f.router, http.MethodGet, "/api/v1/settings/"+settingsID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)
	assert.Contains(t, getW.Body.String(), "Shared runtime settings")

	updateW := performJSON(f.router, http.MethodPut, "/api/v1/settings/"+settingsID.String(), map[string]any{
		"description": "Updated runtime settings",
	})
	require.Equal(t, http.StatusOK, updateW.Code)
	assert.Contains(t, updateW.Body.String(), "Updated runtime settings")

	directoryW := performJSON(f.router, http.MethodGet, "/api/v1/settings/"+settingsID.String()+"/directory", nil)
	require.Equal(t, http.StatusOK, directoryW.Code)
	assert.Contains(t, directoryW.Body.String(), "config.json")
	assert.Contains(t, directoryW.Body.String(), "nested")

	fileW := performJSON(f.router, http.MethodGet, "/api/v1/settings/"+settingsID.String()+"/file?path=config.json", nil)
	require.Equal(t, http.StatusOK, fileW.Code)
	assert.Equal(t, "application/json", fileW.Header().Get("Content-Type"))
	assert.Contains(t, fileW.Body.String(), `"model":"test"`)

	agentID := insertAPITestAgent(t, f.db, "Settings Owner")
	bindW := performJSON(f.router, http.MethodPost, "/api/v1/agent-roles/"+agentID.String()+"/settings", map[string]any{
		"settingsIds": []string{settingsID.String()},
	})
	require.Equal(t, http.StatusNoContent, bindW.Code)

	agentSettingsW := performJSON(f.router, http.MethodGet, "/api/v1/agent-roles/"+agentID.String()+"/settings", nil)
	require.Equal(t, http.StatusOK, agentSettingsW.Code)
	assert.Contains(t, agentSettingsW.Body.String(), `"count":1`)
	assert.Contains(t, agentSettingsW.Body.String(), "runtime-settings")

	boundAgentsW := performJSON(f.router, http.MethodGet, "/api/v1/settings/"+settingsID.String()+"/agents", nil)
	require.Equal(t, http.StatusOK, boundAgentsW.Code)
	assert.Contains(t, boundAgentsW.Body.String(), `"count":1`)
	assert.Contains(t, boundAgentsW.Body.String(), "Settings Owner")

	deleteWhileBoundW := performJSON(f.router, http.MethodDelete, "/api/v1/settings/"+settingsID.String(), nil)
	assert.Equal(t, http.StatusInternalServerError, deleteWhileBoundW.Code)

	unbindW := performJSON(f.router, http.MethodDelete, "/api/v1/agent-roles/"+agentID.String()+"/settings/"+settingsID.String(), nil)
	require.Equal(t, http.StatusNoContent, unbindW.Code)

	deleteW := performJSON(f.router, http.MethodDelete, "/api/v1/settings/"+settingsID.String(), nil)
	require.Equal(t, http.StatusNoContent, deleteW.Code)
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-24
func TestSettingsHandler_RejectsInvalidRequests(t *testing.T) {
	f := setupAPISurfaceFixture(t)
	settingsID := insertAPISettings(t, f.db, "invalid-branches", t.TempDir())

	invalidGetW := performJSON(f.router, http.MethodGet, "/api/v1/settings/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, invalidGetW.Code)

	missingGetW := performJSON(f.router, http.MethodGet, "/api/v1/settings/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, missingGetW.Code)

	invalidUpdateW := performJSON(f.router, http.MethodPut, "/api/v1/settings/not-a-uuid", map[string]any{
		"description": "bad",
	})
	assert.Equal(t, http.StatusBadRequest, invalidUpdateW.Code)

	missingPathW := performJSON(f.router, http.MethodGet, "/api/v1/settings/"+settingsID.String()+"/file", nil)
	assert.Equal(t, http.StatusBadRequest, missingPathW.Code)

	invalidBindW := performJSON(f.router, http.MethodPost, "/api/v1/agent-roles/not-a-uuid/settings", map[string]any{
		"settingsIds": []string{settingsID.String()},
	})
	assert.Equal(t, http.StatusBadRequest, invalidBindW.Code)

	invalidUnbindW := performJSON(f.router, http.MethodDelete, "/api/v1/agent-roles/not-a-uuid/settings/"+settingsID.String(), nil)
	assert.Equal(t, http.StatusBadRequest, invalidUnbindW.Code)

	createMissingNameW := performJSON(f.router, http.MethodPost, "/api/v1/settings", nil)
	assert.Equal(t, http.StatusBadRequest, createMissingNameW.Code)
}

var _ = model.Settings{}
