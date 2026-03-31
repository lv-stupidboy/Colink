-- isdp/sql-change/migrations/202603310003_drop_subagent_content.sql
-- 变更说明：移除 subagents 表的 content 字段（内容已存储在文件系统）
-- 作者：ISDP Team
-- 日期：2026-03-31

SET NAMES utf8mb4;

START TRANSACTION;

-- 移除 content 字段
ALTER TABLE subagents DROP COLUMN content;

COMMIT;

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE subagents ADD COLUMN content LONGTEXT AFTER description;