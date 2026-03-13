-- ISDP Database Initialization Script
-- Version: 1.0
-- Date: 2026-03-12

-- 项目表
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    mode VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'draft',
    git_repo VARCHAR(500),
    config JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 开发会话表
CREATE TABLE IF NOT EXISTS threads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    status VARCHAR(50) DEFAULT 'idle',
    current_phase VARCHAR(50),
    current_agent VARCHAR(50),
    depth INTEGER DEFAULT 0,
    abort_token VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 消息表
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id UUID REFERENCES threads(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL,
    agent_id VARCHAR(50),
    content TEXT,
    message_type VARCHAR(50) DEFAULT 'text',
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Agent配置表
CREATE TABLE IF NOT EXISTS agent_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id VARCHAR(100) UNIQUE NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    description TEXT,
    phase VARCHAR(50),
    routing_config JSONB,
    tools JSONB,
    system_prompt TEXT,
    model VARCHAR(50) DEFAULT 'claude-sonnet-4-6',
    is_active BOOLEAN DEFAULT true,
    is_builtin BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Agent调用记录表
CREATE TABLE IF NOT EXISTS agent_invocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id UUID REFERENCES threads(id) ON DELETE CASCADE,
    agent_id VARCHAR(50) NOT NULL,
    session_id VARCHAR(100),
    status VARCHAR(50) DEFAULT 'running',
    depth INTEGER DEFAULT 0,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    started_at TIMESTAMP DEFAULT NOW(),
    ended_at TIMESTAMP
);

-- 产物表
CREATE TABLE IF NOT EXISTS artifacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id UUID REFERENCES threads(id) ON DELETE CASCADE,
    phase VARCHAR(50) NOT NULL,
    type VARCHAR(50) NOT NULL,
    name VARCHAR(255),
    path VARCHAR(500),
    content TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- 沙箱容器表
CREATE TABLE IF NOT EXISTS sandbox_containers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    container_id VARCHAR(100) UNIQUE,
    name VARCHAR(255),
    status VARCHAR(50) DEFAULT 'created',
    image VARCHAR(255),
    ports JSONB,
    cpu_limit INTEGER DEFAULT 2,
    memory_limit INTEGER DEFAULT 4096,
    network_name VARCHAR(100),
    started_at TIMESTAMP,
    stopped_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_threads_project_id ON threads(project_id);
CREATE INDEX IF NOT EXISTS idx_messages_thread_id ON messages(thread_id);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_agent_invocations_thread_id ON agent_invocations(thread_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_thread_id ON artifacts(thread_id);
CREATE INDEX IF NOT EXISTS idx_sandbox_project_id ON sandbox_containers(project_id);

-- 初始内置Agent角色数据
INSERT INTO agent_configs (agent_id, display_name, description, phase, is_builtin, routing_config, tools) VALUES
('requirement-analyst', '需求分析师', '负责理解和结构化用户需求', 'requirement', true,
 '{"use_when": ["用户输入自然语言需求", "需要解析和结构化需求"], "not_for": ["技术方案设计", "代码编写"], "output": ["需求文档", "功能列表", "验收标准"]}',
 '["Read", "Write", "Edit"]'),
('architect', '架构师', '负责技术方案设计与决策', 'design', true,
 '{"use_when": ["需要设计技术方案", "架构设计", "技术选型"], "not_for": ["需求分析", "代码编写"], "output": ["架构图", "API文档", "技术选型报告"]}',
 '["Read", "Write", "Edit", "Glob", "Grep"]'),
('developer', '开发者', '负责代码实现', 'implement', true,
 '{"use_when": ["编写代码", "修复Bug", "重构代码"], "not_for": ["需求分析", "架构设计"], "output": ["源代码", "配置文件"]}',
 '["Read", "Write", "Edit", "Glob", "Grep", "Bash"]'),
('reviewer', '审查员', '负责代码审查和质量检测', 'review', true,
 '{"use_when": ["代码审查", "安全检查", "质量检测"], "not_for": ["编写代码", "需求分析"], "output": ["审查报告"]}',
 '["Read", "Glob", "Grep"]'),
('test-engineer', '测试工程师', '负责测试设计和执行', 'test', true,
 '{"use_when": ["测试设计", "测试执行", "测试报告"], "not_for": ["编写功能代码", "需求分析"], "output": ["测试用例", "测试报告"]}',
 '["Read", "Write", "Edit", "Bash"]'),
('devops', '运维工程师', '负责部署和环境管理', 'deploy', true,
 '{"use_when": ["部署服务", "环境配置", "监控告警"], "not_for": ["编写业务代码", "需求分析"], "output": ["部署产物", "配置文件"]}',
 '["Read", "Write", "Edit", "Bash"]')
ON CONFLICT (agent_id) DO NOTHING;