-- +goose Up
-- +goose StatementBegin

-- 移除所有资产表的 supported_agents 字段
-- 配置生成与运行时不再按 base agent 类型过滤资产，由用户绑定决定
-- SQLite 3.35.0+ 支持 ALTER TABLE DROP COLUMN

ALTER TABLE skills DROP COLUMN supported_agents;
ALTER TABLE commands DROP COLUMN supported_agents;
ALTER TABLE subagents DROP COLUMN supported_agents;
ALTER TABLE rules DROP COLUMN supported_agents;
ALTER TABLE settings DROP COLUMN supported_agents;
ALTER TABLE mcp_servers DROP COLUMN supported_agents;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回滚：重新添加 supported_agents 字段，默认 '[]'（向后兼容）
ALTER TABLE skills ADD COLUMN supported_agents TEXT DEFAULT '[]';
ALTER TABLE commands ADD COLUMN supported_agents TEXT DEFAULT '[]';
ALTER TABLE subagents ADD COLUMN supported_agents TEXT DEFAULT '[]';
ALTER TABLE rules ADD COLUMN supported_agents TEXT DEFAULT '[]';
ALTER TABLE settings ADD COLUMN supported_agents TEXT DEFAULT '[]';
ALTER TABLE mcp_servers ADD COLUMN supported_agents TEXT DEFAULT '[]';

-- +goose StatementEnd
