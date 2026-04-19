-- +goose Up
-- +goose StatementBegin

-- 添加缺少的字段
ALTER TABLE human_tasks ADD COLUMN agent_config_id TEXT;
ALTER TABLE human_tasks ADD COLUMN agent_name TEXT;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite 不支持 DROP COLUMN
-- +goose StatementEnd
