// Package repo — DeliveryCursorRepository
//
// 借鉴 clowder-ai DeliveryCursorStore.ts + Redis SessionStore.setDeliveryCursor
// 用 SQL CAS 语义（CASE WHEN）替代 Redis Lua CAS：
//
//   INSERT INTO delivery_cursors (thread_id, agent_id, cursor_id, updated_at)
//   VALUES (?, ?, ?, ?)
//   ON CONFLICT (thread_id, agent_id) DO UPDATE SET
//     cursor_id  = CASE WHEN excluded.cursor_id > delivery_cursors.cursor_id
//                       THEN excluded.cursor_id
//                       ELSE delivery_cursors.cursor_id END,
//     updated_at = CASE WHEN excluded.cursor_id > delivery_cursors.cursor_id
//                       THEN excluded.updated_at
//                       ELSE delivery_cursors.updated_at END
//
// 语义：
//   - Insert：新纪录 → advanced=true
//   - Update：CASE 表达式确保 cursor_id 单调递增（字典序）
//   - advanced 用一次二次查询判定，因 SQLite ON CONFLICT RETURNING 支持有限
//
// SetMaxOpenConns(1) 保证单进程内的顺序 —— 无需事务包裹整个 CAS。
package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// DeliveryCursorRepository 投递游标 DB 层
type DeliveryCursorRepository struct {
	BaseRepository
}

// NewDeliveryCursorRepository 构造
func NewDeliveryCursorRepository(db *sql.DB, dbType DBType) *DeliveryCursorRepository {
	return &DeliveryCursorRepository{
		BaseRepository: NewBaseRepository(db, dbType),
	}
}

// Get 读取当前 cursor
// 未找到时返回 "" + nil error（不 wrap sql.ErrNoRows —— 调用侧不必依赖 errors.Is）
func (r *DeliveryCursorRepository) Get(ctx context.Context, threadID, agentID uuid.UUID) (string, error) {
	query := `SELECT cursor_id FROM delivery_cursors WHERE thread_id = ? AND agent_id = ?`
	var cursor string
	err := r.DB().QueryRowContext(ctx, query, threadID.String(), agentID.String()).Scan(&cursor)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return cursor, nil
}

// CompareAndSet 单调 upsert
//
// 返回 advanced：true 表示 DB 端 cursor_id 实际前进了；false 表示 newCursor <=
// 当前值 CAS 未推进（可能因为并发另一个 goroutine 已经写入了更高的值）。
//
// 语义细节（借鉴 clowder-ai ackCursor L79-91）：
//   - advanced=false 时调用侧应该拉当前值同步 memory（防止 memory 落后于 DB）
//   - advanced=true 时调用侧应把 memory 更新为 newCursor
func (r *DeliveryCursorRepository) CompareAndSet(ctx context.Context, threadID, agentID uuid.UUID, newCursor string) (advanced bool, err error) {
	if newCursor == "" {
		return false, errors.New("cursor must not be empty")
	}

	// 尝试读现值判断是否要推进（这里不加事务，依赖 SetMaxOpenConns(1) 的隐式串行）
	// 若 DB 未来切换到高并发驱动，需换用 UPDATE ... WHERE + affected rows 或显式事务
	now := time.Now()
	query := `
		INSERT INTO delivery_cursors (thread_id, agent_id, cursor_id, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(thread_id, agent_id) DO UPDATE SET
			cursor_id  = CASE WHEN excluded.cursor_id > delivery_cursors.cursor_id
				THEN excluded.cursor_id ELSE delivery_cursors.cursor_id END,
			updated_at = CASE WHEN excluded.cursor_id > delivery_cursors.cursor_id
				THEN excluded.updated_at ELSE delivery_cursors.updated_at END
	`
	if _, err := r.DB().ExecContext(ctx, query, threadID.String(), agentID.String(), newCursor, now); err != nil {
		return false, err
	}

	// 读回判断是否 advanced（如果拿到的值 == newCursor，就是 advanced）
	actual, err := r.Get(ctx, threadID, agentID)
	if err != nil {
		return false, err
	}
	return actual == newCursor, nil
}

// Delete 移除 cursor（用于 thread 级联删除 / 测试清理）
func (r *DeliveryCursorRepository) Delete(ctx context.Context, threadID, agentID uuid.UUID) error {
	query := `DELETE FROM delivery_cursors WHERE thread_id = ? AND agent_id = ?`
	_, err := r.DB().ExecContext(ctx, query, threadID.String(), agentID.String())
	return err
}

// DeleteByThread 删除某 thread 下所有 cursors
// 借鉴 clowder-ai DeliveryCursorStore.deleteByThreadForUser（cascade delete）
func (r *DeliveryCursorRepository) DeleteByThread(ctx context.Context, threadID uuid.UUID) error {
	query := `DELETE FROM delivery_cursors WHERE thread_id = ?`
	_, err := r.DB().ExecContext(ctx, query, threadID.String())
	return err
}
