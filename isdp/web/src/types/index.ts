// Agent角色
export type AgentRole =
  | 'requirement'
  | 'architect'
  | 'developer'
  | 'reviewer'
  | 'testengineer'
  | 'devops'
  | 'custom';

// 基础Agent类型
export type BaseAgentType = 'claude_code' | 'open_code';

// Thread状态
export type ThreadStatus =
  | 'idle'
  | 'running'
  | 'paused'
  | 'complete'
  | 'failed';

// 工作阶段
export type Phase =
  | 'requirement'
  | 'design'
  | 'development'
  | 'review'
  | 'test'
  | 'merge'
  | 'complete';

// 消息角色
export type MessageRole = 'user' | 'agent' | 'system';

// 消息类型
export type MessageType = 'text' | 'code' | 'artifact' | 'command';

// 项目
export interface Project {
  id: string;
  name: string;
  description: string;
  type?: 'service' | 'app' | 'task';
  mode?: 'new' | 'enhance';
  localPath: string; // 本地路径（必填）
  repositoryUrl?: string;
  status: 'active' | 'archived';
  workflowTemplateId?: string;
  workflowTemplate?: WorkflowTemplate;
  createdAt: string;
  updatedAt: string;
}

// Thread
export interface Thread {
  id: string;
  projectId: string;
  name: string; // 任务名称
  status: ThreadStatus;
  currentPhase: Phase;
  currentAgent?: string;
  depth: number;
  abortToken?: string;
  workflowTemplateId?: string; // 绑定的工作流模板ID
  createdAt: string;
  updatedAt: string;
}

// 消息
export interface Message {
  id: string;
  threadId: string;
  role: MessageRole;
  agentId?: string;
  content: string;
  messageType: MessageType;
  metadata?: Record<string, unknown>;
  createdAt: string;
}

// 基础Agent配置
export interface BaseAgent {
  id: string;
  name: string;
  type: BaseAgentType;
  apiUrl?: string;
  apiToken?: string;
  defaultModel: string;
  cliPath: string;
  gitBashPath?: string; // Windows下git-bash路径，用于Claude CLI
  maxTokens: number;
  timeoutMinutes: number;
  createdAt: string;
  updatedAt: string;
}

// 基础Agent类型信息
export interface BaseAgentTypeInfo {
  type: BaseAgentType;
  name: string;
  description: string;
}

// Agent配置（现在叫Agent角色）
export interface AgentConfig {
  id: string;
  name: string;
  role: AgentRole;
  baseAgentId?: string;
  baseAgent?: BaseAgent;
  description: string;
  systemPrompt: string;
  modelName: string;
  maxTokens: number;
  temperature: number;
  routingConfig: RoutingConfig;
  isDefault: boolean;
  isSystem: boolean;  // 是否为系统预置角色
  createdAt: string;
  updatedAt: string;
}

// 路由配置
export interface RoutingConfig {
  canRouteTo: AgentRole[];
  routeOnSignal: string[];
}

// Agent调用
export interface AgentInvocation {
  id: string;
  threadId: string;
  agentConfigId: string;
  role: AgentRole;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  input: string;
  output?: string;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
}

// 工作产物
export interface Artifact {
  id: string;
  threadId: string;
  type: 'code' | 'document' | 'review' | 'test' | 'config';
  name: string;
  path?: string;
  content: string;
  metadata?: Record<string, unknown>;
  createdAt: string;
}

// WebSocket消息
export interface WSMessage {
  type: string;
  threadId: string;
  timestamp: number;
  payload: Record<string, unknown>;
}

// 评审等级
export type ReviewGrade = 'P1' | 'P2' | 'P3';

// 评审问题
export interface ReviewIssue {
  id: string;
  grade: ReviewGrade;
  description: string;
  file?: string;
  line?: number;
  status: 'open' | 'resolved';
}

// 合并检查结果
export interface MergeCheckResult {
  decision: 'allow' | 'block' | 'conditional';
  summary: string;
  p1Issues: number;
  p2Issues: number;
  p3Issues: number;
  resolvedP1: number;
  unresolved: ReviewIssue[];
  recommendations: string[];
}

