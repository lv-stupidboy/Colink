export type DeploymentType = 'windows' | 'linux' | 'docker';

// 图片附件类型（用于多模态输入）
export interface ImageAttachment {
  id: string;           // 唯一标识
  base64: string;       // base64 数据（不含 data:image/xxx;base64, 前缀）
  mimeType: string;     // MIME 类型：image/png, image/jpeg, image/gif 等
  filename?: string;    // 文件名（可选）
  width?: number;       // 图片宽度（可选）
  height?: number;      // 图片高度（可选）
}

export interface RuntimeConfig {
  deploymentType: DeploymentType;
  workspacePath: string;
  defaultPath: string;
}

export interface MemoryScopeIdentity {
  teamId?: string;
  teamName?: string;
  projectId?: string;
  projectName?: string;
  workspacePath?: string;
}

export interface RawMemoryFile {
  name: string;
  path: string;
  content: string;
}

export interface RawMemoryGroup {
  type?: 'team' | 'project';
  indexPath?: string;
  indexExists?: boolean;
  index?: string;
  files: RawMemoryFile[];
  missing?: string[];
  scope?: MemoryScopeIdentity;
}

export interface RawMemoryResponse {
  scope: MemoryScopeIdentity;
  team: RawMemoryGroup;
  project: RawMemoryGroup;
}

// Agent角色（human 已废弃，仅保留用于兼容）
/** @deprecated 'human' 类型已废弃 */
export type AgentRole = 'agent' | 'human';

// 基础Agent类型（动态从 API 获取，不限制具体值）
export type BaseAgentType = string;

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
  description?: string;
  type?: 'service' | 'app' | 'task';
  mode?: 'new' | 'enhance';
  localPath: string; // 本地路径（必填）
  repositoryUrl?: string; // 仓库地址
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
  workflowTemplateId?: string; // 绑定的Agent团队ID
  createdAt: string;
  updatedAt: string;
}

// 消息可见性
export type MessageVisibility = 'normal' | 'whisper';

// 消息
export interface Message {
  id: string;
  threadId: string;
  role: MessageRole;
  agentId?: string;
  agentName?: string;
  content: string;
  messageType: MessageType;
  metadata?: Record<string, unknown>;
  createdAt: string;
  // 结构化内容块（按返回顺序穿插显示）
  contentBlocks?: MessageContentBlock[];
  // 回复引用
  replyTo?: string;           // 被引用的消息ID
  replyPreview?: string;      // 引用内容预览（截断后的文本）
  replyToAgentName?: string;  // 被引用消息的Agent名称
  // 消息可见性（悄悄话）
  visibility?: MessageVisibility;
  revealedAt?: string;        // 悄悄话揭秘时间
  // Token使用统计
  tokenUsage?: TokenUsage;
  // 来源标识（影响样式）
  origin?: 'stream' | 'callback';
}

// Token使用统计
export interface TokenUsage {
  inputTokens?: number;
  outputTokens?: number;
  totalTokens?: number;
  estimatedCost?: number;  // 估算成本（美元）
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
  isDefault: boolean; // 是否为默认基础Agent
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
  maxTokens: number;
  temperature: number;
  isDefault: boolean;
  isSystem: boolean;  // 是否为系统预置角色
  requiresHuman: boolean;  // 是否需要人工参与
  mentionPatterns?: string[];  // @mention 触发模式列表
  configGeneratedAt?: string;
  configPath?: string;
  createdAt: string;
  updatedAt: string;
}

// Agent调用
export interface AgentInvocation {
  id: string;
  threadId: string;
  agentConfigId: string;
  role: AgentRole;
  requiresHuman: boolean;  // 是否需要人工参与
  status: 'pending' | 'running' | 'streaming' | 'completed' | 'failed' | 'cancelled' | 'interrupted';
  input: string;
  fullPrompt?: string; // 完整提示词（系统提示 + 历史 + 输入）
  output?: string;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
  agentName?: string;
  // Token 使用统计
  inputTokens?: number;
  outputTokens?: number;
  cacheReadTokens?: number;
  cacheCreationTokens?: number;
  costUsd?: number;
  // 耗时统计
  durationMs?: number;
  durationApiMs?: number;
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
  agent: 'AI代理',
  human: '人工',
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
  triggerHint?: string;  // "@前端开发工程师 当需要前端实现时" - 注入到源 Agent 的 system prompt
  waitFor?: string[];    // 汇聚等待的 Agent ID 列表
}

