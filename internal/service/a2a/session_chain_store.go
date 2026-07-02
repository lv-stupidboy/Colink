package a2a

import (
	"database/sql"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SessionStatus 会话状态
type SessionStatus string

const (
	SessionStatusActive  SessionStatus = "active"
	SessionStatusSealing SessionStatus = "sealing"
	SessionStatusSealed  SessionStatus = "sealed"
)

// ContextHealth 上下文健康状态
type ContextHealth string

const (
	ContextHealthHealthy   ContextHealth = "healthy"
	ContextHealthDegraded  ContextHealth = "degraded"
	ContextHealthExhausted ContextHealth = "exhausted"
)

// SealReason 封存原因
type SealReason string

const (
	SealReasonThreshold SealReason = "threshold"
	SealReasonManual    SealReason = "manual"
	SealReasonError     SealReason = "error"
)

// SessionRecord 会话记录
// 参考 Clowder AI 的 SessionRecord
type SessionRecord struct {
	ID                       string        `json:"id"`
	CLISessionID             string        `json:"cliSessionId"`
	ThreadID                 string        `json:"threadId"`
	CatID                    string        `json:"catId"`
	UserID                   string        `json:"userId"`
	Seq                      int           `json:"seq"`
	Status                   SessionStatus `json:"status"`
	ContextHealth            ContextHealth `json:"contextHealth,omitempty"`
	LastUsage                *int64        `json:"lastUsage,omitempty"`
	MessageCount             int           `json:"messageCount"`
	SealReason               SealReason    `json:"sealReason,omitempty"`
	SealedAt                 *int64        `json:"sealedAt,omitempty"`
	CompressionCount         int           `json:"compressionCount,omitempty"`
	ConsecutiveRestoreFailures int          `json:"consecutiveRestoreFailures,omitempty"`
	CreatedAt                int64         `json:"createdAt"`
	UpdatedAt                int64         `json:"updatedAt"`
}

// GetConsecutiveRestoreFailures 实现 CircuitBreakerSession 接口
func (r *SessionRecord) GetConsecutiveRestoreFailures() int {
	return r.ConsecutiveRestoreFailures
}

// CreateSessionInput 创建会话输入
type CreateSessionInput struct {
	CLISessionID string
	ThreadID     string
	CatID        string
	UserID       string
}

// SessionRecordPatch 会话记录补丁
type SessionRecordPatch struct {
	CLISessionID             *string
	Status                   *SessionStatus
	ContextHealth            *ContextHealth
	LastUsage                *int64
	MessageCount             *int
	SealReason               *SealReason
	SealedAt                 *int64
	CompressionCount         *int
	ConsecutiveRestoreFailures *int
	UpdatedAt                *int64
}

const maxRecords = 1000

// SessionChainStore 会话链存储
// 参考 Clowder AI 的 SessionChainStore
// 用于管理每个 Agent 在每个线程中的会话链
type SessionChainStore struct {
	records     map[string]*SessionRecord  // id -> record
	chains      map[string][]string        // catId:threadId -> session IDs (ordered by seq)
	activeIndex map[string]string          // catId:threadId -> active session ID
	cliIndex    map[string]string          // cliSessionId -> record ID
	cliSessions map[string]string          // 内存缓存：threadID:configID -> sessionID（新增）
	db          *sql.DB                    // 数据库连接（新增）
	mu          sync.RWMutex
}

// NewSessionChainStore 创建会话链存储
func NewSessionChainStore(db *sql.DB) *SessionChainStore {
	s := &SessionChainStore{
		records:     make(map[string]*SessionRecord),
		chains:      make(map[string][]string),
		activeIndex: make(map[string]string),
		cliIndex:    make(map[string]string),
		cliSessions: make(map[string]string),
		db:          db,
	}
	// 启动时恢复缓存
	// C5 修复：原代码丢弃 RestoreFromDB 返回值，migration 未跑 / 表不存在等错误
	// 全部静默。改为 warn log，便于运维在启动阶段发现。
	if db != nil {
		if err := s.RestoreFromDB(); err != nil {
			zap.L().Warn("RestoreFromDB failed at startup (cliSessions cache empty this session)",
				zap.Error(err))
		}
	}
	return s
}

// chainKey 生成链键
func (s *SessionChainStore) chainKey(catID, threadID string) string {
	return catID + ":" + threadID
}

// Create 创建会话记录
// seq 自动递增，status 默认为 active
func (s *SessionChainStore) Create(input CreateSessionInput) *SessionRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	key := s.chainKey(input.CatID, input.ThreadID)

	// 计算下一个 seq
	chain := s.chains[key]
	seq := len(chain)

	id := uuid.New().String()
	record := &SessionRecord{
		ID:           id,
		CLISessionID: input.CLISessionID,
		ThreadID:     input.ThreadID,
		CatID:        input.CatID,
		UserID:       input.UserID,
		Seq:          seq,
		Status:       SessionStatusActive,
		MessageCount: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.records[id] = record
	chain = append(chain, id)
	s.chains[key] = chain
	s.activeIndex[key] = id
	s.cliIndex[input.CLISessionID] = id

	// 容量检查
	if len(s.records) > maxRecords {
		evicted := s.evictOne()
		if !evicted {
			// 回滚：移除刚创建的记录
			s.removeRecord(id)
			return nil
		}
	}

	return record
}

// Get 根据 ID 获取会话记录
func (s *SessionChainStore) Get(id string) *SessionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.records[id]
}

