-- +goose Up
-- +goose StatementBegin

-- Session 记录表
-- 用于持久化 Session 信息，支持两种模式：
-- 1. Claude CLI resume 模式：存储 CliSessionID，用于 --resume
-- 2. OpenCode/CodeAgent 长连接模式：存储对话历史和状态

CREATE TABLE IF NOT EXISTS session_records (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    agent_type TEXT NOT NULL,

    -- Claude CLI resume 模式字段
    cli_session_id TEXT,
    resume_expiry INTEGER DEFAULT 0,

    -- OpenCode/CodeAgent 长连接模式字段
    status TEXT DEFAULT 'active',
    turn_count INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    conversation BLOB,
    key_entities BLOB,

    -- 进程信息
    process_pid INTEGER DEFAULT 0,

    -- 时间戳
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    last_active_at INTEGER DEFAULT 0,
    sealed_at INTEGER DEFAULT 0,

    -- 错误信息
    last_error TEXT
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_session_records_thread_id ON session_records(thread_id);
CREATE INDEX IF NOT EXISTS idx_session_records_agent_id ON session_records(agent_id);
CREATE INDEX IF NOT EXISTS idx_session_records_status ON session_records(status);
CREATE INDEX IF NOT EXISTS idx_session_records_agent_type ON session_records(agent_type);
CREATE INDEX IF NOT EXISTS idx_session_records_last_active ON session_records(last_active_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS session_records;

-- +goose StatementEnd