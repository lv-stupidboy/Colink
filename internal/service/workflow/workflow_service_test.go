package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestWorkflowServiceCreateUpdateAndAgentIDs(t *testing.T) {
	ctx := context.Background()
	db := openWorkflowTestDB(t)
	service := NewService(repo.NewWorkflowTemplateRepository(db, repo.DBTypeSQLite))

	agentID := uuid.New()
	otherAgentID := uuid.New()
	created, err := service.Create(ctx, &model.CreateWorkflowTemplateRequest{
		Name:          "Delivery Team",
		Description:   "ships features",
		AgentIDs:      []string{agentID.String(), "not-a-uuid"},
		Transitions:   []model.Transition{{FromAgentID: agentID.String(), ToAgentID: otherAgentID.String(), Type: model.TransitionTypeSequence}},
		Checkpoints:   []string{"review"},
		EstimatedTime: "2h",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID == uuid.Nil || created.IsSystem || created.Name != "Delivery Team" {
		t.Fatalf("created template = %#v", created)
	}

	got, err := service.GetByID(ctx, created.ID)
	if err != nil || got.Name != "Delivery Team" {
		t.Fatalf("GetByID = %#v err=%v", got, err)
	}
	var checkpoints []string
	if err := json.Unmarshal(got.Checkpoints, &checkpoints); err != nil || len(checkpoints) != 1 || checkpoints[0] != "review" {
		t.Fatalf("checkpoints = %q err=%v", got.Checkpoints, err)
	}

	ids, err := service.GetAgentIDs(ctx, created.ID)
	if err != nil || len(ids) != 1 || ids[0] != agentID {
		t.Fatalf("GetAgentIDs = %v err=%v", ids, err)
	}

	updated, err := service.Update(ctx, created.ID, &model.UpdateWorkflowTemplateRequest{
		Name:          "Updated Team",
		Description:   "updated",
		AgentIDs:      []string{otherAgentID.String()},
		Checkpoints:   []string{"qa"},
		EstimatedTime: "3h",
		RoutableTeams: []string{"ops", "support"},
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Name != "Updated Team" || updated.Description != "updated" || updated.EstimatedTime != "3h" {
		t.Fatalf("updated template = %#v", updated)
	}
	var routable []string
	if err := json.Unmarshal(updated.RoutableTeams, &routable); err != nil || len(routable) != 2 || routable[0] != "ops" {
		t.Fatalf("routable teams = %q err=%v", updated.RoutableTeams, err)
	}

	all, err := service.List(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("List = %#v err=%v", all, err)
	}
}

func TestWorkflowServiceDefaultAndDeleteGuards(t *testing.T) {
	ctx := context.Background()
	db := openWorkflowTestDB(t)
	service := NewService(repo.NewWorkflowTemplateRepository(db, repo.DBTypeSQLite))

	first := insertWorkflowTemplate(t, db, "First", false, false)
	second := insertWorkflowTemplate(t, db, "Second", false, false)
	defaultTemplate, err := service.SetDefault(ctx, first)
	if err != nil || defaultTemplate.ID != first || !defaultTemplate.IsDefault {
		t.Fatalf("SetDefault = %#v err=%v", defaultTemplate, err)
	}
	gotDefault, err := service.GetDefault(ctx)
	if err != nil || gotDefault.ID != first {
		t.Fatalf("GetDefault = %#v err=%v", gotDefault, err)
	}
	if err := service.Delete(ctx, first); err == nil || !strings.Contains(err.Error(), "系统默认工作流") {
		t.Fatalf("delete default error = %v", err)
	}

	insertWorkflowProjectReference(t, db, second)
	if err := service.Delete(ctx, second); err == nil || !strings.Contains(err.Error(), "项目绑定") {
		t.Fatalf("delete referenced error = %v", err)
	}

	third := insertWorkflowTemplate(t, db, "Third", false, false)
	if err := service.Delete(ctx, third); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := service.GetByID(ctx, third); err == nil {
		t.Fatalf("deleted workflow should not be found")
	}
}

func openWorkflowTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
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

func insertWorkflowTemplate(t *testing.T, db *sql.DB, name string, isSystem, isDefault bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	systemInt := 0
	if isSystem {
		systemInt = 1
	}
	defaultInt := 0
	if isDefault {
		defaultInt = 1
	}
	_, err := db.Exec(`INSERT INTO workflow_templates (id, name, description, agent_ids, transitions, checkpoints, estimated_time, is_system, is_default, routable_teams, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, "", []byte(`[]`), []byte(`[]`), []byte(`[]`), "1h", systemInt, defaultInt, []byte(`[]`), now, now)
	if err != nil {
		t.Fatalf("insert workflow template: %v", err)
	}
	return id
}

func insertWorkflowProjectReference(t *testing.T, db *sql.DB, workflowID uuid.UUID) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO projects (id, workflow_template_id) VALUES (?, ?)`, uuid.New().String(), workflowID.String())
	if err != nil {
		t.Fatalf("insert workflow project reference: %v", err)
	}
}
