-- v1.2.5/mysql/00012_add_messages_reported_at.sql
-- 新增 messages 表的 reported_at 字段，用于标记消息是否已上报

-- +goose Up
-- +goose StatementBegin
ALTER TABLE messages ADD COLUMN reported_at DATETIME NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE messages DROP COLUMN reported_at;
-- +goose StatementEnd