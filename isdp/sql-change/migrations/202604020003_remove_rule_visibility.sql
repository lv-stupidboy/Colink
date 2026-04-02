-- 文件路径: isdp/sql-change/migrations/202604020003_remove_rule_visibility.sql
-- 变更说明：移除 rules 表的 visibility 字段，简化规约模型
-- 作者：axiang
-- 日期：2026-04-02

SET NAMES utf8mb4;

-- 移除 rules 表的 visibility 字段
ALTER TABLE rules DROP COLUMN IF EXISTS visibility;

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE rules ADD COLUMN visibility VARCHAR(10) DEFAULT 'private' AFTER description;