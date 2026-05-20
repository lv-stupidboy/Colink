-- +goose Up
CREATE TABLE IF NOT EXISTS cloud_auth (
    id INTEGER PRIMARY KEY CHECK (id = 1),  -- Single-row constraint
    cloud_url TEXT NOT NULL,
    token TEXT NOT NULL,
    user_id TEXT NOT NULL,
    user_email TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE IF EXISTS cloud_auth;