// Agent团队
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
  routableTeams?: string[]; // A2A Enhancement: 可路由到的目标 Team ID 列表
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

// 工具调用事件
export interface ToolEvent {
  id: string;
  invocationId: string;
  name: string;           // Bash, Read, Edit, etc.
  status: 'running' | 'success' | 'failed';
  input?: Record<string, unknown>;
  output?: string;
  startedAt: number;
  completedAt?: number;
  duration?: number;      // ms
}

// ========== 消息内容块类型（按返回顺序穿插显示） ==========

// 内容块类型（扩展支持富内容）
export type MessageContentBlockType = 'thinking' | 'tool_use' | 'text' | 'question' | 'rich' | 'error';

// 内容块状态
export type ContentBlockStatus = 'streaming' | 'waiting_user_input' | 'success' | 'failed';

// 基础内容块接口
export interface MessageContentBlockBase {
  id: string;
  type: MessageContentBlockType;
  timestamp: number;
}

// 思考块
export interface ThinkingBlock extends MessageContentBlockBase {
  type: 'thinking';
  content: string;
  duration?: number;      // ms
  status: ContentBlockStatus;
}

// 工具调用块
export interface ToolUseBlock extends MessageContentBlockBase {
  type: 'tool_use';
  toolName: string;
  toolId: string;
  toolIndex?: number;       // 工具在消息中的索引（用于 input_json_delta 定位）
  input?: Record<string, unknown>;
  inputJSON?: string;       // 累积的 JSON（input_json_delta 使用）
  output?: string;
  duration?: number;      // ms
  status: ContentBlockStatus;
  startedAt: number;
  completedAt?: number;
  isError?: boolean;
}

// AskUserQuestion 选项
export interface QuestionOption {
  label: string;
  description?: string;
  preview?: string;
}

// AskUserQuestion 问题项
export interface QuestionItem {
  header: string;
  question: string;
  multiSelect: boolean;
  options: QuestionOption[];
}

// AskUserQuestion 问题块
export interface QuestionBlock extends MessageContentBlockBase {
  type: 'question';
  toolName: string;
  toolId: string;
  invocationId: string;  // 关联的 invocation ID，用于提交答案时定位
  agentId?: string;      // 提出问题的 Agent ID
  agentName?: string;    // 提出问题的 Agent 名称（用于 resume 调用）
  input?: Record<string, unknown>;
  questions: QuestionItem[];
  output?: string;         // 用户选择的答案
  status: ContentBlockStatus;
  startedAt: number;
  completedAt?: number;
}

// 文本块
export interface TextBlock extends MessageContentBlockBase {
  type: 'text';
  content: string;
}

// 错误/提示块（CLI stderr 限流/重试等运行时提示，作为对话流中的常驻条目）
export interface ErrorBlock extends MessageContentBlockBase {
  type: 'error';
  content: string;
  status?: ContentBlockStatus;
}

// ========== 富内容块类型 ==========

// 富内容块类型
export type RichBlockType =
  | 'card'           // 信息卡片
  | 'diff'           // 代码差异
  | 'checklist'      // 待办清单
  | 'media_gallery'  // 图片画廊
  | 'audio'          // TTS音频
  | 'interactive'    // 交互选择/确认
  | 'html_widget'    // 沙箱iframe
  | 'file';          // 文件附件

// 富内容块基础接口
export interface RichBlockBase extends MessageContentBlockBase {
  type: 'rich';
  richType: RichBlockType;
}

// 信息卡片块
export interface CardRichBlock extends RichBlockBase {
  richType: 'card';
  title: string;
  description?: string;
  icon?: string;
  actions?: CardAction[];
  metadata?: Record<string, unknown>;
}

// 卡片动作
export interface CardAction {
  id: string;
  label: string;
  type: 'button' | 'link';
  url?: string;
  onClick?: string;  // 动作标识，由前端处理
}

// 代码差异块
export interface DiffRichBlock extends RichBlockBase {
  richType: 'diff';
  filename: string;
  language?: string;
  additions: number;
  deletions: number;
  diffContent: string;  // unified diff格式
}

// 待办清单块
export interface ChecklistRichBlock extends RichBlockBase {
  richType: 'checklist';
  title?: string;
  items: ChecklistItem[];
}

