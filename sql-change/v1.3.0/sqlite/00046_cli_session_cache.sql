-- +goose Up
CREATE TABLE IF NOT EXISTS cli_session_cache (
    thread_id TEXT NOT NULL,
    config_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (thread_id, config_id)
);

-- 添加索引提升查询性能
CREATE INDEX IF NOT EXISTS idx_cli_session_updated
ON cli_session_cache(updated_at);

-- +goose Down
DROP TABLE IF EXISTS cli_session_cache;
DROP INDEX IF EXISTS idx_cli_session_updated;