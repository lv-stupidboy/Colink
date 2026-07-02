-- +goose Up
-- +goose StatementBegin

-- S2W2: DeliveryCursor 表
--
-- 目的：跟踪每个 (thread_id, agent_id) 对已"投递到 prompt"的最新 message.sortable_id。
-- 语义：Agent 完成一次 invocation 后 ack cursor 到本次 assembleIncrementalContext
-- 里最大的 message.sortable_id；下一次 spawn 该 agent 时按 cursor 拉未读消息。
--
-- 借鉴 clowder-ai：
--   - DeliveryCursorStore.ts:29-104 的 ackCursor 语义（单调不回退）
--   - Redis SessionKeys.deliveryCursor(u,c,t) → `delivery-cursor:${u}:${c}:${t}`
--   - Lua CAS `if cur and ARGV[1] <= cur then return 0`
-- Colink 用 SQL CAS 替代 Redis Lua，SetMaxOpenConns(1) + WAL 天然串行化。
--
-- 关键设计：
--   - 主键 (thread_id, agent_id)，单条记录 upsert，不做历史归档
--   - cursor_id 是 messages.sortable_id 的取值域（TEXT）
--   - updated_at 冗余便于监控 / 后续清理长时未活跃的 cursor

CREATE TABLE IF NOT EXISTS delivery_cursors (
    thread_id  TEXT NOT NULL,
    agent_id   TEXT NOT NULL,
    cursor_id  TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (thread_id, agent_id)
);

-- 单独索引 updated_at 供后续"驱逐 30 天未活跃 cursor"扫描使用
CREATE INDEX IF NOT EXISTS idx_delivery_cursors_updated ON delivery_cursors(updated_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_delivery_cursors_updated;
DROP TABLE IF EXISTS delivery_cursors;

-- +goose StatementEnd
