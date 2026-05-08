-- +goose Up
-- +goose StatementBegin
ALTER TABLE skills ADD COLUMN source_path VARCHAR(255) DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE skills DROP COLUMN source_path;
-- +goose StatementEnd