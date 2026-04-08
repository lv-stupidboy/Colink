-- 文件路径: isdp/sql-change/migrations/202604030001_cleanup_legacy_fields.sql
-- 变更说明：清理遗留字段
--   subagents.skill_id - 已迁移到 subagent_skill_bindings 绑定表
-- 作者：axiang
-- 日期：2026-04-03

SET NAMES utf8mb4;

-- 清理 subagents 表的 skill_id 字段（先删除外键约束）
ALTER TABLE subagents DROP FOREIGN KEY fk_subagents_skill;
ALTER TABLE subagents DROP COLUMN skill_id;

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE subagents ADD COLUMN skill_id VARCHAR(64) COMMENT '关联技能包ID' AFTER description;
-- ALTER TABLE subagents ADD CONSTRAINT fk_subagents_skill FOREIGN KEY (skill_id) REFERENCES skills(id) ON DELETE SET NULL;