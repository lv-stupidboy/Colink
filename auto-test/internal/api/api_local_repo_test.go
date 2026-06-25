package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropic/isdp/auto-test/internal/testutil"
	"github.com/anthropic/isdp/internal/api"
	"github.com/anthropic/isdp/internal/repo"
	localreposervice "github.com/anthropic/isdp/internal/service/local_repo"
	"github.com/anthropic/isdp/internal/service/workspace"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupLocalRepoRouter() *gin.Engine {
	return setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewLocalRepoHandler(nil).RegisterRoutes(group)
	})
}

// @feature F013 - 本地代码仓
// @priority P2
// @id API-02-31
func TestLocalRepoHandler_RejectsInvalidIDs(t *testing.T) {
	router := setupLocalRepoRouter()

	getW := performJSON(router, http.MethodGet, "/api/v1/repos/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, getW.Code)

	deleteW := performJSON(router, http.MethodDelete, "/api/v1/repos/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, deleteW.Code)

	syncW := performJSON(router, http.MethodPost, "/api/v1/repos/not-a-uuid/sync", nil)
	assert.Equal(t, http.StatusBadRequest, syncW.Code)

	configureW := performJSON(router, http.MethodPut, "/api/v1/repos/not-a-uuid/git-config", map[string]any{
		"gitUrl": "git@github.com:example/repo.git",
	})
	assert.Equal(t, http.StatusBadRequest, configureW.Code)
}

// @feature F013 - 本地代码仓
// @priority P2
// @id API-02-32
func TestLocalRepoHandler_RejectsMalformedRequests(t *testing.T) {
	router := setupLocalRepoRouter()

	uploadW := performJSON(router, http.MethodPost, "/api/v1/repos/upload", nil)
	assert.Equal(t, http.StatusBadRequest, uploadW.Code)

	cloneW := performJSON(router, http.MethodPost, "/api/v1/repos/clone", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, cloneW.Code)

	branchesW := performJSON(router, http.MethodPost, "/api/v1/repos/remote-branches", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, branchesW.Code)

	folderW := performJSON(router, http.MethodPost, "/api/v1/repos/folder", map[string]any{
		"path": "/tmp",
	})
	assert.Equal(t, http.StatusBadRequest, folderW.Code)
}

// @feature F013 - 本地代码仓
// @priority P2
// @id API-02-53
func TestLocalRepoHandler_UploadBrowseConfigureAndDeleteLifecycle(t *testing.T) {
	db, _, err := testutil.SetupTestDB()
	require.NoError(t, err)
	t.Cleanup(func() { testutil.CleanupTestDB(db) })

	workspaceRoot := t.TempDir()
	guard, err := workspace.NewGuard(workspaceRoot)
	require.NoError(t, err)
	svc := localreposervice.NewService(
		repo.NewLocalRepoRepository(db, repo.DBTypeSQLite),
		guard,
		&config.GitURLConversionConfig{},
	)
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewLocalRepoHandler(svc).RegisterRoutes(group)
	})

	uploadDir := filepath.Join(workspaceRoot, "uploads")
	require.NoError(t, os.MkdirAll(uploadDir, 0755))
	zipBytes := makeZip(t, map[string]string{
		"README.md": "repo readme",
		"src/app.go": "package main\n",
	})
	uploadW := performMultipart(router, "/api/v1/repos/upload", map[string]string{
		"name":       "uploaded-repo",
		"targetPath": uploadDir,
	}, "file", "repo.zip", zipBytes)
	require.Equal(t, http.StatusCreated, uploadW.Code)
	assert.Contains(t, uploadW.Body.String(), "uploaded-repo")

	listW := performJSON(router, http.MethodGet, "/api/v1/repos", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), "uploaded-repo")

	var repoID string
	require.NoError(t, db.QueryRow(`SELECT id FROM local_repos WHERE name = ?`, "uploaded-repo").Scan(&repoID))

	getW := performJSON(router, http.MethodGet, "/api/v1/repos/"+repoID, nil)
	require.Equal(t, http.StatusOK, getW.Code)
	assert.Contains(t, getW.Body.String(), filepath.Join(uploadDir, "uploaded-repo"))

	browseW := performJSON(router, http.MethodGet, "/api/v1/repos/browse?path="+uploadDir, nil)
	require.Equal(t, http.StatusOK, browseW.Code)
	assert.Contains(t, browseW.Body.String(), "uploaded-repo")

	folderW := performJSON(router, http.MethodPost, "/api/v1/repos/folder", map[string]any{
		"path": uploadDir,
		"name": "new-folder",
	})
	require.Equal(t, http.StatusOK, folderW.Code)
	assert.DirExists(t, filepath.Join(uploadDir, "new-folder"))

	configureW := performJSON(router, http.MethodPut, "/api/v1/repos/"+repoID+"/git-config", map[string]any{
		"gitUrl": "https://github.com/example/repo",
		"branch": "main",
	})
	assert.Equal(t, http.StatusInternalServerError, configureW.Code)

	deleteW := performJSON(router, http.MethodDelete, "/api/v1/repos/"+repoID, nil)
	require.Equal(t, http.StatusNoContent, deleteW.Code)
	assert.NoDirExists(t, filepath.Join(uploadDir, "uploaded-repo"))

	_, err = uuid.Parse(repoID)
	require.NoError(t, err)
}
