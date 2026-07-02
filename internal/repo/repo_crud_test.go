package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

func TestProjectThreadMessageRepositories(t *testing.T) {
	db := newRepoTestDB(t)
	ctx := context.Background()
	projectRepo := NewProjectRepository(db, DBTypeSQLite)
	threadRepo := NewThreadRepository(db, DBTypeSQLite)
	messageRepo := NewMessageRepository(db, DBTypeSQLite)

	desc := "description"
	gitRepo := "https://example.test/repo.git"
	workflowID := uuid.New()
	project := &model.Project{
		ID:                 uuid.New(),
		Name:               "Project",
		Description:        &desc,
		Type:               model.ProjectTypeService,
		Mode:               model.ProjectModeNew,
		Status:             model.ProjectStatusDraft,
		LocalPath:          "/tmp/project",
		RepositoryUrl:      &gitRepo,
		Config:             json.RawMessage(`{"k":"v"}`),
		WorkflowTemplateID: &workflowID,
	}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatal(err)
	}
	foundProject, err := projectRepo.FindByID(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if foundProject.Name != "Project" || foundProject.Description == nil || *foundProject.Description != desc || foundProject.RepositoryUrl == nil {
		t.Fatalf("unexpected project: %#v", foundProject)
	}

	project.Name = "Updated"
	project.Status = model.ProjectStatusDeveloping
	if err := projectRepo.Update(ctx, project); err != nil {
		t.Fatal(err)
	}
	projects, err := projectRepo.FindAll(ctx, 10, 0)
	if err != nil || len(projects) != 1 {
		t.Fatalf("FindAll len=%d err=%v", len(projects), err)
	}
	all, err := projectRepo.ListAll(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("ListAll len=%d err=%v", len(all), err)
	}

	abortToken := "abort"
	thread := &model.Thread{
		ID:           uuid.New(),
		ProjectID:    project.ID,
		Name:         "Thread",
		Status:       model.ThreadStatusIdle,
		CurrentPhase: model.PhaseRequirement,
		CurrentAgent: "agent",
		Depth:        1,
		AbortToken:   &abortToken,
	}
	if err := threadRepo.Create(ctx, thread); err != nil {
		t.Fatal(err)
	}
	thread.Status = model.ThreadStatusRunning
	thread.Depth = 2
	if err := threadRepo.Update(ctx, thread); err != nil {
		t.Fatal(err)
	}
	foundThread, err := threadRepo.FindByID(ctx, thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	if foundThread.Status != model.ThreadStatusRunning || foundThread.Depth != 2 {
		t.Fatalf("unexpected thread: %#v", foundThread)
	}
	threads, err := threadRepo.FindByProjectID(ctx, project.ID)
	if err != nil || len(threads) != 1 {
		t.Fatalf("FindByProjectID len=%d err=%v", len(threads), err)
	}
	projectByThread, err := projectRepo.GetByThreadID(ctx, thread.ID)
	if err != nil || projectByThread.ID != project.ID {
		t.Fatalf("GetByThreadID=%#v err=%v", projectByThread, err)
	}

	replyTo := uuid.New()
	msg1 := &model.Message{ThreadID: thread.ID, Role: model.MessageRoleUser, AgentID: "user", Content: "hello", ContentBlocks: json.RawMessage(`[]`), Metadata: json.RawMessage(`{"a":1}`), MessageType: model.MessageTypeText, Mentions: []string{"agent"}, Origin: "user", ReplyTo: &replyTo}
	msg2 := &model.Message{ThreadID: thread.ID, Role: model.MessageRoleAgent, AgentID: "agent", Content: "world", ContentBlocks: json.RawMessage(`[]`), Metadata: json.RawMessage(`{}`), MessageType: model.MessageTypeText}
	if err := messageRepo.Create(ctx, msg1); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if err := messageRepo.Create(ctx, msg2); err != nil {
		t.Fatal(err)
	}
	messages, err := messageRepo.FindByThreadID(ctx, thread.ID, 10)
	if err != nil || len(messages) != 2 || messages[0].Content != "hello" || messages[1].Content != "world" {
		t.Fatalf("messages=%#v err=%v", messages, err)
	}
	before, err := messageRepo.FindByThreadIDBeforeCursor(ctx, thread.ID, msg2.ID.String(), 10)
	if err != nil || len(before) != 1 || before[0].ID != msg1.ID {
		t.Fatalf("before=%#v err=%v", before, err)
	}
	count, err := messageRepo.CountByThreadID(ctx, thread.ID)
	if err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	recent, err := messageRepo.GetRecent(ctx, thread.ID, 1)
	if err != nil || len(recent) != 1 || recent[0].ID != msg2.ID {
		t.Fatalf("recent=%#v err=%v", recent, err)
	}
	mentions, err := messageRepo.FindMentionsForAgent(ctx, thread.ID, "agent", 10)
	if err != nil || len(mentions) != 1 || mentions[0].ID != msg1.ID {
		t.Fatalf("mentions=%#v err=%v", mentions, err)
	}
	byID, err := messageRepo.GetByID(ctx, msg1.ID)
	if err != nil || byID.ID != msg1.ID || len(byID.Mentions) != 1 {
		t.Fatalf("byID=%#v err=%v", byID, err)
	}
	msg1.Content = "changed"
	msg1.ContentBlocks = json.RawMessage(`[{"type":"text"}]`)
	if err := messageRepo.Update(ctx, msg1); err != nil {
		t.Fatal(err)
	}
	unreported, err := messageRepo.FindUnreportedForReporting(ctx, 10)
	if err != nil || len(unreported) != 2 {
		t.Fatalf("unreported len=%d err=%v", len(unreported), err)
	}
	if err := messageRepo.BatchUpdateReportedAt(ctx, []uuid.UUID{msg1.ID, msg2.ID}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := messageRepo.BatchUpdateReportedAt(ctx, nil, time.Now()); err != nil {
		t.Fatal(err)
	}

	if err := threadRepo.Delete(ctx, thread.ID); err != nil {
		t.Fatal(err)
	}
	if afterDelete, err := messageRepo.CountByThreadID(ctx, thread.ID); err != nil || afterDelete != 0 {
		t.Fatalf("messages should be deleted with thread, count=%d err=%v", afterDelete, err)
	}
	if err := projectRepo.Delete(ctx, project.ID); err != nil {
		t.Fatal(err)
	}
}

func TestArtifactHumanTaskSessionAndContentBlockRepositories(t *testing.T) {
	db := newRepoTestDB(t)
	ctx := context.Background()

	threadID := uuid.New()
	invocationID := uuid.New()
	artifactRepo := NewArtifactRepository(db, DBTypeSQLite)
	artifact := &model.Artifact{
		ID:        uuid.New(),
		ThreadID:  threadID,
		Type:      "report",
		Name:      "report.md",
		Path:      "/tmp/report.md",
		Content:   "content",
		Metadata:  map[string]interface{}{"source": "test"},
		CreatedAt: time.Now(),
	}
	if err := artifactRepo.Create(ctx, artifact); err != nil {
		t.Fatal(err)
	}
	foundArtifact, err := artifactRepo.FindByID(ctx, artifact.ID)
	if err != nil || foundArtifact.Metadata["source"] != "test" {
		t.Fatalf("artifact=%#v err=%v", foundArtifact, err)
	}
	artifacts, err := artifactRepo.FindByThreadID(ctx, threadID)
	if err != nil || len(artifacts) != 1 {
		t.Fatalf("artifacts=%#v err=%v", artifacts, err)
	}
	if err := artifactRepo.Delete(ctx, artifact.ID); err != nil {
		t.Fatal(err)
	}

	taskRepo := NewHumanTaskRepository(db, DBTypeSQLite)
	task := &model.HumanTask{
		ID:            uuid.New(),
		ThreadID:      threadID,
		InvocationID:  invocationID,
		AgentConfigID: uuid.New(),
		AgentName:     "agent",
		WaitReason:    "question",
		ProjectID:     uuid.New(),
		ProjectName:   "Project",
		ThreadName:    "Thread",
		Status:        model.HumanTaskStatusPending,
		CreatedAt:     time.Now(),
	}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatal(err)
	}
	if found, err := taskRepo.FindByID(ctx, task.ID); err != nil || found.ID != task.ID {
		t.Fatalf("FindByID=%#v err=%v", found, err)
	}
	if found, err := taskRepo.FindByInvocation(ctx, invocationID); err != nil || found == nil || found.ID != task.ID {
		t.Fatalf("FindByInvocation=%#v err=%v", found, err)
	}
	if tasks, err := taskRepo.ListByThread(ctx, threadID); err != nil || len(tasks) != 1 {
		t.Fatalf("ListByThread=%#v err=%v", tasks, err)
	}
	if tasks, err := taskRepo.ListByStatus(ctx, model.HumanTaskStatusPending); err != nil || len(tasks) != 1 {
		t.Fatalf("ListByStatus=%#v err=%v", tasks, err)
	}
	counts, err := taskRepo.CountByStatus(ctx)
	if err != nil || counts[string(model.HumanTaskStatusPending)] != 1 {
		t.Fatalf("counts=%v err=%v", counts, err)
	}
	if err := taskRepo.CompleteByInvocation(ctx, invocationID); err != nil {
		t.Fatal(err)
	}
	if found, err := taskRepo.FindByInvocation(ctx, invocationID); err != nil || found != nil {
		t.Fatalf("completed invocation should not be pending, found=%#v err=%v", found, err)
	}
	task.Status = model.HumanTaskStatusCancelled
	now := time.Now()
	task.CompletedAt = &now
	if err := taskRepo.Update(ctx, task); err != nil {
		t.Fatal(err)
	}
	if err := taskRepo.Delete(ctx, task.ID); err != nil {
		t.Fatal(err)
	}

	sessionRepo := NewSessionRecordRepository(db)
	record := &model.SessionRecord{ThreadID: threadID, AgentID: uuid.New(), AgentType: "open_code", AcpSessionID: "acp", CliSessionID: "cli"}
	record.SetResumeExpiry(-24)
	if err := sessionRepo.Create(ctx, record); err != nil {
		t.Fatal(err)
	}
	if found, err := sessionRepo.FindByID(ctx, record.ID); err != nil || found.ID != record.ID {
		t.Fatalf("session=%#v err=%v", found, err)
	}
	if found, err := sessionRepo.FindByThreadAndAgent(ctx, threadID.String(), record.AgentID.String()); err != nil || found.ID != record.ID {
		t.Fatalf("thread/agent session=%#v err=%v", found, err)
	}
	if c, err := sessionRepo.CountByThread(ctx, threadID.String()); err != nil || c != 1 {
		t.Fatalf("CountByThread=%d err=%v", c, err)
	}
	if c, err := sessionRepo.CountByAgentType(ctx, "open_code"); err != nil || c != 1 {
		t.Fatalf("CountByAgentType=%d err=%v", c, err)
	}
	if expired, err := sessionRepo.FindExpiredRecords(ctx, time.Hour); err != nil || len(expired) != 1 {
		t.Fatalf("expired=%#v err=%v", expired, err)
	}
	record.Status = "idle"
	if err := sessionRepo.Update(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := sessionRepo.DeleteExpiredRecords(ctx, time.Hour); err != nil {
		t.Fatal(err)
	}
	if found, err := sessionRepo.FindByID(ctx, record.ID); err != nil || found != nil {
		t.Fatalf("expired record should be deleted, found=%#v err=%v", found, err)
	}

	blockRepo := NewContentBlockRepository(db, DBTypeSQLite)
	block := model.InvocationContentBlock{
		ID:           "block-1",
		InvocationID: invocationID.String(),
		Type:         "tool_use",
		Content:      "content",
		ToolName:     "read",
		ToolID:       "tool-1",
		Input:        map[string]interface{}{"path": "README.md"},
		Output:       "ok",
		Status:       "completed",
		Timestamp:    1,
		StartedAt:    1,
		CompletedAt:  2,
	}
	if err := blockRepo.Upsert(ctx, &block); err != nil {
		t.Fatal(err)
	}
	if err := blockRepo.BatchUpsert(ctx, []model.InvocationContentBlock{{ID: "block-2", InvocationID: invocationID.String(), Type: "text", Content: "hello", Timestamp: 2}}); err != nil {
		t.Fatal(err)
	}
	if err := blockRepo.BatchUpsert(ctx, nil); err != nil {
		t.Fatal(err)
	}
	blocks, err := blockRepo.FindByInvocation(ctx, invocationID)
	if err != nil || len(blocks) != 2 || blocks[0].Input["path"] != "README.md" {
		t.Fatalf("blocks=%#v err=%v", blocks, err)
	}
	if raw, err := blockRepo.FindByInvocationRaw(ctx, invocationID); err == nil && len(raw) == 0 {
		t.Fatalf("expected raw json, got empty")
	}
	if err := blockRepo.DeleteOlderThan(ctx, -1); err != nil {
		t.Fatal(err)
	}
	if err := blockRepo.DeleteByInvocation(ctx, invocationID); err != nil {
		t.Fatal(err)
	}
}

func newRepoTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE projects (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT, type TEXT NOT NULL,
			mode TEXT NOT NULL, status TEXT, local_path TEXT NOT NULL, git_repo TEXT,
			config BLOB, workflow_template_id TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE threads (
			id TEXT PRIMARY KEY, project_id TEXT, name TEXT, status TEXT, current_phase TEXT,
			current_agent TEXT, depth INTEGER, workflow_template_id TEXT, abort_token TEXT,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE messages (
			id TEXT PRIMARY KEY, thread_id TEXT NOT NULL, role TEXT NOT NULL, agent_id TEXT,
			content TEXT, content_blocks BLOB, message_type TEXT, metadata BLOB,
			created_at TEXT NOT NULL, reported_at TIMESTAMP NULL, mentions BLOB, origin TEXT, reply_to TEXT
		)`,
		`CREATE TABLE artifacts (
			id TEXT PRIMARY KEY, thread_id TEXT NOT NULL, type TEXT, name TEXT, path TEXT,
			content TEXT, metadata BLOB, created_at TEXT NOT NULL
		)`,
		`CREATE TABLE human_tasks (
			id TEXT PRIMARY KEY, thread_id TEXT NOT NULL, invocation_id TEXT NOT NULL,
			agent_config_id TEXT NOT NULL, agent_name TEXT, wait_reason TEXT,
			status TEXT NOT NULL, created_at TEXT NOT NULL, completed_at TEXT,
			project_id TEXT, project_name TEXT, thread_name TEXT
		)`,
		`CREATE TABLE session_records (
			id TEXT PRIMARY KEY, thread_id TEXT NOT NULL, agent_id TEXT NOT NULL,
			agent_type TEXT NOT NULL, acp_session_id TEXT, cli_session_id TEXT,
			resume_expiry INTEGER, status TEXT, last_active_at INTEGER,
			created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE invocation_content_blocks (
			id TEXT PRIMARY KEY, invocation_id TEXT NOT NULL, type TEXT NOT NULL,
			content TEXT, tool_name TEXT, tool_id TEXT, input BLOB, output TEXT,
			is_error BOOLEAN, status TEXT, timestamp INTEGER NOT NULL,
			started_at INTEGER, completed_at INTEGER, created_at TIMESTAMP
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create table failed: %v\n%s", err, stmt)
		}
	}
	return db
}
