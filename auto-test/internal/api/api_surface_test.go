package api_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropic/isdp/auto-test/internal/testutil"
	"github.com/anthropic/isdp/internal/api"
	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	agentservice "github.com/anthropic/isdp/internal/service/agent"
	commandservice "github.com/anthropic/isdp/internal/service/command"
	humantaskservice "github.com/anthropic/isdp/internal/service/humantask"
	knowledgeservice "github.com/anthropic/isdp/internal/service/knowledge"
	mcpservice "github.com/anthropic/isdp/internal/service/mcp"
	messageservice "github.com/anthropic/isdp/internal/service/message"
	projectservice "github.com/anthropic/isdp/internal/service/project"
	ruleservice "github.com/anthropic/isdp/internal/service/rule"
	settingsservice "github.com/anthropic/isdp/internal/service/settings"
	skillservice "github.com/anthropic/isdp/internal/service/skill"
	subagentservice "github.com/anthropic/isdp/internal/service/subagent"
	threadservice "github.com/anthropic/isdp/internal/service/thread"
	workflowservice "github.com/anthropic/isdp/internal/service/workflow"
	"github.com/anthropic/isdp/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type apiSurfaceFixture struct {
	router     *gin.Engine
	db         *sql.DB
	projectID  uuid.UUID
	workflowID uuid.UUID
}

