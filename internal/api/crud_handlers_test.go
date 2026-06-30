package api

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	agentservice "github.com/anthropic/isdp/internal/service/agent"
	commandservice "github.com/anthropic/isdp/internal/service/command"
	projectservice "github.com/anthropic/isdp/internal/service/project"
	ruleservice "github.com/anthropic/isdp/internal/service/rule"
	settingsservice "github.com/anthropic/isdp/internal/service/settings"
	skillservice "github.com/anthropic/isdp/internal/service/skill"
	subagentservice "github.com/anthropic/isdp/internal/service/subagent"
	workflowservice "github.com/anthropic/isdp/internal/service/workflow"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"
)

func TestBaseAgentHandlerCRUDDefaultAndInvalidRequests(t *testing.T) {
	db := openAPICRUDTestDB(t)
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		NewBaseAgentHandler(agentservice.NewBaseAgentService(repo.NewBaseAgentRepository(db, repo.DBTypeSQLite))).RegisterRoutes(group)
	})

	createW := performAPILightJSON(router, http.MethodPost, "/api/v1/base-agents", map[string]any{
		"name":         "Hermes Runtime",
		"type":         "hermes",
		"apiUrl":       "https://example.invalid",
		"apiToken":     "secret-token",
		"defaultModel": "qwen",
	})
	if createW.Code != http.StatusCreated {
		t.Fatalf("Create code=%d body=%s", createW.Code, createW.Body.String())
	}
	var created model.BaseAgent
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal base agent: %v", err)
	}
	if created.ID == uuid.Nil || created.ApiToken != "" || created.MaxTokens != 4096 || created.TimeoutMinutes != 30 {
		t.Fatalf("created base agent = %#v", created)
	}

	listW := performAPILightJSON(router, http.MethodGet, "/api/v1/base-agents", nil)
	if listW.Code != http.StatusOK || !bytes.Contains(listW.Body.Bytes(), []byte("Hermes Runtime")) {
		t.Fatalf("List code=%d body=%s", listW.Code, listW.Body.String())
	}
	getW := performAPILightJSON(router, http.MethodGet, "/api/v1/base-agents/"+created.ID.String(), nil)
	if getW.Code != http.StatusOK || bytes.Contains(getW.Body.Bytes(), []byte("secret-token")) {
		t.Fatalf("Get code=%d body=%s", getW.Code, getW.Body.String())
	}
	updateW := performAPILightJSON(router, http.MethodPut, "/api/v1/base-agents/"+created.ID.String(), map[string]any{
		"name":           "Updated Hermes",
		"type":           "open_code",
		"defaultModel":   "glm",
		"timeoutMinutes": 45,
	})
	if updateW.Code != http.StatusOK || !bytes.Contains(updateW.Body.Bytes(), []byte("Updated Hermes")) {
		t.Fatalf("Update code=%d body=%s", updateW.Code, updateW.Body.String())
	}
	defaultW := performAPILightJSON(router, http.MethodPut, "/api/v1/base-agents/"+created.ID.String()+"/default", nil)
	if defaultW.Code != http.StatusOK || !bytes.Contains(defaultW.Body.Bytes(), []byte(`"is_default":true`)) {
		t.Fatalf("SetDefault code=%d body=%s", defaultW.Code, defaultW.Body.String())
	}
	clearW := performAPILightJSON(router, http.MethodDelete, "/api/v1/base-agents/"+created.ID.String()+"/default", nil)
	if clearW.Code != http.StatusOK {
		t.Fatalf("ClearDefault code=%d body=%s", clearW.Code, clearW.Body.String())
	}
	typesW := performAPILightJSON(router, http.MethodGet, "/api/v1/base-agents/types", nil)
	if typesW.Code != http.StatusOK {
		t.Fatalf("Types code=%d body=%s", typesW.Code, typesW.Body.String())
	}
	testW := performAPILightJSON(router, http.MethodPost, "/api/v1/base-agents/"+created.ID.String()+"/test", nil)
	if testW.Code != http.StatusBadRequest || !bytes.Contains(testW.Body.Bytes(), []byte(`"success":false`)) {
		t.Fatalf("Test connection code=%d body=%s", testW.Code, testW.Body.String())
	}
	deleteW := performAPILightJSON(router, http.MethodDelete, "/api/v1/base-agents/"+created.ID.String(), nil)
	if deleteW.Code != http.StatusNoContent {
		t.Fatalf("Delete code=%d body=%s", deleteW.Code, deleteW.Body.String())
	}

	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/base-agents", map[string]any{"name": "missing type"}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid create code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/base-agents/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid get code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/base-agents/"+uuid.New().String(), nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing get code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPut, "/api/v1/base-agents/not-a-uuid", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid update id code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/base-agents/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid delete id code=%d", w.Code)
	}
}

func TestWorkflowHandlerCRUDDefaultDeleteGuardsAndInvalidRequests(t *testing.T) {
	db := openAPICRUDTestDB(t)
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		NewWorkflowHandler(workflowservice.NewService(repo.NewWorkflowTemplateRepository(db, repo.DBTypeSQLite))).RegisterRoutes(group)
	})

	agentID := uuid.New().String()
	createW := performAPILightJSON(router, http.MethodPost, "/api/v1/workflows", map[string]any{
		"name":          "Delivery Flow",
		"description":   "ships changes",
		"agentIds":      []string{agentID},
		"checkpoints":   []string{"review"},
		"estimatedTime": "1h",
	})
	if createW.Code != http.StatusCreated {
		t.Fatalf("Create workflow code=%d body=%s", createW.Code, createW.Body.String())
	}
	var created model.WorkflowTemplate
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal workflow: %v", err)
	}
	if created.ID == uuid.Nil || created.Name != "Delivery Flow" {
		t.Fatalf("created workflow = %#v", created)
	}

	listW := performAPILightJSON(router, http.MethodGet, "/api/v1/workflows", nil)
	if listW.Code != http.StatusOK || !bytes.Contains(listW.Body.Bytes(), []byte("Delivery Flow")) {
		t.Fatalf("List workflow code=%d body=%s", listW.Code, listW.Body.String())
	}
	getW := performAPILightJSON(router, http.MethodGet, "/api/v1/workflows/"+created.ID.String(), nil)
	if getW.Code != http.StatusOK || !bytes.Contains(getW.Body.Bytes(), []byte(agentID)) {
		t.Fatalf("Get workflow code=%d body=%s", getW.Code, getW.Body.String())
	}
	updateW := performAPILightJSON(router, http.MethodPut, "/api/v1/workflows/"+created.ID.String(), map[string]any{
		"name":          "Updated Flow",
		"description":   "updated",
		"routableTeams": []string{"ops"},
	})
	if updateW.Code != http.StatusOK || !bytes.Contains(updateW.Body.Bytes(), []byte("Updated Flow")) {
		t.Fatalf("Update workflow code=%d body=%s", updateW.Code, updateW.Body.String())
	}
	defaultW := performAPILightJSON(router, http.MethodPut, "/api/v1/workflows/"+created.ID.String()+"/default", nil)
	if defaultW.Code != http.StatusOK || !bytes.Contains(defaultW.Body.Bytes(), []byte(`"isDefault":true`)) {
		t.Fatalf("SetDefault workflow code=%d body=%s", defaultW.Code, defaultW.Body.String())
	}
	deleteDefaultW := performAPILightJSON(router, http.MethodDelete, "/api/v1/workflows/"+created.ID.String(), nil)
	if deleteDefaultW.Code != http.StatusBadRequest {
		t.Fatalf("Delete default workflow code=%d body=%s", deleteDefaultW.Code, deleteDefaultW.Body.String())
	}

	deletableID := insertAPIWorkflow(t, db, "Deletable", false)
	deleteW := performAPILightJSON(router, http.MethodDelete, "/api/v1/workflows/"+deletableID.String(), nil)
	if deleteW.Code != http.StatusNoContent {
		t.Fatalf("Delete workflow code=%d body=%s", deleteW.Code, deleteW.Body.String())
	}
	referencedID := insertAPIWorkflow(t, db, "Referenced", false)
	insertAPIWorkflowProjectReference(t, db, referencedID)
	deleteReferencedW := performAPILightJSON(router, http.MethodDelete, "/api/v1/workflows/"+referencedID.String(), nil)
	if deleteReferencedW.Code != http.StatusBadRequest {
		t.Fatalf("Delete referenced workflow code=%d body=%s", deleteReferencedW.Code, deleteReferencedW.Body.String())
	}

	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/workflows/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid get workflow code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/workflows/"+uuid.New().String(), nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing get workflow code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPut, "/api/v1/workflows/not-a-uuid", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid update workflow code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPut, "/api/v1/workflows/not-a-uuid/default", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid default workflow code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/workflows/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid delete workflow code=%d", w.Code)
	}
}

