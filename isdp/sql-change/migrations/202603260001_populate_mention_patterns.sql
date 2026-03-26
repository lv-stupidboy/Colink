-- 文件路径: isdp/sql-change/migrations/202603260001_populate_mention_patterns.sql
-- 变更说明: 为 agent_configs 表填充默认的 mention_patterns 数据
-- 作者: Claude
-- 日期: 2026-03-26

SET NAMES utf8mb4;

-- 更新需求分析师的 mention_patterns
UPDATE agent_configs
SET mention_patterns = JSON_ARRAY(
    '@requirement', '@需求', '@需求分析', '@需求分析师'
)
WHERE role = 'requirement_analyst';

-- 更新架构师的 mention_patterns
UPDATE agent_configs
SET mention_patterns = JSON_ARRAY(
    '@architect', '@架构师', '@架构', '@架构设计师'
)
WHERE role = 'architect';

-- 更新前端开发工程师的 mention_patterns
UPDATE agent_configs
SET mention_patterns = JSON_ARRAY(
    '@frontend', '@前端', '@前端开发', '@前端工程师', '@前端开发工程师'
)
WHERE role = 'frontend_developer';

-- 更新后端开发工程师的 mention_patterns
UPDATE agent_configs
SET mention_patterns = JSON_ARRAY(
    '@backend', '@后端', '@后端开发', '@后端工程师', '@后端开发工程师'
)
WHERE role = 'backend_developer';

-- 更新代码审查工程师的 mention_patterns
UPDATE agent_configs
SET mention_patterns = JSON_ARRAY(
    '@reviewer', '@评审', '@代码审查', '@审查员', '@代码审查工程师'
)
WHERE role = 'code_reviewer';

-- 更新测试工程师的 mention_patterns
UPDATE agent_configs
SET mention_patterns = JSON_ARRAY(
    '@testengineer', '@测试', '@测试工程师'
)
WHERE role = 'test_engineer';

-- 更新运维SRE工程师的 mention_patterns
UPDATE agent_configs
SET mention_patterns = JSON_ARRAY(
    '@devops', '@sre', '@运维', '@部署', '@运维工程师'
)
WHERE role = 'sre_engineer';

-- 更新项目经理的 mention_patterns
UPDATE agent_configs
SET mention_patterns = JSON_ARRAY(
    '@pm', '@项目经理'
)
WHERE role = 'project_manager';

-- 更新UI/UX设计师的 mention_patterns
UPDATE agent_configs
SET mention_patterns = JSON_ARRAY(
    '@ui', '@设计师', '@UI设计师'
)
WHERE role = 'ui_designer';

-- 更新数据库设计师的 mention_patterns
UPDATE agent_configs
SET mention_patterns = JSON_ARRAY(
    '@db', '@数据库', '@数据库设计师'
)
WHERE role = 'database_designer';

-- 更新安全工程师的 mention_patterns
UPDATE agent_configs
SET mention_patterns = JSON_ARRAY(
    '@security', '@安全', '@安全工程师'
)
WHERE role = 'security_engineer';

-- 更新技术文档工程师的 mention_patterns
UPDATE agent_configs
SET mention_patterns = JSON_ARRAY(
    '@doc', '@文档', '@技术文档'
)
WHERE role = 'tech_writer';

-- 验证更新结果
SELECT role, name, mention_patterns FROM agent_configs WHERE mention_patterns IS NOT NULL;

-- 回滚语句（如需回滚执行以下语句）
-- UPDATE agent_configs SET mention_patterns = NULL;