package api_test

import (
	"net/http"
	"testing"

	"github.com/anthropic/isdp/internal/api"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStandaloneRouter(register func(*gin.RouterGroup)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	register(router.Group("/api/v1"))
	return router
}

// @feature F009 - 用户帮助反馈
// @priority P2
// @id API-02-14
func TestHelpHandler_ConfigAndFeedbackValidation(t *testing.T) {
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewHelpHandler("support", "https://colink.example", "https://docs.example", "").RegisterRoutes(group)
	})

	configW := performJSON(router, http.MethodGet, "/api/v1/help/config", nil)
	require.Equal(t, http.StatusOK, configW.Code)
	assert.Contains(t, configW.Body.String(), `"support_group":"support"`)
	assert.Contains(t, configW.Body.String(), `"feedback_enabled":false`)

	missingTypeW := performJSON(router, http.MethodPost, "/api/v1/help/feedback", map[string]any{
		"description": "Something is wrong",
	})
	assert.Equal(t, http.StatusBadRequest, missingTypeW.Code)

	emptyContentW := performJSON(router, http.MethodPost, "/api/v1/help/feedback", map[string]any{
		"type": "bug",
	})
	assert.Equal(t, http.StatusBadRequest, emptyContentW.Code)

	disabledW := performJSON(router, http.MethodPost, "/api/v1/help/feedback", map[string]any{
		"type":        "bug",
		"description": "Something is wrong",
	})
	assert.Equal(t, http.StatusServiceUnavailable, disabledW.Code)
}

// @feature F010 - IM Webhook 接入
// @priority P2
// @id API-02-15
func TestFeishuWebhookHandler_VerificationAndIgnoredPayloads(t *testing.T) {
	router := setupStandaloneRouter(func(group *gin.RouterGroup) {
		api.NewFeishuWebhookHandler(nil, "verify-token").RegisterRoutes(group)
	})

	verifyW := performJSON(router, http.MethodPost, "/api/v1/feishu/webhook", map[string]any{
		"type":      "url_verification",
		"token":     "verify-token",
		"challenge": "challenge-code",
	})
	require.Equal(t, http.StatusOK, verifyW.Code)
	assert.Contains(t, verifyW.Body.String(), `"challenge":"challenge-code"`)

	wrongTokenW := performJSON(router, http.MethodPost, "/api/v1/feishu/webhook", map[string]any{
		"type":      "url_verification",
		"token":     "wrong-token",
		"challenge": "challenge-code",
	})
	require.Equal(t, http.StatusOK, wrongTokenW.Code)
	assert.Contains(t, wrongTokenW.Body.String(), `"status":"ok"`)

	malformedW := performJSON(router, http.MethodPost, "/api/v1/feishu/webhook", "{")
	require.Equal(t, http.StatusOK, malformedW.Code)
	assert.Contains(t, malformedW.Body.String(), `"status":"ok"`)
}
