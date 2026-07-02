package repo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestSandboxRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	db := openSandboxRepoTestDB(t)
	repository := NewSandboxRepository(db, DBTypeSQLite)

	threadID := uuid.New()
	endedAt := time.Now().Add(time.Hour).Truncate(time.Second)
	sandbox := &model.Sandbox{
		ID:          uuid.New(),
		ThreadID:    threadID,
		Name:        "preview",
		Image:       "node:22-alpine",
		Status:      model.SandboxStatusCreated,
		ContainerID: "",
		Port:        0,
		CreatedAt:   time.Now().Truncate(time.Second),
		EndedAt:     nil,
	}
	if err := repository.Create(ctx, sandbox); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	found, err := repository.FindByID(ctx, sandbox.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if found.ID != sandbox.ID || found.ThreadID != threadID || found.Name != "preview" || found.Port != 0 || found.EndedAt != nil {
		t.Fatalf("FindByID got unexpected sandbox: %#v", found)
	}

	sandbox.Status = model.SandboxStatusRunning
	sandbox.ContainerID = "container-1"
	sandbox.Port = 18080
	sandbox.EndedAt = &endedAt
	if err := repository.Update(ctx, sandbox); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	updated, err := repository.FindByID(ctx, sandbox.ID)
	if err != nil {
		t.Fatalf("FindByID after update returned error: %v", err)
	}
	if updated.Status != model.SandboxStatusRunning || updated.ContainerID != "container-1" || updated.Port != 18080 || updated.EndedAt == nil {
		t.Fatalf("updated sandbox = %#v", updated)
	}

	second := &model.Sandbox{
		ID:        uuid.New(),
		ThreadID:  threadID,
		Name:      "older",
		Image:     "python:3.11-slim",
		Status:    model.SandboxStatusStopped,
		CreatedAt: time.Now().Add(-time.Hour),
	}
	if err := repository.Create(ctx, second); err != nil {
		t.Fatalf("Create second returned error: %v", err)
	}
	byThread, err := repository.FindByThreadID(ctx, threadID)
	if err != nil {
		t.Fatalf("FindByThreadID returned error: %v", err)
	}
	if len(byThread) != 2 || byThread[0].ID != sandbox.ID || byThread[1].ID != second.ID {
		t.Fatalf("FindByThreadID order = %#v", byThread)
	}
	empty, err := repository.FindByThreadID(ctx, uuid.New())
	if err != nil || len(empty) != 0 {
		t.Fatalf("FindByThreadID empty = %#v err=%v", empty, err)
	}

	if err := repository.Delete(ctx, sandbox.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := repository.FindByID(ctx, sandbox.ID); err == nil {
		t.Fatalf("FindByID should fail after delete")
	}
}

func openSandboxRepoTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE sandboxes (
		id TEXT PRIMARY KEY,
		thread_id TEXT,
		name TEXT,
		image TEXT,
		status TEXT,
		container_id TEXT,
		port INTEGER,
		created_at TIMESTAMP,
		ended_at TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("exec schema: %v", err)
	}
	return db
}
