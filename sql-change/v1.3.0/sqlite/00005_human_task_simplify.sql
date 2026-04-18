-- +goose Up
-- +goose StatementBegin

-- 添加新字段（SQLite ALTER TABLE 只支持 ADD COLUMN）
ALTER TABLE human_tasks ADD COLUMN invocation_id TEXT;
ALTER TABLE human_tasks ADD COLUMN wait_reason TEXT;
ALTER TABLE human_tasks ADD COLUMN completed_at TEXT;

-- 创建普通索引（用于加速查询）
CREATE INDEX IF NOT EXISTS idx_human_tasks_invocation
ON human_tasks(invocation_id);

-- 注意：唯一约束通过应用层幂等检查实现
-- CreateTaskFromWaiting 中已有 existing task 检查

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- SQLite 不支持 DROP COLUMN，保留字段
-- 移除索引
DROP INDEX IF EXISTS idx_human_tasks_invocation;

-- +goose StatementEnd