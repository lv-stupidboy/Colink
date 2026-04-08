-- 变更说明：为 agent_configs 表添加能力声明字段
-- 作者：系统迁移
-- 日期：2026-03-20
-- 来源：scripts/202603200001_add_agent_capabilities.sql

-- 正向变更
ALTER TABLE agent_configs ADD COLUMN capabilities JSON DEFAULT NULL COMMENT 'Agent能力声明';
ALTER TABLE agent_configs ADD COLUMN dependencies JSON DEFAULT NULL COMMENT 'Agent依赖配置';
ALTER TABLE agent_configs ADD COLUMN outputs JSON DEFAULT NULL COMMENT 'Agent输出配置';

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE agent_configs DROP COLUMN capabilities;
-- ALTER TABLE agent_configs DROP COLUMN dependencies;
-- ALTER TABLE agent_configs DROP COLUMN outputs;