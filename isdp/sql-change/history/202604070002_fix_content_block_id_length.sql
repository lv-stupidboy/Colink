-- 文件路径: isdp/sql-change/migrations/202604070002_fix_content_block_id_length.sql
-- 变更说明：修复内容块 ID 字段长度不足的问题
-- 作者：Claude
-- 日期：2026-04-07

SET NAMES utf8mb4;

-- 将 id 字段长度从 36 扩展到 128，以支持 Claude 的工具调用 ID 格式
ALTER TABLE invocation_content_blocks MODIFY COLUMN id VARCHAR(128) NOT NULL;

-- 同时扩展 tool_id 字段
ALTER TABLE invocation_content_blocks MODIFY COLUMN tool_id VARCHAR(128);

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE invocation_content_blocks MODIFY COLUMN id VARCHAR(36) NOT NULL;
-- ALTER TABLE invocation_content_blocks MODIFY COLUMN tool_id VARCHAR(36);
