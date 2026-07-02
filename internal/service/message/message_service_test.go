package message

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type recordingSpawner struct {
	calls chan spawnCall
}

type spawnCall struct {
	threadID uuid.UUID
	content  string
	images   []model.ImageContent
}

func (s *recordingSpawner) SpawnAgentForUserMessage(ctx context.Context, threadID uuid.UUID, userMessage string, images []model.ImageContent) error {
	s.calls <- spawnCall{threadID: threadID, content: userMessage, images: images}
	return nil
}

func TestMessageServiceCreateImagesAndQueries(t *testing.T) {
	ctx := context.Background()
	db := openMessageTestDB(t)
	service := NewService(repo.NewMessageRepository(db, repo.DBTypeSQLite), nil)
	spawner := &recordingSpawner{calls: make(chan spawnCall, 1)}
	service.SetAgentSpawner(spawner)
	threadID := uuid.New()

	userMsg, err := service.CreateWithImages(ctx, threadID, model.MessageRoleUser, "", "hello", []model.ImageContent{
		{MimeType: "image/png", Data: "abc123"},
	}, false)
	if err != nil {
		t.Fatalf("CreateWithImages user returned error: %v", err)
	}
	if userMsg.ID == uuid.Nil || len(userMsg.ContentBlocks) == 0 {
		t.Fatalf("user message missing id/content blocks: %#v", userMsg)
	}
	var blocks []map[string]interface{}
	if err := json.Unmarshal(userMsg.ContentBlocks, &blocks); err != nil {
		t.Fatalf("unmarshal content blocks: %v", err)
	}
	if len(blocks) != 2 || blocks[0]["type"] != "text" || blocks[1]["richType"] != "media_gallery" {
		t.Fatalf("content blocks = %#v", blocks)
	}
	images := blocks[1]["images"].([]interface{})
	if len(images) != 1 || images[0].(map[string]interface{})["url"] != "data:image/png;base64,abc123" {
		t.Fatalf("image block = %#v", images)
	}

	select {
	case call := <-spawner.calls:
		if call.threadID != threadID || call.content != "hello" || len(call.images) != 1 {
			t.Fatalf("spawn call = %#v", call)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("agent spawner was not called")
	}

	agentMsg, err := service.Create(ctx, threadID, model.MessageRoleAgent, "agent-1", "answer", true)
	if err != nil {
		t.Fatalf("Create agent returned error: %v", err)
	}
	if agentMsg.MessageType != model.MessageTypeText || agentMsg.AgentID != "agent-1" {
		t.Fatalf("agent message = %#v", agentMsg)
	}
	if _, err := service.Create(ctx, threadID, model.MessageRoleUser, "", "skip spawn", true); err != nil {
		t.Fatalf("Create skip spawn returned error: %v", err)
	}
	select {
	case call := <-spawner.calls:
		t.Fatalf("unexpected spawn call after skip: %#v", call)
	case <-time.After(50 * time.Millisecond):
	}

	messages, err := service.GetByThreadID(ctx, threadID, 0)
	if err != nil {
		t.Fatalf("GetByThreadID returned error: %v", err)
	}
	if len(messages) != 3 || messages[0].ID != userMsg.ID || messages[1].ID != agentMsg.ID {
		t.Fatalf("messages = %#v", messages)
	}
	count, err := service.GetMessageCount(ctx, threadID)
	if err != nil || count != 3 {
		t.Fatalf("count = %d err=%v", count, err)
	}
	before, err := service.GetByThreadIDBeforeCursor(ctx, threadID, agentMsg.ID.String(), 0)
	if err != nil {
		t.Fatalf("GetByThreadIDBeforeCursor returned error: %v", err)
	}
	if len(before) != 1 || before[0].ID != userMsg.ID {
		t.Fatalf("before cursor = %#v", before)
	}
	if _, err := service.GetByThreadIDBeforeCursor(ctx, threadID, uuid.New().String(), 10); err == nil {
		t.Fatalf("missing cursor should fail")
	}
}

func TestJsonMarshalHelper(t *testing.T) {
	data, err := jsonMarshal(map[string]string{"a": "b"})
	if err != nil || string(data) != `{"a":"b"}` {
		t.Fatalf("jsonMarshal = %s err=%v", data, err)
	}
}

func openMessageTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE messages (
		id TEXT PRIMARY KEY,
		thread_id TEXT NOT NULL,
		role TEXT NOT NULL,
		agent_id TEXT,
		content TEXT,
		content_blocks BLOB,
		message_type TEXT,
		metadata BLOB,
		created_at TIMESTAMP,
		reported_at TIMESTAMP,
		mentions BLOB,
		origin TEXT,
		reply_to TEXT
	)`)
	if err != nil {
		t.Fatalf("create messages table: %v", err)
	}
	return db
}
