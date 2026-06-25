package api_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertAPIHumanTask(t *testing.T, db *sql.DB, status model.HumanTaskStatus) (uuid.UUID, uuid.UUID) {
	t.Helper()

	taskID := uuid.New()
	invocationID := uuid.New()
	_, err := db.Exec(
		`INSERT INTO human_tasks (
			id, thread_id, invocation_id, agent_config_id, agent_name, wait_reason,
			status, created_at, project_id, project_name, thread_name
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID.String(),
		uuid.New().String(),
		invocationID.String(),
		uuid.New().String(),
		"ReviewAgent",
		"waiting for user approval",
		status,
		time.Now().Format("2006-01-02 15:04:05"),
		uuid.New().String(),
		"Human Task Project",
		"Human Task Thread",
	)
	require.NoError(t, err)
	return taskID, invocationID
}

// @feature F006 - 人工任务
// @priority P1
// @id API-02-16
func TestHumanTaskHandler_ListGetCompleteAndStats(t *testing.T) {
	f := setupAPISurfaceFixture(t)
	taskID, invocationID := insertAPIHumanTask(t, f.db, model.HumanTaskStatusPending)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/human-tasks", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), taskID.String())
	assert.Contains(t, listW.Body.String(), "ReviewAgent")

	getW := performJSON(f.router, http.MethodGet, "/api/v1/human-tasks/"+taskID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)
	assert.Contains(t, getW.Body.String(), "waiting for user approval")

	statsW := performJSON(f.router, http.MethodGet, "/api/v1/human-tasks/stats", nil)
	require.Equal(t, http.StatusOK, statsW.Code)
	assert.Contains(t, statsW.Body.String(), `"pending":1`)

	completeW := performJSON(f.router, http.MethodPut, "/api/v1/human-tasks/"+taskID.String()+"/complete", nil)
	require.Equal(t, http.StatusOK, completeW.Code)
	assert.Contains(t, completeW.Body.String(), `"status":"completed"`)

	completeAgainW := performJSON(f.router, http.MethodPut, "/api/v1/human-tasks/"+taskID.String()+"/complete", nil)
	assert.Equal(t, http.StatusInternalServerError, completeAgainW.Code)

	byInvocationW := performJSON(f.router, http.MethodPut, "/api/v1/human-tasks/invocation/"+invocationID.String()+"/complete", nil)
	require.Equal(t, http.StatusOK, byInvocationW.Code)
}

// @feature F006 - 人工任务
// @priority P1
// @id API-02-17
func TestHumanTaskHandler_CancelAndRejectInvalidInputs(t *testing.T) {
	f := setupAPISurfaceFixture(t)
	taskID, _ := insertAPIHumanTask(t, f.db, model.HumanTaskStatusPending)

	invalidListW := performJSON(f.router, http.MethodGet, "/api/v1/human-tasks?status=unknown", nil)
	assert.Equal(t, http.StatusBadRequest, invalidListW.Code)

	invalidGetW := performJSON(f.router, http.MethodGet, "/api/v1/human-tasks/not-a-uuid", nil)
	assert.Equal(t, http.StatusBadRequest, invalidGetW.Code)

	missingW := performJSON(f.router, http.MethodGet, "/api/v1/human-tasks/"+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, missingW.Code)

	cancelW := performJSON(f.router, http.MethodPut, "/api/v1/human-tasks/"+taskID.String()+"/cancel", nil)
	require.Equal(t, http.StatusOK, cancelW.Code)
	assert.Contains(t, cancelW.Body.String(), `"status":"cancelled"`)

	cancelAgainW := performJSON(f.router, http.MethodPut, "/api/v1/human-tasks/"+taskID.String()+"/cancel", nil)
	assert.Equal(t, http.StatusBadRequest, cancelAgainW.Code)

	invalidInvocationW := performJSON(f.router, http.MethodPut, "/api/v1/human-tasks/invocation/not-a-uuid/complete", nil)
	assert.Equal(t, http.StatusBadRequest, invalidInvocationW.Code)
}
