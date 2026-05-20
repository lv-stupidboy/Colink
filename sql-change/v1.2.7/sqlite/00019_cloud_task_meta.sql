-- +goose Up
CREATE TABLE IF NOT EXISTS cloud_task_meta (
    thread_id TEXT PRIMARY KEY,
    queue_item_id TEXT NOT NULL,
    cloud_task_id TEXT NOT NULL,
    cloud_workspace_id TEXT NOT NULL,
    assignee_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE IF EXISTS cloud_task_meta;