-- +goose Up
CREATE TABLE IF NOT EXISTS mcp_servers (
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
    supported_agents TEXT NOT NULL DEFAULT '["claude_code"]',
    status TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS agent_mcp_bindings (
    id TEXT PRIMARY KEY,
    agent_role_id TEXT NOT NULL,
    mcp_server_id TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(agent_role_id, mcp_server_id),
    FOREIGN KEY(agent_role_id) REFERENCES agent_configs(id) ON DELETE CASCADE,
    FOREIGN KEY(mcp_server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_mcp_servers_name ON mcp_servers(name);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_status ON mcp_servers(status);
CREATE INDEX IF NOT EXISTS idx_agent_mcp_bindings_agent ON agent_mcp_bindings(agent_role_id);
CREATE INDEX IF NOT EXISTS idx_agent_mcp_bindings_server ON agent_mcp_bindings(mcp_server_id);

-- +goose Down
DROP TABLE IF EXISTS agent_mcp_bindings;
DROP TABLE IF EXISTS mcp_servers;
