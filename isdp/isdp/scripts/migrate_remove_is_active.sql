-- ISDP Database Migration Script
-- Version: 1.3 - 移除 base_agents 表的 is_active 字段
-- 说明：移除基础Agent的启用/禁用功能

-- 从 base_agents 表中移除 is_active 字段
-- 注意：SQLite 不支持直接 DROP COLUMN，需要重建表

-- 1. 创建新表（不含 is_active）
CREATE TABLE IF NOT EXISTS base_agents_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    api_url TEXT,
    api_token TEXT,
    default_model TEXT,
    cli_path TEXT DEFAULT 'claude',
    git_bash_path TEXT,
    max_tokens INTEGER DEFAULT 4096,
    timeout_minutes INTEGER DEFAULT 30,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 2. 复制旧表数据到新表（不包括 is_active）
INSERT INTO base_agents_new (id, name, type, api_url, api_token, default_model, cli_path, git_bash_path, max_tokens, timeout_minutes, created_at, updated_at)
SELECT id, name, type, api_url, api_token, default_model, cli_path, git_bash_path, max_tokens, timeout_minutes, created_at, updated_at
FROM base_agents;

-- 3. 删除旧表
DROP TABLE base_agents;

-- 4. 重命名新表
ALTER TABLE base_agents_new RENAME TO base_agents;

-- 5. 重建索引
CREATE INDEX IF NOT EXISTS idx_base_agents_type ON base_agents(type);

-- 完成
-- 提示：基础Agent不再有启用/禁用状态