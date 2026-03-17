-- ISDP Database Migration Script
-- Version: 1.2 - 移除 agent_configs 表的 model_name 字段
-- 说明：模型配置现在统一从 base_agents 表获取

-- 备份现有数据（可选）
-- CREATE TABLE agent_configs_backup AS SELECT * FROM agent_configs;

-- 从 agent_configs 表中移除 model_name 字段
-- 注意：SQLite 不支持直接 DROP COLUMN，需要重建表

-- 1. 创建新表（不含 model_name）
CREATE TABLE IF NOT EXISTS agent_configs_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    role TEXT NOT NULL,
    base_agent_id TEXT REFERENCES base_agents(id),
    description TEXT,
    system_prompt TEXT,
    max_tokens INTEGER DEFAULT 4096,
    temperature REAL DEFAULT 0.7,
    routing_config TEXT,
    is_default INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 2. 复制旧表数据到新表（不包括 model_name）
INSERT INTO agent_configs_new (id, name, role, base_agent_id, description, system_prompt, max_tokens, temperature, routing_config, is_default, created_at, updated_at)
SELECT id, name, role, base_agent_id, description, system_prompt, max_tokens, temperature, routing_config, is_default, created_at, updated_at
FROM agent_configs;

-- 3. 删除旧表
DROP TABLE agent_configs;

-- 4. 重命名新表
ALTER TABLE agent_configs_new RENAME TO agent_configs;

-- 5. 重建索引
CREATE INDEX IF NOT EXISTS idx_agent_configs_base_agent_id ON agent_configs(base_agent_id);

-- 完成
-- 提示：现有 Agent 配置的模型现在将从关联的 base_agents 表的 default_model 字段获取
