-- +goose Up
-- +goose StatementBegin

ALTER TABLE agent_configs ADD COLUMN requires_human TINYINT(1) DEFAULT 0 COMMENT '是否需要人工参与';
ALTER TABLE agent_invocations ADD COLUMN requires_human TINYINT(1) DEFAULT 0 COMMENT '是否需要人工参与';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE agent_configs DROP COLUMN requires_human;
ALTER TABLE agent_invocations DROP COLUMN requires_human;

-- +goose StatementEnd