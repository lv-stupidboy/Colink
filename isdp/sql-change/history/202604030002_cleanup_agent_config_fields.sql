-- 文件路径: isdp/sql-change/migrations/202604030002_cleanup_agent_config_fields.sql
-- 变更说明：清理 agent_configs 表的废弃字段 model_name
--   注意：routing_config, capabilities, dependencies, outputs 已在 202603270001 中删除
--   本次只删除 model_name 字段
-- 作者：axiang
-- 日期：2026-04-03

SET NAMES utf8mb4;

-- 清理 agent_configs 表的废弃字段 model_name（模型由 base_agent_id 关联）
ALTER TABLE agent_configs DROP COLUMN model_name;

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE agent_configs ADD COLUMN model_name VARCHAR(128) DEFAULT 'claude-sonnet-4-6' AFTER system_prompt;