// 待办项
export interface ChecklistItem {
  id: string;
  content: string;
  checked: boolean;
  status?: 'pending' | 'done' | 'failed';
}

// 图片画廊块
export interface MediaGalleryRichBlock extends RichBlockBase {
  richType: 'media_gallery';
  images: MediaItem[];
  caption?: string;
}

// 媒体项
export interface MediaItem {
  id: string;
  url: string;
  thumbnailUrl?: string;
  caption?: string;
  width?: number;
  height?: number;
}

// 音频块（TTS）
export interface AudioRichBlock extends RichBlockBase {
  richType: 'audio';
  audioUrl?: string;
  duration?: number;
  transcript?: string;
  status?: 'generating' | 'ready' | 'error';
}

// 交互块
export interface InteractiveRichBlock extends RichBlockBase {
  richType: 'interactive';
  interactiveType: 'choice' | 'confirm' | 'input' | 'multi_select';
  prompt: string;
  options?: InteractiveOption[];
  groupId?: string;  // 用于分组多个交互块
  placeholder?: string;
  selectedOptionId?: string;
  selectedOptionIds?: string[];
  inputValue?: string;
  confirmed?: boolean;
}

// 交互选项
export interface InteractiveOption {
  id: string;
  label: string;
  description?: string;
  icon?: string;
  value?: string;
  disabled?: boolean;
}

// HTML Widget块（沙箱iframe）
export interface HtmlWidgetRichBlock extends RichBlockBase {
  richType: 'html_widget';
  iframeUrl: string;
  width?: number;
  height?: number;
  title?: string;
}

// 文件附件块
export interface FileRichBlock extends RichBlockBase {
  richType: 'file';
  filename: string;
  fileSize?: number;
  downloadUrl?: string;
  mimeType?: string;
}

// 联合类型（所有富内容块）
export type RichBlock =
  | RichBlockBase
  | CardRichBlock
  | DiffRichBlock
  | ChecklistRichBlock
  | MediaGalleryRichBlock
  | AudioRichBlock
  | InteractiveRichBlock
  | HtmlWidgetRichBlock
  | FileRichBlock;

// 联合类型（所有消息内容块）
export type MessageContentBlock =
  | ThinkingBlock
  | ToolUseBlock
  | QuestionBlock
  | TextBlock
  | ErrorBlock
  | RichBlock;

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
  sourcePath?: string; // 联邦源仓库相对路径
  authorId?: string;
  projectId?: string;
  useCount: number;
  status: SkillStatus;
  isPublic: boolean;
  createdAt: string;
  updatedAt: string;
}

// Skill 更新响应（包含受影响的角色信息）
export interface SkillUpdateResponse extends Skill {
  affectedAgents?: { id: string; name: string }[];
  affectedCount?: number;
}

// 资产绑定角色响应
export interface AssetAgentsResponse {
  agents: { id: string; name: string }[];
  count: number;
}

// 创建Skill请求
export interface CreateSkillRequest {
  name: string;
  description?: string;
  tags?: string[];
  sourceType: SkillSourceType;
  isPublic?: boolean;
}

// 更新Skill请求
export interface UpdateSkillRequest {
  description?: string;
  tags?: string[];
  status?: SkillStatus;
  isPublic?: boolean;
}

