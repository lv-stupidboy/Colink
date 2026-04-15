-- +goose Up
-- +goose StatementBegin
-- 为已存在的 agent_invocations 表添加 session_id 列
-- 用于存储 A2A 对话的 sessionId，便于问题定位
ALTER TABLE agent_invocations ADD COLUMN session_id TEXT DEFAULT NULL;

-- 添加索引以提高按 session_id 查询的性能
CREATE INDEX IF NOT EXISTS idx_agent_invocations_session_id ON agent_invocations(session_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite 不支持 DROP COLUMN，需要重建表
-- 创建临时表（不含 session_id）
CREATE TABLE agent_invocations_backup (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    agent_config_id TEXT DEFAULT NULL,
    role TEXT NOT NULL,
    agent_name TEXT DEFAULT NULL,
    status TEXT DEFAULT 'running',
    input TEXT,
    output TEXT,
    started_at TEXT DEFAULT NULL,
    completed_at TEXT DEFAULT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    process_id TEXT DEFAULT NULL,
    full_prompt TEXT,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cache_read_tokens INTEGER DEFAULT 0,
    cache_creation_tokens INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0.0,
    duration_ms INTEGER DEFAULT 0,
    duration_api_ms INTEGER DEFAULT 0
);

-- 复制数据到临时表
INSERT INTO agent_invocations_backup
SELECT id, thread_id, agent_config_id, role, agent_name, status, input, output,
       started_at, completed_at, created_at, process_id, full_prompt,
       input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
       cost_usd, duration_ms, duration_api_ms
FROM agent_invocations;

-- 删除原表
DROP TABLE agent_invocations;

-- 重命名临时表为原表名
ALTER TABLE agent_invocations_backup RENAME TO agent_invocations;

-- 重建索引
CREATE INDEX IF NOT EXISTS idx_agent_invocations_thread_id ON agent_invocations(thread_id);
CREATE INDEX IF NOT EXISTS idx_agent_invocations_created_at ON agent_invocations(created_at);
-- +goose StatementEnd