func setupAPISurfaceFixture(t *testing.T) *apiSurfaceFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, _, err := testutil.SetupTestDB()
	require.NoError(t, err)
	t.Cleanup(func() { testutil.CleanupTestDB(db) })

	baseAgentRepo := repo.NewBaseAgentRepository(db, repo.DBTypeSQLite)
	threadRepo := repo.NewThreadRepository(db, repo.DBTypeSQLite)
	projectRepo := repo.NewProjectRepository(db, repo.DBTypeSQLite)
	workflowRepo := repo.NewWorkflowTemplateRepository(db, repo.DBTypeSQLite)
	messageRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	humanTaskRepo := repo.NewHumanTaskRepository(db, repo.DBTypeSQLite)
	settingsRepo := repo.NewSettingsRepository(db, repo.DBTypeSQLite)
	agentSettingsBindingRepo := repo.NewAgentSettingsBindingRepository(db, repo.DBTypeSQLite)
	skillRegistryRepo := repo.NewSkillRegistryRepository(db, repo.DBTypeSQLite)
	knowledgeRepo := repo.NewKnowledgeBaseRepository(db, repo.DBTypeSQLite)
	subagentRepo := repo.NewSubagentRepository(db, repo.DBTypeSQLite)
	agentConfigRepo := repo.NewAgentConfigRepository(db, repo.DBTypeSQLite)
	agentSubagentBindingRepo := repo.NewAgentSubagentBindingRepository(db, repo.DBTypeSQLite)
	subagentSkillBindingRepo := repo.NewSubagentSkillBindingRepository(db, repo.DBTypeSQLite)
	skillRepo := repo.NewSkillRepository(db, repo.DBTypeSQLite)
	commandRepo := repo.NewCommandRepository(db, repo.DBTypeSQLite)
	commandSkillBindingRepo := repo.NewCommandSkillBindingRepository(db, repo.DBTypeSQLite)
	agentCommandBindingRepo := repo.NewAgentCommandBindingRepository(db, repo.DBTypeSQLite)
	mcpServerRepo := repo.NewMCPServerRepository(db, repo.DBTypeSQLite)
	agentMCPBindingRepo := repo.NewAgentMCPBindingRepository(db, repo.DBTypeSQLite)
	ruleRepo := repo.NewRuleRepository(db, repo.DBTypeSQLite)
	agentRuleBindingRepo := repo.NewAgentRuleBindingRepository(db, repo.DBTypeSQLite)
	artifactRepo := repo.NewArtifactRepository(db, repo.DBTypeSQLite)

	baseAgentSvc := agentservice.NewBaseAgentService(baseAgentRepo)
	threadSvc := threadservice.NewService(threadRepo, projectRepo, workflowRepo)
	messageSvc := messageservice.NewService(messageRepo, nil)
	humanTaskSvc := humantaskservice.NewService(humanTaskRepo, threadRepo, projectRepo, nil)
	projectSvc := projectservice.NewService(projectRepo, workflowRepo, nil)
	workflowSvc := workflowservice.NewService(workflowRepo)
	assetStorage := t.TempDir()
	settingsSvc := settingsservice.NewService(
		settingsRepo,
		agentSettingsBindingRepo,
		agentConfigRepo,
		assetStorage,
		zap.NewNop(),
	)
	skillSvc := skillservice.NewService(
		skillRepo,
		repo.NewAgentSkillBindingRepository(db, repo.DBTypeSQLite),
		agentConfigRepo,
		subagentSkillBindingRepo,
		commandSkillBindingRepo,
		subagentRepo,
		commandRepo,
		assetStorage,
		zap.NewNop(),
	)
	registrySvc := skillservice.NewRegistryService(skillRegistryRepo, skillRepo, nil)
	knowledgeSvc := knowledgeservice.NewService(knowledgeRepo)
	subagentSvc := subagentservice.NewService(
		subagentRepo,
		agentSubagentBindingRepo,
		subagentSkillBindingRepo,
		agentConfigRepo,
		skillRepo,
		assetStorage,
		zap.NewNop(),
	)
	commandSvc := commandservice.NewService(
		commandRepo,
		commandSkillBindingRepo,
		agentCommandBindingRepo,
		agentConfigRepo,
		skillRepo,
		assetStorage,
		zap.NewNop(),
	)
	mcpSvc := mcpservice.NewService(
		mcpServerRepo,
		agentMCPBindingRepo,
		agentConfigRepo,
		zap.NewNop(),
	)
	ruleSvc := ruleservice.NewService(
		ruleRepo,
		agentRuleBindingRepo,
		agentConfigRepo,
		assetStorage,
		zap.NewNop(),
	)

	router := gin.New()
	group := router.Group("/api/v1")
	api.NewBaseAgentHandler(baseAgentSvc).RegisterRoutes(group)
	api.NewThreadHandler(threadSvc).RegisterRoutes(group)
	api.NewMessageHandler(messageSvc).RegisterRoutes(group)
	api.NewHumanTaskHandler(humanTaskSvc).RegisterRoutes(group)
	api.NewSkillHandler(skillSvc, nil, assetStorage, 1024*1024, nil, &config.GitURLConversionConfig{}).RegisterRoutes(group)
	api.NewSettingsHandler(settingsSvc, assetStorage, nil, agentConfigRepo).RegisterRoutes(group)
	api.NewRegistryHandler(registrySvc).RegisterRoutes(group)
	api.NewKnowledgeHandler(knowledgeSvc).RegisterRoutes(group)
	api.NewSubagentHandler(subagentSvc, "", 0, nil, agentConfigRepo).RegisterRoutes(group)
	api.NewCommandHandler(commandSvc, assetStorage, 0, nil, agentConfigRepo).RegisterRoutes(group)
	api.NewMCPHandler(mcpSvc).RegisterRoutes(group)
	api.NewRuleHandler(ruleSvc, assetStorage, 0, nil, agentConfigRepo).RegisterRoutes(group)
	api.NewProjectHandler(projectSvc).RegisterRoutes(group)
	api.NewWorkflowHandler(workflowSvc).RegisterRoutes(group)
	api.NewArtifactHandler(artifactRepo).RegisterRoutes(group)
	api.NewRuntimeConfigHandler(&config.Config{
		Deployment: config.DeploymentConfig{
			Type:          config.DeploymentTypeLinux,
			WorkspacePath: "/tmp/colink-workspace",
		},
	}).RegisterRoutes(group)
	api.NewMergeHandler(nil).RegisterRoutes(group)
	api.NewDashboardHandler(db).RegisterRoutes(group)

	workflowID := uuid.New()
	require.NoError(t, workflowRepo.Create(testutil.TestContext(), &model.WorkflowTemplate{
		ID:            workflowID,
		Name:          "Default Test Team",
		Description:   "default test workflow",
		AgentIDs:      json.RawMessage(`[]`),
		Transitions:   json.RawMessage(`[]`),
		Checkpoints:   json.RawMessage(`[]`),
		EstimatedTime: "1h",
		IsDefault:     true,
		RoutableTeams: json.RawMessage(`[]`),
	}))

	project := &model.Project{
		ID:        uuid.New(),
		Name:      "API Surface Project",
		Type:      model.ProjectTypeService,
		Mode:      model.ProjectModeNew,
		Status:    model.ProjectStatusDraft,
		LocalPath: "/tmp/api-surface-project",
	}
	require.NoError(t, projectRepo.Create(testutil.TestContext(), project))

	return &apiSurfaceFixture{
		router:     router,
		db:         db,
		projectID:  project.ID,
		workflowID: workflowID,
	}
}