func TestAgentHandlerCRUDCopyReferencesAndBatchOperations(t *testing.T) {
	db := openAPICRUDTestDB(t)
	configRepo := repo.NewAgentConfigRepository(db, repo.DBTypeSQLite)
	baseRepo := repo.NewBaseAgentRepository(db, repo.DBTypeSQLite)
	workflowRepo := repo.NewWorkflowTemplateRepository(db, repo.DBTypeSQLite)
	skillBindingRepo := repo.NewAgentSkillBindingRepository(db, repo.DBTypeSQLite)
	commandBindingRepo := repo.NewAgentCommandBindingRepository(db, repo.DBTypeSQLite)
	subagentBindingRepo := repo.NewAgentSubagentBindingRepository(db, repo.DBTypeSQLite)
	ruleBindingRepo := repo.NewAgentRuleBindingRepository(db, repo.DBTypeSQLite)
	settingsBindingRepo := repo.NewAgentSettingsBindingRepository(db, repo.DBTypeSQLite)
	handler := NewAgentHandler(
		agentservice.NewConfigService(configRepo, baseRepo),
		agentservice.NewBaseAgentService(baseRepo),
		nil,
		nil,
		workflowRepo,
		nil,
		nil,
		skillBindingRepo,
		subagentBindingRepo,
		commandBindingRepo,
		ruleBindingRepo,
		settingsBindingRepo,
	)
	router := setupAPILightRouter(handler.RegisterRoutes)

	now := time.Now()
	baseA := uuid.New()
	baseB := uuid.New()
	insertAPIBaseAgent(t, db, baseA, "Hermes", "hermes", true)
	insertAPIBaseAgent(t, db, baseB, "OpenCode", "open_code", false)

	createW := performAPILightJSON(router, http.MethodPost, "/api/v1/agents", map[string]any{
		"name":            "Planner",
		"description":     "plans",
		"systemPrompt":    "plan carefully",
		"baseAgentId":     baseA.String(),
		"mentionPatterns": []string{"@planner"},
		"isDefault":       true,
	})
	if createW.Code != http.StatusCreated {
		t.Fatalf("Create agent code=%d body=%s", createW.Code, createW.Body.String())
	}
	var created model.AgentRoleConfig
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created agent: %v", err)
	}
	if created.ID == uuid.Nil || created.Role != model.AgentRoleAgent {
		t.Fatalf("created agent = %#v", created)
	}

	listW := performAPILightJSON(router, http.MethodGet, "/api/v1/agents", nil)
	if listW.Code != http.StatusOK || !bytes.Contains(listW.Body.Bytes(), []byte("Planner")) {
		t.Fatalf("List agents code=%d body=%s", listW.Code, listW.Body.String())
	}
	getW := performAPILightJSON(router, http.MethodGet, "/api/v1/agents/"+created.ID.String(), nil)
	if getW.Code != http.StatusOK || !bytes.Contains(getW.Body.Bytes(), []byte("Planner")) {
		t.Fatalf("Get agent code=%d body=%s", getW.Code, getW.Body.String())
	}
	roleW := performAPILightJSON(router, http.MethodGet, "/api/v1/agents/role/agent", nil)
	if roleW.Code != http.StatusOK || !bytes.Contains(roleW.Body.Bytes(), []byte("@planner")) {
		t.Fatalf("GetByRole code=%d body=%s", roleW.Code, roleW.Body.String())
	}
	updateW := performAPILightJSON(router, http.MethodPut, "/api/v1/agents/"+created.ID.String(), map[string]any{
		"name":         "Coder",
		"role":         "reviewer",
		"description":  "codes",
		"systemPrompt": "code carefully",
		"baseAgentId":  baseB.String(),
	})
	if updateW.Code != http.StatusOK || !bytes.Contains(updateW.Body.Bytes(), []byte("Coder")) {
		t.Fatalf("Update agent code=%d body=%s", updateW.Code, updateW.Body.String())
	}

	skillID, commandID, subagentID, ruleID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustAPIExec(t, db, `INSERT INTO agent_skill_bindings (id, agent_role_id, skill_id, created_at) VALUES (?, ?, ?, ?)`, uuid.New().String(), created.ID.String(), skillID.String(), now)
	mustAPIExec(t, db, `INSERT INTO agent_command_bindings (id, agent_role_id, command_id, created_at) VALUES (?, ?, ?, ?)`, uuid.New().String(), created.ID.String(), commandID.String(), now)
	mustAPIExec(t, db, `INSERT INTO agent_subagent_bindings (id, agent_role_id, subagent_id, created_at) VALUES (?, ?, ?, ?)`, uuid.New().String(), created.ID.String(), subagentID.String(), now)
	mustAPIExec(t, db, `INSERT INTO agent_rule_bindings (id, agent_role_id, rule_id, created_at) VALUES (?, ?, ?, ?)`, uuid.New().String(), created.ID.String(), ruleID.String(), now)

	copyW := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/"+created.ID.String()+"/copy", nil)
	if copyW.Code != http.StatusCreated || !bytes.Contains(copyW.Body.Bytes(), []byte("副本")) {
		t.Fatalf("Copy agent code=%d body=%s", copyW.Code, copyW.Body.String())
	}
	var copied model.AgentRoleConfig
	if err := json.Unmarshal(copyW.Body.Bytes(), &copied); err != nil {
		t.Fatalf("unmarshal copied agent: %v", err)
	}
	if ids, err := skillBindingRepo.FindByAgentRoleID(nilContext(), copied.ID); err != nil || len(ids) != 1 || ids[0] != skillID {
		t.Fatalf("copied skill bindings = %#v err=%v", ids, err)
	}

	refsW := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/"+created.ID.String()+"/refs", nil)
	if refsW.Code != http.StatusOK || !bytes.Contains(refsW.Body.Bytes(), []byte(`"referenced":false`)) {
		t.Fatalf("refs before workflow code=%d body=%s", refsW.Code, refsW.Body.String())
	}
	insertAPIWorkflowWithAgent(t, db, "Uses Coder", created.ID)
	refsW = performAPILightJSON(router, http.MethodPost, "/api/v1/agents/"+created.ID.String()+"/refs", nil)
	if refsW.Code != http.StatusOK || !bytes.Contains(refsW.Body.Bytes(), []byte(`"referenced":true`)) {
		t.Fatalf("refs after workflow code=%d body=%s", refsW.Code, refsW.Body.String())
	}
	deleteReferencedW := performAPILightJSON(router, http.MethodDelete, "/api/v1/agents/"+created.ID.String(), nil)
	if deleteReferencedW.Code != http.StatusBadRequest {
		t.Fatalf("Delete referenced code=%d body=%s", deleteReferencedW.Code, deleteReferencedW.Body.String())
	}
	batchReferencedW := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/batch-delete", map[string]any{"ids": []string{created.ID.String()}})
	if batchReferencedW.Code != http.StatusBadRequest || !bytes.Contains(batchReferencedW.Body.Bytes(), []byte("referencedAgents")) {
		t.Fatalf("BatchDelete referenced code=%d body=%s", batchReferencedW.Code, batchReferencedW.Body.String())
	}

	batchUpdateW := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/batch-update-base-agent", map[string]any{
		"agentIds":    []string{copied.ID.String(), uuid.New().String()},
		"baseAgentId": baseA.String(),
	})
	if batchUpdateW.Code != http.StatusOK || !bytes.Contains(batchUpdateW.Body.Bytes(), []byte(`"success":1`)) {
		t.Fatalf("BatchUpdateBaseAgent code=%d body=%s", batchUpdateW.Code, batchUpdateW.Body.String())
	}
	batchDeleteW := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/batch-delete", map[string]any{"ids": []string{copied.ID.String()}})
	if batchDeleteW.Code != http.StatusNoContent {
		t.Fatalf("BatchDelete copied code=%d body=%s", batchDeleteW.Code, batchDeleteW.Body.String())
	}

	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/agents/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid get agent code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/agents/"+uuid.New().String(), nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing get agent code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPut, "/api/v1/agents/not-a-uuid", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid update agent code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/agents/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid delete agent code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/batch-delete", map[string]any{"ids": []string{}}); w.Code != http.StatusBadRequest {
		t.Fatalf("empty batch delete code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/batch-generate-config", map[string]any{"agentIds": []string{"bad"}, "cliType": "hermes"}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid batch generate id code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/batch-update-base-agent", map[string]any{"agentIds": []string{"bad"}, "baseAgentId": baseA.String()}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid batch update id code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/"+created.ID.String()+"/refresh", nil); w.Code != http.StatusInternalServerError {
		t.Fatalf("refresh without auto generator code=%d", w.Code)
	}
}

