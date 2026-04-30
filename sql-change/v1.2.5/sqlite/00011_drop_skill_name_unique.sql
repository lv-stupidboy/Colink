-- v1.2.5/sqlite/00011_drop_skill_name_unique.sql
-- 删除 skills 表的名称唯一约束，允许 skill 重名
-- SQLite 不支持 ALTER TABLE DROP CONSTRAINT，需要重建表

-- +goose Up
-- +goose StatementBegin

-- Step 1: 创建新表（移除 name 的 UNIQUE 约束）
CREATE TABLE skills_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    tags TEXT DEFAULT NULL,
    source_type TEXT NOT NULL,
    source_registry_id TEXT DEFAULT NULL,
    author_id TEXT DEFAULT NULL,
    project_id TEXT DEFAULT NULL,
    supported_agents TEXT DEFAULT NULL,
    use_count INTEGER DEFAULT 0,
    status TEXT DEFAULT 'active',
    is_public INTEGER DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Step 2: 复制数据
INSERT INTO skills_new SELECT * FROM skills;

-- Step 3: 删除旧表
DROP TABLE skills;

-- Step 4: 重命名新表
ALTER TABLE skills_new RENAME TO skills;

-- Step 5: 重建索引
CREATE INDEX IF NOT EXISTS idx_skills_source_type ON skills(source_type);
CREATE INDEX IF NOT EXISTS idx_skills_project_id ON skills(project_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- 回滚：重新添加 UNIQUE 约束（同样需要重建表）
CREATE TABLE skills_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    tags TEXT DEFAULT NULL,
    source_type TEXT NOT NULL,
    source_registry_id TEXT DEFAULT NULL,
    author_id TEXT DEFAULT NULL,
    project_id TEXT DEFAULT NULL,
    supported_agents TEXT DEFAULT NULL,
    use_count INTEGER DEFAULT 0,
    status TEXT DEFAULT 'active',
    is_public INTEGER DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 复制数据（如果有重名记录会失败，这是预期的行为）
INSERT INTO skills_new SELECT * FROM skills;

DROP TABLE skills;

ALTER TABLE skills_new RENAME TO skills;

-- 重建索引
CREATE INDEX IF NOT EXISTS idx_skills_source_type ON skills(source_type);
CREATE INDEX IF NOT EXISTS idx_skills_project_id ON skills(project_id);

-- +goose StatementEnd