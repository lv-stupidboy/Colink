-- +goose Up

CREATE TABLE IF NOT EXISTS local_repos (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    git_url TEXT NOT NULL,
    local_path TEXT NOT NULL,
    branch TEXT,
    last_commit TEXT,
    status TEXT DEFAULT 'pending',
    error_message TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_local_repos_name ON local_repos(name);
CREATE INDEX IF NOT EXISTS idx_local_repos_status ON local_repos(status);

-- +goose Down

DROP TABLE IF EXISTS local_repos;