func TestCommandHandlerCRUDAndBindings(t *testing.T) {
	db := openAPICRUDTestDB(t)
	storage := t.TempDir()
	commandRepo := repo.NewCommandRepository(db, repo.DBTypeSQLite)
	skillRepo := repo.NewSkillRepository(db, repo.DBTypeSQLite)
	agentRepo := repo.NewAgentConfigRepository(db, repo.DBTypeSQLite)
	commandSkillRepo := repo.NewCommandSkillBindingRepository(db, repo.DBTypeSQLite)
	agentCommandRepo := repo.NewAgentCommandBindingRepository(db, repo.DBTypeSQLite)
	svc := commandservice.NewService(commandRepo, commandSkillRepo, agentCommandRepo, agentRepo, skillRepo, storage, zap.NewNop())
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		NewCommandHandler(svc, storage, 1024*1024, nil, agentRepo).RegisterRoutes(group)
	})

	agentID := insertAPIAgentConfig(t, db, "Coder")
	skillID := insertAPISkill(t, db, "review-skill")

	createW := performAPILightJSON(router, http.MethodPost, "/api/v1/commands", map[string]any{
		"name":        "build-app",
		"description": "builds app",
		"content":     "# Build\nrun tests",
	})
	if createW.Code != http.StatusCreated {
		t.Fatalf("Create command code=%d body=%s", createW.Code, createW.Body.String())
	}
	var created model.Command
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal command: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatalf("created command = %#v", created)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/commands", map[string]any{"name": "BadName"}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid command name code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/commands", map[string]any{"name": "build-app"}); w.Code != http.StatusConflict {
		t.Fatalf("duplicate command code=%d body=%s", w.Code, w.Body.String())
	}

	listW := performAPILightJSON(router, http.MethodGet, "/api/v1/commands?search=build", nil)
	if listW.Code != http.StatusOK || !bytes.Contains(listW.Body.Bytes(), []byte("build-app")) {
		t.Fatalf("List command code=%d body=%s", listW.Code, listW.Body.String())
	}
	getW := performAPILightJSON(router, http.MethodGet, "/api/v1/commands/"+created.ID.String(), nil)
	if getW.Code != http.StatusOK || !bytes.Contains(getW.Body.Bytes(), []byte("run tests")) {
		t.Fatalf("Get command code=%d body=%s", getW.Code, getW.Body.String())
	}
	updateW := performAPILightJSON(router, http.MethodPut, "/api/v1/commands/"+created.ID.String(), map[string]any{
		"description": "updated",
		"content":     "# Updated",
	})
	if updateW.Code != http.StatusOK || !bytes.Contains(updateW.Body.Bytes(), []byte("updated")) {
		t.Fatalf("Update command code=%d body=%s", updateW.Code, updateW.Body.String())
	}

	bindSkillsW := performAPILightJSON(router, http.MethodPost, "/api/v1/commands/"+created.ID.String()+"/skills", map[string]any{"skillIds": []string{skillID.String()}})
	if bindSkillsW.Code != http.StatusNoContent {
		t.Fatalf("BindSkills code=%d body=%s", bindSkillsW.Code, bindSkillsW.Body.String())
	}
	getSkillsW := performAPILightJSON(router, http.MethodGet, "/api/v1/commands/"+created.ID.String()+"/skills", nil)
	if getSkillsW.Code != http.StatusOK || !bytes.Contains(getSkillsW.Body.Bytes(), []byte("review-skill")) {
		t.Fatalf("GetSkills code=%d body=%s", getSkillsW.Code, getSkillsW.Body.String())
	}
	unbindSkillW := performAPILightJSON(router, http.MethodDelete, "/api/v1/commands/"+created.ID.String()+"/skills/"+skillID.String(), nil)
	if unbindSkillW.Code != http.StatusNoContent {
		t.Fatalf("UnbindSkill code=%d body=%s", unbindSkillW.Code, unbindSkillW.Body.String())
	}

	bindCommandW := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/"+agentID.String()+"/commands", map[string]any{"commandIds": []string{created.ID.String()}})
	if bindCommandW.Code != http.StatusNoContent {
		t.Fatalf("BindCommands code=%d body=%s", bindCommandW.Code, bindCommandW.Body.String())
	}
	getAgentCommandsW := performAPILightJSON(router, http.MethodGet, "/api/v1/agents/"+agentID.String()+"/commands", nil)
	if getAgentCommandsW.Code != http.StatusOK || !bytes.Contains(getAgentCommandsW.Body.Bytes(), []byte("build-app")) {
		t.Fatalf("GetAgentCommands code=%d body=%s", getAgentCommandsW.Code, getAgentCommandsW.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/commands/"+created.ID.String()+"/agents", nil); w.Code != http.StatusInternalServerError {
		t.Fatalf("GetBoundAgents without generator code=%d", w.Code)
	}
	deleteBoundW := performAPILightJSON(router, http.MethodDelete, "/api/v1/commands/"+created.ID.String(), nil)
	if deleteBoundW.Code != http.StatusInternalServerError {
		t.Fatalf("Delete bound command code=%d body=%s", deleteBoundW.Code, deleteBoundW.Body.String())
	}
	unbindCommandW := performAPILightJSON(router, http.MethodDelete, "/api/v1/agents/"+agentID.String()+"/commands/"+created.ID.String(), nil)
	if unbindCommandW.Code != http.StatusNoContent {
		t.Fatalf("UnbindCommand code=%d body=%s", unbindCommandW.Code, unbindCommandW.Body.String())
	}
	deleteW := performAPILightJSON(router, http.MethodDelete, "/api/v1/commands/"+created.ID.String(), nil)
	if deleteW.Code != http.StatusNoContent {
		t.Fatalf("Delete command code=%d body=%s", deleteW.Code, deleteW.Body.String())
	}

	for _, tc := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/v1/commands/not-a-uuid", nil},
		{http.MethodPut, "/api/v1/commands/not-a-uuid", map[string]any{}},
		{http.MethodDelete, "/api/v1/commands/not-a-uuid", nil},
		{http.MethodGet, "/api/v1/commands/not-a-uuid/skills", nil},
		{http.MethodPost, "/api/v1/commands/not-a-uuid/skills", map[string]any{}},
		{http.MethodDelete, "/api/v1/commands/" + uuid.New().String() + "/skills/not-a-uuid", nil},
		{http.MethodGet, "/api/v1/agents/not-a-uuid/commands", nil},
		{http.MethodPost, "/api/v1/agents/not-a-uuid/commands", map[string]any{}},
		{http.MethodDelete, "/api/v1/agents/" + uuid.New().String() + "/commands/not-a-uuid", nil},
	} {
		if w := performAPILightJSON(router, tc.method, tc.path, tc.body); w.Code != http.StatusBadRequest {
			t.Fatalf("%s %s code=%d body=%s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/commands/"+uuid.New().String(), nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing command get code=%d", w.Code)
	}
}

func TestSettingsHandlerUploadReadBindingsAndInvalidRequests(t *testing.T) {
	db := openAPICRUDTestDB(t)
	storage := t.TempDir()
	settingsRepo := repo.NewSettingsRepository(db, repo.DBTypeSQLite)
	agentSettingsRepo := repo.NewAgentSettingsBindingRepository(db, repo.DBTypeSQLite)
	agentRepo := repo.NewAgentConfigRepository(db, repo.DBTypeSQLite)
	svc := settingsservice.NewService(settingsRepo, agentSettingsRepo, agentRepo, storage, zap.NewNop())
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		NewSettingsHandler(svc, storage, nil, agentRepo).RegisterRoutes(group)
	})
	agentID := insertAPIAgentConfig(t, db, "Ops")

	createW := performAPIMultipart(router, http.MethodPost, "/api/v1/settings", map[string]string{
		"name":        "ops-settings",
		"description": "ops config",
	}, "settings.zip", map[string]string{
		"config/app.json": `{"enabled":true}`,
		"README.md":       "# Ops",
	})
	if createW.Code != http.StatusCreated {
		t.Fatalf("Create settings code=%d body=%s", createW.Code, createW.Body.String())
	}
	var created model.Settings
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	if created.ID == uuid.Nil || created.DirectoryPath == "" {
		t.Fatalf("created settings = %#v", created)
	}

	listW := performAPILightJSON(router, http.MethodGet, "/api/v1/settings?search=ops", nil)
	if listW.Code != http.StatusOK || !bytes.Contains(listW.Body.Bytes(), []byte("ops-settings")) {
		t.Fatalf("List settings code=%d body=%s", listW.Code, listW.Body.String())
	}
	getW := performAPILightJSON(router, http.MethodGet, "/api/v1/settings/"+created.ID.String(), nil)
	if getW.Code != http.StatusOK || !bytes.Contains(getW.Body.Bytes(), []byte("ops-settings")) {
		t.Fatalf("Get settings code=%d body=%s", getW.Code, getW.Body.String())
	}
	updateW := performAPILightJSON(router, http.MethodPut, "/api/v1/settings/"+created.ID.String(), map[string]any{"description": "updated"})
	if updateW.Code != http.StatusOK || !bytes.Contains(updateW.Body.Bytes(), []byte("updated")) {
		t.Fatalf("Update settings code=%d body=%s", updateW.Code, updateW.Body.String())
	}
	dirW := performAPILightJSON(router, http.MethodGet, "/api/v1/settings/"+created.ID.String()+"/directory", nil)
	if dirW.Code != http.StatusOK || !bytes.Contains(dirW.Body.Bytes(), []byte("README.md")) {
		t.Fatalf("ReadDirectory code=%d body=%s", dirW.Code, dirW.Body.String())
	}
	fileW := performAPILightJSON(router, http.MethodGet, "/api/v1/settings/"+created.ID.String()+"/file?path=config/app.json", nil)
	if fileW.Code != http.StatusOK || !bytes.Contains(fileW.Body.Bytes(), []byte("enabled")) {
		t.Fatalf("ReadFile code=%d body=%s", fileW.Code, fileW.Body.String())
	}

	bindW := performAPILightJSON(router, http.MethodPost, "/api/v1/agent-roles/"+agentID.String()+"/settings", map[string]any{"settingsIds": []string{created.ID.String()}})
	if bindW.Code != http.StatusNoContent {
		t.Fatalf("Bind settings code=%d body=%s", bindW.Code, bindW.Body.String())
	}
	agentSettingsW := performAPILightJSON(router, http.MethodGet, "/api/v1/agent-roles/"+agentID.String()+"/settings", nil)
	if agentSettingsW.Code != http.StatusOK || !bytes.Contains(agentSettingsW.Body.Bytes(), []byte("ops-settings")) {
		t.Fatalf("GetAgentSettings code=%d body=%s", agentSettingsW.Code, agentSettingsW.Body.String())
	}
	boundAgentsW := performAPILightJSON(router, http.MethodGet, "/api/v1/settings/"+created.ID.String()+"/agents", nil)
	if boundAgentsW.Code != http.StatusOK || !bytes.Contains(boundAgentsW.Body.Bytes(), []byte("Ops")) {
		t.Fatalf("GetBoundAgents code=%d body=%s", boundAgentsW.Code, boundAgentsW.Body.String())
	}
	unbindW := performAPILightJSON(router, http.MethodDelete, "/api/v1/agent-roles/"+agentID.String()+"/settings/"+created.ID.String(), nil)
	if unbindW.Code != http.StatusNoContent {
		t.Fatalf("Unbind settings code=%d body=%s", unbindW.Code, unbindW.Body.String())
	}
	deleteW := performAPILightJSON(router, http.MethodDelete, "/api/v1/settings/"+created.ID.String(), nil)
	if deleteW.Code != http.StatusNoContent {
		t.Fatalf("Delete settings code=%d body=%s", deleteW.Code, deleteW.Body.String())
	}

	if w := performAPIMultipart(router, http.MethodPost, "/api/v1/settings", map[string]string{}, "settings.zip", map[string]string{"a.txt": "a"}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing name create code=%d", w.Code)
	}
	if w := performAPIMultipart(router, http.MethodPost, "/api/v1/settings", map[string]string{"name": "bad"}, "settings.txt", map[string]string{"a.txt": "a"}); w.Code != http.StatusBadRequest {
		t.Fatalf("bad extension create code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/settings/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid get settings code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/settings/"+uuid.New().String(), nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing get settings code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPut, "/api/v1/settings/not-a-uuid", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid update settings code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/settings/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid delete settings code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/settings/not-a-uuid/directory", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid directory settings code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/settings/"+uuid.New().String()+"/file", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("missing file path code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/agent-roles/not-a-uuid/settings", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid bind agent code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/agent-roles/not-a-uuid/settings", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid get agent settings code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/agent-roles/"+agentID.String()+"/settings/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid unbind settings code=%d", w.Code)
	}
}

