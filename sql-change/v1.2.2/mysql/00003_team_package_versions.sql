-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS team_package_versions (
    id CHAR(36) PRIMARY KEY,
    workflow_id CHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(255),
    version VARCHAR(50) NOT NULL,
    description TEXT,
    last_synced_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_workflow (workflow_id),
    UNIQUE KEY uk_name (name),
    FOREIGN KEY (workflow_id) REFERENCES workflow_templates(id) ON DELETE CASCADE,
    INDEX idx_tpv_workflow (workflow_id),
    INDEX idx_tpv_name (name)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS team_package_versions;
-- +goose StatementEnd