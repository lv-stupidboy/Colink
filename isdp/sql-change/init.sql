-- ISDP 数据库初始化脚本
-- 版本: 1.0.0
-- 生成日期: 2026-04-08
-- 说明: 新环境初始化时执行此脚本创建所有表结构

SET NAMES utf8mb4;

-- 表: agent_command_bindings
CREATE TABLE `agent_command_bindings` (
  `id` varchar(64) NOT NULL COMMENT '绑定唯一标识符',
  `agent_role_id` varchar(64) NOT NULL COMMENT 'AgentRole ID',
  `command_id` varchar(64) NOT NULL COMMENT 'Command ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_agent_command` (`agent_role_id`,`command_id`),
  KEY `idx_agent_command_bindings_agent_role_id` (`agent_role_id`),
  KEY `idx_agent_command_bindings_command_id` (`command_id`),
  CONSTRAINT `fk_agent_command_agent` FOREIGN KEY (`agent_role_id`) REFERENCES `agent_configs` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_agent_command_command` FOREIGN KEY (`command_id`) REFERENCES `commands` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Agent与Command关联表'
;

-- 表: agent_configs
CREATE TABLE `agent_configs` (
  `id` varchar(64) NOT NULL COMMENT 'Agent配置唯一标识符',
  `name` varchar(255) NOT NULL COMMENT '配置名称',
  `role` varchar(64) NOT NULL COMMENT 'Agent角色',
  `base_agent_id` varchar(64) DEFAULT NULL COMMENT '关联基础Agent ID',
  `description` text COMMENT '角色描述',
  `system_prompt` text COMMENT '系统提示词',
  `max_tokens` int DEFAULT '4096' COMMENT '最大生成token数',
  `temperature` decimal(3,2) DEFAULT '0.70' COMMENT '温度参数',
  `is_default` tinyint DEFAULT '0' COMMENT '是否默认配置',
  `is_system` tinyint(1) DEFAULT '0',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `config_generated_at` timestamp NULL DEFAULT NULL COMMENT '配置文件生成时间',
  `config_path` varchar(512) DEFAULT NULL COMMENT '配置文件存储路径',
  `mention_patterns` json DEFAULT NULL COMMENT '@mention 触发模式列表，如 ["@architect", "@架构师", "@架构"]',
  PRIMARY KEY (`id`),
  KEY `idx_agent_configs_base_agent_id` (`base_agent_id`),
  KEY `idx_agent_configs_is_system` (`is_system`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Agent角色配置表'
;

-- 表: agent_invocations
CREATE TABLE `agent_invocations` (
  `id` varchar(64) NOT NULL COMMENT '调用记录唯一标识符',
  `thread_id` varchar(64) NOT NULL COMMENT '关联会话ID',
  `agent_config_id` varchar(64) DEFAULT NULL COMMENT '关联Agent配置ID',
  `role` varchar(64) NOT NULL COMMENT '执行角色',
  `agent_name` varchar(255) DEFAULT NULL COMMENT 'Agent名称（从 agent_configs.name 复制）',
  `status` varchar(32) DEFAULT 'running' COMMENT '调用状态',
  `input` longtext COMMENT '输入内容',
  `output` longtext COMMENT '输出结果',
  `started_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '开始时间',
  `completed_at` timestamp NULL DEFAULT NULL COMMENT '完成时间',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `process_id` varchar(36) DEFAULT NULL,
  `full_prompt` text COMMENT '完整提示词（系统提示 + 历史 + 输入）',
  `input_tokens` bigint DEFAULT '0' COMMENT '输入 Token 数量',
  `output_tokens` bigint DEFAULT '0' COMMENT '输出 Token 数量',
  `cache_read_tokens` bigint DEFAULT '0' COMMENT '缓存读取 Token 数量',
  `cache_creation_tokens` bigint DEFAULT '0' COMMENT '缓存创建 Token 数量',
  `cost_usd` decimal(10,6) DEFAULT '0.000000' COMMENT 'API 成本（美元）',
  `duration_ms` bigint DEFAULT '0' COMMENT '总执行时长（毫秒）',
  `duration_api_ms` bigint DEFAULT '0' COMMENT 'API 响应时长（毫秒）',
  PRIMARY KEY (`id`),
  KEY `idx_agent_invocations_thread_id` (`thread_id`),
  KEY `idx_agent_invocations_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Agent调用记录表'
;

-- 表: agent_rule_bindings
CREATE TABLE `agent_rule_bindings` (
  `id` varchar(64) NOT NULL COMMENT '绑定唯一标识符',
  `agent_role_id` varchar(64) NOT NULL COMMENT 'AgentRole ID',
  `rule_id` varchar(64) NOT NULL COMMENT 'Rule ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_agent_rule` (`agent_role_id`,`rule_id`),
  KEY `idx_agent_rule_bindings_agent_role_id` (`agent_role_id`),
  KEY `idx_agent_rule_bindings_rule_id` (`rule_id`),
  CONSTRAINT `fk_agent_rule_agent` FOREIGN KEY (`agent_role_id`) REFERENCES `agent_configs` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_agent_rule_rule` FOREIGN KEY (`rule_id`) REFERENCES `rules` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Agent与Rule关联表'
;

-- 表: agent_settings_bindings
CREATE TABLE `agent_settings_bindings` (
  `id` varchar(64) NOT NULL COMMENT '绑定唯一标识符',
  `agent_role_id` varchar(64) NOT NULL COMMENT 'AgentRole ID',
  `settings_id` varchar(64) NOT NULL COMMENT 'Settings ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_agent_settings` (`agent_role_id`,`settings_id`),
  KEY `idx_agent_settings_bindings_agent_role_id` (`agent_role_id`),
  KEY `idx_agent_settings_bindings_settings_id` (`settings_id`),
  CONSTRAINT `fk_agent_settings_agent` FOREIGN KEY (`agent_role_id`) REFERENCES `agent_configs` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_agent_settings_settings` FOREIGN KEY (`settings_id`) REFERENCES `settings` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Agent与Settings关联表'
;

-- 表: agent_skill_bindings
CREATE TABLE `agent_skill_bindings` (
  `id` varchar(64) NOT NULL COMMENT '关联唯一标识符',
  `agent_role_id` varchar(64) NOT NULL COMMENT 'AgentRole ID',
  `skill_id` varchar(64) NOT NULL COMMENT 'Skill ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_binding` (`agent_role_id`,`skill_id`),
  KEY `idx_agent_skill_bindings_agent_role_id` (`agent_role_id`),
  KEY `idx_agent_skill_bindings_skill_id` (`skill_id`),
  CONSTRAINT `fk_binding_agent` FOREIGN KEY (`agent_role_id`) REFERENCES `agent_configs` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_binding_skill` FOREIGN KEY (`skill_id`) REFERENCES `skills` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Agent与Skill关联表'
;

-- 表: agent_subagent_bindings
CREATE TABLE `agent_subagent_bindings` (
  `id` varchar(64) NOT NULL COMMENT '关联唯一标识符',
  `agent_role_id` varchar(64) NOT NULL COMMENT 'AgentRole ID',
  `subagent_id` varchar(64) NOT NULL COMMENT 'Subagent ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_agent_subagent_binding` (`agent_role_id`,`subagent_id`),
  KEY `idx_agent_subagent_bindings_agent_role_id` (`agent_role_id`),
  KEY `idx_agent_subagent_bindings_subagent_id` (`subagent_id`),
  CONSTRAINT `fk_agent_subagent_agent` FOREIGN KEY (`agent_role_id`) REFERENCES `agent_configs` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_agent_subagent_subagent` FOREIGN KEY (`subagent_id`) REFERENCES `subagents` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Agent与Subagent关联表'
;

-- 表: artifacts
CREATE TABLE `artifacts` (
  `id` varchar(64) NOT NULL COMMENT '产物唯一标识符',
  `thread_id` varchar(64) NOT NULL COMMENT '关联会话ID',
  `name` varchar(255) DEFAULT NULL COMMENT '产物名称',
  `content` longtext COMMENT '产物内容',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `type` varchar(64) DEFAULT NULL COMMENT '产物类型',
  `path` varchar(512) DEFAULT NULL COMMENT '文件路径',
  `metadata` json DEFAULT NULL COMMENT '元数据',
  PRIMARY KEY (`id`),
  KEY `idx_artifacts_thread_id` (`thread_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='开发产物表'
;

-- 表: base_agents
CREATE TABLE `base_agents` (
  `id` varchar(64) NOT NULL COMMENT '基础Agent唯一标识符',
  `name` varchar(255) NOT NULL COMMENT 'Agent名称',
  `type` varchar(64) NOT NULL COMMENT 'Agent类型',
  `api_url` varchar(512) DEFAULT NULL COMMENT 'API服务地址',
  `api_token` text COMMENT 'API认证令牌',
  `default_model` varchar(128) DEFAULT NULL COMMENT '默认模型名称',
  `cli_path` varchar(512) DEFAULT 'claude' COMMENT 'CLI工具路径',
  `git_bash_path` varchar(512) DEFAULT NULL COMMENT 'Git Bash路径',
  `max_tokens` int DEFAULT NULL COMMENT '最大token数',
  `timeout_minutes` int DEFAULT '30' COMMENT '超时时间(分钟)',
  `is_default` tinyint(1) DEFAULT '0',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_base_agents_type` (`type`),
  KEY `idx_base_agents_is_default` (`is_default`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='基础Agent配置表'
;

-- 表: command_skill_bindings
CREATE TABLE `command_skill_bindings` (
  `id` varchar(64) NOT NULL COMMENT '绑定唯一标识符',
  `command_id` varchar(64) NOT NULL COMMENT 'Command ID',
  `skill_id` varchar(64) NOT NULL COMMENT 'Skill ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_command_skill` (`command_id`,`skill_id`),
  KEY `idx_command_skill_bindings_command_id` (`command_id`),
  KEY `idx_command_skill_bindings_skill_id` (`skill_id`),
  CONSTRAINT `fk_command_skill_command` FOREIGN KEY (`command_id`) REFERENCES `commands` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_command_skill_skill` FOREIGN KEY (`skill_id`) REFERENCES `skills` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Command与Skill关联表'
;

-- 表: commands
CREATE TABLE `commands` (
  `id` varchar(64) NOT NULL COMMENT '命令唯一标识符',
  `name` varchar(255) NOT NULL COMMENT '命令名称(唯一标识)',
  `description` text COMMENT '命令描述',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_commands_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='命令表'
;

-- 表: invocation_content_blocks
CREATE TABLE `invocation_content_blocks` (
  `id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `invocation_id` varchar(36) COLLATE utf8mb4_unicode_ci NOT NULL,
  `type` varchar(20) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'thinking, text, tool_use, tool_result',
  `content` text COLLATE utf8mb4_unicode_ci,
  `tool_name` varchar(100) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `tool_id` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `input` json DEFAULT NULL,
  `output` text COLLATE utf8mb4_unicode_ci,
  `is_error` tinyint(1) DEFAULT '0',
  `status` varchar(20) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'streaming, completed',
  `timestamp` bigint NOT NULL,
  `started_at` bigint DEFAULT NULL,
  `completed_at` bigint DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_invocation_id` (`invocation_id`),
  KEY `idx_timestamp` (`timestamp`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
;

-- 表: knowledge_bases
CREATE TABLE `knowledge_bases` (
  `id` varchar(64) NOT NULL COMMENT '知识库唯一标识符',
  `name` varchar(255) NOT NULL COMMENT '知识库名称(唯一标识)',
  `display_name` varchar(255) DEFAULT NULL COMMENT '显示名称',
  `description` text COMMENT '描述',
  `type` varchar(50) NOT NULL COMMENT '类型(git/mcp/api)',
  `config` json DEFAULT NULL COMMENT '配置信息(加密存储)',
  `query_endpoint` varchar(500) DEFAULT NULL COMMENT '查询端点URL',
  `status` varchar(50) DEFAULT 'active' COMMENT '状态(active/inactive)',
  `last_query_at` timestamp NULL DEFAULT NULL COMMENT '最后查询时间',
  `query_count` int DEFAULT '0' COMMENT '查询次数',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_knowledge_bases_name` (`name`),
  KEY `idx_knowledge_bases_type` (`type`),
  KEY `idx_knowledge_bases_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='知识库配置表'
;

-- 表: messages
CREATE TABLE `messages` (
  `id` varchar(64) NOT NULL COMMENT '消息唯一标识符',
  `thread_id` varchar(64) NOT NULL COMMENT '关联会话ID',
  `role` varchar(32) NOT NULL COMMENT '消息角色',
  `agent_id` varchar(64) DEFAULT NULL COMMENT '产生消息的Agent ID',
  `content` longtext COMMENT '消息内容',
  `content_blocks` json DEFAULT NULL COMMENT '结构化内容块(thinking/tool_use/text等)',
  `message_type` varchar(32) DEFAULT 'text' COMMENT '消息类型',
  `metadata` json DEFAULT NULL COMMENT '消息元数据',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `mentions` json DEFAULT NULL COMMENT '被 @mention 的 Agent IDs',
  `origin` varchar(20) DEFAULT NULL COMMENT '消息来源: user, callback, stream',
  `reply_to` varchar(36) DEFAULT NULL COMMENT '回复的消息 ID',
  PRIMARY KEY (`id`),
  KEY `idx_messages_thread_id` (`thread_id`),
  KEY `idx_messages_created_at` (`created_at`),
  KEY `idx_messages_origin` (`origin`),
  KEY `idx_messages_reply_to` (`reply_to`),
  KEY `idx_messages_mentions` ((cast(`mentions` as char(500) charset utf8mb4)))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='对话消息表'
;

-- 表: projects
CREATE TABLE `projects` (
  `id` varchar(64) NOT NULL COMMENT '项目唯一标识符',
  `name` varchar(255) NOT NULL COMMENT '项目名称',
  `type` varchar(64) NOT NULL COMMENT '项目类型',
  `mode` varchar(64) NOT NULL COMMENT '开发模式',
  `status` varchar(32) DEFAULT 'draft' COMMENT '项目状态',
  `local_path` varchar(512) NOT NULL COMMENT '项目本地路径',
  `git_repo` varchar(512) DEFAULT NULL COMMENT 'Git仓库地址',
  `config` json DEFAULT NULL COMMENT '项目配置',
  `workflow_template_id` varchar(64) DEFAULT NULL COMMENT '工作流模板ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_projects_workflow_template_id` (`workflow_template_id`),
  KEY `idx_projects_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='项目信息表'
;

-- 表: rules
CREATE TABLE `rules` (
  `id` varchar(64) NOT NULL COMMENT '规约唯一标识符',
  `name` varchar(255) NOT NULL COMMENT '规约名称(唯一标识)',
  `description` text COMMENT '规约描述',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_rules_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='规约表'
;

-- 表: sandboxes
CREATE TABLE `sandboxes` (
  `id` varchar(64) NOT NULL COMMENT '沙箱唯一标识符',
  `thread_id` varchar(64) NOT NULL COMMENT '关联会话ID',
  `config` json DEFAULT NULL COMMENT '沙箱配置',
  `status` varchar(32) DEFAULT NULL COMMENT '容器状态',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_sandboxes_thread_id` (`thread_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='沙箱容器表'
;

-- 表: settings
CREATE TABLE `settings` (
  `id` varchar(64) NOT NULL COMMENT 'Settings唯一标识符',
  `name` varchar(255) NOT NULL COMMENT 'Settings名称(唯一标识)',
  `description` text COMMENT 'Settings描述',
  `directory_path` varchar(500) DEFAULT NULL COMMENT 'Settings目录路径',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_settings_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Settings配置表'
;

-- 表: skill_registries
CREATE TABLE `skill_registries` (
  `id` varchar(64) NOT NULL COMMENT '注册表唯一标识符',
  `name` varchar(255) NOT NULL COMMENT '注册表名称(唯一标识)',
  `display_name` varchar(255) DEFAULT NULL COMMENT '显示名称',
  `type` varchar(50) NOT NULL COMMENT '类型(github/gitlab/api/custom)',
  `url` varchar(500) NOT NULL COMMENT '注册表URL',
  `auth_config` json DEFAULT NULL COMMENT '认证配置(加密存储)',
  `sync_interval` int DEFAULT '3600' COMMENT '同步间隔(秒)',
  `last_sync_at` timestamp NULL DEFAULT NULL COMMENT '最后同步时间',
  `sync_status` varchar(50) DEFAULT 'pending' COMMENT '同步状态(pending/success/failed)',
  `skill_count` int DEFAULT '0' COMMENT '技能数量',
  `status` varchar(50) DEFAULT 'active' COMMENT '状态(active/inactive)',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_skill_registries_name` (`name`),
  KEY `idx_skill_registries_type` (`type`),
  KEY `idx_skill_registries_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='联邦技能源配置表'
;

-- 表: skills
CREATE TABLE `skills` (
  `id` varchar(64) NOT NULL COMMENT 'Skill唯一标识符',
  `name` varchar(255) NOT NULL COMMENT 'Skill名称(唯一标识)',
  `description` text COMMENT '描述',
  `tags` json DEFAULT NULL,
  `source_type` varchar(50) NOT NULL COMMENT '来源类型(built_in/uploaded/federated)',
  `source_registry_id` varchar(64) DEFAULT NULL COMMENT '联邦来源ID',
  `author_id` varchar(64) DEFAULT NULL COMMENT '创建者ID',
  `project_id` varchar(64) DEFAULT NULL COMMENT '所属项目ID',
  `supported_agents` json DEFAULT NULL COMMENT '支持的智能体列表',
  `use_count` int DEFAULT '0' COMMENT '使用次数',
  `status` varchar(50) DEFAULT 'active' COMMENT '状态(active/deprecated)',
  `is_public` tinyint DEFAULT '0' COMMENT '是否公开(0-否,1-是)',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_skills_name` (`name`),
  KEY `idx_skills_source_type` (`source_type`),
  KEY `idx_skills_project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='技能表'
;

-- 表: subagent_skill_bindings
CREATE TABLE `subagent_skill_bindings` (
  `id` varchar(64) NOT NULL COMMENT '绑定唯一标识符',
  `subagent_id` varchar(64) NOT NULL COMMENT 'Subagent ID',
  `skill_id` varchar(64) NOT NULL COMMENT 'Skill ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_subagent_skill` (`subagent_id`,`skill_id`),
  KEY `idx_subagent_skill_bindings_subagent_id` (`subagent_id`),
  KEY `idx_subagent_skill_bindings_skill_id` (`skill_id`),
  CONSTRAINT `fk_subagent_skill_skill` FOREIGN KEY (`skill_id`) REFERENCES `skills` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_subagent_skill_subagent` FOREIGN KEY (`subagent_id`) REFERENCES `subagents` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Subagent与Skill关联表'
;

-- 表: subagents
CREATE TABLE `subagents` (
  `id` varchar(64) NOT NULL COMMENT 'Subagent唯一标识符',
  `name` varchar(255) NOT NULL COMMENT 'Subagent名称(唯一标识)',
  `description` text COMMENT '描述',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_subagents_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='子代理配置表'
;

-- 表: threads
CREATE TABLE `threads` (
  `id` varchar(64) NOT NULL COMMENT '会话唯一标识符',
  `project_id` varchar(64) NOT NULL COMMENT '关联项目ID',
  `status` varchar(32) DEFAULT 'idle' COMMENT '会话状态',
  `current_phase` varchar(64) DEFAULT NULL COMMENT '当前开发阶段',
  `current_agent` varchar(64) DEFAULT NULL COMMENT '当前执行Agent ID',
  `depth` int DEFAULT '0' COMMENT '会话嵌套深度',
  `abort_token` text COMMENT '中止令牌',
  `workflow_template_id` varchar(64) DEFAULT NULL COMMENT '工作流模板ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `name` varchar(255) NOT NULL DEFAULT '' COMMENT '会话名称',
  PRIMARY KEY (`id`),
  KEY `idx_threads_project_id` (`project_id`),
  KEY `idx_threads_workflow_template_id` (`workflow_template_id`),
  KEY `idx_threads_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='开发会话表'
;

-- 表: workflow_templates
CREATE TABLE `workflow_templates` (
  `id` varchar(64) NOT NULL COMMENT '模板唯一标识符',
  `name` varchar(255) NOT NULL COMMENT '模板名称',
  `description` text COMMENT '模板描述',
  `agent_ids` json DEFAULT NULL COMMENT 'Agent ID列表',
  `checkpoints` json DEFAULT NULL COMMENT '检查点配置',
  `estimated_time` varchar(64) DEFAULT NULL COMMENT '预计完成时间',
  `is_system` tinyint DEFAULT '0' COMMENT '是否系统模板',
  `is_default` tinyint DEFAULT '0' COMMENT '是否默认模板',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `transitions` json DEFAULT NULL COMMENT 'A2A路由转换规则',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='工作流模板表'
;