func TestDashboardHandlerStatsWorkflowsAndThreads(t *testing.T) {
	db := openAPICRUDTestDB(t)
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		NewDashboardHandler(db).RegisterRoutes(group)
	})

	agentID := insertAPIAgentConfig(t, db, "Planner")
	workflowID := insertAPIWorkflowWithAgent(t, db, "Delivery Team", agentID)
	projectID := uuid.New()
	threadID := uuid.New()
	now := time.Now()
	mustAPIExec(t, db, `INSERT INTO projects (id, name, workflow_template_id) VALUES (?, ?, ?)`, projectID.String(), "Colink", workflowID.String())
	mustAPIExec(t, db, `INSERT INTO threads (id, project_id, name, status, current_phase, current_agent, workflow_template_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		threadID.String(), projectID.String(), "Ship feature", "running", "development", "Planner", workflowID.String(), now, now)
	mustAPIExec(t, db, `INSERT INTO agent_invocations (id, thread_id, agent_config_id, agent_name, status, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), threadID.String(), agentID.String(), "Planner", "running", now)
	skillID := insertAPISkill(t, db, "review-skill")
	commandID := insertAPICommandRow(t, db, "build-app")
	subagentID := insertAPISubagentRow(t, db, "reviewer")
	ruleID := insertAPIRuleRow(t, db, "secure-rule")
	mustAPIExec(t, db, `INSERT INTO agent_skill_bindings (id, agent_role_id, skill_id, created_at) VALUES (?, ?, ?, ?)`, uuid.New().String(), agentID.String(), skillID.String(), now)
	mustAPIExec(t, db, `INSERT INTO agent_command_bindings (id, agent_role_id, command_id, created_at) VALUES (?, ?, ?, ?)`, uuid.New().String(), agentID.String(), commandID.String(), now)
	mustAPIExec(t, db, `INSERT INTO agent_subagent_bindings (id, agent_role_id, subagent_id, created_at) VALUES (?, ?, ?, ?)`, uuid.New().String(), agentID.String(), subagentID.String(), now)
	mustAPIExec(t, db, `INSERT INTO agent_rule_bindings (id, agent_role_id, rule_id, created_at) VALUES (?, ?, ?, ?)`, uuid.New().String(), agentID.String(), ruleID.String(), now)

	statsW := performAPILightJSON(router, http.MethodGet, "/api/v1/dashboard/stats", nil)
	if statsW.Code != http.StatusOK || !bytes.Contains(statsW.Body.Bytes(), []byte(`"totalProjects":1`)) || !bytes.Contains(statsW.Body.Bytes(), []byte(`"activeThreads":1`)) || !bytes.Contains(statsW.Body.Bytes(), []byte(`"totalRules":1`)) {
		t.Fatalf("stats code=%d body=%s", statsW.Code, statsW.Body.String())
	}
	workflowsW := performAPILightJSON(router, http.MethodGet, "/api/v1/dashboard/workflows-with-assets", nil)
	if workflowsW.Code != http.StatusOK || !bytes.Contains(workflowsW.Body.Bytes(), []byte("Delivery Team")) || !bytes.Contains(workflowsW.Body.Bytes(), []byte("review-skill")) || !bytes.Contains(workflowsW.Body.Bytes(), []byte(`"totalAssets":4`)) {
		t.Fatalf("workflows code=%d body=%s", workflowsW.Code, workflowsW.Body.String())
	}
	activeW := performAPILightJSON(router, http.MethodGet, "/api/v1/dashboard/active-threads", nil)
	if activeW.Code != http.StatusOK || !bytes.Contains(activeW.Body.Bytes(), []byte("Ship feature")) || !bytes.Contains(activeW.Body.Bytes(), []byte("Planner")) {
		t.Fatalf("active threads code=%d body=%s", activeW.Code, activeW.Body.String())
	}
	recentW := performAPILightJSON(router, http.MethodGet, "/api/v1/dashboard/recent-threads", nil)
	if recentW.Code != http.StatusOK || !bytes.Contains(recentW.Body.Bytes(), []byte("Colink")) || !bytes.Contains(recentW.Body.Bytes(), []byte("Delivery Team")) {
		t.Fatalf("recent threads code=%d body=%s", recentW.Code, recentW.Body.String())
	}

	handler := NewDashboardHandler(db)
	if got := handler.queryCount(context.Background(), "SELECT COUNT(*) FROM missing_table"); got != 0 {
		t.Fatalf("queryCount bad query = %d", got)
	}
	if got := parseJSONArray(`["a", "b"]`); len(got) != 2 || got[1] != "b" {
		t.Fatalf("parseJSONArray standard = %#v", got)
	}
	if got := parseJSONArray(`["a","b",]`); len(got) != 2 {
		t.Fatalf("parseJSONArray fallback = %#v", got)
	}
	if got := parseJSONArray("not-json"); got != nil {
		t.Fatalf("parseJSONArray invalid = %#v", got)
	}
}

