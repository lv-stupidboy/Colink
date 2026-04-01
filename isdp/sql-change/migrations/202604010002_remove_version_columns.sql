-- 文件路径: isdp/sql-change/migrations/202604010002_remove_version_columns.sql
-- 变更说明：移除各类资产表中的 version 字段
-- 作者：axiang
-- 日期：2026-04-01

SET NAMES utf8mb4;

-- 移除 skills 表的 version 字段
ALTER TABLE `skills` DROP COLUMN IF EXISTS `version`;

-- 移除 commands 表的 version 字段
ALTER TABLE `commands` DROP COLUMN IF EXISTS `version`;

-- 移除 subagents 表的 version 字段
ALTER TABLE `subagents` DROP COLUMN IF EXISTS `version`;

-- 移除 rules 表的 version 字段
ALTER TABLE `rules` DROP COLUMN IF EXISTS `version`;

-- 移除 settings 表的 version 字段
ALTER TABLE `settings` DROP COLUMN IF EXISTS `version`;

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE `skills` ADD COLUMN `version` VARCHAR(50) DEFAULT '' AFTER `supported_agents`;
-- ALTER TABLE `commands` ADD COLUMN `version` VARCHAR(50) DEFAULT '' AFTER `content`;
-- ALTER TABLE `subagents` ADD COLUMN `version` VARCHAR(50) DEFAULT '' AFTER `content`;
-- ALTER TABLE `rules` ADD COLUMN `version` VARCHAR(50) DEFAULT '' AFTER `visibility`;
-- ALTER TABLE `settings` ADD COLUMN `version` VARCHAR(50) DEFAULT '' AFTER `directory_path`;