-- 文件路径: isdp/sql-change/migrations/202604080001_add_full_prompt_to_invocations.sql
-- 变更说明: 为 agent_invocations 表添加 full_prompt 字段，存储完整提示词
-- 作者: Claude
-- 日期: 2026-04-08

SET NAMES utf8mb4;

-- 添加 full_prompt 字段（TEXT 类型，可存储较长内容）
ALTER TABLE agent_invocations
ADD COLUMN full_prompt TEXT DEFAULT NULL COMMENT '完整提示词（系统提示 + 历史 + 输入）';

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE agent_invocations DROP COLUMN full_prompt;