-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS markets (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(random_blob(16)))),
    name VARCHAR(255) NOT NULL,
    url VARCHAR(500) NOT NULL,
    branch VARCHAR(100) DEFAULT 'main',
    enabled BOOLEAN DEFAULT 1,
    auto_update BOOLEAN DEFAULT 0,
    check_interval VARCHAR(20) DEFAULT '24h',
    last_synced_at DATETIME,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(url)
);

CREATE INDEX IF NOT EXISTS idx_markets_url ON markets(url);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS markets;
-- +goose StatementEnd