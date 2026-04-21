-- +goose Up
-- A2A Enhancement: 新增 callback_token、triggered_by、routable_teams 字段

-- agent_invocations 表新增字段（MCP 认证）
ALTER TABLE agent_invocations ADD COLUMN callback_token VARCHAR(64);
ALTER TABLE agent_invocations ADD COLUMN triggered_by VARCHAR(36);

-- workflow_templates 表新增字段（路由范围配置）
ALTER TABLE workflow_templates ADD COLUMN routable_teams TEXT;

-- +goose Down
-- SQLite 不支持 DROP COLUMN，需要重建表
-- 实际生产环境应使用更安全的迁移策略

-- 对于 rollback，记录变更以便手动处理
-- ALTER TABLE agent_invocations DROP COLUMN callback_token;
-- ALTER TABLE agent_invocations DROP COLUMN triggered_by;
-- ALTER TABLE workflow_templates DROP COLUMN routable_teams;