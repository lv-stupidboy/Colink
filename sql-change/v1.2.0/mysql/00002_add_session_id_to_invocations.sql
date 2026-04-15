-- +goose Up
-- +goose StatementBegin
-- 为已存在的 agent_invocations 表添加 session_id 列
-- 用于存储 A2A 对话的 sessionId，便于问题定位
ALTER TABLE agent_invocations ADD COLUMN session_id VARCHAR(64) DEFAULT NULL;

-- 添加索引以提高按 session_id 查询的性能
CREATE INDEX idx_agent_invocations_session_id ON agent_invocations(session_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- MySQL 支持 DROP COLUMN
ALTER TABLE agent_invocations DROP COLUMN session_id;

-- 删除索引（MySQL 会自动删除索引，但显式删除更安全）
DROP INDEX idx_agent_invocations_session_id ON agent_invocations;
-- +goose StatementEnd