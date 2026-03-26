package a2a

import (
	"context"
	"testing"
	"time"
)

func TestSessionChainStore_Create(t *testing.T) {
	store := NewSessionChainStore()

	input := CreateSessionInput{
		CLISessionID: "cli-123",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	}

	record := store.Create(input)
	if record == nil {
		t.Fatal("expected record to be created")
	}

	if record.CLISessionID != "cli-123" {
		t.Errorf("expected CLISessionID cli-123, got %s", record.CLISessionID)
	}
	if record.ThreadID != "thread-1" {
		t.Errorf("expected ThreadID thread-1, got %s", record.ThreadID)
	}
	if record.CatID != "developer" {
		t.Errorf("expected CatID developer, got %s", record.CatID)
	}
	if record.Status != SessionStatusActive {
		t.Errorf("expected Status active, got %s", record.Status)
	}
	if record.Seq != 0 {
		t.Errorf("expected Seq 0, got %d", record.Seq)
	}
}

func TestSessionChainStore_GetActive(t *testing.T) {
	store := NewSessionChainStore()

	// 创建第一个会话
	input1 := CreateSessionInput{
		CLISessionID: "cli-1",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	}
	record1 := store.Create(input1)

	// 获取活跃会话
	active := store.GetActive("developer", "thread-1")
	if active == nil {
		t.Fatal("expected active session")
	}
	if active.ID != record1.ID {
		t.Errorf("expected ID %s, got %s", record1.ID, active.ID)
	}

	// 创建第二个会话（同 cat 同 thread）
	input2 := CreateSessionInput{
		CLISessionID: "cli-2",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	}
	record2 := store.Create(input2)

	// 第二个会话应该是活跃的
	active = store.GetActive("developer", "thread-1")
	if active == nil {
		t.Fatal("expected active session")
	}
	if active.ID != record2.ID {
		t.Errorf("expected ID %s, got %s", record2.ID, active.ID)
	}
	if active.Seq != 1 {
		t.Errorf("expected Seq 1, got %d", active.Seq)
	}
}

func TestSessionChainStore_GetChain(t *testing.T) {
	store := NewSessionChainStore()

	// 创建多个会话
	for i := 0; i < 3; i++ {
		store.Create(CreateSessionInput{
			CLISessionID: "cli-" + string(rune('0'+i)),
			ThreadID:     "thread-1",
			CatID:        "developer",
			UserID:       "user-1",
		})
	}

	chain := store.GetChain("developer", "thread-1")
	if len(chain) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(chain))
	}

	// 验证按 seq 排序
	for i, record := range chain {
		if record.Seq != i {
			t.Errorf("expected Seq %d, got %d", i, record.Seq)
		}
	}
}

func TestSessionChainStore_Update(t *testing.T) {
	store := NewSessionChainStore()

	record := store.Create(CreateSessionInput{
		CLISessionID: "cli-1",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	})

	// 更新状态
	now := time.Now().UnixMilli()
	updated := store.Update(record.ID, SessionRecordPatch{
		Status:    ptrSessionStatus(SessionStatusSealing),
		SealReason: ptrSealReason(SealReasonThreshold),
		UpdatedAt:  &now,
	})

	if updated == nil {
		t.Fatal("expected updated record")
	}
	if updated.Status != SessionStatusSealing {
		t.Errorf("expected Status sealing, got %s", updated.Status)
	}
	if updated.SealReason != SealReasonThreshold {
		t.Errorf("expected SealReason threshold, got %s", updated.SealReason)
	}

	// 验证活跃索引已清除
	active := store.GetActive("developer", "thread-1")
	if active != nil {
		t.Error("expected no active session after sealing")
	}
}

func TestSessionChainStore_GetByCLISessionID(t *testing.T) {
	store := NewSessionChainStore()

	record := store.Create(CreateSessionInput{
		CLISessionID: "cli-special",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	})

	found := store.GetByCLISessionID("cli-special")
	if found == nil {
		t.Fatal("expected to find record by CLI session ID")
	}
	if found.ID != record.ID {
		t.Errorf("expected ID %s, got %s", record.ID, found.ID)
	}

	// 查找不存在的
	notFound := store.GetByCLISessionID("cli-nonexistent")
	if notFound != nil {
		t.Error("expected nil for nonexistent CLI session ID")
	}
}

func TestSessionChainStore_ListSealingSessions(t *testing.T) {
	store := NewSessionChainStore()

	// 创建多个会话
	r1 := store.Create(CreateSessionInput{
		CLISessionID: "cli-1",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	})
	r2 := store.Create(CreateSessionInput{
		CLISessionID: "cli-2",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	})
	r3 := store.Create(CreateSessionInput{
		CLISessionID: "cli-3",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	})

	// 将 r1 和 r2 设置为 sealing
	store.Update(r1.ID, SessionRecordPatch{Status: ptrSessionStatus(SessionStatusSealing)})
	store.Update(r2.ID, SessionRecordPatch{Status: ptrSessionStatus(SessionStatusSealing)})

	sealingIDs := store.ListSealingSessions()
	if len(sealingIDs) != 2 {
		t.Fatalf("expected 2 sealing sessions, got %d", len(sealingIDs))
	}

	// 验证 ID
	foundR1, foundR2 := false, false
	for _, id := range sealingIDs {
		if id == r1.ID {
			foundR1 = true
		}
		if id == r2.ID {
			foundR2 = true
		}
	}
	if !foundR1 || !foundR2 {
		t.Error("expected to find r1 and r2 in sealing sessions")
	}

	// r3 应该不在列表中
	for _, id := range sealingIDs {
		if id == r3.ID {
			t.Error("r3 should not be in sealing sessions")
		}
	}
}