// Phase显示名称
export const PhaseLabels: Record<Phase, string> = {
  requirement: '需求分析',
  design: '架构设计',
  development: '开发实现',
  review: '代码评审',
  test: '测试验证',
  merge: '合并部署',
  complete: '完成',
};

// Agent角色显示名称
export const AgentRoleLabels: Record<AgentRole, string> = {
  requirement: '需求分析师',
  architect: '架构师',
  developer: '开发者',
  reviewer: '评审者',
  testengineer: '测试工程师',
  devops: '运维工程师',
  custom: '自定义',
};

// Phase对应的颜色
export const PhaseColors: Record<Phase, string> = {
  requirement: '#1890ff',
  design: '#722ed1',
  development: '#52c41a',
  review: '#faad14',
  test: '#eb2f96',
  merge: '#13c2c2',
  complete: '#52c41a',
};
// Phase 状态
export type PhaseStatus = 'completed' | 'running' | 'pending' | 'needs_review';

// 产物类型显示名称
export const ArtifactTypeLabels: Record<string, string> = {
  document: '文档',
  code: '代码',
  review: '评审报告',
  config: '配置文件',
  test: '测试文件',
};

// Transition类型
export type TransitionType = 'sequence' | 'parallel' | 'merge';

// Transition转换规则
export interface Transition {
  fromAgentId: string;
  toAgentId: string;
  type: TransitionType;
  trigger?: string;
  condition?: string;
  waitFor?: string[];
  messageTemplate?: string;
  description?: string;
}

// 工作流模板
export interface WorkflowTemplate {
  id: string;
  name: string;
  description: string;
  agentIds: string[];
  transitions: Transition[];
  checkpoints: string[];
  estimatedTime: string;
  isSystem: boolean;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
}

// 文件信息
export interface FileInfo {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  modTime: string;
}

// 文件列表响应
export interface ListFilesResponse {
  path: string;
  files: FileInfo[];
  hasMore: boolean;
}

// ========== Debug功能相关类型 ==========

// WebSocket消息类型（用于debug功能）
export type WSMessageType =
  | 'agent_output_chunk'
  | 'agent_message'
  | 'system_message'
  | 'sandbox_ready'
  | 'thread_expired';

// Agent输出块
export interface AgentOutputChunk {
  chunk: string;
  invocationId: string;
  agentId: string;
  agentName: string;
}

// Agent完整消息
export interface AgentMessage {
  messageId: string;
  agentId: string;
  agentName: string;
  agentRole: string;
  content: string;
}

// 系统消息
export interface SystemMessage {
  content: string;
  level?: 'info' | 'warning' | 'error';
}

// 沙箱就绪消息
export interface SandboxReady {
  url: string;
}

// Debug用的WebSocket消息（扩展版）
export interface WSMessageDebug {
  type: WSMessageType;
  payload: Record<string, unknown>;
  threadId?: string;
  timestamp: number; // Unix timestamp from backend (int64)
}

// 沙箱服务器信息
export interface SandboxServer {
  id: string;
  threadId: string;
  projectPath: string;
  mode: string;
  port: number;
  url: string;
  status: string;
  containerId?: string;
}

// ========== Skill 相关类型 ==========

// Skill来源类型
export type SkillSourceType = 'platform' | 'personal' | 'federated';

// Skill状态
export type SkillStatus = 'active' | 'deprecated';

// Skill
export interface Skill {
  id: string;
  name: string;
  description?: string;
  tags?: string[];
  sourceType: SkillSourceType;
  sourceRegistryId?: string;
  authorId?: string;
  projectId?: string;
  supportedAgents?: string[];
  version: string;
  useCount: number;
  status: SkillStatus;
  isPublic: boolean;
  createdAt: string;
  updatedAt: string;
}

// 创建Skill请求
export interface CreateSkillRequest {
  name: string;
  description?: string;
  tags?: string[];
  sourceType: SkillSourceType;
  supportedAgents?: string[];
  version?: string;
  isPublic?: boolean;
}

// 更新Skill请求
export interface UpdateSkillRequest {
  description?: string;
  tags?: string[];
  supportedAgents?: string[];
  version?: string;
  status?: SkillStatus;
  isPublic?: boolean;
}

