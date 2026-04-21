-- +goose Up
-- +goose StatementBegin

-- 添加 projects 表的 description 字段
ALTER TABLE projects ADD COLUMN description TEXT DEFAULT NULL AFTER name;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE projects DROP COLUMN description;

-- +goose StatementEnd