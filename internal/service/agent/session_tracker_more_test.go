package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
)

func TestInvocationTrackerLifecycle(t *testing.T) {
	ctx := context.Background()
	db := openExecutionPersistenceDB(t)
	invocationRepo := repo.NewAgentInvocationRepository(db, repo.DBTypeSQLite)
	tracker := NewInvocationTracker(invocationRepo)

	invocationID := uuid.New()
	invocation := &model.AgentInvocation{
		ID:            invocationID,
		ThreadID:      uuid.New(),
		AgentConfigID: uuid.New(),
		Role:          model.AgentRoleDeveloper,
		AgentName:     "Coder",
		Status:        model.InvocationStatusPending,
		Input:         "implement",
		CreatedAt:     time.Now(),
	}
	if err := invocationRepo.Create(ctx, invocation); err != nil {
		t.Fatalf("create invocation: %v", err)
	}

	if err := tracker.StartTracking(ctx, invocation, 0); err != nil {
		t.Fatalf("StartTracking() error = %v", err)
	}
	if invocation.Status != model.InvocationStatusRunning || invocation.StartedAt == nil {
		t.Fatalf("started invocation = %+v", invocation)
	}
	info, err := tracker.GetProcessInfo(invocationID)
	if err != nil || info.Status != "running" || info.PID != 0 {
		t.Fatalf("GetProcessInfo() = %+v err=%v", info, err)
	}
	active := tracker.GetActiveInvocations(invocation.ThreadID)
	if len(active) != 1 || active[0] != invocationID {
		t.Fatalf("GetActiveInvocations() = %v", active)
	}

	if err := tracker.StopTracking(ctx, invocationID, model.InvocationStatusCompleted, "done"); err != nil {
		t.Fatalf("StopTracking() error = %v", err)
	}
	if _, err := tracker.GetProcessInfo(invocationID); !errors.Is(err, ErrProcessNotFound) {
		t.Fatalf("GetProcessInfo(after stop) error = %v, want ErrProcessNotFound", err)
	}
	stopped, err := invocationRepo.FindByID(ctx, invocationID)
	if err != nil {
		t.Fatalf("FindByID(stopped) error = %v", err)
	}
	if stopped.Status != model.InvocationStatusCompleted || stopped.Output != "done" || stopped.CompletedAt == nil {
		t.Fatalf("stopped invocation = %+v", stopped)
	}
}

func TestInvocationTrackerCancelPaths(t *testing.T) {
	ctx := context.Background()
	db := openExecutionPersistenceDB(t)
	invocationRepo := repo.NewAgentInvocationRepository(db, repo.DBTypeSQLite)
	tracker := NewInvocationTracker(invocationRepo)

	if err := tracker.Cancel(ctx, uuid.New()); !errors.Is(err, ErrProcessNotFound) {
		t.Fatalf("Cancel(missing) error = %v, want ErrProcessNotFound", err)
	}

	invocationID := uuid.New()
	invocation := &model.AgentInvocation{
		ID:            invocationID,
		ThreadID:      uuid.New(),
		AgentConfigID: uuid.New(),
		Role:          model.AgentRoleDeveloper,
		AgentName:     "Coder",
		Status:        model.InvocationStatusPending,
		Input:         "implement",
		CreatedAt:     time.Now(),
	}
	if err := invocationRepo.Create(ctx, invocation); err != nil {
		t.Fatalf("create invocation: %v", err)
	}
	if err := tracker.StartTracking(ctx, invocation, 0); err != nil {
		t.Fatalf("StartTracking() error = %v", err)
	}
	if err := tracker.Cancel(ctx, invocationID); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	info, err := tracker.GetProcessInfo(invocationID)
	if err != nil || info.Status != "cancelled" {
		t.Fatalf("process info after cancel = %+v err=%v", info, err)
	}
	cancelled, err := invocationRepo.FindByID(ctx, invocationID)
	if err != nil {
		t.Fatalf("FindByID(cancelled) error = %v", err)
	}
	if cancelled.Status != model.InvocationStatusCancelled || cancelled.CompletedAt == nil {
		t.Fatalf("cancelled invocation = %+v", cancelled)
	}
}