func performJSON(router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, _ := json.Marshal(body)
		reader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func insertAPITestAgent(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()

	agentID := uuid.New()
	_, err := db.Exec(
		`INSERT INTO agent_configs (id, name, role, system_prompt, max_tokens, temperature, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		agentID.String(), name, model.AgentRoleAgent, "Owns asset bindings", 4096, 0.7,
	)
	require.NoError(t, err)
	return agentID
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id API-02-01
func TestBaseAgentHandler_CRUDAndDefaultLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	createW := performJSON(f.router, http.MethodPost, "/api/v1/base-agents", map[string]any{
		"name":         "Claude Test Runtime",
		"type":         "claude_code",
		"apiUrl":       "https://example.invalid",
		"apiToken":     "secret-token",
		"defaultModel": "claude-sonnet",
		"maxTokens":    8192,
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.BaseAgent
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "Claude Test Runtime", created.Name)
	assert.Empty(t, created.ApiToken, "API token should be sanitized in handler responses")

	getW := performJSON(f.router, http.MethodGet, "/api/v1/base-agents/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)

	updateW := performJSON(f.router, http.MethodPut, "/api/v1/base-agents/"+created.ID.String(), map[string]any{
		"name":           "Updated Runtime",
		"type":           "open_code",
		"defaultModel":   "opencode-main",
		"timeoutMinutes": 45,
	})
	require.Equal(t, http.StatusOK, updateW.Code)

	var updated model.BaseAgent
	require.NoError(t, json.Unmarshal(updateW.Body.Bytes(), &updated))
	assert.Equal(t, "Updated Runtime", updated.Name)
	assert.Equal(t, model.BaseAgentType("open_code"), updated.Type)
	assert.Equal(t, 45, updated.TimeoutMinutes)

	defaultW := performJSON(f.router, http.MethodPut, "/api/v1/base-agents/"+created.ID.String()+"/default", nil)
	require.Equal(t, http.StatusOK, defaultW.Code)
	assert.Contains(t, defaultW.Body.String(), `"is_default":true`)

	clearW := performJSON(f.router, http.MethodDelete, "/api/v1/base-agents/"+created.ID.String()+"/default", nil)
	require.Equal(t, http.StatusOK, clearW.Code)
	assert.NotContains(t, clearW.Body.String(), `"is_default":true`)

	typesW := performJSON(f.router, http.MethodGet, "/api/v1/base-agents/types", nil)
	require.Equal(t, http.StatusOK, typesW.Code)
	var typeResponse []model.BaseAgentTypeInfo
	require.NoError(t, json.Unmarshal(typesW.Body.Bytes(), &typeResponse))

	deleteW := performJSON(f.router, http.MethodDelete, "/api/v1/base-agents/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, deleteW.Code)
}

// @feature F005 - 线程管理
// @priority P0
// @id API-02-02
func TestThreadHandler_CreateListUpdateAndDelete(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	createW := performJSON(f.router, http.MethodPost, "/api/v1/threads/project/"+f.projectID.String(), map[string]any{
		"name": "Build coverage task",
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.Thread
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	assert.Equal(t, "Build coverage task", created.Name)
	assert.Equal(t, model.ThreadStatusIdle, created.Status)
	require.NotNil(t, created.WorkflowTemplateID)
	assert.Equal(t, f.workflowID, *created.WorkflowTemplateID)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/threads/project/"+f.projectID.String(), nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), created.ID.String())

	statusW := performJSON(f.router, http.MethodPut, "/api/v1/threads/"+created.ID.String()+"/status", map[string]any{
		"status": "running",
	})
	require.Equal(t, http.StatusOK, statusW.Code)

	phaseW := performJSON(f.router, http.MethodPut, "/api/v1/threads/"+created.ID.String()+"/phase", map[string]any{
		"phase": "development",
		"agent": "DevAgent",
	})
	require.Equal(t, http.StatusOK, phaseW.Code)

	getW := performJSON(f.router, http.MethodGet, "/api/v1/threads/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)

	var fetched model.Thread
	require.NoError(t, json.Unmarshal(getW.Body.Bytes(), &fetched))
	assert.Equal(t, model.ThreadStatusRunning, fetched.Status)
	assert.Equal(t, model.PhaseDevelopment, fetched.CurrentPhase)
	assert.Equal(t, "DevAgent", fetched.CurrentAgent)

	deleteW := performJSON(f.router, http.MethodDelete, "/api/v1/threads/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, deleteW.Code)

	missingW := performJSON(f.router, http.MethodGet, "/api/v1/threads/"+created.ID.String(), nil)
	assert.Equal(t, http.StatusNotFound, missingW.Code)
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id API-02-03
func TestMessageHandler_CreateWithImagesListAndHistory(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	threadW := performJSON(f.router, http.MethodPost, "/api/v1/threads/project/"+f.projectID.String(), map[string]any{
		"name": "Message flow task",
	})
	require.Equal(t, http.StatusCreated, threadW.Code)

	var thread model.Thread
	require.NoError(t, json.Unmarshal(threadW.Body.Bytes(), &thread))

	createW := performJSON(f.router, http.MethodPost, "/api/v1/messages/thread/"+thread.ID.String(), map[string]any{
		"content": "review this screenshot",
		"images": []map[string]string{
			{"mimeType": "image/png", "data": "aGVsbG8="},
		},
		"skipAgentTrigger": true,
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.Message
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	assert.Equal(t, model.MessageRoleUser, created.Role)
	assert.Equal(t, "anonymous", created.AgentID)
	assert.Contains(t, string(created.ContentBlocks), "media_gallery")
	assert.Contains(t, string(created.ContentBlocks), "data:image/png;base64,aGVsbG8=")

	listW := performJSON(f.router, http.MethodGet, "/api/v1/messages/thread/"+thread.ID.String()+"?limit=1", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), `"total":1`)
	assert.Contains(t, listW.Body.String(), created.ID.String())

	historyW := performJSON(f.router, http.MethodGet, "/api/v1/messages/thread/"+thread.ID.String()+"/history?cursor="+created.ID.String()+"&limit=10", nil)
	require.Equal(t, http.StatusOK, historyW.Code)
	assert.Contains(t, historyW.Body.String(), `"messages":[]`)
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id API-02-04
func TestSubagentHandler_CRUDBindAndUnbindLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	createW := performJSON(f.router, http.MethodPost, "/api/v1/subagents", map[string]any{
		"name":        "review-helper",
		"description": "Helps review generated code",
		"content":     "Focus on regressions and missing tests.",
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.Subagent
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "review-helper", created.Name)
	assert.Equal(t, "Focus on regressions and missing tests.", created.Content)

	invalidW := performJSON(f.router, http.MethodPost, "/api/v1/subagents", map[string]any{
		"name":    "Bad_Name",
		"content": "invalid name should be rejected",
	})
	assert.Equal(t, http.StatusBadRequest, invalidW.Code)

	conflictW := performJSON(f.router, http.MethodPost, "/api/v1/subagents", map[string]any{
		"name":    "review-helper",
		"content": "duplicate",
	})
	assert.Equal(t, http.StatusConflict, conflictW.Code)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/subagents?search=review&page=1&page_size=5", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), `"total":1`)
	assert.Contains(t, listW.Body.String(), `"page_size":5`)

	getW := performJSON(f.router, http.MethodGet, "/api/v1/subagents/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)
	assert.Contains(t, getW.Body.String(), "Focus on regressions")

	updateW := performJSON(f.router, http.MethodPut, "/api/v1/subagents/"+created.ID.String(), map[string]any{
		"description": "Updated review helper",
		"content":     "Now focus on API contracts too.",
	})
	require.Equal(t, http.StatusOK, updateW.Code)
	assert.Contains(t, updateW.Body.String(), "Updated review helper")

	agentID := insertAPITestAgent(t, f.db, "Subagent Owner")

	bindW := performJSON(f.router, http.MethodPost, "/api/v1/agents/"+agentID.String()+"/subagents", map[string]any{
		"subagentIds": []string{created.ID.String()},
	})
	require.Equal(t, http.StatusNoContent, bindW.Code)

	agentSubagentsW := performJSON(f.router, http.MethodGet, "/api/v1/agents/"+agentID.String()+"/subagents", nil)
	require.Equal(t, http.StatusOK, agentSubagentsW.Code)
	assert.Contains(t, agentSubagentsW.Body.String(), `"count":1`)
	assert.Contains(t, agentSubagentsW.Body.String(), "review-helper")

	deleteWhileBoundW := performJSON(f.router, http.MethodDelete, "/api/v1/subagents/"+created.ID.String(), nil)
	assert.Equal(t, http.StatusInternalServerError, deleteWhileBoundW.Code)

	unbindW := performJSON(f.router, http.MethodDelete, "/api/v1/agents/"+agentID.String()+"/subagents/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, unbindW.Code)

	skillsW := performJSON(f.router, http.MethodGet, "/api/v1/subagents/"+created.ID.String()+"/skills", nil)
	require.Equal(t, http.StatusOK, skillsW.Code)
	assert.Contains(t, skillsW.Body.String(), `"count":0`)

	deleteW := performJSON(f.router, http.MethodDelete, "/api/v1/subagents/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, deleteW.Code)

	missingW := performJSON(f.router, http.MethodGet, "/api/v1/subagents/"+created.ID.String(), nil)
	assert.Equal(t, http.StatusNotFound, missingW.Code)
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id API-02-05
func TestCommandHandler_CRUDBindAndUnbindLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	createW := performJSON(f.router, http.MethodPost, "/api/v1/commands", map[string]any{
		"name":        "run-checks",
		"description": "Runs local validation",
		"content":     "go test ./...",
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.Command
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	assert.Equal(t, "run-checks", created.Name)
	assert.Equal(t, "go test ./...", created.Content)

	invalidW := performJSON(f.router, http.MethodPost, "/api/v1/commands", map[string]any{
		"name":    "Run_Checks",
		"content": "invalid",
	})
	assert.Equal(t, http.StatusBadRequest, invalidW.Code)

	conflictW := performJSON(f.router, http.MethodPost, "/api/v1/commands", map[string]any{
		"name":    "run-checks",
		"content": "duplicate",
	})
	assert.Equal(t, http.StatusConflict, conflictW.Code)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/commands?search=run&page=1&page_size=5", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), `"total":1`)

	getW := performJSON(f.router, http.MethodGet, "/api/v1/commands/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)
	assert.Contains(t, getW.Body.String(), "go test ./...")

	updateW := performJSON(f.router, http.MethodPut, "/api/v1/commands/"+created.ID.String(), map[string]any{
		"description": "Runs focused validation",
		"content":     "go test ./auto-test/internal/api",
	})
	require.Equal(t, http.StatusOK, updateW.Code)
	assert.Contains(t, updateW.Body.String(), "Runs focused validation")

	agentID := insertAPITestAgent(t, f.db, "Command Owner")
	bindW := performJSON(f.router, http.MethodPost, "/api/v1/agents/"+agentID.String()+"/commands", map[string]any{
		"commandIds": []string{created.ID.String()},
	})
	require.Equal(t, http.StatusNoContent, bindW.Code)

	agentCommandsW := performJSON(f.router, http.MethodGet, "/api/v1/agents/"+agentID.String()+"/commands", nil)
	require.Equal(t, http.StatusOK, agentCommandsW.Code)
	assert.Contains(t, agentCommandsW.Body.String(), `"count":1`)
	assert.Contains(t, agentCommandsW.Body.String(), "run-checks")

	deleteWhileBoundW := performJSON(f.router, http.MethodDelete, "/api/v1/commands/"+created.ID.String(), nil)
	assert.Equal(t, http.StatusInternalServerError, deleteWhileBoundW.Code)

	unbindW := performJSON(f.router, http.MethodDelete, "/api/v1/agents/"+agentID.String()+"/commands/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, unbindW.Code)

	skillsW := performJSON(f.router, http.MethodGet, "/api/v1/commands/"+created.ID.String()+"/skills", nil)
	require.Equal(t, http.StatusOK, skillsW.Code)
	assert.Contains(t, skillsW.Body.String(), `"count":0`)

	deleteW := performJSON(f.router, http.MethodDelete, "/api/v1/commands/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, deleteW.Code)
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id API-02-06
func TestMCPHandler_CRUDBindAndFilterLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	createW := performJSON(f.router, http.MethodPost, "/api/v1/mcp-servers", map[string]any{
		"name":        "filesystem-tools",
		"displayName": "Filesystem Tools",
		"description": "Provides file operations",
		"transport":   "stdio",
		"command":     "node",
		"args":        []string{"server.js"},
		"env":         map[string]string{"ROOT": "/tmp"},
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.MCPServer
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	assert.Equal(t, "filesystem-tools", created.Name)
	assert.Equal(t, model.MCPTransportStdio, created.Transport)
	assert.Equal(t, []string{"server.js"}, created.Args)

	invalidW := performJSON(f.router, http.MethodPost, "/api/v1/mcp-servers", map[string]any{
		"name":      "Bad_Name",
		"transport": "stdio",
		"command":   "node",
	})
	assert.Equal(t, http.StatusBadRequest, invalidW.Code)

	conflictW := performJSON(f.router, http.MethodPost, "/api/v1/mcp-servers", map[string]any{
		"name":      "filesystem-tools",
		"transport": "stdio",
		"command":   "node",
	})
	assert.Equal(t, http.StatusConflict, conflictW.Code)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/mcp-servers?search=file&page=1&page_size=5&status=active", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), `"total":1`)
	assert.Contains(t, listW.Body.String(), "Filesystem Tools")

	getW := performJSON(f.router, http.MethodGet, "/api/v1/mcp-servers/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)
	assert.Contains(t, getW.Body.String(), "server.js")

	updateW := performJSON(f.router, http.MethodPut, "/api/v1/mcp-servers/"+created.ID.String(), map[string]any{
		"displayName": "Updated Filesystem Tools",
		"description": "Provides scoped file operations",
		"command":     "npx",
		"args":        []string{"@example/mcp-fs"},
	})
	require.Equal(t, http.StatusOK, updateW.Code)
	assert.Contains(t, updateW.Body.String(), "Updated Filesystem Tools")

	agentID := insertAPITestAgent(t, f.db, "MCP Owner")
	bindW := performJSON(f.router, http.MethodPut, "/api/v1/agents/"+agentID.String()+"/mcp-servers", map[string]any{
		"mcpServerIds": []string{created.ID.String()},
	})
	require.Equal(t, http.StatusOK, bindW.Code)

	agentServersW := performJSON(f.router, http.MethodGet, "/api/v1/agents/"+agentID.String()+"/mcp-servers", nil)
	require.Equal(t, http.StatusOK, agentServersW.Code)
	assert.Contains(t, agentServersW.Body.String(), "filesystem-tools")

	clearBindingsW := performJSON(f.router, http.MethodPut, "/api/v1/agents/"+agentID.String()+"/mcp-servers", map[string]any{
		"mcpServerIds": []string{},
	})
	require.Equal(t, http.StatusOK, clearBindingsW.Code)

	deleteW := performJSON(f.router, http.MethodDelete, "/api/v1/mcp-servers/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, deleteW.Code)

	missingW := performJSON(f.router, http.MethodGet, "/api/v1/mcp-servers/"+created.ID.String(), nil)
	assert.Equal(t, http.StatusNotFound, missingW.Code)
}

// @feature F001 - Agent 对话核心
// @priority P0
// @id API-02-07
func TestRuleHandler_CRUDBindAndUnbindLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	createW := performJSON(f.router, http.MethodPost, "/api/v1/rules", map[string]any{
		"name":        "api-contracts",
		"description": "Protects API compatibility",
		"content":     "Do not change response fields without migration.",
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.Rule
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	assert.Equal(t, "api-contracts", created.Name)
	assert.Equal(t, "Do not change response fields without migration.", created.Content)

	invalidW := performJSON(f.router, http.MethodPost, "/api/v1/rules", map[string]any{
		"name": "API_Contracts",
	})
	assert.Equal(t, http.StatusBadRequest, invalidW.Code)

	conflictW := performJSON(f.router, http.MethodPost, "/api/v1/rules", map[string]any{
		"name": "api-contracts",
	})
	assert.Equal(t, http.StatusConflict, conflictW.Code)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/rules?search=api&page=1&page_size=5", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), `"total":1`)

	getW := performJSON(f.router, http.MethodGet, "/api/v1/rules/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)
	assert.Contains(t, getW.Body.String(), "Do not change response fields")

	updateW := performJSON(f.router, http.MethodPut, "/api/v1/rules/"+created.ID.String(), map[string]any{
		"description": "Protects public API compatibility",
		"content":     "Document every response shape change.",
	})
	require.Equal(t, http.StatusOK, updateW.Code)
	assert.Contains(t, updateW.Body.String(), "Protects public API compatibility")

	agentID := insertAPITestAgent(t, f.db, "Rule Owner")
	bindW := performJSON(f.router, http.MethodPost, "/api/v1/agents/"+agentID.String()+"/rules", map[string]any{
		"ruleIds": []string{created.ID.String()},
	})
	require.Equal(t, http.StatusNoContent, bindW.Code)

	agentRulesW := performJSON(f.router, http.MethodGet, "/api/v1/agents/"+agentID.String()+"/rules", nil)
	require.Equal(t, http.StatusOK, agentRulesW.Code)
	assert.Contains(t, agentRulesW.Body.String(), `"count":1`)
	assert.Contains(t, agentRulesW.Body.String(), "api-contracts")

	deleteWhileBoundW := performJSON(f.router, http.MethodDelete, "/api/v1/rules/"+created.ID.String(), nil)
	assert.Equal(t, http.StatusInternalServerError, deleteWhileBoundW.Code)

	unbindW := performJSON(f.router, http.MethodDelete, "/api/v1/agents/"+agentID.String()+"/rules/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, unbindW.Code)

	deleteW := performJSON(f.router, http.MethodDelete, "/api/v1/rules/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, deleteW.Code)
}

// @feature F005 - 线程管理
// @priority P0
// @id API-02-08
func TestProjectHandler_CRUDLifecycle(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	createW := performJSON(f.router, http.MethodPost, "/api/v1/projects", map[string]any{
		"name":      "Coverage Project",
		"type":      "service",
		"mode":      "new",
		"localPath": "/tmp/coverage-project",
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.Project
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	assert.Equal(t, "Coverage Project", created.Name)
	assert.Equal(t, model.ProjectStatusDraft, created.Status)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/projects", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), created.ID.String())

	getW := performJSON(f.router, http.MethodGet, "/api/v1/projects/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)

	updateW := performJSON(f.router, http.MethodPut, "/api/v1/projects/"+created.ID.String(), map[string]any{
		"name":      "Updated Coverage Project",
		"status":    "developing",
		"localPath": "/tmp/coverage-project-updated",
	})
	require.Equal(t, http.StatusOK, updateW.Code)
	assert.Contains(t, updateW.Body.String(), "Updated Coverage Project")
	assert.Contains(t, updateW.Body.String(), "developing")

	deleteW := performJSON(f.router, http.MethodDelete, "/api/v1/projects/"+created.ID.String(), nil)
	require.Equal(t, http.StatusNoContent, deleteW.Code)

	missingW := performJSON(f.router, http.MethodGet, "/api/v1/projects/"+created.ID.String(), nil)
	assert.Equal(t, http.StatusNotFound, missingW.Code)
}

// @feature F005 - 线程管理
// @priority P0
// @id API-02-09
func TestWorkflowHandler_CRUDDefaultAndDeleteProtection(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	createW := performJSON(f.router, http.MethodPost, "/api/v1/workflows", map[string]any{
		"name":          "Coverage Workflow",
		"description":   "workflow under test",
		"agentIds":      []string{},
		"transitions":   []map[string]any{},
		"checkpoints":   []string{"review"},
		"estimatedTime": "2h",
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.WorkflowTemplate
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	assert.Equal(t, "Coverage Workflow", created.Name)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/workflows", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), created.ID.String())

	getW := performJSON(f.router, http.MethodGet, "/api/v1/workflows/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)

	updateW := performJSON(f.router, http.MethodPut, "/api/v1/workflows/"+created.ID.String(), map[string]any{
		"name":          "Updated Coverage Workflow",
		"description":   "updated workflow",
		"agentIds":      []string{},
		"routableTeams": []string{f.workflowID.String()},
	})
	require.Equal(t, http.StatusOK, updateW.Code)
	assert.Contains(t, updateW.Body.String(), "Updated Coverage Workflow")

	defaultW := performJSON(f.router, http.MethodPut, "/api/v1/workflows/"+created.ID.String()+"/default", nil)
	require.Equal(t, http.StatusOK, defaultW.Code)
	assert.Contains(t, defaultW.Body.String(), `"isDefault":true`)

	deleteDefaultW := performJSON(f.router, http.MethodDelete, "/api/v1/workflows/"+created.ID.String(), nil)
	assert.Equal(t, http.StatusBadRequest, deleteDefaultW.Code)
}

// @feature F005 - 线程管理
// @priority P0
// @id API-02-10
func TestArtifactHandler_CreateListAndGet(t *testing.T) {
	f := setupAPISurfaceFixture(t)

	threadW := performJSON(f.router, http.MethodPost, "/api/v1/threads/project/"+f.projectID.String(), map[string]any{
		"name": "Artifact task",
	})
	require.Equal(t, http.StatusCreated, threadW.Code)

	var thread model.Thread
	require.NoError(t, json.Unmarshal(threadW.Body.Bytes(), &thread))

	createW := performJSON(f.router, http.MethodPost, "/api/v1/threads/"+thread.ID.String()+"/artifacts", map[string]any{
		"type":    "document",
		"name":    "Design Note",
		"path":    "docs/design.md",
		"content": "artifact content",
		"metadata": map[string]any{
			"source": "test",
		},
	})
	require.Equal(t, http.StatusCreated, createW.Code)

	var created model.Artifact
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	assert.Equal(t, model.ArtifactTypeDocument, created.Type)
	assert.Equal(t, "Design Note", created.Name)

	listW := performJSON(f.router, http.MethodGet, "/api/v1/threads/"+thread.ID.String()+"/artifacts", nil)
	require.Equal(t, http.StatusOK, listW.Code)
	assert.Contains(t, listW.Body.String(), created.ID.String())

	getW := performJSON(f.router, http.MethodGet, "/api/v1/artifacts/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, getW.Code)
	assert.Contains(t, getW.Body.String(), "artifact content")
}