// GetActive 获取指定 Agent 在指定线程中的活跃会话
func (s *SessionChainStore) GetActive(catID, threadID string) *SessionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.chainKey(catID, threadID)
	activeID := s.activeIndex[key]
	if activeID == "" {
		return nil
	}

	record := s.records[activeID]
	if record == nil || record.Status != SessionStatusActive {
		return nil
	}
	return record
}

// GetChain 获取指定 Agent 在指定线程中的完整会话链（按 seq 排序）
func (s *SessionChainStore) GetChain(catID, threadID string) []*SessionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.chainKey(catID, threadID)
	chain := s.chains[key]

	result := make([]*SessionRecord, 0, len(chain))
	for _, id := range chain {
		if r := s.records[id]; r != nil {
			result = append(result, r)
		}
	}

	// 按 seq 排序
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Seq > result[j].Seq {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// GetChainByThread 获取线程中所有 Agent 的会话
func (s *SessionChainStore) GetChainByThread(threadID string) []*SessionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*SessionRecord, 0)
	for _, record := range s.records {
		if record.ThreadID == threadID {
			result = append(result, record)
		}
	}

	// 按 catId, seq 排序
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].CatID > result[j].CatID ||
				(result[i].CatID == result[j].CatID && result[i].Seq > result[j].Seq) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// Update 更新会话记录的部分字段
func (s *SessionChainStore) Update(id string, patch SessionRecordPatch) *SessionRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := s.records[id]
	if record == nil {
		return nil
	}

	if patch.CLISessionID != nil {
		// 更新 CLI 索引
		delete(s.cliIndex, record.CLISessionID)
		record.CLISessionID = *patch.CLISessionID
		s.cliIndex[*patch.CLISessionID] = id
	}

	if patch.Status != nil {
		record.Status = *patch.Status
		// 如果封存，从活跃索引中移除
		if *patch.Status != SessionStatusActive {
			key := s.chainKey(record.CatID, record.ThreadID)
			if s.activeIndex[key] == id {
				delete(s.activeIndex, key)
			}
		}
	}

	if patch.ContextHealth != nil {
		record.ContextHealth = *patch.ContextHealth
	}
	if patch.LastUsage != nil {
		record.LastUsage = patch.LastUsage
	}
	if patch.MessageCount != nil {
		record.MessageCount = *patch.MessageCount
	}
	if patch.SealReason != nil {
		record.SealReason = *patch.SealReason
	}
	if patch.SealedAt != nil {
		record.SealedAt = patch.SealedAt
	}
	if patch.CompressionCount != nil {
		record.CompressionCount = *patch.CompressionCount
	}
	if patch.ConsecutiveRestoreFailures != nil {
		record.ConsecutiveRestoreFailures = *patch.ConsecutiveRestoreFailures
	}

	if patch.UpdatedAt != nil {
		record.UpdatedAt = *patch.UpdatedAt
	} else {
		record.UpdatedAt = time.Now().UnixMilli()
	}

	return record
}

