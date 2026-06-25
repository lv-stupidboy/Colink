package api_test

import (
	"net/http"
	"testing"

	"github.com/anthropic/isdp/internal/api"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// @feature F016 - 团队包
// @priority P2
// @id API-02-43
func TestTeamPackageHandler_RejectsMalformedRequests(t *testing.T) {
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewTeamPackageHandler(nil).RegisterRoutes(group)
	})

	importW := performJSON(router, http.MethodPost, "/api/v1/team-packages/import", nil)
	assert.Equal(t, http.StatusBadRequest, importW.Code)

	confirmW := performJSON(router, http.MethodPost, "/api/v1/team-packages/import/confirm", nil)
	assert.Equal(t, http.StatusBadRequest, confirmW.Code)

	exportW := performJSON(router, http.MethodPost, "/api/v1/team-packages/export", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, exportW.Code)
}

// @feature F016 - 团队包
// @priority P2
// @id API-02-44
func TestTeamPackageSyncHandler_RejectsMalformedRequests(t *testing.T) {
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewTeamPackageSyncHandler(nil, zap.NewNop()).RegisterRoutes(group)
	})

	previewW := performJSON(router, http.MethodPost, "/api/v1/team-package-sync/preview", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, previewW.Code)

	syncW := performJSON(router, http.MethodPost, "/api/v1/team-package-sync/sync", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, syncW.Code)

	previewBatchW := performJSON(router, http.MethodPost, "/api/v1/team-package-sync/preview-batch", nil)
	assert.Equal(t, http.StatusBadRequest, previewBatchW.Code)

	syncBatchW := performJSON(router, http.MethodPost, "/api/v1/team-package-sync/sync-batch", nil)
	assert.Equal(t, http.StatusBadRequest, syncBatchW.Code)
}