func TestSessionManagerResumeAndSaveLifecycle(t *testing.T) {
	ctx := context.Background()
	threadID := uuid.New()
	agentID := uuid.New()
	repository := newFakeSessionRecordRepo()
	manager := NewSessionManager(repository, SessionManagerConfig{ResumeExpiry: 1, CleanupInterval: 1})
	defer manager.Stop()

	baseAgent := &model.BaseAgent{Type: model.BaseAgentType("open_code")}
	handle, err := manager.GetOrCreateSession(ctx, threadID, agentID, baseAgent)
	if err != nil {
		t.Fatalf("GetOrCreateSession(new resume) error = %v", err)
	}
	resumeHandle, ok := handle.(*ResumeSessionHandle)
	if !ok || resumeHandle.GetStrategy() != SessionStrategyNew || resumeHandle.GetACPSessionID() != "" || resumeHandle.GetAgentType() != baseAgent.Type {
		t.Fatalf("new resume handle = %#v", handle)
	}

	if err := manager.SaveACPSessionID(ctx, threadID.String(), agentID.String(), "acp-session-1", baseAgent.Type); err != nil {
		t.Fatalf("SaveACPSessionID(create) error = %v", err)
	}
	handle, err = manager.GetOrCreateSession(ctx, threadID, agentID, baseAgent)
	if err != nil {
		t.Fatalf("GetOrCreateSession(existing) error = %v", err)
	}
	resumeHandle, ok = handle.(*ResumeSessionHandle)
	if !ok || resumeHandle.GetStrategy() != SessionStrategyResume || resumeHandle.GetACPSessionID() != "acp-session-1" {
		t.Fatalf("existing resume handle = %#v", handle)
	}

	if err := manager.SaveACPSessionID(ctx, threadID.String(), agentID.String(), "acp-session-2", baseAgent.Type); err != nil {
		t.Fatalf("SaveACPSessionID(update) error = %v", err)
	}
	record, _ := repository.FindByThreadAndAgent(ctx, threadID.String(), agentID.String())
	if record.AcpSessionID != "acp-session-2" || record.ResumeExpiry == 0 || record.LastActiveAt == 0 {
		t.Fatalf("updated session record = %+v", record)
	}

	handle, err = manager.GetOrCreateSession(ctx, uuid.New(), uuid.New(), &model.BaseAgent{Type: model.BaseAgentType("custom_agent")})
	if err != nil {
		t.Fatalf("GetOrCreateSession(custom) error = %v", err)
	}
	newHandle, ok := handle.(*NewSessionHandle)
	if !ok || newHandle.GetStrategy() != SessionStrategyNew || newHandle.GetACPSessionID() != "" || newHandle.GetAgentType() != model.BaseAgentType("custom_agent") {
		t.Fatalf("custom new handle = %#v", handle)
	}
}

func TestSessionManagerExpiryTypeMismatchCancelAndCleanup(t *testing.T) {
	ctx := context.Background()
	threadID := uuid.New()
	agentID := uuid.New()
	repository := newFakeSessionRecordRepo()
	manager := NewSessionManager(repository, SessionManagerConfig{ResumeExpiry: 1, CleanupInterval: 1})
	defer func() {
		if err := manager.Shutdown(); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	}()

	expired := &model.SessionRecord{
		ID:           uuid.New(),
		ThreadID:     threadID,
		AgentID:      agentID,
		AgentType:    model.BaseAgentType("open_code"),
		AcpSessionID: "expired-session",
		Status:       "active",
		ResumeExpiry: time.Now().Add(-time.Hour).Unix(),
		LastActiveAt: time.Now().Add(-2 * time.Hour).Unix(),
	}
	if err := repository.Create(ctx, expired); err != nil {
		t.Fatalf("create expired record: %v", err)
	}
	handle, err := manager.GetOrCreateSession(ctx, threadID, agentID, &model.BaseAgent{Type: model.BaseAgentType("open_code")})
	if err != nil {
		t.Fatalf("GetOrCreateSession(expired) error = %v", err)
	}
	if resumeHandle := handle.(*ResumeSessionHandle); resumeHandle.GetStrategy() != SessionStrategyNew || resumeHandle.GetACPSessionID() != "" {
		t.Fatalf("expired handle = %#v", resumeHandle)
	}
	if _, err := repository.FindByID(ctx, expired.ID); err == nil {
		t.Fatal("expired record should be deleted")
	}

	mismatched := &model.SessionRecord{
		ID:           uuid.New(),
		ThreadID:     threadID,
		AgentID:      agentID,
		AgentType:    model.BaseAgentType("claude_code"),
		AcpSessionID: "claude-session",
		Status:       "active",
		LastActiveAt: time.Now().Unix(),
	}
	mismatched.SetResumeExpiry(1)
	if err := repository.Create(ctx, mismatched); err != nil {
		t.Fatalf("create mismatched record: %v", err)
	}
	handle, err = manager.GetOrCreateSession(ctx, threadID, agentID, &model.BaseAgent{Type: model.BaseAgentType("open_code")})
	if err != nil {
		t.Fatalf("GetOrCreateSession(mismatch) error = %v", err)
	}
	if resumeHandle := handle.(*ResumeSessionHandle); resumeHandle.GetStrategy() != SessionStrategyNew {
		t.Fatalf("mismatch handle = %#v", resumeHandle)
	}
	if _, err := repository.FindByID(ctx, mismatched.ID); err == nil {
		t.Fatal("mismatched record should be deleted")
	}

	if err := manager.SaveACPSessionID(ctx, threadID.String(), agentID.String(), "active-session", model.BaseAgentType("open_code")); err != nil {
		t.Fatalf("SaveACPSessionID(active) error = %v", err)
	}
	if err := manager.Cancel(ctx, threadID.String(), agentID.String(), model.BaseAgentType("open_code")); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if record, _ := repository.FindByThreadAndAgent(ctx, threadID.String(), agentID.String()); record != nil {
		t.Fatalf("record should be deleted after cancel: %+v", record)
	}
	if err := manager.Cancel(ctx, threadID.String(), agentID.String(), model.BaseAgentType("open_code")); err != nil {
		t.Fatalf("Cancel(missing) error = %v", err)
	}

	old := &model.SessionRecord{
		ID:           uuid.New(),
		ThreadID:     uuid.New(),
		AgentID:      uuid.New(),
		AgentType:    model.BaseAgentType("open_code"),
		AcpSessionID: "old",
		ResumeExpiry: time.Now().Add(-3 * time.Hour).Unix(),
	}
	if err := repository.Create(ctx, old); err != nil {
		t.Fatalf("create old record: %v", err)
	}
	manager.cleanupExpiredRecords(ctx)
	if _, err := repository.FindByID(ctx, old.ID); err == nil {
		t.Fatal("cleanup should delete expired records")
	}
	if metrics := manager.GetMetrics(ctx); metrics.ActiveSessions != 0 || metrics.TotalSessionsCreated != 0 {
		t.Fatalf("metrics = %+v", metrics)
	}
}

