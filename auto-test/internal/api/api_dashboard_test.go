package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// @feature F012 - 工作台统计
// @priority P2
// @id API-02-27
func TestDashboardHandler_StatsAndThreadLists(t *testing.T) {
	f := setupAPISurfaceFixture(t)
	agentID := insertAPITestAgent(t, f.db, "Dashboard Agent")
	threadID := uuid.New()

	_, err := f.db.Exec(
		`INSERT INTO threads (
			id, project_id, name, status, current_phase, workflow_template_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		threadID.String(),
		f.projectID.String(),
		"Dashboard running task",
		"running",
		"development",
		f.workflowID.String(),
		time.Now().Format("2006-01-02 15:04:05"),
		time.Now().Format("2006-01-02 15:04:05"),
	)
	require.NoError(t, err)

	_, err = f.db.Exec(
		`INSERT INTO agent_invocations (
			id, thread_id, agent_config_id, role, agent_name, status, input, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(),
		threadID.String(),
		agentID.String(),
		"agent",
		"Dashboard Agent",
		"running",
		"work on dashboard",
		time.Now().Format("2006-01-02 15:04:05"),
	)
	require.NoError(t, err)

	statsW := performJSON(f.router, http.MethodGet, "/api/v1/dashboard/stats", nil)
	require.Equal(t, http.StatusOK, statsW.Code)
	assert.Contains(t, statsW.Body.String(), `"totalProjects":1`)
	assert.Contains(t, statsW.Body.String(), `"activeThreads":1`)
	assert.Contains(t, statsW.Body.String(), "Dashboard Agent")

	activeW := performJSON(f.router, http.MethodGet, "/api/v1/dashboard/active-threads", nil)
	require.Equal(t, http.StatusOK, activeW.Code)
	assert.Contains(t, activeW.Body.String(), "Dashboard running task")
	assert.Contains(t, activeW.Body.String(), "Dashboard Agent")

	recentW := performJSON(f.router, http.MethodGet, "/api/v1/dashboard/recent-threads", nil)
	require.Equal(t, http.StatusOK, recentW.Code)
	assert.Contains(t, recentW.Body.String(), "Dashboard running task")
	assert.Contains(t, recentW.Body.String(), "API Surface Project")
}

// @feature F012 - 工作台统计
// @priority P2
// @id API-02-28
func TestDashboardHandler_WorkflowsWithAssets(t *testing.T) {
	f := setupAPISurfaceFixture(t)
	agentID := insertAPITestAgent(t, f.db, "Asset Dashboard Agent")

	_, err := f.db.Exec(`UPDATE workflow_templates SET agent_ids = ? WHERE id = ?`, `["`+agentID.String()+`"]`, f.workflowID.String())
	require.NoError(t, err)

	skillID := uuid.New()
	_, err = f.db.Exec(
		`INSERT INTO skills (id, name, description, tags, source_type, supported_agents, status, is_public, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		skillID.String(),
		"dashboard-skill",
		"dashboard skill",
		`["Go"]`,
		"personal",
		`["claude_code"]`,
		"active",
		false,
		time.Now().Format("2006-01-02 15:04:05"),
		time.Now().Format("2006-01-02 15:04:05"),
	)
	require.NoError(t, err)

	_, err = f.db.Exec(
		`INSERT INTO agent_skill_bindings (id, agent_role_id, skill_id, created_at)
		VALUES (?, ?, ?, ?)`,
		uuid.New().String(),
		agentID.String(),
		skillID.String(),
		time.Now().Format("2006-01-02 15:04:05"),
	)
	require.NoError(t, err)

	workflowsW := performJSON(f.router, http.MethodGet, "/api/v1/dashboard/workflows-with-assets", nil)
	require.Equal(t, http.StatusOK, workflowsW.Code)
	assert.Contains(t, workflowsW.Body.String(), "Default Test Team")
	assert.Contains(t, workflowsW.Body.String(), "Asset Dashboard Agent")
	assert.Contains(t, workflowsW.Body.String(), "dashboard-skill")
	assert.Contains(t, workflowsW.Body.String(), `"skillsCount":1`)
}
