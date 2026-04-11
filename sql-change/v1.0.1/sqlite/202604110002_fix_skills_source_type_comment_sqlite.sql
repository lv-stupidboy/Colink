-- 文件路径: sql-change/migrations/v1.0.1/202604110002_fix_skills_source_type_comment_sqlite.sql
-- 版本号: 202604110002
-- 变更说明：修正 skills.source_type 字段注释（SQLite 版本）
-- 作者：axiang
-- 日期：2026-04-11
-- 影响范围：skills 表
-- 回滚风险：低
-- 注意：SQLite 不支持 MODIFY COLUMN 注释，此文件仅作记录
--       SQLite 字段注释应在 init.sql 中维护，迁移时无需执行

-- SQLite 无需执行变更（注释不影响数据和功能）
-- 实际注释修正将在下次更新 init_sqlite.sql 时同步

-- 仅记录：source_type 定义值应为 platform/personal/federated
-- 对应 Go 代码定义：
--   SkillSourcePlatform  = "platform"
--   SkillSourcePersonal  = "personal"
--   SkillSourceFederated = "federated"