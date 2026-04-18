-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS markets (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(500) NOT NULL,
    branch VARCHAR(100) DEFAULT 'main',
    enabled BOOLEAN DEFAULT TRUE,
    auto_update BOOLEAN DEFAULT FALSE,
    check_interval VARCHAR(20) DEFAULT '24h',
    last_synced_at DATETIME,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_url (url),
    INDEX idx_markets_url (url)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS markets;
-- +goose StatementEnd