package agent

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/anthropic/isdp/internal/repo"
	"github.com/anthropic/isdp/pkg/sortid"
	"github.com/google/uuid"

	_ "modernc.org/sqlite"
)

// setupTestDB creates a fresh SQLite DB with the delivery_cursors schema.
func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)

	// Inline schema —— 与 sql-change/v1.3.4/sqlite/00049 保持一致
	schema := `
	CREATE TABLE delivery_cursors (
	    thread_id  TEXT NOT NULL,
	    agent_id   TEXT NOT NULL,
	    cursor_id  TEXT NOT NULL,
	    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    PRIMARY KEY (thread_id, agent_id)
	);
	CREATE INDEX idx_delivery_cursors_updated ON delivery_cursors(updated_at);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(dir)
	}
	return db, cleanup
}

// -----------------------------------------------------------------------------
// DeliveryCursorRepository — DB CAS 语义
// -----------------------------------------------------------------------------

func TestDeliveryCursorRepo_GetEmpty(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	r := repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)
	cursor, err := r.Get(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if cursor != "" {
		t.Fatalf("expected empty cursor for missing row, got %q", cursor)
	}
}

func TestDeliveryCursorRepo_CompareAndSet_Insert(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	r := repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)
	ctx := context.Background()
	tid, aid := uuid.New(), uuid.New()

	// 首次 CAS
	c1 := sortid.New()
	advanced, err := r.CompareAndSet(ctx, tid, aid, c1)
	if err != nil {
		t.Fatal(err)
	}
	if !advanced {
		t.Fatal("first CAS should advance")
	}

	got, _ := r.Get(ctx, tid, aid)
	if got != c1 {
		t.Fatalf("expected cursor=%q, got %q", c1, got)
	}
}

func TestDeliveryCursorRepo_CompareAndSet_Monotonic(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	r := repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)
	ctx := context.Background()
	tid, aid := uuid.New(), uuid.New()

	// 先写 c2（较大）
	c1, c2 := sortid.New(), sortid.New()
	_, _ = r.CompareAndSet(ctx, tid, aid, c2)

	// 尝试用 c1（较小）覆盖 → 应不 advance，且 DB 值仍是 c2
	advanced, err := r.CompareAndSet(ctx, tid, aid, c1)
	if err != nil {
		t.Fatal(err)
	}
	if advanced {
		t.Fatal("older cursor should NOT advance")
	}

	got, _ := r.Get(ctx, tid, aid)
	if got != c2 {
		t.Fatalf("db should still hold c2=%q, got %q", c2, got)
	}
}

func TestDeliveryCursorRepo_CompareAndSet_EmptyCursorRejected(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	r := repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)
	_, err := r.CompareAndSet(context.Background(), uuid.New(), uuid.New(), "")
	if err == nil {
		t.Fatal("expected error for empty cursor")
	}
}

func TestDeliveryCursorRepo_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	r := repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)
	ctx := context.Background()
	tid, aid := uuid.New(), uuid.New()

	c1 := sortid.New()
	_, _ = r.CompareAndSet(ctx, tid, aid, c1)

	if err := r.Delete(ctx, tid, aid); err != nil {
		t.Fatal(err)
	}
	got, _ := r.Get(ctx, tid, aid)
	if got != "" {
		t.Fatalf("expected empty after delete, got %q", got)
	}
}

func TestDeliveryCursorRepo_DeleteByThread(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	r := repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)
	ctx := context.Background()
	tid := uuid.New()
	a1, a2 := uuid.New(), uuid.New()
	otherTid := uuid.New()
	a3 := uuid.New()

	c := sortid.New()
	_, _ = r.CompareAndSet(ctx, tid, a1, c)
	_, _ = r.CompareAndSet(ctx, tid, a2, c)
	_, _ = r.CompareAndSet(ctx, otherTid, a3, c) // 隔壁 thread 的记录

	if err := r.DeleteByThread(ctx, tid); err != nil {
		t.Fatal(err)
	}
	got1, _ := r.Get(ctx, tid, a1)
	got2, _ := r.Get(ctx, tid, a2)
	got3, _ := r.Get(ctx, otherTid, a3)

	if got1 != "" || got2 != "" {
		t.Fatalf("target thread cursors should be gone, got %q %q", got1, got2)
	}
	if got3 != c {
		t.Fatalf("other thread's cursor should survive, got %q", got3)
	}
}

// -----------------------------------------------------------------------------
// DeliveryCursorStore — memory + DB 组合语义
// -----------------------------------------------------------------------------

func TestDeliveryCursorStore_MemoryOnlyMode(t *testing.T) {
	// repo == nil：纯内存模式（用于测试或 DB 未就绪场景）
	s := NewDeliveryCursorStore(nil)
	ctx := context.Background()
	tid, aid := uuid.New(), uuid.New()

	got, _ := s.GetCursor(ctx, tid, aid)
	if got != "" {
		t.Fatalf("empty cache should return empty, got %q", got)
	}

	c1 := sortid.New()
	if err := s.AckCursor(ctx, tid, aid, c1); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetCursor(ctx, tid, aid)
	if got != c1 {
		t.Fatalf("expected cursor=%q, got %q", c1, got)
	}
}

func TestDeliveryCursorStore_MonotonicAck(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	s := NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite))
	ctx := context.Background()
	tid, aid := uuid.New(), uuid.New()

	c1, c2 := sortid.New(), sortid.New() // c2 > c1
	_ = s.AckCursor(ctx, tid, aid, c2)
	_ = s.AckCursor(ctx, tid, aid, c1) // 试图回退

	got, _ := s.GetCursor(ctx, tid, aid)
	if got != c2 {
		t.Fatalf("cursor should not regress: expected %q, got %q", c2, got)
	}
}

func TestDeliveryCursorStore_MaxMemDB(t *testing.T) {
	// 场景：DB 里有值，内存没有 —— 读时应从 DB 补齐 memory
	db, cleanup := setupTestDB(t)
	defer cleanup()

	r := repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite)
	ctx := context.Background()
	tid, aid := uuid.New(), uuid.New()

	// 先 DB 写入 c1（绕过 store，模拟"重启后 DB 有值 memory 空"）
	c1 := sortid.New()
	_, _ = r.CompareAndSet(ctx, tid, aid, c1)

	s := NewDeliveryCursorStore(r)
	got, _ := s.GetCursor(ctx, tid, aid)
	if got != c1 {
		t.Fatalf("expected memory to backfill from DB: want %q, got %q", c1, got)
	}
	// 第二次读走 memory 路径
	got, _ = s.GetCursor(ctx, tid, aid)
	if got != c1 {
		t.Fatalf("second read should return same value: %q", got)
	}
}

func TestDeliveryCursorStore_DBFailure_FallbackToMemory(t *testing.T) {
	// 关闭 DB 后再操作 —— 应降级到 memory 不 panic
	db, cleanup := setupTestDB(t)
	defer cleanup()

	s := NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite))
	ctx := context.Background()
	tid, aid := uuid.New(), uuid.New()

	c1 := sortid.New()
	_ = s.AckCursor(ctx, tid, aid, c1)

	// 关掉 DB，再读应该还能从 memory 拿到
	db.Close()
	got, err := s.GetCursor(ctx, tid, aid)
	// GetCursor 内部 catch 了 DB 错误，err 应该是 nil 或 memory cursor
	if err != nil {
		t.Logf("GetCursor returned err after DB close (acceptable): %v", err)
	}
	if got != c1 {
		t.Fatalf("expected memory fallback to return %q, got %q", c1, got)
	}
}

// -----------------------------------------------------------------------------
// LRU 驱逐
// -----------------------------------------------------------------------------

func TestDeliveryCursorStore_LRU_Eviction(t *testing.T) {
	const maxSize = 3
	s := NewDeliveryCursorStoreWithSize(nil, maxSize)
	ctx := context.Background()

	tids := make([]uuid.UUID, 5)
	cursors := make([]string, 5)
	for i := range tids {
		tids[i] = uuid.New()
		cursors[i] = sortid.New()
		_ = s.AckCursor(ctx, tids[i], uuid.Nil, cursors[i])
	}

	// 只有最近 3 条应该留在 cache 里
	if s.Size() != maxSize {
		t.Fatalf("LRU size should stay at %d, got %d", maxSize, s.Size())
	}

	// tids[0], tids[1] 应被驱逐
	if got, _ := s.GetCursor(ctx, tids[0], uuid.Nil); got != "" {
		t.Fatalf("tids[0] should have been evicted, got %q", got)
	}
	if got, _ := s.GetCursor(ctx, tids[1], uuid.Nil); got != "" {
		t.Fatalf("tids[1] should have been evicted, got %q", got)
	}
	// tids[2..4] 应还在
	for i := 2; i < 5; i++ {
		if got, _ := s.GetCursor(ctx, tids[i], uuid.Nil); got != cursors[i] {
			t.Fatalf("tids[%d] should still be cached, got %q want %q", i, got, cursors[i])
		}
	}
}

// -----------------------------------------------------------------------------
// 并发 CAS
// -----------------------------------------------------------------------------

func TestDeliveryCursorStore_ConcurrentAck_Monotonic(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	s := NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite))
	ctx := context.Background()
	tid, aid := uuid.New(), uuid.New()

	// 生成一组按字典序递增的 cursor
	const n = 200
	cursors := make([]string, n)
	for i := 0; i < n; i++ {
		cursors[i] = sortid.New()
	}

	// 32 个 goroutine 乱序 ack，最终值必须 == cursors[n-1]（最大的）
	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	var counter int64
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for {
				i := atomic.AddInt64(&counter, 1) - 1
				if i >= int64(n) {
					return
				}
				_ = s.AckCursor(ctx, tid, aid, cursors[i])
			}
		}()
	}
	wg.Wait()

	got, _ := s.GetCursor(ctx, tid, aid)
	if got != cursors[n-1] {
		t.Fatalf("concurrent ack should converge to max cursor:\n  want %q\n  got  %q", cursors[n-1], got)
	}
}

// -----------------------------------------------------------------------------
// DeleteByThread 级联
// -----------------------------------------------------------------------------

func TestDeliveryCursorStore_DeleteByThread(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	s := NewDeliveryCursorStore(repo.NewDeliveryCursorRepository(db, repo.DBTypeSQLite))
	ctx := context.Background()

	tid := uuid.New()
	a1, a2 := uuid.New(), uuid.New()
	otherTid := uuid.New()
	a3 := uuid.New()

	c := sortid.New()
	_ = s.AckCursor(ctx, tid, a1, c)
	_ = s.AckCursor(ctx, tid, a2, c)
	_ = s.AckCursor(ctx, otherTid, a3, c)

	if err := s.DeleteByThread(ctx, tid); err != nil {
		t.Fatal(err)
	}

	// 目标 thread 下的都空
	if got, _ := s.GetCursor(ctx, tid, a1); got != "" {
		t.Fatalf("tid/a1 should be gone, got %q", got)
	}
	if got, _ := s.GetCursor(ctx, tid, a2); got != "" {
		t.Fatalf("tid/a2 should be gone, got %q", got)
	}
	// 其他 thread 的存活
	if got, _ := s.GetCursor(ctx, otherTid, a3); got != c {
		t.Fatalf("otherTid/a3 should survive, got %q", got)
	}
}

// -----------------------------------------------------------------------------
// cursorCacheKey 格式稳定
// -----------------------------------------------------------------------------

func TestCursorCacheKey_Format(t *testing.T) {
	tid := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	aid := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	got := cursorCacheKey(tid, aid)
	want := "00000000-0000-0000-0000-000000000001:00000000-0000-0000-0000-000000000002"
	if got != want {
		t.Fatalf("cursorCacheKey format changed: got %q, want %q", got, want)
	}
	// 顺序敏感
	if cursorCacheKey(tid, aid) == cursorCacheKey(aid, tid) {
		t.Fatal("threadID/agentID order must matter")
	}
	// 内部无双冒号可歧义
	if strings.Count(got, ":") != 1 {
		t.Fatalf("key should contain exactly one ':' separator, got %q", got)
	}
}
