-- +goose Up
-- +goose StatementBegin

-- 添加 projects 表的 description 字段
ALTER TABLE projects ADD COLUMN description TEXT DEFAULT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- SQLite 不支持 DROP COLUMN，需要重建表
CREATE TABLE projects_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    mode TEXT NOT NULL,
    status TEXT DEFAULT 'draft',
    local_path TEXT NOT NULL,
    git_repo TEXT DEFAULT NULL,
    config TEXT DEFAULT NULL,
    workflow_template_id TEXT DEFAULT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO projects_new SELECT id, name, type, mode, status, local_path, git_repo, config, workflow_template_id, created_at, updated_at FROM projects;

DROP TABLE projects;
ALTER TABLE projects_new RENAME TO projects;

-- 重建索引
CREATE INDEX IF NOT EXISTS idx_projects_workflow_template_id ON projects(workflow_template_id);
CREATE INDEX IF NOT EXISTS idx_projects_created_at ON projects(created_at);

-- +goose StatementEnd