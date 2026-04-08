-- 文件路径：isdp/sql-change/migrations/202604070001_add_content_blocks_to_messages.sql
-- 变更说明：添加 content_blocks 字段存储结构化消息内容
-- 作者：Claude
-- 日期：2026-04-07

SET NAMES utf8mb4;

-- 添加 content_blocks 字段
ALTER TABLE messages ADD COLUMN content_blocks JSON COMMENT '结构化内容块(thinking/tool_use/text等)' AFTER content;

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE messages DROP COLUMN content_blocks;