// GetByCLISessionID 根据 CLI 会话 ID 获取会话记录
func (s *SessionChainStore) GetByCLISessionID(cliSessionID string) *SessionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id := s.cliIndex[cliSessionID]
	if id == "" {
		return nil
	}
	return s.records[id]
}

// IncrementCompressionCount 原子递增压缩计数
func (s *SessionChainStore) IncrementCompressionCount(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := s.records[id]
	if record == nil || record.Status != SessionStatusActive {
		return -1
	}

	record.CompressionCount++
	record.UpdatedAt = time.Now().UnixMilli()
	return record.CompressionCount
}

// IncrementConsecutiveFailures 增加连续恢复失败计数
// 参数: configID (catId), cliSessionID
func (s *SessionChainStore) IncrementConsecutiveFailures(configID, cliSessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	recordID := s.cliIndex[cliSessionID]
	if recordID == "" {
		return
	}

	record := s.records[recordID]
	if record == nil {
		return
	}

	record.ConsecutiveRestoreFailures++
	record.UpdatedAt = time.Now().UnixMilli()
}

// ResetConsecutiveFailures 重置连续恢复失败计数
// 参数: configID (catId), cliSessionID
func (s *SessionChainStore) ResetConsecutiveFailures(configID, cliSessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	recordID := s.cliIndex[cliSessionID]
	if recordID == "" {
		return
	}

	record := s.records[recordID]
	if record == nil {
		return
	}

	record.ConsecutiveRestoreFailures = 0
	record.UpdatedAt = time.Now().UnixMilli()
}

// ListSealingSessions 列出所有处于 sealing 状态的会话 ID
func (s *SessionChainStore) ListSealingSessions() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0)
	for id, record := range s.records {
		if record.Status == SessionStatusSealing {
			ids = append(ids, id)
		}
	}
	return ids
}

// evictOne 驱逐一条记录以保持容量
// 优先级：sealed > non-active > superseded active
func (s *SessionChainStore) evictOne() bool {
	// 收集当前活跃 ID
	currentActiveIDs := make(map[string]bool)
	for _, id := range s.activeIndex {
		currentActiveIDs[id] = true
	}

	// 第一轮：sealed 记录（最安全）
	var victim string
	for id, r := range s.records {
		if r.Status == SessionStatusSealed {
			victim = id
			break
		}
	}

	// 第二轮：non-active, non-sealed (e.g., sealing)
	if victim == "" {
		for id, r := range s.records {
			if r.Status != SessionStatusActive {
				victim = id
				break
			}
		}
	}

	// 第三轮：active 记录但不在 activeIndex 中（已被取代）
	if victim == "" {
		for id := range s.records {
			if !currentActiveIDs[id] {
				victim = id
				break
			}
		}
	}

	// 拒绝驱逐真正活跃的会话
	if victim == "" {
		return false
	}

	s.removeRecord(victim)
	return true
}

// removeRecord 移除记录并清理所有索引
func (s *SessionChainStore) removeRecord(id string) {
	record := s.records[id]
	if record == nil {
		return
	}

	delete(s.cliIndex, record.CLISessionID)

	key := s.chainKey(record.CatID, record.ThreadID)
	if s.activeIndex[key] == id {
		delete(s.activeIndex, key)
	}

	chain := s.chains[key]
	if chain != nil {
		newChain := make([]string, 0, len(chain)-1)
		for _, cid := range chain {
			if cid != id {
				newChain = append(newChain, cid)
			}
		}
		if len(newChain) == 0 {
			delete(s.chains, key)
		} else {
			s.chains[key] = newChain
		}
	}

	delete(s.records, id)
}

