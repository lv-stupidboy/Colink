package api_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// @feature F005 - 线程管理
// @priority P1
// @id API-02-11
func TestRuntimeConfigHandler_Get(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	configW := performJSON(f.router, http.MethodGet, "/api/v1/runtime/config", nil)
	require.Equal(t, http.StatusOK, configW.Code)
	assert.Contains(t, configW.Body.String(), `"deploymentType":"linux"`)
	assert.Contains(t, configW.Body.String(), `/tmp/colink-workspace`)
}

// @feature F005 - 线程管理
// @priority P1
// @id API-02-12
func TestMergeHandler_ApproveAndHandover(t *testing.T) {
	f := setupAPISurfaceFixture(t)
	threadID := uuid.New()

	approveW := performJSON(f.router, http.MethodPost, "/api/v1/threads/"+threadID.String()+"/merge/approve", nil)
	require.Equal(t, http.StatusOK, approveW.Code)
	assert.Contains(t, approveW.Body.String(), `"status":"approved"`)
	assert.Contains(t, approveW.Body.String(), threadID.String())

	handoverW := performJSON(f.router, http.MethodGet, "/api/v1/threads/"+threadID.String()+"/merge/handover", nil)
	require.Equal(t, http.StatusOK, handoverW.Code)
	assert.Contains(t, handoverW.Body.String(), `"status":"handover"`)
	assert.Contains(t, handoverW.Body.String(), threadID.String())
}

// @feature F005 - 线程管理
// @priority P1
// @id API-02-13
func TestMergeHandler_RejectsInvalidThreadIDs(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	approveW := performJSON(f.router, http.MethodPost, "/api/v1/threads/not-a-uuid/merge/approve", nil)
	assert.Equal(t, http.StatusBadRequest, approveW.Code)

	handoverW := performJSON(f.router, http.MethodGet, "/api/v1/threads/not-a-uuid/merge/handover", nil)
	assert.Equal(t, http.StatusBadRequest, handoverW.Code)

	checkW := performJSON(f.router, http.MethodGet, "/api/v1/threads/not-a-uuid/merge/check", nil)
	assert.Equal(t, http.StatusBadRequest, checkW.Code)
}
