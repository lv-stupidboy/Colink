package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	agentservice "github.com/anthropic/isdp/internal/service/agent"
	commandservice "github.com/anthropic/isdp/internal/service/command"
	workflowservice "github.com/anthropic/isdp/internal/service/workflow"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
	"go.uber.org/zap"
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
		`CREATE TABLE projects (id TEXT PRIMARY KEY, workflow_template_id TEXT)`,
		`CREATE TABLE commands (id TEXT PRIMARY KEY, name TEXT, description TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE skills (id TEXT PRIMARY KEY, name TEXT, description TEXT, tags BLOB, source_type TEXT, source_registry_id TEXT, source_path TEXT, author_id TEXT, project_id TEXT, use_count INTEGER, status TEXT, is_public INTEGER, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE agent_skill_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_command_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, command_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_subagent_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, subagent_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_rule_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, rule_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE agent_settings_bindings (id TEXT PRIMARY KEY, agent_role_id TEXT, settings_id TEXT, created_at TIMESTAMP)`,
		`CREATE TABLE command_skill_bindings (id TEXT PRIMARY KEY, command_id TEXT, skill_id TEXT, created_at TIMESTAMP)`,
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
