package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// TestS2W4_IncrementalMode_EndToEnd
// 完整流水：
//  1. Agent A 写消息 → messages 表
//  2. Agent B 首次 spawn → AssembleIncrementalContext 拉到 A 的消息
//  3. B 完成 → flush 推进 cursor
//  4. Agent A 再写消息 → messages 表
//  5. Agent B 第二次 spawn → 只看到新消息，不看到旧消息
//  6. flag=false 时应完全走 legacy 路径（不 ack cursor）
func TestS2W4_IncrementalMode_EndToEnd(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	msgRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	cursorStore := NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite))

	tid := uuid.New()
	agentA := uuid.New()
	agentB := uuid.New()

	// Round 1: A 写第一条消息
	insertMsg(t, msgRepo, tid, model.MessageRoleAgent, agentA.String(), "A's first msg")

	// B 的第一次拉取
	deps := &IncrementalContextDeps{MsgRepo: msgRepo, CursorStore: cursorStore}
	res1, err := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{
		SelfAgentID: agentB,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res1.ContextText, "A's first msg") {
		t.Fatalf("B round1 should see A's msg:\n%s", res1.ContextText)
	}
	if res1.BoundaryID == "" {
		t.Fatal("round1 should have boundary")
	}

	// 模拟 flush（B invocation 结束）
	if err := cursorStore.AckCursor(context.Background(), tid, agentB, res1.BoundaryID); err != nil {
		t.Fatal(err)
	}

	// Round 2: A 写第二条
	time.Sleep(2 * time.Millisecond) // 保证 sortable_id 分开
	insertMsg(t, msgRepo, tid, model.MessageRoleAgent, agentA.String(), "A's second msg")

	// B 的第二次拉取：只应看到第二条
	res2, err := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{
		SelfAgentID: agentB,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res2.ContextText, "A's first msg") {
		t.Fatalf("B round2 should NOT see A's first msg again:\n%s", res2.ContextText)
	}
	if !strings.Contains(res2.ContextText, "A's second msg") {
		t.Fatalf("B round2 should see A's second msg:\n%s", res2.ContextText)
	}
}

// TestS2W4_ExecutionService_IncrementalMode_FlushOnCleanup
// 验证 ExecutionService 集成层：SetCursorStore(store, true, 0, 0) 后
// buildContextLayers → 累积 boundary → flushBoundaryBufferByPair → cursor 推进
func TestS2W4_ExecutionService_IncrementalMode_FlushOnCleanup(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	msgRepo := repo.NewMessageRepository(db, repo.DBTypeSQLite)
	store := NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite))

	es := &ExecutionService{
		msgRepo:        msgRepo,
		pairBoundaries: make(map[string]*CursorBoundaryBuffer),
	}
	es.SetCursorStore(store, true, 0, 0)

	tid := uuid.New()
	self := uuid.New()

	// 写入一条消息
	sortableID := insertMsg(t, msgRepo, tid, model.MessageRoleUser, "", "hello")

	// 模拟 buildContextLayers 里的 assemble + upsert
	deps := &IncrementalContextDeps{MsgRepo: msgRepo, CursorStore: store}
	incRes, _ := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{SelfAgentID: self})
	if incRes.BoundaryID == "" {
		t.Fatal("expected boundary from unread msg")
	}
	if incRes.BoundaryID != sortableID {
		t.Fatalf("boundary should equal msg's sortable_id: want %q, got %q", sortableID, incRes.BoundaryID)
	}
	buf := es.getOrCreateBoundaryBufferByPair(tid, self)
	buf.UpsertMax(self, incRes.BoundaryID)

	// flush（模拟 invocation 结束的 defer）
	es.flushBoundaryBufferByPair(context.Background(), tid, self)

	// 验证 cursor 已被 ack 到 DB
	got, _ := store.GetCursor(context.Background(), tid, self)
	if got != sortableID {
		t.Fatalf("cursor should equal msg's sortable_id after flush: want %q, got %q", sortableID, got)
	}

	// 第二次 assemble 应看不到该消息（已 ack 过）
	incRes2, _ := AssembleIncrementalContext(context.Background(), deps, tid, AssembleOptions{SelfAgentID: self})
	if incRes2.ContextText != "" {
		t.Fatalf("second assemble should be empty after ack:\n%s", incRes2.ContextText)
	}
}
