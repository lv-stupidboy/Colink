package api_test

import (
	"net/http"
	"testing"

	"github.com/anthropic/isdp/internal/api"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func setupConfigGenRouter() *gin.Engine {
	return setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewConfigGenHandler(nil, nil, nil, nil).RegisterRoutes(group)
	})
}

// @feature F001 - Agent 对话核心
// @priority P1
// @id API-02-33
func TestConfigGenHandler_RejectsInvalidRequests(t *testing.T) {
	router := setupConfigGenRouter()

	syncMissingTypeW := performJSON(router, http.MethodPost, "/api/v1/projects/"+uuid.New().String()+"/config/sync", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, syncMissingTypeW.Code)

	syncUnsupportedTypeW := performJSON(router, http.MethodPost, "/api/v1/projects/"+uuid.New().String()+"/config/sync", map[string]any{
		"baseAgentType": "unsupported-agent",
	})
	assert.Equal(t, http.StatusBadRequest, syncUnsupportedTypeW.Code)

	generateInvalidIDW := performJSON(router, http.MethodPost, "/api/v1/agents/not-a-uuid/config/generate", map[string]any{
		"baseAgentType": "claude_code",
	})
	assert.Equal(t, http.StatusBadRequest, generateInvalidIDW.Code)

	generateUnsupportedTypeW := performJSON(router, http.MethodPost, "/api/v1/agents/"+uuid.New().String()+"/config/generate", map[string]any{
		"baseAgentType": "unsupported-agent",
	})
	assert.Equal(t, http.StatusBadRequest, generateUnsupportedTypeW.Code)

	previewInvalidIDW := performJSON(router, http.MethodGet, "/api/v1/agents/not-a-uuid/config/preview", nil)
	assert.Equal(t, http.StatusBadRequest, previewInvalidIDW.Code)

	refreshInvalidIDW := performJSON(router, http.MethodPost, "/api/v1/agents/not-a-uuid/config/refresh", nil)
	assert.Equal(t, http.StatusBadRequest, refreshInvalidIDW.Code)

	refreshWithoutGeneratorW := performJSON(router, http.MethodPost, "/api/v1/agents/"+uuid.New().String()+"/config/refresh", nil)
	assert.Equal(t, http.StatusInternalServerError, refreshWithoutGeneratorW.Code)
}
