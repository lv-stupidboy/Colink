package api_test

import (
	"net/http"
	"testing"

	"github.com/anthropic/isdp/internal/api"
	sandboxservice "github.com/anthropic/isdp/internal/service/sandbox"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSandboxRouter() *gin.Engine {
	return setupStandaloneRouter(func(group *gin.RouterGroup) {
		handler := api.NewSandboxHandler(nil)
		handler.RegisterRoutes(group)
		group.GET("/sandbox/:id/proxy-url", handler.GetProxyURL)
	})
}

// @feature F015 - 沙箱预览
// @priority P2
// @id API-02-41
func TestSandboxHandler_RejectsInvalidIDs(t *testing.T) {
	router := setupSandboxRouter()

	getW := performJSON(router, http.MethodGet, "/api/v1/sandbox/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, getW.Code)

	logsW := performJSON(router, http.MethodGet, "/api/v1/sandbox/not-a-uuid/logs", nil)
	assert.Equal(t, http.StatusBadRequest, logsW.Code)

	stopW := performJSON(router, http.MethodPost, "/api/v1/sandbox/not-a-uuid/stop", nil)
	assert.Equal(t, http.StatusBadRequest, stopW.Code)

	previewW := performJSON(router, http.MethodGet, "/api/v1/sandbox/preview/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, previewW.Code)

	proxyW := performJSON(router, http.MethodGet, "/api/v1/sandbox/not-a-uuid/proxy/", nil)
	assert.Equal(t, http.StatusBadRequest, proxyW.Code)
}

// @feature F015 - 沙箱预览
// @priority P2
// @id API-02-42
func TestSandboxHandler_RunValidationAndProxyURL(t *testing.T) {
	router := setupSandboxRouter()

	runInvalidThreadW := performJSON(router, http.MethodPost, "/api/v1/sandbox/run", map[string]any{
		"threadId":    "not-a-uuid",
		"projectPath": "/tmp/project",
		"mode":        "local",
	})
	assert.Equal(t, http.StatusBadRequest, runInvalidThreadW.Code)

	id := uuid.New()
	proxyURLW := performJSON(router, http.MethodGet, "/api/v1/sandbox/"+id.String()+"/proxy-url", nil)
	require.Equal(t, http.StatusOK, proxyURLW.Code)
	assert.Contains(t, proxyURLW.Body.String(), "/api/v1/sandbox/"+id.String()+"/proxy/")
}

// @feature F015 - 沙箱预览
// @priority P2
// @id API-02-58
func TestSandboxHandler_ServiceBackedEmptyStates(t *testing.T) {
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewSandboxHandler(sandboxservice.NewSandboxService(nil, nil)).RegisterRoutes(group)
	})

	listW := performJSON(router, http.MethodGet, "/api/v1/sandbox", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.JSONEq(t, `[]`, listW.Body.String())

	dockerW := performJSON(router, http.MethodGet, "/api/v1/sandbox/docker/status", nil)
	require.Equal(t, http.StatusOK, dockerW.Code)
	assert.Contains(t, dockerW.Body.String(), `"available":false`)

	id := uuid.New().String()
	getW := performJSON(router, http.MethodGet, "/api/v1/sandbox/"+id, nil)
	assert.Equal(t, http.StatusNotFound, getW.Code)

	logsW := performJSON(router, http.MethodGet, "/api/v1/sandbox/"+id+"/logs", nil)
	assert.Equal(t, http.StatusNotFound, logsW.Code)

	stopW := performJSON(router, http.MethodPost, "/api/v1/sandbox/"+id+"/stop", nil)
	assert.Equal(t, http.StatusNotFound, stopW.Code)

	previewW := performJSON(router, http.MethodGet, "/api/v1/sandbox/preview/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, previewW.Code)
}
