-- +goose Up
-- project_memories 表：项目级共享记忆，跨团队可见
CREATE TABLE project_memories (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,  -- 关联 projects 表
    content TEXT NOT NULL,
    category TEXT,  -- 'preference' | 'decision' | 'convention' | 'technical'
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    metadata TEXT  -- JSON 扩展字段
);

CREATE INDEX idx_project_memories_project ON project_memories(project_id);

-- +goose Down
DROP TABLE IF EXISTS project_memories;