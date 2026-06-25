package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// @feature F005 - 线程管理
// @priority P1
// @id API-02-47
func TestProjectHandler_FileBrowsingAndContentLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)
	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "README.md"), []byte("# Project\ncontent"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "image.png"), []byte{0x89, 0x50, 0x4e, 0x47}, 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, "src"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "src", "main.go"), []byte("package main\n"), 0644))

	projectID := uuid.New()
	_, err := f.db.Exec(
		`INSERT INTO projects (id, name, type, mode, status, local_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		projectID.String(), "File Project", "service", "new", "draft", projectDir,
	)
	require.NoError(t, err)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/projects/"+projectID.String()+"/files", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), "README.md")
	assert.Contains(t, listW.Body.String(), "src")

	listByPathW := performJSON(f.router, http.MethodGet, "/api/v1/files?basePath="+projectDir+"&path=src", nil)
	require.Equal(t, http.StatusOK, listByPathW.Code)
	assert.Contains(t, listByPathW.Body.String(), "main.go")

	browseW := performJSON(f.router, http.MethodGet, "/api/v1/files/browse?path="+projectDir, nil)
	require.Equal(t, http.StatusOK, browseW.Code)
	assert.Contains(t, browseW.Body.String(), `"isValid":true`)
	assert.Contains(t, browseW.Body.String(), "src")

	validateW := performJSON(f.router, http.MethodGet, "/api/v1/files/validate?path="+projectDir, nil)
	require.Equal(t, http.StatusOK, validateW.Code)
	assert.Contains(t, validateW.Body.String(), `"isValid":true`)
	assert.Contains(t, validateW.Body.String(), `"exists":true`)

	contentW := performJSON(f.router, http.MethodGet, "/api/v1/files/content?basePath="+projectDir+"&path=README.md", nil)
	require.Equal(t, http.StatusOK, contentW.Code)
	assert.Contains(t, contentW.Body.String(), "# Project")
	assert.Contains(t, contentW.Body.String(), `"isBinary":false`)

	imageW := performJSON(f.router, http.MethodGet, "/api/v1/files/image?basePath="+projectDir+"&path=image.png", nil)
	require.Equal(t, http.StatusOK, imageW.Code)

	createFolderW := performJSON(f.router, http.MethodPost, "/api/v1/files/folder", map[string]any{
		"path": projectDir,
		"name": "generated",
	})
	require.Equal(t, http.StatusOK, createFolderW.Code)
	assert.DirExists(t, filepath.Join(projectDir, "generated"))
}

// @feature F005 - 线程管理
// @priority P1
// @id API-02-48
func TestProjectHandler_FileAPIsRejectInvalidRequests(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	invalidProjectW := performJSON(f.router, http.MethodGet, "/api/v1/projects/not-a-uuid/files", nil)
	assert.Equal(t, http.StatusBadRequest, invalidProjectW.Code)

	missingListBaseW := performJSON(f.router, http.MethodGet, "/api/v1/files", nil)
	assert.Equal(t, http.StatusBadRequest, missingListBaseW.Code)

	missingBaseW := performJSON(f.router, http.MethodGet, "/api/v1/files/content", nil)
	assert.Equal(t, http.StatusBadRequest, missingBaseW.Code)

	missingPathW := performJSON(f.router, http.MethodGet, "/api/v1/files/content?basePath="+t.TempDir(), nil)
	assert.Equal(t, http.StatusBadRequest, missingPathW.Code)

	missingImageBaseW := performJSON(f.router, http.MethodGet, "/api/v1/files/image", nil)
	assert.Equal(t, http.StatusBadRequest, missingImageBaseW.Code)

	folderMissingNameW := performJSON(f.router, http.MethodPost, "/api/v1/files/folder", map[string]any{
		"path": t.TempDir(),
	})
	assert.Equal(t, http.StatusBadRequest, folderMissingNameW.Code)
}
