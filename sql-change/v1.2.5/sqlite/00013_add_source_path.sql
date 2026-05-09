-- +goose Up
-- +goose StatementBegin
ALTER TABLE skills ADD COLUMN source_path TEXT DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE skills DROP COLUMN source_path;
-- +goose StatementEnd