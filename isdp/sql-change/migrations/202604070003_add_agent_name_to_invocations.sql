-- 文件路径: isdp/sql-change/migrations/202604070003_add_agent_name_to_invocations.sql
-- 变更说明: agent_invocations 表添加 agent_name 字段，用于保存 Agent 名称
-- 作者: Claude
-- 日期: 2026-04-07

SET NAMES utf8mb4;

-- 添加 agent_name 字段
ALTER TABLE agent_invocations
ADD COLUMN agent_name VARCHAR(255) COMMENT 'Agent名称（从 agent_configs.name 复制）' AFTER role;

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE agent_invocations DROP COLUMN agent_name;