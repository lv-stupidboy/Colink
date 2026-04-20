-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS team_package_versions (
    id TEXT PRIMARY KEY DEFAULT (lower(hex(random_blob(16)))),
    workflow_id TEXT NOT NULL REFERENCES workflow_templates(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(255),
    version VARCHAR(50) NOT NULL,
    description TEXT,
    last_synced_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workflow_id),
    UNIQUE(name)
);

CREATE INDEX IF NOT EXISTS idx_tpv_workflow ON team_package_versions(workflow_id);
CREATE INDEX IF NOT EXISTS idx_tpv_name ON team_package_versions(name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS team_package_versions;
-- +goose StatementEnd