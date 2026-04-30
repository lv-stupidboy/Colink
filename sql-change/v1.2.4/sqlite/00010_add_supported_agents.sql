-- +goose Up
-- +goose StatementBegin

-- 为资产表添加 supported_agents 字段，支持多Agent类型
-- 空数组向后兼容：默认只支持 claude_code

-- commands 表
ALTER TABLE commands ADD COLUMN supported_agents TEXT DEFAULT '[]';

-- subagents 表
ALTER TABLE subagents ADD COLUMN supported_agents TEXT DEFAULT '[]';

-- rules 表
ALTER TABLE rules ADD COLUMN supported_agents TEXT DEFAULT '[]';

-- settings 表
ALTER TABLE settings ADD COLUMN supported_agents TEXT DEFAULT '[]';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回滚：删除 supported_agents 字段
-- SQLite 不支持 DROP COLUMN，但现代版本（3.35.0+）支持
-- 如果版本不支持，需要重建表

ALTER TABLE commands DROP COLUMN supported_agents;
ALTER TABLE subagents DROP COLUMN supported_agents;
ALTER TABLE rules DROP COLUMN supported_agents;
ALTER TABLE settings DROP COLUMN supported_agents;

-- +goose StatementEnd