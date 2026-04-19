-- +goose Up
-- +goose StatementBegin

-- Human Task System 最终结构
-- 背景：原设计将 Human 作为角色类型（role_config_id NOT NULL），现已改为 Agent 主导触发
-- 变更：简化表结构，添加 invocation_id 关联、项目信息字段，移除 NOT NULL 约束

-- SQLite 不支持 ALTER COLUMN，需要重建表

-- 1. 创建新表（最终结构）
CREATE TABLE IF NOT EXISTS human_tasks_new (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,

    -- 新增字段：Agent 主导触发机制
    invocation_id TEXT,          -- Agent 调用实例 ID，唯一标识一次执行
    agent_config_id TEXT,        -- 等待用户的 Agent 配置 ID
    agent_name TEXT,             -- Agent 名称（便于显示）
    wait_reason TEXT,            -- 等待原因（Agent 输出摘要）

    -- 新增字段：项目信息（便于显示）
    project_id TEXT,
    project_name TEXT,
    thread_name TEXT,

    -- 核心状态字段
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TEXT,

    -- 保留旧字段（兼容历史数据，可空）
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

-- 2. 复制数据（如果原表存在）
INSERT OR IGNORE INTO human_tasks_new (
    id, thread_id, invocation_id, agent_config_id, agent_name,
    wait_reason, project_id, project_name, thread_name,
    status, created_at, completed_at,
    role_config_id, role_name, task_type, task_content,
    expected_output, source_agent_id, source_agent_name,
    submitted_at, submitted_by, output_content, output_files,
    target_agent_id, updated_at
)
SELECT
    id, thread_id, invocation_id, agent_config_id, agent_name,
    wait_reason, project_id, project_name, thread_name,
    status, created_at, completed_at,
    role_config_id, role_name, task_type, task_content,
    expected_output, source_agent_id, source_agent_name,
    submitted_at, submitted_by, output_content, output_files,
    target_agent_id, updated_at
FROM human_tasks;

-- 3. 删除旧表
DROP TABLE IF EXISTS human_tasks;

-- 4. 重命名新表
ALTER TABLE human_tasks_new RENAME TO human_tasks;

-- 5. 创建索引
CREATE INDEX IF NOT EXISTS idx_human_tasks_thread ON human_tasks(thread_id);
CREATE INDEX IF NOT EXISTS idx_human_tasks_status ON human_tasks(status);
CREATE INDEX IF NOT EXISTS idx_human_tasks_invocation ON human_tasks(invocation_id);
CREATE INDEX IF NOT EXISTS idx_human_tasks_agent_config ON human_tasks(agent_config_id);
CREATE INDEX IF NOT EXISTS idx_human_tasks_project ON human_tasks(project_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回滚：重建原始表结构（恢复 role_config_id NOT NULL）
CREATE TABLE IF NOT EXISTS human_tasks_original (
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
    output_files TEXT,
    target_agent_id TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- 新字段保留（兼容）
    invocation_id TEXT,
    agent_config_id TEXT,
    agent_name TEXT,
    wait_reason TEXT,
    project_id TEXT,
    project_name TEXT,
    thread_name TEXT,
    completed_at TEXT
);

-- 复制数据
INSERT OR IGNORE INTO human_tasks_original (
    id, thread_id, role_config_id, role_name, task_type, task_content,
    expected_output, source_agent_id, source_agent_name, status,
    submitted_at, submitted_by, output_content, output_files,
    target_agent_id, created_at, updated_at,
    invocation_id, agent_config_id, agent_name, wait_reason,
    project_id, project_name, thread_name, completed_at
)
SELECT
    id, thread_id, role_config_id, role_name, task_type, task_content,
    expected_output, source_agent_id, source_agent_name, status,
    submitted_at, submitted_by, output_content, output_files,
    target_agent_id, created_at, updated_at,
    invocation_id, agent_config_id, agent_name, wait_reason,
    project_id, project_name, thread_name, completed_at
FROM human_tasks;

DROP TABLE human_tasks;
ALTER TABLE human_tasks_original RENAME TO human_tasks;

-- 重建索引
CREATE INDEX IF NOT EXISTS idx_human_tasks_thread ON human_tasks(thread_id);
CREATE INDEX IF NOT EXISTS idx_human_tasks_status ON human_tasks(status);
CREATE INDEX IF NOT EXISTS idx_human_tasks_role_config ON human_tasks(role_config_id);

-- +goose StatementEnd