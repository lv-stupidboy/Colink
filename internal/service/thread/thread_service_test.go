package thread

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestThreadServiceWorkflowSelectionAndUpdates(t *testing.T) {
	ctx := context.Background()
	db := openThreadTestDB(t)
	projectRepo := repo.NewProjectRepository(db, repo.DBTypeSQLite)
	workflowRepo := repo.NewWorkflowTemplateRepository(db, repo.DBTypeSQLite)
	service := NewService(repo.NewThreadRepository(db, repo.DBTypeSQLite), projectRepo, workflowRepo)

	defaultWorkflow := insertThreadWorkflow(t, db, "Default Team", true)
	overrideWorkflow := insertThreadWorkflow(t, db, "Override Team", false)
	project := insertThreadProject(t, db, "Project", nil)

	created, err := service.Create(ctx, project.ID, "First task", nil)
	if err != nil {
		t.Fatalf("Create default returned error: %v", err)
	}
	if created.Status != model.ThreadStatusIdle || created.CurrentPhase != model.PhaseRequirement || created.WorkflowTemplateID == nil || *created.WorkflowTemplateID != defaultWorkflow {
		t.Fatalf("created default thread = %#v", created)
	}

	withOverride, err := service.Create(ctx, project.ID, "Override task", &overrideWorkflow)
	if err != nil {
		t.Fatalf("Create override returned error: %v", err)
	}
	if withOverride.WorkflowTemplateID == nil || *withOverride.WorkflowTemplateID != overrideWorkflow {
		t.Fatalf("override thread = %#v", withOverride)
	}
	if _, err := service.Create(ctx, project.ID, "Bad override", ptrUUID(uuid.New())); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("bad override error = %v", err)
	}

	boundProject := insertThreadProject(t, db, "Bound Project", &overrideWorkflow)
	boundThread, err := service.Create(ctx, boundProject.ID, "Bound task", nil)
	if err != nil {
		t.Fatalf("Create bound returned error: %v", err)
	}
	if boundThread.WorkflowTemplateID == nil || *boundThread.WorkflowTemplateID != overrideWorkflow {
		t.Fatalf("bound thread = %#v", boundThread)
	}

	got, err := service.GetByID(ctx, created.ID)
	if err != nil || got.ID != created.ID {
		t.Fatalf("GetByID = %#v err=%v", got, err)
	}
	threads, err := service.GetByProjectID(ctx, project.ID)
	if err != nil || len(threads) != 2 {
		t.Fatalf("GetByProjectID = %#v err=%v", threads, err)
	}
	if err := service.UpdateStatus(ctx, created.ID, model.ThreadStatusRunning); err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}
	if err := service.SetPhase(ctx, created.ID, model.PhaseDevelopment, "coder"); err != nil {
		t.Fatalf("SetPhase returned error: %v", err)
	}
	got, err = service.GetByID(ctx, created.ID)
	if err != nil || got.Status != model.ThreadStatusRunning || got.CurrentPhase != model.PhaseDevelopment || got.CurrentAgent != "coder" {
		t.Fatalf("updated thread = %#v err=%v", got, err)
	}
	updated, err := service.Update(ctx, created.ID, &overrideWorkflow)
	if err != nil || updated.WorkflowTemplateID == nil || *updated.WorkflowTemplateID != overrideWorkflow {
		t.Fatalf("Update workflow = %#v err=%v", updated, err)
	}
	cleared, err := service.Update(ctx, created.ID, nil)
	if err != nil || cleared.WorkflowTemplateID != nil {
		t.Fatalf("clear workflow = %#v err=%v", cleared, err)
	}
	if _, err := service.Update(ctx, created.ID, ptrUUID(uuid.New())); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("bad update workflow error = %v", err)
	}
	if err := service.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := service.GetByID(ctx, created.ID); err == nil {
		t.Fatalf("deleted thread should not be found")
	}
}

func TestThreadServiceCreateWithoutDefaultWorkflow(t *testing.T) {
	ctx := context.Background()
	db := openThreadTestDB(t)
	service := NewService(repo.NewThreadRepository(db, repo.DBTypeSQLite), repo.NewProjectRepository(db, repo.DBTypeSQLite), repo.NewWorkflowTemplateRepository(db, repo.DBTypeSQLite))
	project := insertThreadProject(t, db, "Project", nil)
	if _, err := service.Create(ctx, project.ID, "No workflow", nil); err == nil || !strings.Contains(err.Error(), "默认工作流") {
		t.Fatalf("missing default workflow error = %v", err)
	}
}

func openThreadTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE projects (id TEXT PRIMARY KEY, name TEXT, description TEXT, type TEXT, mode TEXT, status TEXT, local_path TEXT, git_repo TEXT, config BLOB, workflow_template_id TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE workflow_templates (id TEXT PRIMARY KEY, name TEXT, description TEXT, agent_ids BLOB, transitions BLOB, checkpoints BLOB, estimated_time TEXT, is_system INTEGER, is_default INTEGER, routable_teams BLOB, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE threads (id TEXT PRIMARY KEY, project_id TEXT, name TEXT, status TEXT, current_phase TEXT, current_agent TEXT, depth INTEGER, workflow_template_id TEXT, abort_token TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE messages (id TEXT PRIMARY KEY, thread_id TEXT)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
}

func insertThreadWorkflow(t *testing.T, db *sql.DB, name string, isDefault bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	defaultInt := 0
	if isDefault {
		defaultInt = 1
	}
	_, err := db.Exec(`INSERT INTO workflow_templates (id, name, description, agent_ids, transitions, checkpoints, estimated_time, is_system, is_default, routable_teams, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, "workflow", []byte(`[]`), []byte(`[]`), []byte(`[]`), "1h", 0, defaultInt, []byte(`[]`), now, now)
	if err != nil {
		t.Fatalf("insert workflow: %v", err)
	}
	return id
}

func insertThreadProject(t *testing.T, db *sql.DB, name string, workflowID *uuid.UUID) *model.Project {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	var workflow any
	if workflowID != nil {
		workflow = workflowID.String()
	}
	_, err := db.Exec(`INSERT INTO projects (id, name, description, type, mode, status, local_path, workflow_template_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, "project", model.ProjectTypeService, model.ProjectModeNew, model.ProjectStatusDraft, t.TempDir(), workflow, now, now)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	return &model.Project{ID: id, Name: name, WorkflowTemplateID: workflowID}
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	return &id
}
