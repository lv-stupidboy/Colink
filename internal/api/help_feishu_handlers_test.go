package api

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHelpHandlerConfigFeedbackValidationAndSubmit(t *testing.T) {
	handler := NewHelpHandler("support", "https://colink.example", "https://docs.example", "")
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		handler.RegisterRoutes(group)
	})

	configW := performAPILightJSON(router, http.MethodGet, "/api/v1/help/config", nil)
	if configW.Code != http.StatusOK || !strings.Contains(configW.Body.String(), `"feedback_enabled":false`) {
		t.Fatalf("config code=%d body=%s", configW.Code, configW.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/help/feedback", map[string]any{"description": "missing type"}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing type code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/help/feedback", map[string]any{"type": "bug"}); w.Code != http.StatusBadRequest {
		t.Fatalf("empty content code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/help/feedback", map[string]any{"type": "bug", "description": "broken"}); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled feedback code=%d", w.Code)
	}

	successHandler := NewHelpHandler("support", "", "", "https://feedback.test/submit")
	successHandler.initClient()
	successHandler.client.Transport = helpRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if req.URL.String() != "https://feedback.test/submit" || !strings.Contains(string(body), `"description":"broken"`) {
			t.Fatalf("feedback request url=%s body=%s", req.URL.String(), body)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})
	successRouter := setupAPILightRouter(func(group *gin.RouterGroup) {
		successHandler.RegisterRoutes(group)
	})
	successW := performAPILightJSON(successRouter, http.MethodPost, "/api/v1/help/feedback", map[string]any{"type": "bug", "description": "broken"})
	if successW.Code != http.StatusOK || !strings.Contains(successW.Body.String(), "反馈已提交") {
		t.Fatalf("success feedback code=%d body=%s", successW.Code, successW.Body.String())
	}

	errorHandler := NewHelpHandler("support", "", "", "https://feedback.test/submit")
	errorHandler.initClient()
	errorHandler.client.Transport = helpRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("bad")), Header: make(http.Header)}, nil
	})
	errorRouter := setupAPILightRouter(func(group *gin.RouterGroup) {
		errorHandler.RegisterRoutes(group)
	})
	errorW := performAPILightJSON(errorRouter, http.MethodPost, "/api/v1/help/feedback", map[string]any{"type": "bug", "description": "broken"})
	if errorW.Code != http.StatusInternalServerError {
		t.Fatalf("error feedback code=%d body=%s", errorW.Code, errorW.Body.String())
	}
}

func TestFeishuWebhookHandlerVerificationAndIgnoredPayloads(t *testing.T) {
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		NewFeishuWebhookHandler(nil, "verify-token").RegisterRoutes(group)
	})

	verifyW := performAPILightJSON(router, http.MethodPost, "/api/v1/feishu/webhook", map[string]any{
		"type":      "url_verification",
		"token":     "verify-token",
		"challenge": "challenge-code",
	})
	if verifyW.Code != http.StatusOK || !strings.Contains(verifyW.Body.String(), `"challenge":"challenge-code"`) {
		t.Fatalf("verify code=%d body=%s", verifyW.Code, verifyW.Body.String())
	}
	wrongTokenW := performAPILightJSON(router, http.MethodPost, "/api/v1/feishu/webhook", map[string]any{
		"type":      "url_verification",
		"token":     "wrong-token",
		"challenge": "challenge-code",
	})
	if wrongTokenW.Code != http.StatusOK || !strings.Contains(wrongTokenW.Body.String(), `"status":"ok"`) {
		t.Fatalf("wrong token code=%d body=%s", wrongTokenW.Code, wrongTokenW.Body.String())
	}
	malformedW := performAPILightJSON(router, http.MethodPost, "/api/v1/feishu/webhook", "{")
	if malformedW.Code != http.StatusOK || !strings.Contains(malformedW.Body.String(), `"status":"ok"`) {
		t.Fatalf("malformed code=%d body=%s", malformedW.Code, malformedW.Body.String())
	}
}

type helpRoundTripFunc func(*http.Request) (*http.Response, error)

func (f helpRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