// Skill列表查询参数
export interface SkillListQuery {
  tag?: string;
  sourceType?: string;
  agentType?: string;
  search?: string;
  page?: number;
  pageSize?: number;
}

// 内置标签分类
export interface BuiltInTagCategory {
  name: string;
  tags: string[];
}

// Skill列表响应
export interface SkillListResponse {
  data: Skill[];
  total: number;
  page: number;
  pageSize: number;
}

// Agent-Skill绑定请求
export interface BindSkillsRequest {
  skillIds: string[];
}

// Agent绑定的Skills响应
export interface AgentSkillsResponse {
  skills: Skill[];
  count: number;
}

// Skill绑定的Agents响应
export interface SkillAgentsResponse {
  agents: AgentConfig[];
  count: number;
}

// ========== Registry 相关类型 ==========

// 注册表类型
export type RegistryType = 'github' | 'gitlab' | 'api' | 'custom';

// 同步状态
export type RegistrySyncStatus = 'pending' | 'success' | 'failed';

// 注册表状态
export type RegistryStatus = 'active' | 'inactive';

// 联邦技能源注册表
export interface SkillRegistry {
  id: string;
  name: string;
  displayName?: string;
  type: RegistryType;
  url: string;
  authConfig?: Record<string, string>;
  syncInterval: number;
  lastSyncAt?: string;
  syncStatus: RegistrySyncStatus;
  skillCount: number;
  status: RegistryStatus;
  createdAt: string;
}

// 创建注册表请求
export interface CreateRegistryRequest {
  name: string;
  displayName?: string;
  type: RegistryType;
  url: string;
  authConfig?: Record<string, string>;
  syncInterval?: number;
}

// 更新注册表请求
export interface UpdateRegistryRequest {
  displayName?: string;
  url?: string;
  authConfig?: Record<string, string>;
  syncInterval?: number;
  status?: RegistryStatus;
}

// 注册表列表查询参数
export interface RegistryListQuery {
  type?: string;
  status?: string;
  search?: string;
  page?: number;
  size?: number;
}

// 注册表列表响应
export interface RegistryListResponse {
  data: SkillRegistry[];
  total: number;
  page: number;
  pageSize: number;
}

// 同步结果
export interface SyncResult {
  registryId: string;
  registryName: string;
  skillsAdded: number;
  skillsUpdated: number;
  skillsRemoved: number;
  error?: string;
}

// ========== Knowledge Base 相关类型 ==========

// 知识库类型
export type KnowledgeBaseType = 'git' | 'mcp' | 'api';

// 知识库状态
export type KnowledgeBaseStatus = 'active' | 'inactive';

// 知识库
export interface KnowledgeBase {
  id: string;
  name: string;
  displayName?: string;
  description?: string;
  type: KnowledgeBaseType;
  config?: Record<string, string>;
  queryEndpoint?: string;
  status: KnowledgeBaseStatus;
  lastQueryAt?: string;
  queryCount: number;
  createdAt: string;
  updatedAt: string;
}

// 创建知识库请求
export interface CreateKnowledgeBaseRequest {
  name: string;
  displayName?: string;
  description?: string;
  type: KnowledgeBaseType;
  config?: Record<string, string>;
  queryEndpoint?: string;
}

// 更新知识库请求
export interface UpdateKnowledgeBaseRequest {
  displayName?: string;
  description?: string;
  config?: Record<string, string>;
  queryEndpoint?: string;
  status?: KnowledgeBaseStatus;
}

// 知识库列表查询参数
export interface KnowledgeBaseListQuery {
  type?: string;
  status?: string;
  search?: string;
  page?: number;
  size?: number;
}

// 知识库列表响应
export interface KnowledgeBaseListResponse {
  data: KnowledgeBase[];
  total: number;
  page: number;
  pageSize: number;
}

// 知识查询请求
export interface KnowledgeQueryRequest {
  query: string;
  limit?: number;
}

// 知识片段
export interface KnowledgeSnippet {
  title?: string;
  content: string;
  source: string;
  url?: string;
  relevance?: number;
  tags?: string[];
}

// 知识查询结果
export interface KnowledgeQueryResult {
  query: string;
  results: KnowledgeSnippet[];
  total: number;
  source: string;
  error?: string;
}
