-- T6(M4): Prompt Digest 审计 - 添加 prompt_digest 和 prompt_length 字段
-- +goose Up
-- +goose StatementBegin
ALTER TABLE agent_invocations ADD COLUMN prompt_digest TEXT;
ALTER TABLE agent_invocations ADD COLUMN prompt_length INTEGER DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite 不支持 DROP COLUMN < 3.35.0，需要重建表
CREATE TABLE agent_invocations_backup AS
SELECT id, threadId, agentConfigId, role, agentName, status, input, fullPrompt, output, startedAt, completedAt, createdAt, processId, sessionId, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens, costUsd, durationMs, durationApiMs, callbackToken, triggeredBy
FROM agent_invocations;
DROP TABLE agent_invocations;
ALTER TABLE agent_invocations_backup RENAME TO agent_invocations;
-- +goose StatementEnd
