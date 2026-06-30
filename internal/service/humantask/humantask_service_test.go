package humantask

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

func TestHumanTaskServiceLifecycle(t *testing.T) {
	ctx := context.Background()
	db := openHumanTaskTestDB(t)
	taskRepo := repo.NewHumanTaskRepository(db, repo.DBTypeSQLite)
	service := NewService(taskRepo, repo.NewThreadRepository(db, repo.DBTypeSQLite), repo.NewProjectRepository(db, repo.DBTypeSQLite), nil)

	projectID := insertHumanTaskProject(t, db, "Platform")
	threadID := insertHumanTaskThread(t, db, projectID, "Investigate alert")
	invocationID := uuid.New()
	agentID := uuid.New()

	task, err := service.CreateTaskFromWaiting(ctx, threadID, invocationID, agentID, "Ops Agent", "needs approval")
	if err != nil {
		t.Fatalf("CreateTaskFromWaiting returned error: %v", err)
	}
	if task.Status != model.HumanTaskStatusPending || task.ProjectID != projectID || task.ProjectName != "Platform" || task.ThreadName != "Investigate alert" {
		t.Fatalf("created task = %#v", task)
	}

	again, err := service.CreateTaskFromWaiting(ctx, threadID, invocationID, agentID, "Ops Agent", "duplicate")
	if err != nil || again.ID != task.ID || again.WaitReason != "needs approval" {
		t.Fatalf("idempotent create = %#v err=%v", again, err)
	}

	pending, err := service.List(ctx, model.HumanTaskStatusPending)
	if err != nil || len(pending) != 1 {
		t.Fatalf("List pending = %#v err=%v", pending, err)
	}
	byThread, err := service.ListByThread(ctx, threadID)
	if err != nil || len(byThread) != 1 {
		t.Fatalf("ListByThread = %#v err=%v", byThread, err)
	}
	stats, err := service.GetStats(ctx)
	if err != nil || stats[string(model.HumanTaskStatusPending)] != 1 {
		t.Fatalf("GetStats = %#v err=%v", stats, err)
	}

	if err := service.Complete(ctx, task.ID); err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	got, err := service.Get(ctx, task.ID)
	if err != nil || got.Status != model.HumanTaskStatusCompleted || got.CompletedAt == nil {
		t.Fatalf("completed task = %#v err=%v", got, err)
	}
	if err := service.Complete(ctx, task.ID); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("complete non-pending error = %v", err)
	}
}

func TestHumanTaskServiceCompleteByInvocationAndCancel(t *testing.T) {
	ctx := context.Background()
	db := openHumanTaskTestDB(t)
	service := NewService(repo.NewHumanTaskRepository(db, repo.DBTypeSQLite), nil, nil, nil)

	threadID := uuid.New()
	invocationID := uuid.New()
	task, err := service.CreateTaskFromWaiting(ctx, threadID, invocationID, uuid.New(), "Agent", "wait")
	if err != nil {
		t.Fatalf("CreateTaskFromWaiting returned error: %v", err)
	}
	if task.ProjectID != uuid.Nil || task.ProjectName != "" || task.ThreadName != "" {
		t.Fatalf("task without repos should omit context: %#v", task)
	}
	if err := service.CompleteTaskFromReply(ctx, invocationID); err != nil {
		t.Fatalf("CompleteTaskFromReply returned error: %v", err)
	}
	got, err := service.Get(ctx, task.ID)
	if err != nil || got.Status != model.HumanTaskStatusCompleted || got.CompletedAt == nil {
		t.Fatalf("completed by invocation = %#v err=%v", got, err)
	}

	cancelTask, err := service.CreateTaskFromWaiting(ctx, uuid.New(), uuid.New(), uuid.New(), "Agent", "cancel me")
	if err != nil {
		t.Fatalf("CreateTaskFromWaiting cancel returned error: %v", err)
	}
	if err := service.CancelTask(ctx, cancelTask.ID); err != nil {
		t.Fatalf("CancelTask returned error: %v", err)
	}
	cancelled, err := service.Get(ctx, cancelTask.ID)
	if err != nil || cancelled.Status != model.HumanTaskStatusCancelled || cancelled.CompletedAt == nil {
		t.Fatalf("cancelled task = %#v err=%v", cancelled, err)
	}
	if err := service.CancelTask(ctx, cancelTask.ID); err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("cancel non-pending error = %v", err)
	}
}

func openHumanTaskTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	schema := []string{
		`CREATE TABLE human_tasks (id TEXT PRIMARY KEY, thread_id TEXT, invocation_id TEXT, agent_config_id TEXT, agent_name TEXT, wait_reason TEXT, status TEXT, created_at TIMESTAMP, completed_at TEXT, project_id TEXT, project_name TEXT, thread_name TEXT)`,
		`CREATE TABLE projects (id TEXT PRIMARY KEY, name TEXT, description TEXT, type TEXT, mode TEXT, status TEXT, local_path TEXT, git_repo TEXT, config BLOB, workflow_template_id TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
		`CREATE TABLE threads (id TEXT PRIMARY KEY, project_id TEXT, name TEXT, status TEXT, current_phase TEXT, current_agent TEXT, depth INTEGER, workflow_template_id TEXT, abort_token TEXT, created_at TIMESTAMP, updated_at TIMESTAMP)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec schema: %v", err)
		}
	}
	return db
}

func insertHumanTaskProject(t *testing.T, db *sql.DB, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	_, err := db.Exec(`INSERT INTO projects (id, name, description, type, mode, status, local_path, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), name, "", model.ProjectTypeService, model.ProjectModeNew, model.ProjectStatusDraft, t.TempDir(), now, now)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	return id
}

func insertHumanTaskThread(t *testing.T, db *sql.DB, projectID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	_, err := db.Exec(`INSERT INTO threads (id, project_id, name, status, current_phase, current_agent, depth, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id.String(), projectID.String(), name, model.ThreadStatusIdle, model.PhaseRequirement, "", 0, now, now)
	if err != nil {
		t.Fatalf("insert thread: %v", err)
	}
	return id
}