func TestSkillHandlerCRUDUploadBindingsAndValidation(t *testing.T) {
	db := openAPICRUDTestDB(t)
	storage := t.TempDir()
	skillRepo := repo.NewSkillRepository(db, repo.DBTypeSQLite)
	agentRepo := repo.NewAgentConfigRepository(db, repo.DBTypeSQLite)
	agentSkillRepo := repo.NewAgentSkillBindingRepository(db, repo.DBTypeSQLite)
	commandSkillRepo := repo.NewCommandSkillBindingRepository(db, repo.DBTypeSQLite)
	subagentSkillRepo := repo.NewSubagentSkillBindingRepository(db, repo.DBTypeSQLite)
	commandRepo := repo.NewCommandRepository(db, repo.DBTypeSQLite)
	subagentRepo := repo.NewSubagentRepository(db, repo.DBTypeSQLite)
	registryRepo := repo.NewSkillRegistryRepository(db, repo.DBTypeSQLite)
	svc := skillservice.NewService(skillRepo, agentSkillRepo, agentRepo, subagentSkillRepo, commandSkillRepo, subagentRepo, commandRepo, storage, zap.NewNop())
	scanner := skillservice.NewSkillScanner(registryRepo, skillRepo, agentSkillRepo, agentRepo, storage, t.TempDir(), zap.NewNop())
	router := setupAPILightRouter(func(group *gin.RouterGroup) {
		NewSkillHandler(svc, scanner, storage, 1024*1024, nil, nil).RegisterRoutes(group)
	})
	agentID := insertAPIAgentConfig(t, db, "Reviewer")

	createW := performAPILightJSON(router, http.MethodPost, "/api/v1/skills", map[string]any{
		"name":        "review-skill",
		"description": "reviews code",
		"tags":        []string{"Go", "review"},
		"sourceType":  "personal",
		"isPublic":    true,
	})
	if createW.Code != http.StatusCreated {
		t.Fatalf("Create skill code=%d body=%s", createW.Code, createW.Body.String())
	}
	var created model.Skill
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal skill: %v", err)
	}
	if created.ID == uuid.Nil || len(created.Tags) != 2 {
		t.Fatalf("created skill = %#v", created)
	}
	listW := performAPILightJSON(router, http.MethodGet, "/api/v1/skills?search=review", nil)
	if listW.Code != http.StatusOK || !bytes.Contains(listW.Body.Bytes(), []byte("review-skill")) {
		t.Fatalf("List skills code=%d body=%s", listW.Code, listW.Body.String())
	}
	getW := performAPILightJSON(router, http.MethodGet, "/api/v1/skills/"+created.ID.String(), nil)
	if getW.Code != http.StatusOK || !bytes.Contains(getW.Body.Bytes(), []byte("reviews code")) {
		t.Fatalf("Get skill code=%d body=%s", getW.Code, getW.Body.String())
	}
	updateW := performAPILightJSON(router, http.MethodPut, "/api/v1/skills/"+created.ID.String(), map[string]any{
		"description": "updated",
		"tags":        []string{"Rust"},
		"status":      "deprecated",
	})
	if updateW.Code != http.StatusOK || !bytes.Contains(updateW.Body.Bytes(), []byte("updated")) {
		t.Fatalf("Update skill code=%d body=%s", updateW.Code, updateW.Body.String())
	}
	tagsW := performAPILightJSON(router, http.MethodGet, "/api/v1/skills/tags", nil)
	if tagsW.Code != http.StatusOK || !bytes.Contains(tagsW.Body.Bytes(), []byte("Rust")) {
		t.Fatalf("GetTags code=%d body=%s", tagsW.Code, tagsW.Body.String())
	}
	builtinTagsW := performAPILightJSON(router, http.MethodGet, "/api/v1/skills/tags/builtin", nil)
	if builtinTagsW.Code != http.StatusOK || !bytes.Contains(builtinTagsW.Body.Bytes(), []byte("编程语言")) {
		t.Fatalf("GetBuiltInTags code=%d body=%s", builtinTagsW.Code, builtinTagsW.Body.String())
	}

	bindW := performAPILightJSON(router, http.MethodPost, "/api/v1/agent-skills/"+agentID.String(), map[string]any{"skillIds": []string{created.ID.String()}})
	if bindW.Code != http.StatusNoContent {
		t.Fatalf("BindSkills code=%d body=%s", bindW.Code, bindW.Body.String())
	}
	agentSkillsW := performAPILightJSON(router, http.MethodGet, "/api/v1/agent-skills/"+agentID.String(), nil)
	if agentSkillsW.Code != http.StatusOK || !bytes.Contains(agentSkillsW.Body.Bytes(), []byte("review-skill")) {
		t.Fatalf("GetAgentSkills code=%d body=%s", agentSkillsW.Code, agentSkillsW.Body.String())
	}
	boundAgentsW := performAPILightJSON(router, http.MethodGet, "/api/v1/skills/"+created.ID.String()+"/agents", nil)
	if boundAgentsW.Code != http.StatusOK || !bytes.Contains(boundAgentsW.Body.Bytes(), []byte("Reviewer")) {
		t.Fatalf("GetBoundAgents code=%d body=%s", boundAgentsW.Code, boundAgentsW.Body.String())
	}
	deleteBoundW := performAPILightJSON(router, http.MethodDelete, "/api/v1/skills/"+created.ID.String(), nil)
	if deleteBoundW.Code != http.StatusInternalServerError {
		t.Fatalf("Delete bound skill code=%d body=%s", deleteBoundW.Code, deleteBoundW.Body.String())
	}
	unbindW := performAPILightJSON(router, http.MethodDelete, "/api/v1/agent-skills/"+agentID.String()+"/"+created.ID.String(), nil)
	if unbindW.Code != http.StatusNoContent {
		t.Fatalf("UnbindSkill code=%d body=%s", unbindW.Code, unbindW.Body.String())
	}

	uploadW := performAPIMultipart(router, http.MethodPost, "/api/v1/skills/upload", map[string]string{
		"directory_name": "uploaded-skill",
		"description":    "from zip",
		"source_type":    "personal",
	}, "skill.zip", map[string]string{
		"SKILL.md": "# Uploaded\n\n## Description\nUploaded desc",
	})
	if uploadW.Code != http.StatusCreated || !bytes.Contains(uploadW.Body.Bytes(), []byte("uploaded-skill")) {
		t.Fatalf("Upload skill code=%d body=%s", uploadW.Code, uploadW.Body.String())
	}

	deleteW := performAPILightJSON(router, http.MethodDelete, "/api/v1/skills/"+created.ID.String(), nil)
	if deleteW.Code != http.StatusNoContent {
		t.Fatalf("Delete skill code=%d body=%s", deleteW.Code, deleteW.Body.String())
	}

	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/skills/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid get skill code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/skills/"+uuid.New().String(), nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing get skill code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPut, "/api/v1/skills/not-a-uuid", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid update skill code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/skills/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid delete skill code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/skills/not-a-uuid/agents", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid bound agents code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/agent-skills/not-a-uuid", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid bind skill agent code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/agent-skills/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid get agent skills code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/agent-skills/"+agentID.String()+"/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid unbind skill code=%d", w.Code)
	}
	if w := performAPIMultipart(router, http.MethodPost, "/api/v1/skills/upload", map[string]string{"directory_name": "bad"}, "skill.txt", map[string]string{"SKILL.md": "# Bad"}); w.Code != http.StatusBadRequest {
		t.Fatalf("bad upload extension code=%d", w.Code)
	}
	if w := performAPIMultipart(router, http.MethodPost, "/api/v1/skills/upload", map[string]string{"directory_name": "bad"}, "skill.zip", map[string]string{"README.md": "# Missing"}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing skill md upload code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/skills/import/repo", map[string]any{"repoUrl": "https://example.com/repo"}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid import repo code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/skills/import/federated/scan", map[string]any{"registryId": "bad"}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid federated scan code=%d", w.Code)
	}
}

