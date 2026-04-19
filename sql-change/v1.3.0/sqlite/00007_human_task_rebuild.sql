-- +goose Up
-- +goose StatementBegin

-- 重建 human_tasks 表，移除 role_config_id 和 role_name 的 NOT NULL 约束
-- SQLite 不支持 ALTER COLUMN，需要重建表

-- 1. 创建新表（符合简化后的模型）
CREATE TABLE IF NOT EXISTS human_tasks_new (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    invocation_id TEXT,
    agent_config_id TEXT,
    agent_name TEXT,
    wait_reason TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TEXT,
    -- 保留旧字段（兼容历史数据），但设为可空
    role_config_id TEXT,
    role_name TEXT,
    task_type TEXT DEFAULT 'task_dispatch',
    task_content TEXT,
    expected_output TEXT,
    source_agent_id TEXT,
    source_agent_name TEXT,
    submitted_at TEXT,
    submitted_by TEXT,
    output_content TEXT,
    output_files TEXT,
    target_agent_id TEXT,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- 2. 复制数据
INSERT INTO human_tasks_new (
    id, thread_id, invocation_id, agent_config_id, agent_name,
    wait_reason, status, created_at, completed_at,
    role_config_id, role_name, task_type, task_content,
    expected_output, source_agent_id, source_agent_name,
    submitted_at, submitted_by, output_content, output_files,
    target_agent_id, updated_at
)
SELECT
    id, thread_id, invocation_id, NULL, NULL,  -- agent_config_id/agent_name 新增，历史数据设为 NULL
    wait_reason, status, created_at, completed_at,
    role_config_id, role_name, task_type, task_content,
    expected_output, source_agent_id, source_agent_name,
    submitted_at, submitted_by, output_content, output_files,
    target_agent_id, updated_at
FROM human_tasks;

-- 3. 删除旧表
DROP TABLE human_tasks;

-- 4. 重命名新表
ALTER TABLE human_tasks_new RENAME TO human_tasks;

-- 5. 重建索引
CREATE INDEX IF NOT EXISTS idx_human_tasks_thread ON human_tasks(thread_id);
CREATE INDEX IF NOT EXISTS idx_human_tasks_status ON human_tasks(status);
CREATE INDEX IF NOT EXISTS idx_human_tasks_invocation ON human_tasks(invocation_id);
CREATE INDEX IF NOT EXISTS idx_human_tasks_agent_config ON human_tasks(agent_config_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回滚：重建原始表结构（恢复 NOT NULL 约束）
CREATE TABLE IF NOT EXISTS human_tasks_original (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    role_config_id TEXT,
    role_name TEXT,
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

-- 复制数据
INSERT INTO human_tasks_original (
    id, thread_id, role_config_id, role_name, task_type, task_content,
    expected_output, source_agent_id, source_agent_name, status,
    submitted_at, submitted_by, output_content, output_files,
    target_agent_id, created_at, updated_at, invocation_id,
    wait_reason, completed_at, agent_config_id, agent_name
)
SELECT
    id, thread_id, role_config_id, role_name, task_type, task_content,
    expected_output, source_agent_id, source_agent_name, status,
    submitted_at, submitted_by, output_content, output_files,
    target_agent_id, created_at, updated_at, invocation_id,
    wait_reason, completed_at, agent_config_id, agent_name
FROM human_tasks;

DROP TABLE human_tasks;
ALTER TABLE human_tasks_original RENAME TO human_tasks;

-- 重建索引
CREATE INDEX IF NOT EXISTS idx_human_tasks_thread ON human_tasks(thread_id);
CREATE INDEX IF NOT EXISTS idx_human_tasks_status ON human_tasks(status);
CREATE INDEX IF NOT EXISTS idx_human_tasks_invocation ON human_tasks(invocation_id);
CREATE INDEX IF NOT EXISTS idx_human_tasks_agent_config ON human_tasks(agent_config_id);

-- +goose StatementEnd