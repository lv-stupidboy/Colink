-- 变更说明：为 workflow_templates 表添加 transitions 字段，支持 A2A 路由转换规则
-- 作者：系统迁移
-- 日期：2026-03-20
-- 来源：scripts/202603190001_add_workflow_transitions.sql

-- 正向变更
ALTER TABLE workflow_templates ADD COLUMN transitions JSON DEFAULT NULL COMMENT 'A2A路由转换规则';

-- 回滚语句（如需回滚执行以下语句）
-- ALTER TABLE workflow_templates DROP COLUMN transitions;