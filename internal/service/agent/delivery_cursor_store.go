// Package agent — DeliveryCursorStore
//
// 借鉴 clowder-ai packages/api/src/domains/cats/services/stores/ports/DeliveryCursorStore.ts
// 的 in-memory cache + 底层持久化的双层结构。差异：
//   - clowder 底层是 Redis（+ Lua CAS），Colink 用 SQL（+ CASE WHEN CAS）
//   - clowder 用 syncMap + LRU=5000；Colink 保持一致，用 container/list 做真 LRU
//
// 语义要点（与 clowder-ai ackCursor L58-104 严格对齐）：
//   1) 读：max(memory, db)  —— 防止 DB 恢复 / 回滚后 memory 领先反被覆盖
//   2) 写：memory 用 effective = max(memory, newCursor)，DB 用 CAS
//   3) CAS 未推进（DB 已有更大值）→ 读 DB 同步 memory 到实际值
//   4) DB 失败 → 只更新 memory，不阻塞主流程
//
// 不变式：cursor 只前进不回退（monotonic ack）。字典序 <= 判断依赖
// pkg/sortid 的 sortable_id 格式。
package agent

import (
	"container/list"
	"context"
	"sync"

	"github.com/anthropic/isdp/internal/repo"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// DefaultMaxCachedCursors LRU 上限，对齐 clowder-ai DeliveryCursorStore.ts:16 MAX_CURSORS = 5000
	DefaultMaxCachedCursors = 5000
)

// DeliveryCursorStore in-memory cache + DB CAS 的组合存储
type DeliveryCursorStore struct {
	repo *repo.DeliveryCursorRepository

	// LRU：list.List 存 *cursorEntry，index 存 key -> *list.Element
	mu       sync.Mutex
	lruList  *list.List
	lruIndex map[string]*list.Element
	maxSize  int
}

type cursorEntry struct {
	key    string // "threadID:agentID"
	cursor string // 当前 cursor 值
}

// NewDeliveryCursorStore 创建 store
// repo 可以为 nil（纯内存模式，用于单元测试或 DB 未就绪场景）
func NewDeliveryCursorStore(r *repo.DeliveryCursorRepository) *DeliveryCursorStore {
	return NewDeliveryCursorStoreWithSize(r, DefaultMaxCachedCursors)
}

// NewDeliveryCursorStoreWithSize 显式指定 LRU 上限（测试用）
func NewDeliveryCursorStoreWithSize(r *repo.DeliveryCursorRepository, maxSize int) *DeliveryCursorStore {
	if maxSize <= 0 {
		maxSize = DefaultMaxCachedCursors
	}
	return &DeliveryCursorStore{
		repo:     r,
		lruList:  list.New(),
		lruIndex: make(map[string]*list.Element),
		maxSize:  maxSize,
	}
}

func cursorCacheKey(threadID, agentID uuid.UUID) string {
	return threadID.String() + ":" + agentID.String()
}

// GetCursor 读 max(memory, db)
//
// 借鉴 clowder-ai DeliveryCursorStore.ts:39-56：
//
//	const memCursor = this.cursors.get(key);
//	if (this.sessionStore) {
//	  const redisCursor = await this.sessionStore.getDeliveryCursor(...);
//	  return memCursor && memCursor > redisCursor ? memCursor : (redisCursor || '');
//	}
//	return memCursor || '';
//
// 未命中返回 ""（对齐 clowder-ai：first-time 拉全 thread）
func (s *DeliveryCursorStore) GetCursor(ctx context.Context, threadID, agentID uuid.UUID) (string, error) {
	key := cursorCacheKey(threadID, agentID)
	memCursor := s.getFromCache(key)

	if s.repo == nil {
		return memCursor, nil
	}

	dbCursor, err := s.repo.Get(ctx, threadID, agentID)
	if err != nil {
		// DB 读失败：降级用 memory（clowder-ai 同样是 fallback 语义）
		if logger := zap.L(); logger != nil {
			logger.Warn("DeliveryCursorStore: DB get failed, falling back to memory",
				zap.String("key", key), zap.Error(err))
		}
		return memCursor, nil
	}

	// max(memory, db)
	if memCursor > dbCursor {
		return memCursor, nil
	}
	// DB 值更大时同步回 memory（memory 落后了）
	if dbCursor != "" && dbCursor > memCursor {
		s.upsertCache(key, dbCursor)
	}
	return dbCursor, nil
}

