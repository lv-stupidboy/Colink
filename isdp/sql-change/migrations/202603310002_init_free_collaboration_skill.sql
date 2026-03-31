-- 文件路径: isdp/sql-change/migrations/202603310002_init_free_collaboration_skill.sql
-- 变更说明：初始化自由协作技能
-- 作者：Claude
-- 日期：2026-03-31

SET NAMES utf8mb4;

-- ========================================
-- 插入 Skill 记录
-- ========================================

-- 检查是否已存在，不存在则插入
INSERT INTO skills (id, name, description, storage_path, is_system, created_at)
SELECT UUID(), 'free-collaboration', '自由协作技能 - 多Agent并行讨论', 'free-collaboration', 1, NOW()
FROM DUAL
WHERE NOT EXISTS (
    SELECT 1 FROM skills WHERE name = 'free-collaboration'
);

-- ========================================
-- 说明
-- ========================================

-- Skill 绑定逻辑在应用层处理：
-- 当创建自由模式团队时，自动将此 Skill 绑定到团队的所有 Agent
--
-- 绑定示例（应用层伪代码）：
-- if template.Mode == TeamModeFree {
--     skillID := GetSkillIDByName("free-collaboration")
--     for _, agentID := range template.AgentIds {
--         CreateAgentSkillBinding(agentID, skillID)
--     }
-- }
--
-- Skill 加载流程：
-- 1. Skill 存储在 {dataDir}/skills/free-collaboration/SKILL.md
-- 2. configgen 服务将 Skill 复制到 {configDir}/skills/
-- 3. Agent CLI 通过 CLAUDE_CONFIG_DIR 环境变量加载 Skill

-- ========================================
-- 回滚语句（如需回滚执行以下语句）
-- ========================================
-- DELETE FROM skills WHERE name = 'free-collaboration';
-- DELETE FROM agent_skill_bindings WHERE skill_id = (SELECT id FROM skills WHERE name = 'free-collaboration');