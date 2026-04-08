-- 添加 is_system 字段到 agent_configs 表
-- 执行时间: 2026-03-20
-- 说明: 标识系统预置的Agent角色

-- 检查字段是否存在，不存在则添加（幂等操作）
SET @column_exists = (
    SELECT COUNT(*) FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'agent_configs'
    AND COLUMN_NAME = 'is_system'
);

SET @sql = IF(@column_exists = 0,
    'ALTER TABLE agent_configs ADD COLUMN is_system BOOLEAN DEFAULT FALSE AFTER is_default',
    'SELECT 1'
);

PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- 创建索引以加速查询（如果索引不存在）
SET @index_exists = (
    SELECT COUNT(*) FROM information_schema.STATISTICS
    WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'agent_configs'
    AND INDEX_NAME = 'idx_agent_configs_is_system'
);

SET @sql = IF(@index_exists = 0,
    'CREATE INDEX idx_agent_configs_is_system ON agent_configs(is_system)',
    'SELECT 1'
);

PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;