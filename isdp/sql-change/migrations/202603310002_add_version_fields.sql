-- isdp/sql-change/migrations/202603310002_add_version_fields.sql
-- 变更说明：为 commands, subagents, rules 表添加 version 字段
-- 作者：ISDP Team
-- 日期：2026-03-31

SET NAMES utf8mb4;

START TRANSACTION;

-- ----------------------------
-- 为 commands 添加 version 字段和索引
-- ----------------------------
ALTER TABLE commands ADD COLUMN version VARCHAR(20) DEFAULT '1.0.0' COMMENT '版本号';
ALTER TABLE commands ADD INDEX idx_commands_version (version);

-- ----------------------------
-- 为 subagents 添加 version 字段和索引
-- ----------------------------
ALTER TABLE subagents ADD COLUMN version VARCHAR(20) DEFAULT '1.0.0' COMMENT '版本号';
ALTER TABLE subagents ADD INDEX idx_subagents_version (version);

-- ----------------------------
-- 为 rules 添加 version 字段和索引
-- ----------------------------
ALTER TABLE rules ADD COLUMN version VARCHAR(20) DEFAULT '1.0.0' COMMENT '版本号';
ALTER TABLE rules ADD INDEX idx_rules_version (version);

COMMIT;

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE rules DROP INDEX idx_rules_version;
-- ALTER TABLE rules DROP COLUMN version;
-- ALTER TABLE subagents DROP INDEX idx_subagents_version;
-- ALTER TABLE subagents DROP COLUMN version;
-- ALTER TABLE commands DROP INDEX idx_commands_version;
-- ALTER TABLE commands DROP COLUMN version;