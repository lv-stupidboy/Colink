-- +goose Up
-- +goose StatementBegin

-- 简化 session_records 表，删除长连接相关字段，添加 ACP session ID
-- SQLite 不支持 DROP COLUMN，需要重建表

-- 1. 创建简化后的新表
CREATE TABLE session_records_new (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    agent_type TEXT NOT NULL,

    -- ACP 原生 session/resume 模式
    acp_session_id TEXT,
    resume_expiry INTEGER DEFAULT 0,

    -- Claude CLI resume 模式（兼容）
    cli_session_id TEXT,

    -- 状态信息
    status TEXT DEFAULT 'active',
    last_active_at INTEGER DEFAULT 0,

    -- 时间戳
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- 2. 迁移数据（保留有用的字段）
INSERT INTO session_records_new (
    id, thread_id, agent_id, agent_type,
    acp_session_id, cli_session_id, resume_expiry,
    status, last_active_at,
    created_at, updated_at
)
SELECT
    id, thread_id, agent_id, agent_type,
    NULL AS acp_session_id,  -- 新字段，迁移时为空
    cli_session_id, resume_expiry,
    status, last_active_at,
    created_at, updated_at
FROM session_records;

-- 3. 删除旧表
DROP TABLE session_records;

-- 4. 重命名新表
ALTER TABLE session_records_new RENAME TO session_records;

-- 5. 重建索引
CREATE INDEX IF NOT EXISTS idx_session_records_thread_id ON session_records(thread_id);
CREATE INDEX IF NOT EXISTS idx_session_records_agent_id ON session_records(agent_id);
CREATE INDEX IF NOT EXISTS idx_session_records_agent_type ON session_records(agent_type);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回退：恢复原始表结构
CREATE TABLE session_records_old (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    agent_type TEXT NOT NULL,

    cli_session_id TEXT,
    resume_expiry INTEGER DEFAULT 0,

    status TEXT DEFAULT 'active',
    turn_count INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    conversation BLOB,
    key_entities BLOB,
    process_pid INTEGER DEFAULT 0,

    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    last_active_at INTEGER DEFAULT 0,
    sealed_at INTEGER DEFAULT 0,

    last_error TEXT
);

INSERT INTO session_records_old (
    id, thread_id, agent_id, agent_type,
    cli_session_id, resume_expiry,
    status, last_active_at,
    created_at, updated_at
)
SELECT
    id, thread_id, agent_id, agent_type,
    cli_session_id, resume_expiry,
    status, last_active_at,
    created_at, updated_at
FROM session_records;

DROP TABLE session_records;
ALTER TABLE session_records_old RENAME TO session_records;

CREATE INDEX IF NOT EXISTS idx_session_records_thread_id ON session_records(thread_id);
CREATE INDEX IF NOT EXISTS idx_session_records_agent_id ON session_records(agent_id);
CREATE INDEX IF NOT EXISTS idx_session_records_status ON session_records(status);
CREATE INDEX IF NOT EXISTS idx_session_records_agent_type ON session_records(agent_type);
CREATE INDEX IF NOT EXISTS idx_session_records_last_active ON session_records(last_active_at);

-- +goose StatementEnd