func TestSessionChainStore_IncrementCompressionCount(t *testing.T) {
	store := NewSessionChainStore()

	record := store.Create(CreateSessionInput{
		CLISessionID: "cli-1",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	})

	// 递增压缩计数
	count := store.IncrementCompressionCount(record.ID)
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	count = store.IncrementCompressionCount(record.ID)
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}

	// 验证记录已更新
	updated := store.Get(record.ID)
	if updated.CompressionCount != 2 {
		t.Errorf("expected CompressionCount 2, got %d", updated.CompressionCount)
	}
}

func TestSessionSealer_RequestSeal(t *testing.T) {
	store := NewSessionChainStore()
	sealer := NewSessionSealer(SessionSealerDeps{Store: store})

	record := store.Create(CreateSessionInput{
		CLISessionID: "cli-1",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	})

	// 请求封存
	result := sealer.RequestSeal(context.Background(), record.ID, SealReasonThreshold)
	if !result.Accepted {
		t.Error("expected seal request to be accepted")
	}
	if result.Status != SessionStatusSealing {
		t.Errorf("expected Status sealing, got %s", result.Status)
	}

	// 验证状态已更新
	updated := store.Get(record.ID)
	if updated.Status != SessionStatusSealing {
		t.Errorf("expected Status sealing, got %s", updated.Status)
	}

	// 再次请求应该被拒绝
	result2 := sealer.RequestSeal(context.Background(), record.ID, SealReasonManual)
	if result2.Accepted {
		t.Error("expected second seal request to be rejected")
	}
	if result2.Status != SessionStatusSealing {
		t.Errorf("expected Status sealing, got %s", result2.Status)
	}
}

func TestSessionSealer_Finalize(t *testing.T) {
	store := NewSessionChainStore()
	sealer := NewSessionSealer(SessionSealerDeps{Store: store})

	record := store.Create(CreateSessionInput{
		CLISessionID: "cli-1",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	})

	// 先请求封存
	sealer.RequestSeal(context.Background(), record.ID, SealReasonThreshold)

	// 完成封存
	sealer.Finalize(context.Background(), record.ID)

	// 验证状态
	updated := store.Get(record.ID)
	if updated.Status != SessionStatusSealed {
		t.Errorf("expected Status sealed, got %s", updated.Status)
	}
	if updated.SealedAt == nil {
		t.Error("expected SealedAt to be set")
	}
}

func TestSessionSealer_ReconcileStuck(t *testing.T) {
	store := NewSessionChainStore()
	sealer := NewSessionSealer(SessionSealerDeps{Store: store})

	// 创建多个会话
	r1 := store.Create(CreateSessionInput{
		CLISessionID: "cli-1",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	})
	r2 := store.Create(CreateSessionInput{
		CLISessionID: "cli-2",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	})

	// 将 r1 设置为 sealing（模拟卡住）
	store.Update(r1.ID, SessionRecordPatch{Status: ptrSessionStatus(SessionStatusSealing)})

	// 手动设置旧的 UpdatedAt
	oldTime := time.Now().Add(-10 * time.Minute).UnixMilli()
	store.Update(r1.ID, SessionRecordPatch{UpdatedAt: &oldTime})

	// 调和卡住的会话（maxAge = 1 分钟）
	count := sealer.ReconcileStuck(context.Background(), "developer", "thread-1", 60000)
	if count != 1 {
		t.Errorf("expected 1 reconciled session, got %d", count)
	}

	// 验证 r1 已被封存
	updated := store.Get(r1.ID)
	if updated.Status != SessionStatusSealed {
		t.Errorf("expected Status sealed, got %s", updated.Status)
	}

	// r2 应该仍然是 active
	updated2 := store.Get(r2.ID)
	if updated2.Status != SessionStatusActive {
		t.Errorf("expected r2 Status active, got %s", updated2.Status)
	}
}

func TestSessionSealer_ReconcileAllStuck(t *testing.T) {
	store := NewSessionChainStore()
	sealer := NewSessionSealer(SessionSealerDeps{Store: store})

	// 创建多个会话（不同 cat/thread）
	r1 := store.Create(CreateSessionInput{
		CLISessionID: "cli-1",
		ThreadID:     "thread-1",
		CatID:        "developer",
		UserID:       "user-1",
	})
	r2 := store.Create(CreateSessionInput{
		CLISessionID: "cli-2",
		ThreadID:     "thread-2",
		CatID:        "architect",
		UserID:       "user-1",
	})

	// 设置为 sealing
	store.Update(r1.ID, SessionRecordPatch{Status: ptrSessionStatus(SessionStatusSealing)})
	store.Update(r2.ID, SessionRecordPatch{Status: ptrSessionStatus(SessionStatusSealing)})

	// 设置旧的 UpdatedAt
	oldTime := time.Now().Add(-10 * time.Minute).UnixMilli()
	store.Update(r1.ID, SessionRecordPatch{UpdatedAt: &oldTime})
	store.Update(r2.ID, SessionRecordPatch{UpdatedAt: &oldTime})

	// 全局调和
	count := sealer.ReconcileAllStuck(context.Background(), 60000)
	if count != 2 {
		t.Errorf("expected 2 reconciled sessions, got %d", count)
	}
}