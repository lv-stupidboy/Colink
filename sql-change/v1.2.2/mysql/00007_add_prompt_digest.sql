-- T6(M4): Prompt Digest 审计 - 添加 prompt_digest 和 prompt_length 字段
-- +goose Up
-- +goose StatementBegin
ALTER TABLE agent_invocations ADD COLUMN prompt_digest VARCHAR(100) DEFAULT NULL;
ALTER TABLE agent_invocations ADD COLUMN prompt_length INT DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE agent_invocations DROP COLUMN prompt_digest;
ALTER TABLE agent_invocations DROP COLUMN prompt_length;
-- +goose StatementEnd
