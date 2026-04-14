package repo

import (
	"context"
	"database/sql"
	"testing"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

func TestIMSessionRepository_Create(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	createTable(db)

	repo := NewIMSessionRepository(db)
	ctx := context.Background()

	session := &model.IMSession{
		ID:        uuid.New(),
		Platform:  model.IMPlatformFeishu,
		ChatID:    "test_chat_123",
		ChatType:  "p2p",
		ThreadID:  uuid.New(),
		ProjectID: uuid.New(),
		UserID:    "user_123",
		UserName:  "Test User",
		IsActive:  true,
	}

	err = repo.Create(ctx, session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if session.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if session.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestIMSessionRepository_FindByChatID(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	createTable(db)

	repo := NewIMSessionRepository(db)
	ctx := context.Background()

	threadID := uuid.New()
	projectID := uuid.New()
	chatID := "test_chat_456"

	session := &model.IMSession{
		ID:        uuid.New(),
		Platform:  model.IMPlatformFeishu,
		ChatID:    chatID,
		ChatType:  "group",
		ThreadID:  threadID,
		ProjectID: projectID,
		IsActive:  true,
	}

	err = repo.Create(ctx, session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	found, err := repo.FindByChatID(ctx, string(model.IMPlatformFeishu), chatID)
	if err != nil {
		t.Fatalf("failed to find session: %v", err)
	}

	if found.ID != session.ID {
		t.Errorf("expected ID %s, got %s", session.ID, found.ID)
	}
	if found.ChatID != chatID {
		t.Errorf("expected ChatID %s, got %s", chatID, found.ChatID)
	}
}

func TestIMSessionRepository_FindByThreadID(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	createTable(db)

	repo := NewIMSessionRepository(db)
	ctx := context.Background()

	threadID := uuid.New()
	session := &model.IMSession{
		ID:        uuid.New(),
		Platform:  model.IMPlatformFeishu,
		ChatID:    "chat_789",
		ThreadID:  threadID,
		ProjectID: uuid.New(),
		IsActive:  true,
	}

	err = repo.Create(ctx, session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	found, err := repo.FindByThreadID(ctx, threadID)
	if err != nil {
		t.Fatalf("failed to find session by thread ID: %v", err)
	}

	if found.ID != session.ID {
		t.Errorf("expected ID %s, got %s", session.ID, found.ID)
	}
}

func TestIMSessionRepository_UpdateLastMessageAt(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	createTable(db)

	repo := NewIMSessionRepository(db)
	ctx := context.Background()

	session := &model.IMSession{
		ID:        uuid.New(),
		Platform:  model.IMPlatformFeishu,
		ChatID:    "chat_update",
		ThreadID:  uuid.New(),
		ProjectID: uuid.New(),
		IsActive:  true,
	}

	err = repo.Create(ctx, session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	err = repo.UpdateLastMessageAt(ctx, session.ID)
	if err != nil {
		t.Fatalf("failed to update last message at: %v", err)
	}
}

func TestIMSessionRepository_Deactivate(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	createTable(db)

	repo := NewIMSessionRepository(db)
	ctx := context.Background()

	session := &model.IMSession{
		ID:        uuid.New(),
		Platform:  model.IMPlatformFeishu,
		ChatID:    "chat_deactivate",
		ThreadID:  uuid.New(),
		ProjectID: uuid.New(),
		IsActive:  true,
	}

	err = repo.Create(ctx, session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	err = repo.Deactivate(ctx, session.ID)
	if err != nil {
		t.Fatalf("failed to deactivate: %v", err)
	}

	found, err := repo.FindByChatID(ctx, string(model.IMPlatformFeishu), "chat_deactivate")
	if err != nil {
		t.Fatalf("failed to find session: %v", err)
	}

	if found.IsActive {
		t.Error("expected IsActive to be false")
	}
}

func createTable(db *sql.DB) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS im_sessions (
			id              VARCHAR(36) PRIMARY KEY,
			platform        VARCHAR(20) NOT NULL DEFAULT 'feishu',
			chat_id         VARCHAR(128) NOT NULL,
			chat_type       VARCHAR(20) NOT NULL DEFAULT 'p2p',
			thread_id       VARCHAR(36) NOT NULL,
			project_id      VARCHAR(36) NOT NULL,
			user_id         VARCHAR(128) DEFAULT '',
			user_name       VARCHAR(128) DEFAULT '',
			last_message_at TIMESTAMP NULL,
			is_active       BOOLEAN NOT NULL DEFAULT 1,
			created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(platform, chat_id)
		)
	`)
	if err != nil {
		panic(err)
	}
}
