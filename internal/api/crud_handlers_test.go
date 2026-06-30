package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	agentservice "github.com/anthropic/isdp/internal/service/agent"
	workflowservice "github.com/anthropic/isdp/internal/service/workflow"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

func openAPICRUDTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE base_agents (id TEXT PRIMARY KEY, name TEXT, type TEXT, api_url TEXT, api_token TEXT, default_model TEXT, cli_path TEXT, git_bash_path TEXT, max_tokens INTEGER, timeout_minutes INTEGER, is_default BOOLEAN, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE workflow_templates (id TEXT PRIMARY KEY, name TEXT, description TEXT, agent_ids BLOB, transitions BLOB, checkpoints BLOB, estimated_time TEXT, is_system INTEGER, is_default INTEGER, routable_teams BLOB, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE projects (id TEXT PRIMARY KEY, workflow_template_id TEXT)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
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