// Size 返回当前记录数量（用于测试）
func (s *SessionChainStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// PersistCliSession 持久化CLI session到数据库
//
// C4 修复：改为同步写入。原实现每次 fire-and-forget 起 goroutine，A2A 密集时
// SetMaxOpenConns=1 会导致 goroutine 排队爆炸。同步写在 SQLite 上就是一次
// 微秒级 INSERT OR REPLACE，反而更快、无 goroutine 泄漏风险。
func (s *SessionChainStore) PersistCliSession(threadID, configID, sessionID string) error {
	// 1. 写入内存缓存（快速路径）
	key := threadID + ":" + configID
	s.mu.Lock()
	s.cliSessions[key] = sessionID
	s.mu.Unlock()

	// 2. 同步写入数据库（持久化）
	if s.db != nil {
		if _, err := s.db.Exec(`
			INSERT OR REPLACE INTO cli_session_cache
			(thread_id, config_id, session_id, updated_at)
			VALUES (?, ?, ?, ?)
		`, threadID, configID, sessionID, time.Now()); err != nil {
			zap.L().Warn("cliSessions持久化失败",
				zap.String("threadID", threadID),
				zap.String("configID", configID),
				zap.Error(err))
			return err
		}
	}
	return nil
}

// GetCliSession C3 修复：暴露内存缓存的 cliSessions 读接口，供 ExecutionService
// 在 es.cliSessions 缓存 miss 时兜底查询（进程重启后回填）。
// 返回 "" 表示无缓存。
func (s *SessionChainStore) GetCliSession(threadID, configID string) string {
	key := threadID + ":" + configID
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cliSessions[key]
}

// RestoreFromDB 启动时从数据库恢复缓存
// DefaultRestoreWindowHours restore 窗口默认值。
// C5 修复：从原来硬编码 24h 提升到 7 天，避免用户周末关机后 AI 失忆。
// main.go 可通过 SetRestoreWindow 覆盖此值（配置化）。
const DefaultRestoreWindowHours = 168

var restoreWindowHours = DefaultRestoreWindowHours

// SetRestoreWindowHours 由 main.go 根据 config.yaml 调整 restore 窗口。
// 传 <=0 表示回退到默认值。仅影响 NewSessionChainStore 后续 RestoreFromDB 调用。
func SetRestoreWindowHours(h int) {
	if h > 0 {
		restoreWindowHours = h
	} else {
		restoreWindowHours = DefaultRestoreWindowHours
	}
}

func (s *SessionChainStore) RestoreFromDB() error {
	if s.db == nil {
		return nil
	}

	windowHours := restoreWindowHours
	if windowHours <= 0 {
		windowHours = DefaultRestoreWindowHours
	}

	rows, err := s.db.Query(`
		SELECT thread_id, config_id, session_id
		FROM cli_session_cache
		WHERE updated_at > ?
	`, time.Now().Add(-time.Duration(windowHours)*time.Hour))
	if err != nil {
		return err
	}
	defer rows.Close()

	s.mu.Lock()
	for rows.Next() {
		var threadID, configID, sessionID string
		if err := rows.Scan(&threadID, &configID, &sessionID); err == nil {
			key := threadID + ":" + configID
			s.cliSessions[key] = sessionID
		}
	}
	count := len(s.cliSessions)
	s.mu.Unlock()

	zap.L().Info("cliSessions缓存已恢复",
		zap.Int("count", count),
		zap.Int("windowHours", windowHours))
	return nil
}

// 全局 SessionChainStore 实例
var globalSessionChainStore *SessionChainStore

// InitGlobalSessionChainStore 初始化全局 SessionChainStore（启动时调用）
func InitGlobalSessionChainStore(db *sql.DB) {
	globalSessionChainStore = NewSessionChainStore(db)
}

// GetSessionChainStore 获取全局 SessionChainStore
func GetSessionChainStore() *SessionChainStore {
	if globalSessionChainStore == nil {
		// 兜底：未初始化时创建无数据库实例
		globalSessionChainStore = NewSessionChainStore(nil)
	}
	return globalSessionChainStore
}