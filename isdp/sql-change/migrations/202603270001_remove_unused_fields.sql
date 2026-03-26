-- 文件路径: isdp/sql-change/migrations/202603270001_remove_unused_fields.sql
-- 变更说明：删除 agent_configs 表中不再使用的字段 (routing_config, capabilities, dependencies, outputs)
-- 作者：Claude
-- 日期：2026-03-27

SET NAMES utf8mb4;

-- 删除 agent_configs 表中不再使用的字段
-- 这些字段已被 workflow_templates.transitions 替代

-- 1. 删除 routing_config 字段（路由配置已迁移到 workflow_templates.transitions）
ALTER TABLE agent_configs DROP COLUMN IF EXISTS routing_config;

-- 2. 删除 capabilities 字段（能力声明未实现，已废弃）
ALTER TABLE agent_configs DROP COLUMN IF EXISTS capabilities;

-- 3. 删除 dependencies 字段（依赖声明未实现，已废弃）
ALTER TABLE agent_configs DROP COLUMN IF EXISTS dependencies;

-- 4. 删除 outputs 字段（产出声明未实现，已废弃）
ALTER TABLE agent_configs DROP COLUMN IF EXISTS outputs;

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE agent_configs ADD COLUMN routing_config JSON COMMENT '路由配置(JSON格式)';
-- ALTER TABLE agent_configs ADD COLUMN capabilities JSON COMMENT '能力列表(JSON格式)';
-- ALTER TABLE agent_configs ADD COLUMN dependencies JSON COMMENT '依赖列表(JSON格式)';
-- ALTER TABLE agent_configs ADD COLUMN outputs JSON COMMENT '产出列表(JSON格式)';