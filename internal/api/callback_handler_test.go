package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/internal/service/a2a"
	"github.com/anthropic/isdp/internal/service/message"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestCallbackHandlerPostMessageAuthStaleAndPersistence(t *testing.T) {
	db := openCallbackHandlerTestDB(t)
	registry := a2a.NewInvocationRegistry()
	msgRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	handler := NewCallbackHandler(
		registry,
		nil,
		message.NewService(msgRepo, nil),
		msgRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	router := setupAPILightRouter(handler.RegisterRoutes)

	threadID := uuid.New()
	invocationID := uuid.New()
	token, err := registry.Register(&a2a.InvocationRecord{ID: invocationID, ThreadID: threadID, CatID: "planner"})
	if err != nil {
		t.Fatalf("register invocation: %v", err)
	}
	replyTo := uuid.New()
	okW := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{
		"invocationId":    invocationID.String(),
		"callbackToken":   token,
		"content":         "hello @coder",
		"replyTo":         replyTo.String(),
		"clientMessageId": "client-1",
		"targetCats":      []string{"coder", "reviewer"},
	})
	if okW.Code != http.StatusOK || !bytes.Contains(okW.Body.Bytes(), []byte(`"status":"ok"`)) || !bytes.Contains(okW.Body.Bytes(), []byte("client-1")) {
		t.Fatalf("PostMessage ok code=%d body=%s", okW.Code, okW.Body.String())
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM messages WHERE thread_id = ? AND agent_id = ? AND origin = ?`, threadID.String(), "planner", "callback").Scan(&count); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 1 {
		t.Fatalf("message count = %d", count)
	}
	var mentions []byte
	if err := db.QueryRow(`SELECT mentions FROM messages WHERE thread_id = ?`, threadID.String()).Scan(&mentions); err != nil {
		t.Fatalf("select mentions: %v", err)
	}
	if !json.Valid(mentions) || !bytes.Contains(mentions, []byte("coder")) || !bytes.Contains(mentions, []byte("reviewer")) {
		t.Fatalf("mentions = %s", mentions)
	}

	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing fields code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{"invocationId": "bad", "callbackToken": "x", "content": "x"}); w.Code != http.StatusBadRequest {
		t.Fatalf("bad invocation code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{"invocationId": invocationID.String(), "callbackToken": "wrong", "content": "x"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{"invocationId": invocationID.String(), "callbackToken": token, "content": "x", "threadId": "bad"}); w.Code != http.StatusBadRequest {
		t.Fatalf("bad thread id code=%d", w.Code)
	}

	staleID := uuid.New()
	staleToken, err := registry.Register(&a2a.InvocationRecord{ID: staleID, ThreadID: threadID, CatID: "planner"})
	if err != nil {
		t.Fatalf("register stale invocation: %v", err)
	}
	_, err = registry.Register(&a2a.InvocationRecord{ID: uuid.New(), ThreadID: threadID, CatID: "planner"})
	if err != nil {
		t.Fatalf("register latest invocation: %v", err)
	}
	staleW := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/post-message", map[string]any{
		"invocationId":  staleID.String(),
		"callbackToken": staleToken,
		"content":       "old",
	})
	if staleW.Code != http.StatusOK || !bytes.Contains(staleW.Body.Bytes(), []byte("stale_ignored")) {
		t.Fatalf("stale code=%d body=%s", staleW.Code, staleW.Body.String())
	}

	if _, err := msgRepo.FindByThreadID(context.Background(), threadID, 10); err != nil {
		t.Fatalf("message repo remains usable: %v", err)
	}
}

func TestCallbackHandlerUnavailableMemoryAndInvalidIdentity(t *testing.T) {
	handler := NewCallbackHandler(a2a.NewInvocationRegistry(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := setupAPILightRouter(handler.RegisterRoutes)

	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/memory", map[string]any{}); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("memory unavailable code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodPost, "/api/v1/callbacks/team/list-agents", map[string]any{}); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("list agents unavailable code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/callbacks/pending-mentions?invocationId=bad&callbackToken=x", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("pending mentions bad request code=%d", w.Code)
	}
	if w := performAPILightJSON(router, http.MethodGet, "/api/v1/callbacks/thread-context?invocationId=bad&callbackToken=x", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("thread context bad request code=%d", w.Code)
	}
}

func openCallbackHandlerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE messages (id TEXT PRIMARY KEY, thread_id TEXT NOT NULL, role TEXT NOT NULL, agent_id TEXT, content TEXT, content_blocks BLOB, message_type TEXT, metadata BLOB, created_at TEXT NOT NULL, reported_at TIMESTAMP NULL, mentions BLOB, origin TEXT, reply_to TEXT)`); err != nil {
		t.Fatalf("create messages: %v", err)
	}
	return db
}
