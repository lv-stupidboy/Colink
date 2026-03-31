-- 文件路径: isdp/sql-change/migrations/202603310001_add_free_discussion_fields.sql
-- 变更说明：新增自由协作模式所需字段和表
-- 作者：Claude
-- 日期：2026-03-31

SET NAMES utf8mb4;

-- ========================================
-- Part 1: 扩展 threads 表
-- ========================================

-- 1.1 新增 type 字段（会话类型）
ALTER TABLE threads ADD COLUMN type VARCHAR(20) DEFAULT 'workflow' COMMENT '会话类型(workflow-工作流模式, free_discussion-自由协作模式)';

-- 1.2 新增 available_agents 字段（可用Agent列表，自由模式使用）
ALTER TABLE threads ADD COLUMN available_agents JSON DEFAULT NULL COMMENT '可用Agent ID列表(JSON数组, 自由协作模式使用)';

-- ========================================
-- Part 2: 扩展 workflow_templates 表
-- ========================================

-- 2.1 新增 mode 字段（团队模式）
ALTER TABLE workflow_templates ADD COLUMN mode VARCHAR(20) DEFAULT 'workflow' COMMENT '团队模式(workflow-工作流模式, free-自由模式)';

-- ========================================
-- Part 3: 新增 multi_mention_requests 表
-- ========================================

DROP TABLE IF EXISTS multi_mention_requests;
CREATE TABLE multi_mention_requests (
    id VARCHAR(64) NOT NULL COMMENT '请求唯一标识符',
    thread_id VARCHAR(64) NOT NULL COMMENT '关联会话ID',
    initiator VARCHAR(100) NOT NULL COMMENT '发起者Agent ID',
    callback_to VARCHAR(100) NOT NULL COMMENT '回调Agent ID',
    targets JSON NOT NULL COMMENT '目标Agent ID列表(JSON数组, 1-3个)',
    question TEXT NOT NULL COMMENT '问题内容',
    context TEXT COMMENT '上下文信息',
    status VARCHAR(20) DEFAULT 'pending' COMMENT '请求状态(pending/running/partial/done/timeout/failed)',
    timeout_minutes INT DEFAULT 8 COMMENT '超时时间(分钟)',
    search_evidence JSON COMMENT '搜索证据引用列表(JSON数组)',
    override_reason TEXT COMMENT '跳过搜索的理由',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (id),
    KEY idx_mm_requests_thread_id (thread_id),
    KEY idx_mm_requests_status (status),
    KEY idx_mm_requests_initiator (initiator),
    CONSTRAINT fk_mm_requests_thread FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多Agent讨论请求表';

-- ========================================
-- Part 4: 新增 multi_mention_responses 表
-- ========================================

DROP TABLE IF EXISTS multi_mention_responses;
CREATE TABLE multi_mention_responses (
    id VARCHAR(64) NOT NULL COMMENT '响应唯一标识符',
    request_id VARCHAR(64) NOT NULL COMMENT '关联请求ID',
    agent_id VARCHAR(100) NOT NULL COMMENT '响应Agent ID',
    content TEXT NOT NULL COMMENT '响应内容',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (id),
    KEY idx_mm_responses_request_id (request_id),
    KEY idx_mm_responses_agent_id (agent_id),
    CONSTRAINT fk_mm_responses_request FOREIGN KEY (request_id) REFERENCES multi_mention_requests(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='多Agent讨论响应表';

-- ========================================
-- 回滚语句（如需回滚执行以下语句）
-- ========================================
-- ALTER TABLE threads DROP COLUMN type;
-- ALTER TABLE threads DROP COLUMN available_agents;
-- ALTER TABLE workflow_templates DROP COLUMN mode;
-- DROP TABLE IF EXISTS multi_mention_responses;
-- DROP TABLE IF EXISTS multi_mention_requests;