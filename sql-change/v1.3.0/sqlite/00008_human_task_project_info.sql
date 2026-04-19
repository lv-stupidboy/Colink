-- +goose Up
-- +goose StatementBegin

-- 添加项目名称和任务名称字段（方便前端展示）
ALTER TABLE human_tasks ADD COLUMN project_id TEXT;
ALTER TABLE human_tasks ADD COLUMN project_name TEXT;
ALTER TABLE human_tasks ADD COLUMN thread_name TEXT;

-- 添加索引
CREATE INDEX IF NOT EXISTS idx_human_tasks_project ON human_tasks(project_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite 不支持 DROP COLUMN
-- +goose StatementEnd