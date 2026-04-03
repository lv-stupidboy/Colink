-- 文件路径: isdp/sql-change/migrations/202604030003_add_base_agent_is_default.sql
-- 变更说明：为基础Agent表添加 is_default 字段，支持设置默认基础Agent
-- 作者：axiang
-- 日期：2026-04-03

SET NAMES utf8mb4;

-- 添加 is_default 字段
ALTER TABLE base_agents ADD COLUMN is_default BOOLEAN DEFAULT FALSE AFTER timeout_minutes;

-- 为 is_default 字段添加索引（用于快速查找默认Agent）
CREATE INDEX idx_base_agents_is_default ON base_agents(is_default);

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE base_agents DROP COLUMN is_default;
-- DROP INDEX idx_base_agents_is_default ON base_agents;