// AckCursor 单调 ack
//
// 借鉴 clowder-ai ackCursor L58-104：
//   1) effective = max(memCursor, deliveredTo)
//   2) DB CAS → advanced?
//   3) advanced: memory 更新为 effective
//      not advanced: 读 DB 同步 memory（DB 有更高值）
//   4) DB 失败：只更新 memory，不返回错误（宽松语义，允许主流程继续）
//
// 返回值：调用侧一般不关心 error（打日志即可，不阻塞 A2A 主流程）
func (s *DeliveryCursorStore) AckCursor(ctx context.Context, threadID, agentID uuid.UUID, deliveredTo string) error {
	if deliveredTo == "" {
		return nil // 空 cursor 不推进（安全语义）
	}
	key := cursorCacheKey(threadID, agentID)

	memCursor := s.getFromCache(key)
	effective := deliveredTo
	if memCursor > effective {
		effective = memCursor
	}

	if s.repo == nil {
		// 纯内存模式
		if effective > memCursor {
			s.upsertCache(key, effective)
		}
		return nil
	}

	advanced, err := s.repo.CompareAndSet(ctx, threadID, agentID, effective)
	if err != nil {
		// DB 失败：只更新 memory（clowder-ai fallback 语义）
		if effective > memCursor {
			s.upsertCache(key, effective)
		}
		if logger := zap.L(); logger != nil {
			logger.Warn("DeliveryCursorStore: DB CAS failed, kept in memory only",
				zap.String("key", key), zap.String("cursor", effective), zap.Error(err))
		}
		return err
	}

	if advanced {
		s.upsertCache(key, effective)
	} else {
		// DB 已有更高值 —— 拉一次同步 memory 到实际值
		actual, getErr := s.repo.Get(ctx, threadID, agentID)
		if getErr == nil && actual > memCursor {
			s.upsertCache(key, actual)
		}
	}
	return nil
}

// DeleteCursor 移除单个 cursor（用于测试或 Agent 卸载）
func (s *DeliveryCursorStore) DeleteCursor(ctx context.Context, threadID, agentID uuid.UUID) error {
	key := cursorCacheKey(threadID, agentID)
	s.mu.Lock()
	if elem, ok := s.lruIndex[key]; ok {
		s.lruList.Remove(elem)
		delete(s.lruIndex, key)
	}
	s.mu.Unlock()

	if s.repo == nil {
		return nil
	}
	return s.repo.Delete(ctx, threadID, agentID)
}

// DeleteByThread 级联删除
func (s *DeliveryCursorStore) DeleteByThread(ctx context.Context, threadID uuid.UUID) error {
	prefix := threadID.String() + ":"
	s.mu.Lock()
	// 遍历 LRU 找 prefix 匹配的项 —— O(N) 但 N ≤ 5000，可接受
	for key, elem := range s.lruIndex {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			s.lruList.Remove(elem)
			delete(s.lruIndex, key)
		}
	}
	s.mu.Unlock()

	if s.repo == nil {
		return nil
	}
	return s.repo.DeleteByThread(ctx, threadID)
}

// Size 当前 LRU 缓存大小（用于监控 / 测试）
func (s *DeliveryCursorStore) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lruList.Len()
}

// ============================================================================
// 内部：LRU cache helpers
// ============================================================================

// getFromCache 读 + 提到 LRU 队首；未命中返回 ""
func (s *DeliveryCursorStore) getFromCache(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if elem, ok := s.lruIndex[key]; ok {
		s.lruList.MoveToFront(elem)
		return elem.Value.(*cursorEntry).cursor
	}
	return ""
}

// upsertCache 写入 / 更新 —— **只在新值大于当前值时才写**
// 借鉴 clowder-ai upsertMap L176-187：delete-and-reinsert on hit 保 LRU 位置
func (s *DeliveryCursorStore) upsertCache(key, cursor string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if elem, ok := s.lruIndex[key]; ok {
		entry := elem.Value.(*cursorEntry)
		// 单调保护：只前进
		if cursor <= entry.cursor {
			s.lruList.MoveToFront(elem)
			return
		}
		entry.cursor = cursor
		s.lruList.MoveToFront(elem)
		return
	}

	// 新纪录：满了就驱逐最旧
	if s.lruList.Len() >= s.maxSize {
		oldest := s.lruList.Back()
		if oldest != nil {
			s.lruList.Remove(oldest)
			delete(s.lruIndex, oldest.Value.(*cursorEntry).key)
		}
	}

	entry := &cursorEntry{key: key, cursor: cursor}
	elem := s.lruList.PushFront(entry)
	s.lruIndex[key] = elem
}
