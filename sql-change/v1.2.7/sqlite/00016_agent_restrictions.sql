-- +goose Up
-- +goose StatementBegin
ALTER TABLE agent_configs ADD COLUMN restrictions TEXT DEFAULT '[]';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE agent_configs DROP COLUMN restrictions;
-- +goose StatementEnd