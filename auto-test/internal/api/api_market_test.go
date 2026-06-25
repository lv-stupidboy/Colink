package api_test

import (
	"net/http"
	"testing"

	"github.com/anthropic/isdp/internal/api"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// @feature F011 - 技能市场
// @priority P2
// @id API-02-36
func TestMarketHandler_DefaultConfigAndValidation(t *testing.T) {
	cfg := &config.Config{
		Market: config.MarketDefaultConfig{
			Name:   "Official",
			URL:    "https://github.com/example/market",
			Branch: "main",
		},
	}
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewMarketHandler(nil, cfg, zap.NewNop()).RegisterRoutes(group)
	})

	defaultW := performJSON(router, http.MethodGet, "/api/v1/markets/default", nil)
	require.Equal(t, http.StatusOK, defaultW.Code)
	assert.Contains(t, defaultW.Body.String(), `"name":"Official"`)
	assert.Contains(t, defaultW.Body.String(), `"branch":"main"`)

	missingURLRouter := setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewMarketHandler(nil, &config.Config{}, zap.NewNop()).RegisterRoutes(group)
	})
	addDefaultW := performJSON(missingURLRouter, http.MethodPost, "/api/v1/markets/default", nil)
	assert.Equal(t, http.StatusBadRequest, addDefaultW.Code)

	addMarketW := performJSON(router, http.MethodPost, "/api/v1/markets", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, addMarketW.Code)
}

// @feature F011 - 技能市场
// @priority P2
// @id API-02-37
func TestMarketHandler_RejectsInvalidIDs(t *testing.T) {
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewMarketHandler(nil, &config.Config{}, zap.NewNop()).RegisterRoutes(group)
	})

	updateW := performJSON(router, http.MethodPut, "/api/v1/markets/not-a-uuid", map[string]any{
		"name": "bad",
	})
	assert.Equal(t, http.StatusBadRequest, updateW.Code)

	deleteW := performJSON(router, http.MethodDelete, "/api/v1/markets/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, deleteW.Code)

	refreshW := performJSON(router, http.MethodPost, "/api/v1/markets/not-a-uuid/refresh", nil)
	assert.Equal(t, http.StatusBadRequest, refreshW.Code)

	validID := "11111111-1111-1111-1111-111111111111"
	malformedUpdateW := performJSON(router, http.MethodPut, "/api/v1/markets/"+validID, nil)
	assert.Equal(t, http.StatusBadRequest, malformedUpdateW.Code)
}
