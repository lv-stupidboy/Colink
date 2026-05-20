-- +goose Up
-- Option C: 混合方案 - memories 表用于 user/agent 级
-- ISDP Memory System: Three-table design for different lifecycle needs

CREATE TABLE memories (
    id TEXT PRIMARY KEY,  -- UUID
    scope TEXT NOT NULL CHECK(scope IN ('user', 'agent')),  -- 只允许 user/agent
    scope_id TEXT NOT NULL,  -- user_id, base_agent_id
    content TEXT NOT NULL,
    category TEXT,  -- 'preference' | 'decision' | 'convention' | 'context'
    entity TEXT,  -- 关联实体（可选）
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    metadata TEXT  -- JSON 扩展字段
);

CREATE INDEX idx_memories_scope ON memories(scope, scope_id);
CREATE INDEX idx_memories_category ON memories(category);
CREATE INDEX idx_memories_entity ON memories(entity);

-- team_memories 表：支持权限 JOIN team_packages
CREATE TABLE team_memories (
    id TEXT PRIMARY KEY,
    team_id TEXT NOT NULL,  -- 关联 team_packages (存储为 JSON team_package manifest 中的 team ID)
    content TEXT NOT NULL,
    category TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    metadata TEXT
);

CREATE INDEX idx_team_memories_team ON team_memories(team_id);

-- thread_memories 表：支持 TTL 自动清理
CREATE TABLE thread_memories (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,  -- 关联 threads 表
    content TEXT NOT NULL,
    category TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,  -- TTL 过期时间（线程结束时设置）
    metadata TEXT
);

CREATE INDEX idx_thread_memories_thread ON thread_memories(thread_id);
CREATE INDEX idx_thread_memories_expires ON thread_memories(expires_at);

-- +goose Down
DROP TABLE IF EXISTS memories;
DROP TABLE IF EXISTS team_memories;
DROP TABLE IF EXISTS thread_memories;