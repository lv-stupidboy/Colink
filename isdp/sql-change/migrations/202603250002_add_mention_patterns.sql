-- 文件路径: isdp/sql-change/migrations/202603250002_add_mention_patterns.sql
-- 变更说明: 为 agent_configs 表添加 mention_patterns 字段，支持动态 @mention 模式匹配
-- 作者: Claude
-- 日期: 2026-03-25

SET NAMES utf8mb4;

-- 添加 mention_patterns 字段
ALTER TABLE agent_configs
ADD COLUMN mention_patterns JSON DEFAULT NULL COMMENT '@mention 触发模式列表，如 ["@architect", "@架构师", "@架构"]';

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE agent_configs DROP COLUMN mention_patterns;