func TestSubagentHandlerCRUDBindingsAndInvalidRequests(t *testing.T) {
	db := openAPICRUDTestDB(t)
	storagePath := t.TempDir()
	subagentRepo := repo.NewSubagentRepository(db, repo.DBTypeSQLite)
	bindingRepo := repo.NewAgentSubagentBindingRepository(db, repo.DBTypeSQLite)
	skillBindingRepo := repo.NewSubagentSkillBindingRepository(db, repo.DBTypeSQLite)
	agentRepo := repo.NewAgentConfigRepository(db, repo.DBTypeSQLite)
	skillRepo := repo.NewSkillRepository(db, repo.DBTypeSQLite)
	handler := NewSubagentHandler(
		subagentservice.NewService(subagentRepo, bindingRepo, skillBindingRepo, agentRepo, skillRepo, storagePath, zap.NewNop()),
		storagePath,
		1024,
		nil,
		agentRepo,
	)
	router := setupAPILightRouter(handler.RegisterRoutes)

	agentID := insertAPIAgentConfig(t, db, "Planner")
	skillID := insertAPISkill(t, db, "debugger")

	createW := performAPILightJSON(router, http.MethodPost, "/api/v1/subagents", map[string]any{
		"name":        "code-helper",
		"description": "helps code",
		"content":     "# helper",
	})
	if createW.Code != http.StatusCreated {
		t.Fatalf("Create subagent code=%d body=%s", createW.Code, createW.Body.String())
	}
	var created model.Subagent
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal subagent: %v", err)
	}
	if created.ID == uuid.Nil || created.Content != "# helper" {
		t.Fatalf("created subagent = %#v", created)
	}
	if _, err := os.Stat(filepath.Join(storagePath, "code-helper.md")); err != nil {
		t.Fatalf("expected content file: %v", err)
	}

	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/subagents?page=0&page_size=0", nil); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("code-helper")) {
		t.Fatalf("List subagents code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/subagents/"+created.ID.String(), nil); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("# helper")) {
		t.Fatalf("Get subagent code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodPut, "/api/v1/subagents/"+created.ID.String(), map[string]any{"description": "updated", "content": "# updated"}); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("updated")) {
		t.Fatalf("Update subagent code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/"+agentID.String()+"/subagents", map[string]any{"subagentIds": []string{created.ID.String()}}); w.Code != http.StatusNoContent {
		t.Fatalf("Bind subagents code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/agents/"+agentID.String()+"/subagents", nil); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("code-helper")) {
		t.Fatalf("Get agent subagents code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/subagents/"+created.ID.String()+"/skills", map[string]any{"skillIds": []string{skillID.String()}}); w.Code != http.StatusNoContent {
		t.Fatalf("Bind subagent skills code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/subagents/"+created.ID.String()+"/skills", nil); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("debugger")) {
		t.Fatalf("Get subagent skills code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/subagents/"+created.ID.String()+"/skills/"+skillID.String(), nil); w.Code != http.StatusNoContent {
		t.Fatalf("Unbind subagent skill code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/agents/"+agentID.String()+"/subagents/"+created.ID.String(), nil); w.Code != http.StatusNoContent {
		t.Fatalf("Unbind agent subagent code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/subagents/"+created.ID.String(), nil); w.Code != http.StatusNoContent {
		t.Fatalf("Delete subagent code=%d body=%s", w.Code, w.Body.String())
	}

	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/subagents", map[string]any{"name": "BadName", "content": "x"}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid subagent name code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/subagents/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid get subagent code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/subagents/"+uuid.New().String(), nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing get subagent code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPut, "/api/v1/subagents/not-a-uuid", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid update subagent code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/subagents/"+uuid.New().String()+"/agents", nil); w.Code != http.StatusInternalServerError {
		t.Fatalf("missing auto generator code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/not-a-uuid/subagents", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid bind subagent agent code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/agents/"+agentID.String()+"/subagents/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid unbind subagent code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/subagents/not-a-uuid/skills", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid bind subagent skills code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/subagents/"+uuid.New().String()+"/skills/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid unbind subagent skill code=%d", w.Code)
	}
}

func TestRuleHandlerCRUDBindingsAndInvalidRequests(t *testing.T) {
	db := openAPICRUDTestDB(t)
	storagePath := t.TempDir()
	ruleRepo := repo.NewRuleRepository(db, repo.DBTypeSQLite)
	agentBindingRepo := repo.NewAgentRuleBindingRepository(db, repo.DBTypeSQLite)
	agentRepo := repo.NewAgentConfigRepository(db, repo.DBTypeSQLite)
	handler := NewRuleHandler(
		ruleservice.NewService(ruleRepo, agentBindingRepo, agentRepo, storagePath, zap.NewNop()),
		storagePath,
		1024,
		nil,
		agentRepo,
	)
	router := setupAPILightRouter(handler.RegisterRoutes)

	agentID := insertAPIAgentConfig(t, db, "Planner")
	createW := performAPILightJSON(router, http.MethodPost, "/api/v1/rules", map[string]any{"name": "code-style", "description": "style", "content": "# style"})
	if createW.Code != http.StatusCreated {
		t.Fatalf("Create rule code=%d body=%s", createW.Code, createW.Body.String())
	}
	var created model.Rule
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal rule: %v", err)
	}
	if created.ID == uuid.Nil || created.Content != "# style" {
		t.Fatalf("created rule = %#v", created)
	}

	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/rules?page=0&page_size=0", nil); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("code-style")) {
		t.Fatalf("List rules code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/rules/"+created.ID.String(), nil); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("# style")) {
		t.Fatalf("Get rule code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodPut, "/api/v1/rules/"+created.ID.String(), map[string]any{"description": "updated", "content": "# updated"}); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("updated")) {
		t.Fatalf("Update rule code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/"+agentID.String()+"/rules", map[string]any{"ruleIds": []string{created.ID.String()}}); w.Code != http.StatusNoContent {
		t.Fatalf("Bind rules code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/agents/"+agentID.String()+"/rules", nil); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("code-style")) {
		t.Fatalf("Get agent rules code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/agents/"+agentID.String()+"/rules/"+created.ID.String(), nil); w.Code != http.StatusNoContent {
		t.Fatalf("Unbind rule code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/rules/"+created.ID.String(), nil); w.Code != http.StatusNoContent {
		t.Fatalf("Delete rule code=%d body=%s", w.Code, w.Body.String())
	}

	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/rules", map[string]any{"name": "BadName"}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid rule name code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/rules/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid get rule code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/rules/"+uuid.New().String(), nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing get rule code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPut, "/api/v1/rules/not-a-uuid", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid update rule code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/rules/"+uuid.New().String()+"/agents", nil); w.Code != http.StatusInternalServerError {
		t.Fatalf("missing auto generator rule code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/agents/not-a-uuid/rules", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid bind rule agent code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/agents/"+agentID.String()+"/rules/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid unbind rule code=%d", w.Code)
	}
}

func TestProjectHandlerCRUDAndFileEndpoints(t *testing.T) {
	db := openAPICRUDTestDB(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	handler := NewProjectHandler(projectservice.NewService(repo.NewProjectRepository(db, repo.DBTypeSQLite), repo.NewWorkflowTemplateRepository(db, repo.DBTypeSQLite), nil))
	router := setupAPILightRouter(handler.RegisterRoutes)

	createW := performAPILightJSON(router, http.MethodPost, "/api/v1/projects", map[string]any{"name": "Demo", "description": "project desc", "localPath": root})
	if createW.Code != http.StatusCreated {
		t.Fatalf("Create project code=%d body=%s", createW.Code, createW.Body.String())
	}
	var created model.Project
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal project: %v", err)
	}
	if created.ID == uuid.Nil || created.Type != model.ProjectTypeService || created.Mode != model.ProjectModeNew {
		t.Fatalf("created project = %#v", created)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/projects", nil); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("Demo")) {
		t.Fatalf("List projects code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/projects/"+created.ID.String(), nil); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("Demo")) {
		t.Fatalf("Get project code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodPut, "/api/v1/projects/"+created.ID.String(), map[string]any{"name": "Demo Updated", "status": model.ProjectStatusDeveloping}); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("Demo Updated")) {
		t.Fatalf("Update project code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/projects/"+created.ID.String()+"/files", nil); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("README.md")) {
		t.Fatalf("List files code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/files/browse?path="+root, nil); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("src")) {
		t.Fatalf("Browse path code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/files/validate?path="+root, nil); w.Code != http.StatusOK {
		t.Fatalf("Validate path code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/files/content?basePath="+root+"&path=README.md", nil); w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("hello")) {
		t.Fatalf("Get content code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/files/folder", map[string]any{"path": root, "name": "docs"}); w.Code != http.StatusOK {
		t.Fatalf("Create folder code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/files/image?basePath="+root+"&path=README.md", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("non image code=%d body=%s", w.Code, w.Body.String())
	}
	if w := performAPILightJSON(router, http.MethodDelete, "/api/v1/projects/"+created.ID.String(), nil); w.Code != http.StatusNoContent {
		t.Fatalf("Delete project code=%d body=%s", w.Code, w.Body.String())
	}

	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/projects", map[string]any{"name": "Missing path"}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid create project code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/projects/not-a-uuid", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid get project code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/projects/"+uuid.New().String(), nil); w.Code != http.StatusNotFound {
		t.Fatalf("missing get project code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPut, "/api/v1/projects/not-a-uuid", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid update project code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/files", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("missing base path code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/files/content?basePath="+root, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("missing content path code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/projects/not-a-uuid/files", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid list files project code=%d", w.Code)
	}
}

func openAPICRUDTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE base_agents (id TEXT PRIMARY KEY, name TEXT, type TEXT, api_url TEXT, api_token TEXT, default_model TEXT, cli_path TEXT, git_bash_path TEXT, max_tokens INTEGER, timeout_minutes INTEGER, is_default BOOLEAN, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE agent_configs (id TEXT PRIMARY KEY, name TEXT, role TEXT, description TEXT, system_prompt TEXT, max_tokens INTEGER, temperature REAL, base_agent_id TEXT, is_default INTEGER, is_system INTEGER, requires_human INTEGER, mention_patterns BLOB, config_generated_at TEXT, config_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE workflow_templates (id TEXT PRIMARY KEY, name TEXT, description TEXT, agent_ids BLOB, transitions BLOB, checkpoints BLOB, estimated_time TEXT, is_system INTEGER, is_default INTEGER, routable_teams BLOB, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE projects (id TEXT PRIMARY KEY, name TEXT, description TEXT, type TEXT, mode TEXT, status TEXT, local_path TEXT, git_repo TEXT, config BLOB, workflow_template_id TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE threads (id TEXT PRIMARY KEY, project_id TEXT, name TEXT, status TEXT, current_phase TEXT, current_agent TEXT, workflow_template_id TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE agent_invocations (id TEXT PRIMARY KEY, thread_id TEXT, agent_config_id TEXT, agent_name TEXT, status TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE commands (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE settings (id TEXT PRIMARY KEY, name TEXT, description TEXT, directory_path TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE skill_registries (id TEXT PRIMARY KEY, name TEXT UNIQUE, display_name TEXT, type TEXT, url TEXT, auth_config BLOB, sync_interval INTEGER, last_sync_at TIMESTAMP, sync_status TEXT, skill_count INTEGER, status TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE skills (id TEXT PRIMARY KEY, name TEXT, description TEXT, tags BLOB, source_type TEXT, source_registry_id TEXT, source_path TEXT, author_id TEXT, project_id TEXT, use_count INTEGER, status TEXT, is_public INTEGER, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE subagents (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE rules (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE agent_skill_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_command_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, command_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_subagent_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, subagent_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_rule_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, rule_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_settings_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, settings_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE command_skill_bindings (id TEXT PRIMARY KEY, command_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE subagent_skill_bindings (id TEXT PRIMARY KEY, subagent_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
}

func insertAPIAgentConfig(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	mustAPIExec(t, db, `INSERT INTO agent_configs (id, name, role, description, system_prompt, max_tokens, temperature, base_agent_id, is_default, is_system, requires_human, mention_patterns, config_generated_at, config_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, "agent", "", "system", 0, 0, nil, 0, 0, 0, []byte(`[]`), nil, "", now, now)
	return id
}

func insertAPISkill(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	mustAPIExec(t, db, `INSERT INTO skills (id, name, description, tags, source_type, source_registry_id, source_path, author_id, project_id, use_count, status, is_public, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, "skill", []byte(`[]`), model.SkillSourcePersonal, nil, "", nil, nil, 0, model.SkillStatusActive, 1, now, now)
	return id
}

func insertAPIBaseAgent(t *testing.T, db *sql.DB, id uuid.UUID, name string, typ string, isDefault bool) {
	t.Helper()
	defaultInt := 0
	if isDefault {
		defaultInt = 1
	}
	mustAPIExec(t, db, `INSERT INTO base_agents (id, name, type, api_url, api_token, default_model, cli_path, git_bash_path, max_tokens, timeout_minutes, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, typ, "", "", "model", typ, "", 4096, 30, defaultInt, time.Now(), time.Now())
}

func insertAPIWorkflowWithAgent(t *testing.T, db *sql.DB, name string, agentID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	agentIDs, _ := json.Marshal([]string{agentID.String()})
	_, err := db.Exec(`INSERT INTO workflow_templates (id, name, description, agent_ids, transitions, checkpoints, estimated_time, is_system, is_default, routable_teams, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, "", agentIDs, []byte(`[]`), []byte(`[]`), "1h", 0, 0, []byte(`[]`), now, now)
	if err != nil {
		t.Fatalf("insert workflow with agent: %v", err)
	}
	return id
}

func mustAPIExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %s: %v", query, err)
	}
}

func nilContext() context.Context {
	return context.Background()
}

func performAPIMultipart(router *gin.Engine, method, path string, fields map[string]string, filename string, files map[string]string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		_ = writer.WriteField(key, value)
	}
	part, _ := writer.CreateFormFile("file", filename)
	var zipBuf bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuf)
	for name, content := range files {
		fileWriter, _ := zipWriter.Create(name)
		_, _ = fileWriter.Write([]byte(content))
	}
	_ = zipWriter.Close()
	_, _ = part.Write(zipBuf.Bytes())
	_ = writer.Close()

	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func insertAPIWorkflow(t *testing.T, db *sql.DB, name string, isDefault bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	defaultInt := 0
	if isDefault {
		defaultInt = 1
	}
	_, err := db.Exec(`INSERT INTO workflow_templates (id, name, description, agent_ids, transitions, checkpoints, estimated_time, is_system, is_default, routable_teams, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, "", []byte(`[]`), []byte(`[]`), []byte(`[]`), "1h", 0, defaultInt, []byte(`[]`), now, now)
	if err != nil {
		t.Fatalf("insert workflow: %v", err)
	}
	return id
}

func insertAPIWorkflowProjectReference(t *testing.T, db *sql.DB, workflowID uuid.UUID) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO projects (id, workflow_template_id) VALUES (?, ?)`, uuid.New().String(), workflowID.String())
	if err != nil {
		t.Fatalf("insert project workflow reference: %v", err)
	}
}

func insertAPICommandRow(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	mustAPIExec(t, db, `INSERT INTO commands (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, id.String(), name, "command", now, now)
	return id
}

func insertAPISubagentRow(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	mustAPIExec(t, db, `INSERT INTO subagents (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, id.String(), name, "subagent", now, now)
	return id
}

func insertAPIRuleRow(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	mustAPIExec(t, db, `INSERT INTO rules (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, id.String(), name, "rule", now, now)
	return id
}