// Skill列表查询参数
export interface SkillListQuery {
  tag?: string;
  sourceType?: string;
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
export type RegistryType = 'github' | 'gitlab' | 'api' | 'custom' | 'codehub';

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

// ========== 同步冲突处理相关类型 ==========

// 同步预览 skill（同源）
export interface SyncPreviewSkill {
  name: string;
  localSkillId: string;
  description: string;
  path?: string; // 远程路径
}

// 同步冲突 skill（异源）
export interface SyncConflictSkill {
  name: string;
  description: string;
  path?: string; // 远程路径
  localSkill: LocalSkillInfo; // 复用已有类型
}

// 同步预览结果
export interface SyncPreviewResult {
  registryId: string;
  registryName: string;
  autoUpdateSkills: SyncPreviewSkill[]; // 同源同名
  conflictSkills: SyncConflictSkill[];  // 异源同名
  newSkills: RemoteSkill[];             // 远程有本地无
  skippedSkills: RemoteSkill[];         // 本地有远程无
}

// 同步操作
export interface SyncOperation {
  action: 'update' | 'skip';
  skillName: string;
  targetSkillId?: string; // 仅 update 时需要
  description: string;    // 远程 skill 描述
}

// 同步确认请求
export interface SyncConfirmRequest {
  registryId: string;
  operations: SyncOperation[];
}

// 同步确认结果
export interface SyncConfirmResult {
  updated: Skill[];
  skipped: SkippedSkill[];
  autoUpdated: number;  // 自动更新数量
  userUpdated: number;  // 用户选择更新数量
  userSkipped: number;  // 用户选择跳过数量
}

// 跳过的 skill
export interface SkippedSkill {
  name: string;
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

// ========== Subagent 相关类型 ==========

// Subagent
export interface Subagent {
  id: string;
  name: string;
  description?: string;
  content: string;
  skillId?: string;
  createdAt: string;
  updatedAt: string;
}

// 创建Subagent请求
export interface CreateSubagentRequest {
  name: string;
  description?: string;
  content: string;
  skillId?: string;
}

// 更新Subagent请求
export interface UpdateSubagentRequest {
  description?: string;
  content?: string;
}

// Subagent 更新响应（包含受影响的角色信息）
export interface SubagentUpdateResponse extends Subagent {
  affectedAgents?: { id: string; name: string }[];
  affectedCount?: number;
}

// Subagent列表查询参数
export interface SubagentListQuery {
  search?: string;
  page?: number;
  pageSize?: number;
}

// Subagent列表响应
export interface SubagentListResponse {
  data: Subagent[];
  total: number;
  page: number;
  size: number;
}

// ========== Command 相关类型 ==========

// Command
export interface Command {
  id: string;
  name: string;
  description?: string;
  content?: string;
  createdAt: string;
  updatedAt: string;
}

// 创建Command请求
export interface CreateCommandRequest {
  name: string;
  description?: string;
  content?: string; // 命令内容（可选，传入则保存文件）
}

// 更新Command请求
export interface UpdateCommandRequest {
  description?: string;
}

// Command 更新响应（包含受影响的角色信息）
export interface CommandUpdateResponse extends Command {
  affectedAgents?: { id: string; name: string }[];
  affectedCount?: number;
}

// Command列表查询参数
export interface CommandListQuery {
  search?: string;
  page?: number;
  pageSize?: number;
}

// Command列表响应
export interface CommandListResponse {
  data: Command[];
  total: number;
  page: number;
  pageSize: number;
}

// ========== Rule 相关类型 ==========

// Rule
export interface Rule {
  id: string;
  name: string;
  description?: string;
  content?: string;
  createdAt: string;
  updatedAt: string;
}

// 创建Rule请求
export interface CreateRuleRequest {
  name: string;
  description?: string;
  content?: string; // 规约内容（可选，传入则保存文件）
}

// 更新Rule请求
export interface UpdateRuleRequest {
  description?: string;
}

// Rule 更新响应（包含受影响的角色信息）
export interface RuleUpdateResponse extends Rule {
  affectedAgents?: { id: string; name: string }[];
  affectedCount?: number;
}

// Rule列表查询参数
export interface RuleListQuery {
  search?: string;
  page?: number;
  pageSize?: number;
}

// Rule列表响应
export interface RuleListResponse {
  data: Rule[];
  total: number;
  page: number;
  pageSize: number;
}

// Command绑定的Skills响应
export interface CommandSkillsResponse {
  skills: Skill[];
  count: number;
}

// Agent绑定的Rules响应
export interface AgentRulesResponse {
  rules: Rule[];
  count: number;
}

// ========== Settings 相关类型 ==========

// Settings
export interface Settings {
  id: string;
  name: string;
  description?: string;
  directoryPath?: string;
  createdAt: string;
  updatedAt: string;
}

// 创建Settings请求
export interface CreateSettingsRequest {
  name: string;
  description?: string;
}

// Settings列表查询参数
export interface SettingsListQuery {
  search?: string;
  page?: number;
  pageSize?: number;
}

// Settings列表响应
export interface SettingsListResponse {
  data: Settings[];
  total: number;
  page: number;
  pageSize: number;
}

// 更新Settings请求
export interface UpdateSettingsRequest {
  description?: string;
}

// Settings 更新响应（包含受影响的角色信息）
export interface SettingsUpdateResponse extends Settings {
  affectedAgents?: { id: string; name: string }[];
  affectedCount?: number;
}

// 绑定Settings请求
export interface BindSettingsRequest {
  settingsIds: string[];
}

// Agent绑定的Settings响应
export interface AgentSettingsResponse {
  settings: Settings[];
  count: number;
}

// ========== MCP Server 相关类型 ==========

export type MCPTransport = 'stdio' | 'http' | 'sse';
export type MCPSourceType = 'platform' | 'personal' | 'team_package' | 'federated';
export type MCPStatus = 'active' | 'disabled';

export interface MCPServer {
  id: string;
  name: string;
  displayName?: string;
  description?: string;
  transport: MCPTransport;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  url?: string;
  headers?: Record<string, string>;
  sourceType: MCPSourceType;
  status: MCPStatus;
  createdAt: string;
  updatedAt: string;
}

export interface CreateMCPServerRequest {
  name: string;
  displayName?: string;
  description?: string;
  transport: MCPTransport;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  url?: string;
  headers?: Record<string, string>;
  sourceType?: MCPSourceType;
  status?: MCPStatus;
}

export interface UpdateMCPServerRequest {
  displayName?: string;
  description?: string;
  transport?: MCPTransport;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  url?: string;
  headers?: Record<string, string>;
  sourceType?: MCPSourceType;
  status?: MCPStatus;
}

export interface MCPServerListQuery {
  search?: string;
  status?: MCPStatus;
  page?: number;
  pageSize?: number;
}

export interface MCPServerListResponse {
  data: MCPServer[];
  total: number;
  page: number;
  pageSize: number;
}

export interface AgentMCPServersResponse {
  data: MCPServer[];
}

// ImportResult 导入结果（简化版）
export interface ImportResult {
  success: number;
  skipped: number;
  failed: number;
  details?: ImportDetail[];
  configGenResults?: ConfigGenResult[]; // 配置生成结果
}

// ImportDetail 导入详情（简化版，无版本）
export interface ImportDetail {
  assetType: string;
  name: string;
  id?: string; // 成功导入时的ID
  status: string; // success, skipped, failed
  message?: string;
}

// ConfigGenResult 配置生成结果
export interface ConfigGenResult {
  agentId: string;
  agentName: string;
  status: string; // success, failed, skipped
  message?: string;
}

// 内容类型
export * from './content';

// 人工任务类型
export * from './humanTask';

// ========== 批量操作相关类型 ==========

// 批量生成配置结果
export interface BatchGenerateResult {
  total: number;
  success: number;
  failed: number;
  results: GenerateResultItem[];
}

export interface GenerateResultItem {
  agentId: string;
  agentName: string;
  status: 'success' | 'failed';
  skillsCount: number;
  commandsCount: number;
  subagentsCount: number;
  rulesCount: number;
  settingsCount: number;
  error?: string;
}

// 批量修改基础Agent结果
export interface BatchUpdateResult {
  total: number;
  success: number;
  failed: number;
  results: UpdateResultItem[];
}

export interface UpdateResultItem {
  agentId: string;
  agentName: string;
  baseAgentName: string;
  status: 'success' | 'failed';
  error?: string;
}

// ========== Team Package Sync 相关类型 ==========

// TeamPackageVersion 团队包版本信息
export interface TeamPackageVersion {
  packageName: string;
  version: string;
  installedAt: string;
  installedBy?: string;
  workflowId?: string;
}

// RemotePackageInfo 远程包信息
export interface RemotePackageInfo {
  name: string;
  version: string;
  description?: string;
  path?: string;
  category?: string; // 从 categories 结构中提取的分类名称
  author?: string;
  updatedAt?: string;
  downloadUrl?: string;
}

// RemotePackageCategory 远程包分类
export interface RemotePackageCategory {
  name: string;
  packages: RemotePackageInfo[];
}

// RemotePackageList 远程包列表
export interface RemotePackageList {
  categories: RemotePackageCategory[];
}

// UpdateCheckItem 更新检查项
export interface UpdateCheckItem {
  packageName: string;
  localVersion: string;
  remoteVersion: string;
  hasUpdate: boolean;
}

// UpdateCheckResult 更新检查结果
export interface UpdateCheckResult {
  hasUpdates: boolean;
  updates: UpdateCheckItem[];
  total: number;
}

// ImportConfirm 导入确认配置（扩展：与后端 model.TeamPackageImportConfirm 对齐）
export interface ImportConfirm {
  mode: 'overwrite' | 'skip' | 'selective';
  workflowAction: 'overwrite' | 'skip';
  roleActions: Array<{ name: string; action: 'overwrite' | 'skip' }>;
  assetActions: Array<{ assetType: string; name: string; action: 'overwrite' | 'skip' }>;
}

// ========== Market 相关类型 ==========

// Market 市场（插件源）
export interface Market {
  id: string;
  name: string;
  url: string;
  branch: string;
  enabled: boolean;
  autoUpdate: boolean;
  checkInterval: string;
  lastSyncedAt?: string;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
}

// MarketPackage 市场包信息
export interface MarketPackage {
  name: string;
  version: string;
  description: string;
  marketId: string;
  marketName: string;
  repository: string;
  source: string;
  localVersion?: string;
  localStatus: 'new' | 'update' | 'latest';
  lastImportedAt?: string;
}

// PackagePreviewResponse 团队包预览响应（扩展：包含冲突检测信息）
export interface PackagePreviewResponse {
  packageName: string;
  version: string;
  description: string;
  conflictCount: number;  // 新增：冲突总数
  previewFailed?: boolean; // 前端标记：预览失败（用于批量导入时的错误提示）
  workflow: {
    name: string;
    description: string;
    exists: boolean;  // 新增：是否已存在
  };
  roles: Array<{
    name: string;
    role: string;
    description: string;
    assets: string[]; // 如 "Skill: xxx", "Command: xxx"
    exists: boolean;  // 新增：是否已存在
  }>;
  assets: {
    skills: Array<{ name: string; description: string; exists: boolean }>;  // 新增 exists
    commands: Array<{ name: string; description: string; exists: boolean }>;
    subagents: Array<{ name: string; description: string; exists: boolean }>;
    rules: Array<{ name: string; description: string; exists: boolean }>;
    settings: Array<{ name: string; description: string; exists: boolean }>;
  };
}

// AddMarketRequest 添加市场请求
export interface AddMarketRequest {
  name: string;
  url: string;
  branch?: string;
}

// UpdateMarketRequest 更新市场请求
export interface UpdateMarketRequest {
  name?: string;
  url?: string;
  branch?: string;
  enabled?: boolean;
  autoUpdate?: boolean;
  checkInterval?: string;
}

// ========== 联邦源导入相关类型 ==========

// 本地同名 Skill 信息（用于冲突展示）
export interface LocalSkillInfo {
  id: string;
  sourceType: SkillSourceType;
  sourceRegistryId?: string;
  sourceRegistryName?: string; // 联邦源名称（如果是 federated）
  sourcePath?: string; // 本地路径
  description: string;
}

// 远程 Skill 信息（扫描结果）
export interface RemoteSkill {
  name: string;
  description: string;
  path: string;          // Skill 在仓库中的相对路径
  existsLocally: boolean; // 是否已存在本地同名 Skill
  localSkill?: LocalSkillInfo; // 本地同名 Skill 信息
}

// 扫描结果
export interface ScanResult {
  registryId: string;
  registryName: string;
  registryUrl: string;
  skills: RemoteSkill[];
}

// Skill 导入项
export interface SkillImportItem {
  name: string;
  path: string;
  description: string;
  tags: string[];
  importMode?: 'create' | 'update'; // 导入模式（默认 create）
  targetSkillId?: string;           // update 时指定目标 Skill ID
}

// 批量导入请求
export interface BatchImportRequest {
  registryId: string;
  skills: SkillImportItem[];
}

// 批量导入结果
export interface BatchImportResult {
  imported: Skill[];
  updated: Skill[];     // 更新的 Skill 列表
  skipped: SkippedSkillInfo[];
  conflictSummary?: ConflictSummary; // 冲突处理汇总
}

// 冲突处理汇总
export interface ConflictSummary {
  autoUpdated: number; // 自动更新的数量（同源）
  userCreated: number; // 用户选择新建的数量
  userUpdated: number; // 用户选择更新的数量
}

// 跳过的 Skill 信息
export interface SkippedSkillInfo {
  name: string;
  reason: string;
}

// HelpConfig 帮助入口配置
export interface HelpConfig {
  supportGroup: string;
  officialWebsite: string;
  docLink: string;
  feedbackEnabled: boolean;
}

// FeedbackRequest 问题反馈请求
export interface FeedbackRequest {
  type: string;
  description: string;
  images?: string[]; // base64数组
}
