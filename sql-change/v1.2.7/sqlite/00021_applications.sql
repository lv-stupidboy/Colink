-- +goose Up
CREATE TABLE IF NOT EXISTS applications (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    user_id      TEXT NOT NULL,
    user_email   TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'pending',
    created_at   TEXT NOT NULL,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
    UNIQUE(workspace_id, user_id)
);

-- +goose Down
DROP TABLE IF EXISTS applications;
