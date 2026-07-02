package agent

import (
	"context"
	"testing"

	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

// TestExecutionService_SetCursorStore_Toggle
// 验证 SetCursorStore 的三种模式：不启用 / 启用 / 关闭
func TestExecutionService_SetCursorStore_Toggle(t *testing.T) {
	es := &ExecutionService{
		pairBoundaries: make(map[string]*CursorBoundaryBuffer),
	}

	if es.IsIncrementalModeEnabled() {
		t.Fatal("default should be disabled")
	}

	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()
	store := NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite))

	// enabled=false → 依然算关闭
	es.SetCursorStore(store, false)
	if es.IsIncrementalModeEnabled() {
		t.Fatal("enabled=false should keep disabled")
	}

	// enabled=true + store!=nil → 开启
	es.SetCursorStore(store, true)
	if !es.IsIncrementalModeEnabled() {
		t.Fatal("enabled=true with store should enable")
	}

	// store=nil → 强制关闭
	es.SetCursorStore(nil, true)
	if es.IsIncrementalModeEnabled() {
		t.Fatal("nil store should force disabled regardless of enabled")
	}
}

// TestExecutionService_BoundaryBuffer_Lifecycle
// 验证 getOrCreateBoundaryBufferByPair + flushBoundaryBufferByPair 生命周期
func TestExecutionService_BoundaryBuffer_Lifecycle(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	store := NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite))
	es := &ExecutionService{
		pairBoundaries: make(map[string]*CursorBoundaryBuffer),
	}
	es.SetCursorStore(store, true)

	tid := uuid.New()
	a1 := uuid.New()

	// 首次调用：新建 buffer
	buf1 := es.getOrCreateBoundaryBufferByPair(tid, a1)
	if buf1 == nil {
		t.Fatal("buffer should be created")
	}
	// 同 (thread, agent) 再调用：返回同一个
	buf2 := es.getOrCreateBoundaryBufferByPair(tid, a1)
	if buf1 != buf2 {
		t.Fatal("same (thread, agent) must return same buffer")
	}

	// 记录 boundary
	b := "0001700000000000-000001-aaaaaaaa"
	buf1.UpsertMax(a1, b)

	// flush：ack 到 DB 并清理 buffer
	es.flushBoundaryBufferByPair(context.Background(), tid, a1)

	got, _ := store.GetCursor(context.Background(), tid, a1)
	if got != b {
		t.Fatalf("flush should ack to store: want %q, got %q", b, got)
	}

	// flush 后 buffer 应从 map 里删除
	key := IdentityKey("", a1.String(), tid.String())
	es.pbMu.Lock()
	_, exists := es.pairBoundaries[key]
	es.pbMu.Unlock()
	if exists {
		t.Fatal("buffer should be removed from map after flush")
	}

	// 重复 flush 不应 panic
	es.flushBoundaryBufferByPair(context.Background(), tid, a1)
}

// TestExecutionService_FlushBoundary_NoOp_WhenDisabled
// cursorStore == nil 时 flush 无副作用
func TestExecutionService_FlushBoundary_NoOp_WhenDisabled(t *testing.T) {
	es := &ExecutionService{
		pairBoundaries: make(map[string]*CursorBoundaryBuffer),
	}

	tid := uuid.New()
	a1 := uuid.New()
	buf := es.getOrCreateBoundaryBufferByPair(tid, a1)
	buf.UpsertMax(a1, "some-boundary")

	// cursorStore == nil → flush 应静默返回
	es.flushBoundaryBufferByPair(context.Background(), tid, a1)

	// buffer 仍应从 map 里被移除（清理逻辑不依赖 store）
	key := IdentityKey("", a1.String(), tid.String())
	es.pbMu.Lock()
	_, exists := es.pairBoundaries[key]
	es.pbMu.Unlock()
	if exists {
		t.Fatal("buffer should be removed even when store is nil")
	}
}

// TestExecutionService_PairKeyIsolation
// 不同 (thread, agent) 对必须完全隔离
func TestExecutionService_PairKeyIsolation(t *testing.T) {
	db, cleanup := setupTestDBWithMessages(t)
	defer cleanup()

	store := NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite))
	es := &ExecutionService{
		pairBoundaries: make(map[string]*CursorBoundaryBuffer),
	}
	es.SetCursorStore(store, true)

	t1, t2 := uuid.New(), uuid.New()
	a1, a2 := uuid.New(), uuid.New()

	b1 := "0001700000000001-000001-aaaaaaaa"
	b2 := "0001700000000002-000002-bbbbbbbb"
	b3 := "0001700000000003-000003-cccccccc"

	es.getOrCreateBoundaryBufferByPair(t1, a1).UpsertMax(a1, b1)
	es.getOrCreateBoundaryBufferByPair(t1, a2).UpsertMax(a2, b2)
	es.getOrCreateBoundaryBufferByPair(t2, a1).UpsertMax(a1, b3)

	// 只 flush (t1, a1)
	es.flushBoundaryBufferByPair(context.Background(), t1, a1)

	got1, _ := store.GetCursor(context.Background(), t1, a1)
	got2, _ := store.GetCursor(context.Background(), t1, a2)
	got3, _ := store.GetCursor(context.Background(), t2, a1)

	if got1 != b1 {
		t.Fatalf("(t1,a1) should be acked: got %q want %q", got1, b1)
	}
	if got2 != "" {
		t.Fatalf("(t1,a2) should NOT be acked: got %q", got2)
	}
	if got3 != "" {
		t.Fatalf("(t2,a1) should NOT be acked: got %q", got3)
	}
}

