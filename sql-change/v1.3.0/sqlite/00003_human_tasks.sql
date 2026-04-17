-- +goose Up
-- +goose StatementBegin

-- human_tasks 人工任务表
CREATE TABLE IF NOT EXISTS human_tasks (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    role_config_id TEXT NOT NULL,
    role_name TEXT NOT NULL,
    task_type TEXT NOT NULL DEFAULT 'task_dispatch',
    task_content TEXT NOT NULL,
    expected_output TEXT,
    source_agent_id TEXT,
    source_agent_name TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    submitted_at TEXT,
    submitted_by TEXT,
    output_content TEXT,
    output_files TEXT,  -- JSON 数组
    target_agent_id TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_human_tasks_thread ON human_tasks(thread_id);
CREATE INDEX IF NOT EXISTS idx_human_tasks_status ON human_tasks(status);
CREATE INDEX IF NOT EXISTS idx_human_tasks_role_config ON human_tasks(role_config_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS human_tasks;
-- +goose StatementEnd