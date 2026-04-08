-- 文件路径: isdp/sql-change/migrations/202604020002_remove_skill_version.sql
-- 变更说明：移除 skills 表的 version 字段（不再使用版本概念）
-- 作者：axiang
-- 日期：2026-04-02

SET NAMES utf8mb4;

-- 移除 skills 表的 version 字段
ALTER TABLE `skills` DROP COLUMN `version`;

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE `skills` ADD COLUMN `version` VARCHAR(50) DEFAULT '1.0.0' COMMENT '版本号' AFTER `supported_agents`;