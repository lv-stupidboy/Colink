-- +goose Up
-- +goose StatementBegin

-- 移除所有资产表的 supported_agents 字段
-- 使用 rename + 重建方式，兼容所有 SQLite 版本

-- === skills ===
CREATE TABLE skills_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    tags TEXT DEFAULT NULL,
    source_type TEXT NOT NULL,
    source_registry_id TEXT DEFAULT NULL,
    author_id TEXT DEFAULT NULL,
    project_id TEXT DEFAULT NULL,
    use_count INTEGER DEFAULT 0,
    status TEXT DEFAULT 'active',
    is_public INTEGER DEFAULT 0,
    source_path TEXT DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO skills_new (id, name, description, tags, source_type, source_registry_id, author_id, project_id, use_count, status, is_public, source_path, created_at, updated_at)
    SELECT id, name, description, tags, source_type, source_registry_id, author_id, project_id, use_count, status, is_public, source_path, created_at, updated_at FROM skills;
DROP TABLE skills;
ALTER TABLE skills_new RENAME TO skills;

-- === commands ===
CREATE TABLE commands_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO commands_new (id, name, description, created_at, updated_at)
    SELECT id, name, description, created_at, updated_at FROM commands;
DROP TABLE commands;
ALTER TABLE commands_new RENAME TO commands;

-- === subagents ===
CREATE TABLE subagents_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO subagents_new (id, name, description, created_at, updated_at)
    SELECT id, name, description, created_at, updated_at FROM subagents;
DROP TABLE subagents;
ALTER TABLE subagents_new RENAME TO subagents;

-- === rules ===
CREATE TABLE rules_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO rules_new (id, name, description, created_at, updated_at)
    SELECT id, name, description, created_at, updated_at FROM rules;
DROP TABLE rules;
ALTER TABLE rules_new RENAME TO rules;

-- === settings ===
CREATE TABLE settings_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    directory_path TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO settings_new (id, name, description, directory_path, created_at, updated_at)
    SELECT id, name, description, directory_path, created_at, updated_at FROM settings;
DROP TABLE settings;
ALTER TABLE settings_new RENAME TO settings;

-- === mcp_servers ===
CREATE TABLE mcp_servers_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT,
    description TEXT,
    transport TEXT NOT NULL DEFAULT 'stdio',
    command TEXT,
    args TEXT NOT NULL DEFAULT '[]',
    env TEXT NOT NULL DEFAULT '{}',
    url TEXT,
    headers TEXT NOT NULL DEFAULT '{}',
    source_type TEXT NOT NULL DEFAULT 'personal',
    status TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO mcp_servers_new (id, name, display_name, description, transport, command, args, env, url, headers, source_type, status, created_at, updated_at)
    SELECT id, name, display_name, description, transport, command, args, env, url, headers, source_type, status, created_at, updated_at FROM mcp_servers;
DROP TABLE mcp_servers;
ALTER TABLE mcp_servers_new RENAME TO mcp_servers;

-- mcp_servers 索引重建
CREATE INDEX IF NOT EXISTS idx_mcp_servers_name ON mcp_servers(name);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_status ON mcp_servers(status);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE skills ADD COLUMN supported_agents TEXT DEFAULT '[]';
ALTER TABLE commands ADD COLUMN supported_agents TEXT DEFAULT '[]';
ALTER TABLE subagents ADD COLUMN supported_agents TEXT DEFAULT '[]';
ALTER TABLE rules ADD COLUMN supported_agents TEXT DEFAULT '[]';
ALTER TABLE settings ADD COLUMN supported_agents TEXT DEFAULT '[]';
ALTER TABLE mcp_servers ADD COLUMN supported_agents TEXT NOT NULL DEFAULT '["claude_code"]';

-- +goose StatementEnd