type fakeSessionRecordRepo struct {
	records map[uuid.UUID]*model.SessionRecord
}

func newFakeSessionRecordRepo() *fakeSessionRecordRepo {
	return &fakeSessionRecordRepo{records: make(map[uuid.UUID]*model.SessionRecord)}
}

func (f *fakeSessionRecordRepo) Create(ctx context.Context, record *model.SessionRecord) error {
	clone := *record
	if clone.ID == uuid.Nil {
		clone.ID = uuid.New()
	}
	if clone.CreatedAt == 0 {
		_ = clone.BeforeCreate()
	}
	f.records[clone.ID] = &clone
	record.ID = clone.ID
	record.CreatedAt = clone.CreatedAt
	record.UpdatedAt = clone.UpdatedAt
	record.LastActiveAt = clone.LastActiveAt
	return nil
}

func (f *fakeSessionRecordRepo) Update(ctx context.Context, record *model.SessionRecord) error {
	if _, ok := f.records[record.ID]; !ok {
		return errors.New("record not found")
	}
	clone := *record
	_ = clone.BeforeUpdate()
	f.records[clone.ID] = &clone
	record.UpdatedAt = clone.UpdatedAt
	return nil
}

func (f *fakeSessionRecordRepo) Delete(ctx context.Context, id uuid.UUID) error {
	delete(f.records, id)
	return nil
}

func (f *fakeSessionRecordRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.SessionRecord, error) {
	record, ok := f.records[id]
	if !ok {
		return nil, errors.New("record not found")
	}
	clone := *record
	return &clone, nil
}

func (f *fakeSessionRecordRepo) FindByThreadAndAgent(ctx context.Context, threadID, agentID string) (*model.SessionRecord, error) {
	for _, record := range f.records {
		if record.ThreadID.String() == threadID && record.AgentID.String() == agentID {
			clone := *record
			return &clone, nil
		}
	}
	return nil, errors.New("record not found")
}

func (f *fakeSessionRecordRepo) FindExpiredRecords(ctx context.Context, expiryDuration time.Duration) ([]*model.SessionRecord, error) {
	threshold := time.Now().Add(-expiryDuration).Unix()
	var expired []*model.SessionRecord
	for _, record := range f.records {
		if record.ResumeExpiry < threshold {
			clone := *record
			expired = append(expired, &clone)
		}
	}
	return expired, nil
}

func (f *fakeSessionRecordRepo) DeleteExpiredRecords(ctx context.Context, expiryDuration time.Duration) error {
	threshold := time.Now().Add(-expiryDuration).Unix()
	for id, record := range f.records {
		if record.ResumeExpiry < threshold {
			delete(f.records, id)
		}
	}
	return nil
}

func (f *fakeSessionRecordRepo) CountByThread(ctx context.Context, threadID string) (int, error) {
	count := 0
	for _, record := range f.records {
		if record.ThreadID.String() == threadID {
			count++
		}
	}
	return count, nil
}

func (f *fakeSessionRecordRepo) CountByAgentType(ctx context.Context, agentType model.BaseAgentType) (int, error) {
	count := 0
	for _, record := range f.records {
		if record.AgentType == agentType {
			count++
		}
	}
	return count, nil
}
