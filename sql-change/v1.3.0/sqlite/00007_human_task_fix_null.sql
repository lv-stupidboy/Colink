-- +goose Up
-- +goose StatementBegin

-- 修复 NOT NULL 约束问题：role_config_id 和 role_name 应该可空
-- SQLite 不支持 ALTER COLUMN，需要重建表

-- 创建备份表
CREATE TABLE human_tasks_backup AS SELECT * FROM human_tasks;

-- 删除原表
DROP TABLE human_tasks;

-- 创建新表（移除 NOT NULL 约束）
CREATE TABLE human_tasks (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    role_config_id TEXT,  -- 可空
    role_name TEXT,       -- 可空
    task_type TEXT DEFAULT 'task_dispatch',
    task_content TEXT,
    expected_output TEXT,
    source_agent_id TEXT,
    source_agent_name TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    submitted_at TEXT,
    submitted_by TEXT,
    output_content TEXT,
    output_files TEXT,
    target_agent_id TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    invocation_id TEXT,
    wait_reason TEXT,
    completed_at TEXT,
    agent_config_id TEXT,
    agent_name TEXT
);

-- 恢复数据
INSERT INTO human_tasks
SELECT * FROM human_tasks_backup;

-- 删除备份表
DROP TABLE human_tasks_backup;

-- 重建索引
CREATE INDEX IF NOT EXISTS idx_human_tasks_thread ON human_tasks(thread_id);
CREATE INDEX IF NOT EXISTS idx_human_tasks_status ON human_tasks(status);
CREATE INDEX IF NOT EXISTS idx_human_tasks_invocation ON human_tasks(invocation_id);
CREATE INDEX IF NOT EXISTS idx_human_tasks_agent_config ON human_tasks(agent_config_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 无法回滚（表结构已改变）
SELECT 1;
-- +goose StatementEnd