package agent

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"

	_ "modernc.org/sqlite"
)

// setupTestDBWithMessages 建一个装了 messages + delivery_cursors 表的临时 SQLite
func setupTestDBWithMessages(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)

	schema := `
	CREATE TABLE threads (
	    id TEXT PRIMARY KEY,
	    project_id TEXT NOT NULL,
	    name TEXT NOT NULL DEFAULT ''
	);
	CREATE TABLE messages (
	    id TEXT PRIMARY KEY,
	    thread_id TEXT NOT NULL,
	    role TEXT NOT NULL,
	    agent_id TEXT DEFAULT NULL,
	    content TEXT,
	    content_blocks TEXT DEFAULT NULL,
	    message_type TEXT DEFAULT 'text',
	    metadata TEXT DEFAULT NULL,
	    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    sortable_id TEXT DEFAULT NULL,
	    mentions TEXT DEFAULT NULL,
	    origin TEXT DEFAULT NULL,
	    reply_to TEXT DEFAULT NULL,
	    reported_at DATETIME NULL
	);
	CREATE INDEX idx_messages_thread_sortable ON messages(thread_id, sortable_id);
	CREATE TABLE delivery_cursors (
	    thread_id  TEXT NOT NULL,
	    agent_id   TEXT NOT NULL,
	    cursor_id  TEXT NOT NULL,
	    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    PRIMARY KEY (thread_id, agent_id)
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	return db, func() { db.Close(); os.RemoveAll(dir) }
}

// insertMsg 便捷插入并返回 sortable_id
func insertMsg(t *testing.T, r *repo.MessageRepository, threadID uuid.UUID, role model.MessageRole, agentID string, content string) string {
	t.Helper()
	msg := &model.Message{
		ThreadID:    threadID,
		Role:        role,
		AgentID:     agentID,
		Content:     content,
		MessageType: model.MessageTypeText,
	}
	if err := r.Create(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	// 时间戳需要错开，保证 sortable_id 严格递增（同 ms 内也 ok，但显式让 ms 分开更稳）
	time.Sleep(1 * time.Millisecond)
	return msg.SortableID
}

// -----------------------------------------------------------------------------
// MessageRepository.GetByThreadAfter
// -----------------------------------------------------------------------------

func TestGetByThreadAfter_EmptyCursor_ReturnsAll(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	r := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	tid := uuid.New()
	insertMsg(t, r, tid, model.MessageRoleUser, "", "m1")
	insertMsg(t, r, tid, model.MessageRoleUser, "", "m2")
	insertMsg(t, r, tid, model.MessageRoleUser, "", "m3")

	msgs, err := r.GetByThreadAfter(context.Background(), tid, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 msgs, got %d", len(msgs))
	}
	if msgs[0].Content != "m1" || msgs[2].Content != "m3" {
		t.Fatalf("wrong order: %q ... %q", msgs[0].Content, msgs[2].Content)
	}
}

func TestGetByThreadAfter_WithCursor(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	r := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	tid := uuid.New()
	insertMsg(t, r, tid, model.MessageRoleUser, "", "m1")
	cursor := insertMsg(t, r, tid, model.MessageRoleUser, "", "m2")
	insertMsg(t, r, tid, model.MessageRoleUser, "", "m3")

	msgs, err := r.GetByThreadAfter(context.Background(), tid, cursor, 0)
	if err != nil {
		t.Fatal(err)
	}
	// 严格大于 cursor：应该只返回 m3
	if len(msgs) != 1 {
		t.Fatalf("expected 1 msg after cursor, got %d", len(msgs))
	}
	if msgs[0].Content != "m3" {
		t.Fatalf("expected m3, got %q", msgs[0].Content)
	}
}

func TestGetByThreadAfter_Limit(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	r := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	tid := uuid.New()
	for i := 0; i < 10; i++ {
		insertMsg(t, r, tid, model.MessageRoleUser, "", "msg")
	}

	msgs, err := r.GetByThreadAfter(context.Background(), tid, "", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("limit=3 should return 3 msgs, got %d", len(msgs))
	}
}

func TestGetByThreadAfter_ThreadIsolation(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	r := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	t1, t2 := uuid.New(), uuid.New()
	insertMsg(t, r, t1, model.MessageRoleUser, "", "t1-msg")
	insertMsg(t, r, t2, model.MessageRoleUser, "", "t2-msg")

	msgs, _ := r.GetByThreadAfter(context.Background(), t1, "", 0)
	if len(msgs) != 1 || msgs[0].Content != "t1-msg" {
		t.Fatalf("thread isolation broken: got %v", msgs)
	}
}

// -----------------------------------------------------------------------------
// AssembleIncrementalContext — 核心组装逻辑
// -----------------------------------------------------------------------------

func TestAssembleIncrementalContext_NilDeps(t *testing.T) {
	res, err := AssembleIncrementalContext(context.Background(), nil, uuid.New(), AssembleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.ContextText != "" || res.BoundaryID != "" {
		t.Fatalf("nil deps should return empty result: %+v", res)
	}
}

func TestAssembleIncrementalContext_NoUnread(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	deps := &IncrementalContextDeps{
		MsgRepo:     repo.NewMessageRepository(db, repo.DBTypeSQLite),
		CursorStore: NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)),
	}
	res, err := AssembleIncrementalContext(context.Background(), deps, uuid.New(), AssembleOptions{
		SelfAgentID: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ContextText != "" || res.BoundaryID != "" {
		t.Fatalf("empty thread should return empty result: %+v", res)
	}
}

func TestAssembleIncrementalContext_FiltersOwnMessages(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	msgRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	deps := &IncrementalContextDeps{
		MsgRepo:     msgRepo,
		CursorStore: NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)),
	}
	tid := uuid.New()
	self := uuid.New()

	// 3 条消息：user / self-agent / other-agent
	insertMsg(t, msgRepo, tid, model.MessageRoleUser, "", "from user")
	insertMsg(t, msgRepo, tid, model.MessageRoleAgent, self.String(), "from self (should filter)")
	insertMsg(t, msgRepo, tid, model.MessageRoleAgent, uuid.New().String(), "from other")

	res, err := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{
		SelfAgentID: self,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.ContextText, "from self") {
		t.Fatalf("self-agent messages should be filtered:\n%s", res.ContextText)
	}
	if !strings.Contains(res.ContextText, "from user") || !strings.Contains(res.ContextText, "from other") {
		t.Fatalf("expected user + other agent messages:\n%s", res.ContextText)
	}
	if res.UnreadCount != 3 {
		t.Fatalf("unread=%d want 3", res.UnreadCount)
	}
	if res.DeliveredCount != 2 {
		t.Fatalf("delivered=%d want 2", res.DeliveredCount)
	}
	if res.BoundaryID == "" {
		t.Fatal("boundary should be set even after filter")
	}
}

func TestAssembleIncrementalContext_FiltersSystemAndEmpty(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	msgRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	deps := &IncrementalContextDeps{
		MsgRepo:     msgRepo,
		CursorStore: NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)),
	}
	tid := uuid.New()

	insertMsg(t, msgRepo, tid, model.MessageRoleSystem, "", "system message")
	insertMsg(t, msgRepo, tid, model.MessageRoleUser, "", "")           // 空内容
	insertMsg(t, msgRepo, tid, model.MessageRoleUser, "", "  \n  ")     // 空白内容
	insertMsg(t, msgRepo, tid, model.MessageRoleUser, "", "real content")

	res, err := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.ContextText, "system message") {
		t.Fatalf("system messages should be filtered:\n%s", res.ContextText)
	}
	if res.DeliveredCount != 1 {
		t.Fatalf("only 'real content' should survive, got delivered=%d\n%s", res.DeliveredCount, res.ContextText)
	}
}

func TestAssembleIncrementalContext_AllFilteredStillReturnsBoundary(t *testing.T) {
	// 借鉴 clowder-ai 关键教训：全过滤空时依然要 ack，否则无限重投
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	msgRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	deps := &IncrementalContextDeps{
		MsgRepo:     msgRepo,
		CursorStore: NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)),
	}
	tid := uuid.New()
	self := uuid.New()

	// 全是 self 或 system，会被完全过滤
	insertMsg(t, msgRepo, tid, model.MessageRoleAgent, self.String(), "self a")
	insertMsg(t, msgRepo, tid, model.MessageRoleAgent, self.String(), "self b")
	insertMsg(t, msgRepo, tid, model.MessageRoleSystem, "", "sys")

	res, err := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{
		SelfAgentID: self,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ContextText != "" {
		t.Fatalf("should be empty when all filtered:\n%s", res.ContextText)
	}
	if res.BoundaryID == "" {
		t.Fatal("boundary MUST be set even when everything is filtered (prevent infinite re-delivery)")
	}
	if res.UnreadCount != 3 {
		t.Fatalf("unread=%d want 3", res.UnreadCount)
	}
	if res.DeliveredCount != 0 {
		t.Fatalf("delivered should be 0")
	}
}

func TestAssembleIncrementalContext_RespectsCursor(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	msgRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	cursorStore := NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite))
	deps := &IncrementalContextDeps{MsgRepo: msgRepo, CursorStore: cursorStore}

	tid := uuid.New()
	self := uuid.New()
	insertMsg(t, msgRepo, tid, model.MessageRoleUser, "", "old1")
	cursorSid := insertMsg(t, msgRepo, tid, model.MessageRoleUser, "", "old2")
	insertMsg(t, msgRepo, tid, model.MessageRoleUser, "", "new1")
	insertMsg(t, msgRepo, tid, model.MessageRoleUser, "", "new2")

	// 把 cursor 推进到 old2
	if err := cursorStore.AckCursor(context.Background(), tid, self, cursorSid); err != nil {
		t.Fatal(err)
	}

	res, err := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{SelfAgentID: self})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.ContextText, "old1") || strings.Contains(res.ContextText, "old2") {
		t.Fatalf("should not pull messages before cursor:\n%s", res.ContextText)
	}
	if !strings.Contains(res.ContextText, "new1") || !strings.Contains(res.ContextText, "new2") {
		t.Fatalf("should pull new1/new2:\n%s", res.ContextText)
	}
	if res.DeliveredCount != 2 {
		t.Fatalf("delivered=%d want 2", res.DeliveredCount)
	}
}

func TestAssembleIncrementalContext_TokenBudget_KeepsTail(t *testing.T) {
	// 保尾丢头：预算不够时保留最新的
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	msgRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	deps := &IncrementalContextDeps{
		MsgRepo:     msgRepo,
		CursorStore: NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)),
	}
	tid := uuid.New()

	// 10 条各 200 字的消息 —— 每条约 50 tokens + 20 overhead = 70
	// MaxTokens=200 应该只能装下最后 2 条（140 tokens），第 3 条超预算
	longBody := strings.Repeat("x", 200)
	for i := 0; i < 10; i++ {
		insertMsg(t, msgRepo, tid, model.MessageRoleUser, "", longBody+"#"+string(rune('0'+i)))
	}

	res, err := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{
		MaxTokens: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Truncated {
		t.Fatal("expected truncation flag when budget exceeded")
	}
	if res.DeliveredCount >= 10 {
		t.Fatalf("all msgs kept unexpectedly: %d", res.DeliveredCount)
	}
	if res.DeliveredCount == 0 {
		t.Fatal("should keep at least the most recent msg")
	}
	// 最后一条 (i=9) 必须在
	if !strings.Contains(res.ContextText, longBody+"#9") {
		t.Fatalf("newest msg (i=9) must be kept:\n%s", res.ContextText[:min(500, len(res.ContextText))])
	}
	// 最老的 (i=0) 不应该在（被裁掉）
	if strings.Contains(res.ContextText, longBody+"#0") {
		t.Fatalf("oldest msg (i=0) should be trimmed:\n%s", res.ContextText[:min(500, len(res.ContextText))])
	}
}

// -----------------------------------------------------------------------------
// CursorBoundaryBuffer — deferred ack
// -----------------------------------------------------------------------------

func TestCursorBoundaryBuffer_UpsertMax(t *testing.T) {
	b := NewCursorBoundaryBuffer()
	aid := uuid.New()

	b.UpsertMax(aid, "0000000000000010-000001-aaaaaaaa")
	b.UpsertMax(aid, "0000000000000005-000001-bbbbbbbb") // 较小
	b.UpsertMax(aid, "0000000000000020-000001-cccccccc") // 较大
	b.UpsertMax(aid, "")                                  // 空 skip

	snap := b.Snapshot()
	if snap[aid] != "0000000000000020-000001-cccccccc" {
		t.Fatalf("should keep max: got %q", snap[aid])
	}
	if b.Size() != 1 {
		t.Fatalf("size=%d want 1", b.Size())
	}
}

func TestCursorBoundaryBuffer_ConcurrentUpsert(t *testing.T) {
	b := NewCursorBoundaryBuffer()
	aid := uuid.New()

	const goroutines = 32
	const perG = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(gg int) {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				// 生成不同的 boundary，最大的保证是 goroutines-1 组的最后一条
				b.UpsertMax(aid, formatBoundary(gg, i))
			}
		}(g)
	}
	wg.Wait()

	// 最大值应该是 gg=goroutines-1, i=perG-1
	want := formatBoundary(goroutines-1, perG-1)
	snap := b.Snapshot()
	if snap[aid] != want {
		t.Fatalf("concurrent upsert should converge to max: want %q, got %q", want, snap[aid])
	}
}

func formatBoundary(gg, i int) string {
	// 构造严格字典可比 boundary（长度固定）
	return "boundary-" +
		padZero(gg, 4) + "-" + padZero(i, 4)
}

func padZero(n, width int) string {
	s := ""
	for x := n; x > 0; x /= 10 {
		s = string(rune('0'+x%10)) + s
	}
	for len(s) < width {
		s = "0" + s
	}
	if s == "" {
		s = strings.Repeat("0", width)
	}
	return s
}

func TestCursorBoundaryBuffer_AckAll(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	store := NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite))
	buf := NewCursorBoundaryBuffer()
	tid := uuid.New()
	a1, a2 := uuid.New(), uuid.New()

	// 用真实 sortable_id 格式的字符串（能通过 CAS 单调检查）
	b1 := "0001700000000000-000001-aaaaaaaa"
	b2 := "0001700000000001-000002-bbbbbbbb"
	buf.UpsertMax(a1, b1)
	buf.UpsertMax(a2, b2)

	if err := buf.AckAll(context.Background(), store, tid); err != nil {
		t.Fatal(err)
	}
	got1, _ := store.GetCursor(context.Background(), tid, a1)
	got2, _ := store.GetCursor(context.Background(), tid, a2)
	if got1 != b1 || got2 != b2 {
		t.Fatalf("acked cursors mismatch: a1=%q a2=%q", got1, got2)
	}
}

// -----------------------------------------------------------------------------
// End-to-end：拉 → assemble → ack → 再拉不到之前的
// -----------------------------------------------------------------------------

func TestAssembleAndAck_EndToEnd(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	msgRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	store := NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite))
	deps := &IncrementalContextDeps{MsgRepo: msgRepo, CursorStore: store}

	tid := uuid.New()
	self := uuid.New()

	// Round 1: 用户先说 hi
	insertMsg(t, msgRepo, tid, model.MessageRoleUser, "", "hi")
	res1, err := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{SelfAgentID: self})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res1.ContextText, "hi") {
		t.Fatalf("round1 should see 'hi':\n%s", res1.ContextText)
	}
	if err := store.AckCursor(context.Background(), tid, self, res1.BoundaryID); err != nil {
		t.Fatal(err)
	}

	// Round 2: 应该看不到 hi 了（cursor 已推进）
	res2, err := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{SelfAgentID: self})
	if err != nil {
		t.Fatal(err)
	}
	if res2.ContextText != "" {
		t.Fatalf("round2 should be empty (cursor advanced):\n%s", res2.ContextText)
	}

	// Round 3: 新消息进来
	insertMsg(t, msgRepo, tid, model.MessageRoleUser, "", "second")
	res3, err := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{SelfAgentID: self})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res3.ContextText, "second") {
		t.Fatalf("round3 should see 'second':\n%s", res3.ContextText)
	}
	if strings.Contains(res3.ContextText, "hi") {
		t.Fatalf("round3 should NOT see 'hi' again:\n%s", res3.ContextText)
	}
}
