-- +goose Up
-- +goose StatementBegin

-- agent_configs 表：新增 requires_human 字段
ALTER TABLE agent_configs ADD COLUMN requires_human INTEGER DEFAULT 0;

-- agent_invocations 表：新增 requires_human 字段
ALTER TABLE agent_invocations ADD COLUMN requires_human INTEGER DEFAULT 0;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- SQLite 不支持 DROP COLUMN，保留字段（回滚时数据仍存在）
-- 如需完全回滚，需要重建表

-- +goose StatementEnd