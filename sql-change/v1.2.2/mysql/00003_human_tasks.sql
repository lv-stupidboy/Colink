-- +goose Up
-- +goose StatementBegin

-- human_tasks 待办任务表
-- 当 Agent 等待用户输入（AskUserQuestion）时自动创建待办任务
-- 用户回复后自动关闭对应的待办任务
CREATE TABLE IF NOT EXISTS human_tasks (
    id VARCHAR(36) PRIMARY KEY,
    thread_id VARCHAR(36) NOT NULL,
    invocation_id VARCHAR(36) NOT NULL,
    agent_config_id VARCHAR(36) NOT NULL,
    agent_name VARCHAR(255),
    wait_reason TEXT,
    project_id VARCHAR(36),
    project_name VARCHAR(255),
    thread_name VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

-- 索引
CREATE INDEX idx_human_tasks_thread ON human_tasks(thread_id);
CREATE INDEX idx_human_tasks_status ON human_tasks(status);
CREATE INDEX idx_human_tasks_invocation ON human_tasks(invocation_id);
CREATE INDEX idx_human_tasks_agent_config ON human_tasks(agent_config_id);
CREATE INDEX idx_human_tasks_project ON human_tasks(project_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS human_tasks;
DROP INDEX IF EXISTS idx_human_tasks_thread ON human_tasks;
DROP INDEX IF EXISTS idx_human_tasks_status ON human_tasks;
DROP INDEX IF EXISTS idx_human_tasks_invocation ON human_tasks;
DROP INDEX IF EXISTS idx_human_tasks_agent_config ON human_tasks;
DROP INDEX IF EXISTS idx_human_tasks_project ON human_tasks;

-- +goose StatementEnd