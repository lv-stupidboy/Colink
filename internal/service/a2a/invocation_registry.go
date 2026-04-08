package a2a

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InvocationRecord 调用记录
type InvocationRecord struct {
	ID                uuid.UUID  // 调用 ID
	ThreadID          uuid.UUID  // 线程 ID
	CatID             string     // Agent ID
	UserID            string     // 用户 ID
	CallbackToken     string     // 回调令牌
	ParentInvocationID *uuid.UUID // A2A 调用链中的父调用
	CreatedAt         time.Time  // 创建时间
	Status            string     // "running", "completed", "cancelled"
}

// InvocationRegistry 调用注册表
// 用于追踪活跃调用、支持过期调用检测
type InvocationRegistry struct {
	records       map[uuid.UUID]*InvocationRecord
	threadLatest  map[uuid.UUID]uuid.UUID // threadID -> latest invocationID
	catLatest     map[string]uuid.UUID    // threadID:catID -> latest invocationID
	tokenRecords  map[string]uuid.UUID    // callbackToken -> invocationID
	mu            sync.RWMutex
}

// NewInvocationRegistry 创建调用注册表
func NewInvocationRegistry() *InvocationRegistry {
	return &InvocationRegistry{
		records:      make(map[uuid.UUID]*InvocationRecord),
		threadLatest: make(map[uuid.UUID]uuid.UUID),
		catLatest:    make(map[string]uuid.UUID),
		tokenRecords: make(map[string]uuid.UUID),
	}
}

// Register 注册新调用
// 返回生成的 callbackToken
func (r *InvocationRegistry) Register(record *InvocationRecord) (string, error) {
	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	record.Status = "running"

	// 生成回调令牌
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	callbackToken := hex.EncodeToString(tokenBytes)
	record.CallbackToken = callbackToken

	r.mu.Lock()
	defer r.mu.Unlock()

	// 存储记录
	r.records[record.ID] = record
	r.tokenRecords[callbackToken] = record.ID

	// 更新线程最新调用
	r.threadLatest[record.ThreadID] = record.ID

	// 更新 slot 最新调用 (threadID:catID)
	slotKey := slotKey(record.ThreadID, record.CatID)
	r.catLatest[slotKey] = record.ID

	return callbackToken, nil
}

// Verify 验证调用并返回记录
// 验证 invocationID + callbackToken 双重匹配
func (r *InvocationRegistry) Verify(invocationID uuid.UUID, token string) *InvocationRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 通过 token 查找 invocationID
	tokenInvocationID, exists := r.tokenRecords[token]
	if !exists {
		return nil
	}

	// 验证 token 对应的 invocationID 是否匹配
	if tokenInvocationID != invocationID {
		return nil
	}

	record, exists := r.records[invocationID]
	if !exists {
		return nil
	}

	return record
}

// IsLatest 检查是否是当前线程最新的调用
// 用于过期调用检测：如果不是最新的，返回 false
func (r *InvocationRegistry) IsLatest(invocationID uuid.UUID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, exists := r.records[invocationID]
	if !exists {
		return false
	}

	// 检查是否是线程最新
	if latestID, exists := r.threadLatest[record.ThreadID]; exists {
		if latestID != invocationID {
			return false
		}
	}

	return true
}

// IsLatestForSlot 检查是否是当前 slot (threadID:catID) 最新的调用
// 用于 per-slot mutex：同一 slot 只有一个活跃调用
func (r *InvocationRegistry) IsLatestForSlot(invocationID uuid.UUID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, exists := r.records[invocationID]
	if !exists {
		return false
	}

	slotKey := slotKey(record.ThreadID, record.CatID)
	if latestID, exists := r.catLatest[slotKey]; exists {
		return latestID == invocationID
	}

	return true
}

// Complete 完成调用
func (r *InvocationRegistry) Complete(invocationID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.records[invocationID]
	if !exists {
		return
	}

	record.Status = "completed"

	// 清理 token 映射
	delete(r.tokenRecords, record.CallbackToken)
}

// Cancel 取消调用
func (r *InvocationRegistry) Cancel(invocationID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, exists := r.records[invocationID]
	if !exists {
		return
	}

	record.Status = "cancelled"

	// 清理 token 映射
	delete(r.tokenRecords, record.CallbackToken)
}

// GetActiveByThread 获取线程中所有活跃调用
func (r *InvocationRegistry) GetActiveByThread(threadID uuid.UUID) []*InvocationRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*InvocationRecord
	for _, record := range r.records {
		if record.ThreadID == threadID && record.Status == "running" {
			result = append(result, record)
		}
	}
	return result
}

// GetActiveSlots 获取线程中活跃的 slots (catIDs)
func (r *InvocationRegistry) GetActiveSlots(threadID uuid.UUID) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []string
	seen := make(map[string]bool)

	for _, record := range r.records {
		if record.ThreadID == threadID && record.Status == "running" {
			if !seen[record.CatID] {
				result = append(result, record.CatID)
				seen[record.CatID] = true
			}
		}
	}
	return result
}

// HasActiveSlot 检查线程中是否有指定 slot 的活跃调用
func (r *InvocationRegistry) HasActiveSlot(threadID uuid.UUID, catID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	slotKey := slotKey(threadID, catID)
	if latestID, exists := r.catLatest[slotKey]; exists {
		if record, exists := r.records[latestID]; exists {
			return record.Status == "running"
		}
	}
	return false
}

// Get 获取调用记录
func (r *InvocationRegistry) Get(invocationID uuid.UUID) *InvocationRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.records[invocationID]
}

// Cleanup 清理过期的记录（可定期调用）
func (r *InvocationRegistry) Cleanup(maxAge time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, record := range r.records {
		if record.CreatedAt.Before(cutoff) || record.Status != "running" {
			delete(r.records, id)
			delete(r.tokenRecords, record.CallbackToken)
		}
	}
}

// slotKey 生成 slot 键
func slotKey(threadID uuid.UUID, catID string) string {
	return threadID.String() + ":" + catID
}

// Errors
var (
	ErrInvocationNotFound = errors.New("invocation not found")
	ErrStaleInvocation    = errors.New("stale invocation")
)