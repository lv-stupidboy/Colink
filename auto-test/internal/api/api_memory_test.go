package api_test

import (
	"net/http"
	"testing"

	"github.com/anthropic/isdp/internal/api"
	"github.com/anthropic/isdp/internal/service/memory"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// @feature F014 - Agent 记忆
// @priority P2
// @id API-02-34
func TestMemoryHandler_RawUnavailable(t *testing.T) {
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewMemoryHandler(nil).RegisterRoutes(group)
	})

	w := performJSON(router, http.MethodGet, "/api/v1/memory/raw", nil)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "memory manager is not initialized")
}

// @feature F014 - Agent 记忆
// @priority P2
// @id API-02-35
func TestMemoryHandler_RawScopeAndTypeValidation(t *testing.T) {
	manager := memory.NewMemoryManagerWithTeamPath(nil, t.TempDir())
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewMemoryHandler(manager).RegisterRoutes(group)
	})

	teamW := performJSON(router, http.MethodGet, "/api/v1/memory/raw?type=team&teamId=t1&teamName=Core&projectId=p1&projectName=Colink&workspacePath=/tmp/work", nil)
	require.Equal(t, http.StatusOK, teamW.Code)
	assert.Contains(t, teamW.Body.String(), `"teamId":"t1"`)
	assert.Contains(t, teamW.Body.String(), `"teamName":"Core"`)
	assert.Contains(t, teamW.Body.String(), `"projectName":"Colink"`)

	invalidTypeW := performJSON(router, http.MethodGet, "/api/v1/memory/raw?type=invalid", nil)
	assert.Equal(t, http.StatusBadRequest, invalidTypeW.Code)
}
