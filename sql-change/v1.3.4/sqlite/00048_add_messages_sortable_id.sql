-- +goose Up
-- +goose StatementBegin

-- S2W1: 引入 sortable_id 列供 A2A 拉模式 cursor 使用
--
-- 目的：现有 messages 表 id 是 uuid.New()（随机 UUID），字典序与写入顺序无关。
--       created_at 列类型 TEXT / SQLite CURRENT_TIMESTAMP 精度到秒 —— 并发同秒
--       插入的两条消息无法稳定分辨顺序。DeliveryCursor 拉模式要求 cursor 可比较
--       且严格单调，因此引入独立 sortable_id 列，格式：
--
--         {ts_ms_16padded}-{seq_6padded}-{uuid_prefix_8}
--
-- 生成器：internal/service/agent/sortable_id.go，在 repo.MessageRepository.Create
-- 内部生成，保证与 CreatedAt 单调对齐。
--
-- 兼容策略：
--   1) 加列时给现有行填 backfill 值（用 rowid + created_at 排序保证单调）
--   2) 加复合索引 (thread_id, sortable_id) 供 GetByThreadAfter 使用
--   3) 保留旧的单列索引，旧查询完全不变

-- 1) 加列（允许 NULL，backfill 之后再由应用层保证非空）
ALTER TABLE messages ADD COLUMN sortable_id TEXT DEFAULT NULL;

-- 2) Backfill：按 (created_at ASC, id ASC) 分配单调 ID
--    格式对齐 Go 端：16位毫秒 - 6位序号 - 8字符后缀（此处用 substr(id) 兜底）
--    SQLite ROW_NUMBER() 从 3.25 开始支持；modernc.org/sqlite 内嵌版本远高于此
UPDATE messages
SET sortable_id = printf(
    '%016d-%06d-%s',
    COALESCE(
        CAST(strftime('%s', created_at) AS INTEGER) * 1000,
        0
    ),
    (SELECT COUNT(*) FROM messages m2 WHERE m2.created_at < messages.created_at
        OR (m2.created_at = messages.created_at AND m2.id < messages.id)),
    substr(replace(id, '-', ''), 1, 8)
)
WHERE sortable_id IS NULL;

-- 3) 复合索引：GetByThreadAfter(threadID, cursor) 的关键索引
CREATE INDEX IF NOT EXISTS idx_messages_thread_sortable
    ON messages(thread_id, sortable_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_messages_thread_sortable;

-- SQLite 不支持 DROP COLUMN 直到 3.35，为兼容采取 rename+rebuild 方式
CREATE TABLE messages_new (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    role TEXT NOT NULL,
    agent_id TEXT DEFAULT NULL,
    content TEXT,
    content_blocks TEXT DEFAULT NULL,
    message_type TEXT DEFAULT 'text',
    metadata TEXT DEFAULT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reported_at DATETIME NULL,
    mentions TEXT DEFAULT NULL,
    origin TEXT DEFAULT NULL,
    reply_to TEXT DEFAULT NULL
);
INSERT INTO messages_new (id, thread_id, role, agent_id, content, content_blocks, message_type, metadata, created_at, reported_at, mentions, origin, reply_to)
    SELECT id, thread_id, role, agent_id, content, content_blocks, message_type, metadata, created_at, reported_at, mentions, origin, reply_to FROM messages;
DROP TABLE messages;
ALTER TABLE messages_new RENAME TO messages;

CREATE INDEX IF NOT EXISTS idx_messages_thread_id ON messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_messages_origin ON messages(origin);
CREATE INDEX IF NOT EXISTS idx_messages_reply_to ON messages(reply_to);

-- +goose StatementEnd
