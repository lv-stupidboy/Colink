// isdp/web/src/components/thread/ChatMessage.tsx
import React, { memo } from 'react';
import { Tag, Button, Tooltip, Alert, Card, Space, Avatar } from 'antd';
import { StopOutlined, ReloadOutlined, FileTextOutlined, ExclamationCircleOutlined, ThunderboltOutlined, UserOutlined } from '@ant-design/icons';
import AgentTypeIcon from '@/components/AgentTypeIcon';
import type { Message, AgentConfig, AgentRole, MessageRole, ReviewIssue, ToolEvent, MessageContentBlock } from '@/types';
import type { HumanTask, HumanTaskStatus } from '@/types';
import type { FileChange } from '@/types/content';
import { getAgentStyle, AGENT_STYLES, USER_MESSAGE_STYLE, SYSTEM_MESSAGE_STYLE } from '@/config/agentStyles';
import { ReviewReport } from '@/components/ReviewReport';
import HumanTaskCard from '@/components/HumanTaskCard';
import MessageContentRenderer from './ContentBlock/MessageContentRenderer';
import { ReplyPill } from './ContentBlock/ReplyPill';
import { WhisperBadge } from './ContentBlock/WhisperBadge';
import './ChatMessage.css';

/**
 * 聊天消息组件
 * 统一使用 contentBlocks 渲染，支持 thinking/tool_use/text 块
 */

// 进度状态类型
export type ProgressStatus = 'thinking' | 'tool_use' | 'generating' | 'idle';

// 进度信息接口
export interface ProgressInfo {
  status: ProgressStatus;
  toolName?: string;
  toolInput?: Record<string, unknown>;
}

interface ChatMessageProps {
  message: Message;
  agentConfig?: AgentConfig;
  agentConfigs?: AgentConfig[];
  projectPath?: string;
  isStreaming?: boolean;
  progress?: ProgressInfo;
  toolEvents?: ToolEvent[];
  onStop?: () => void;
  onRetry?: () => void;
  onOpenCodePanel?: (files: FileChange[]) => void;
  onReplyClick?: (messageId: string) => void;  // 点击回复引用跳转
  onQuestionSubmit?: (blockId: string, answers: Record<number, string | string[]>, invocationId: string) => void;  // AskUserQuestion 答案提交
}

/**
 * 根据消息角色获取样式配置
 */
function getStyleByRole(role: MessageRole, agentRole?: AgentRole) {
  switch (role) {
    case 'user':
      return USER_MESSAGE_STYLE;
    case 'system':
      return SYSTEM_MESSAGE_STYLE;
    case 'agent':
      return agentRole ? getAgentStyle(agentRole) : AGENT_STYLES.agent;
    default:
      return AGENT_STYLES.agent;
  }
}

/**
 * 获取角色显示名称
 */
function getRoleDisplayName(role: MessageRole, agentRole?: AgentRole): string {
  if (role === 'user') return '用户';
  if (role === 'system') return '系统';
  if (agentRole) {
    const roleLabels: Record<AgentRole, string> = {
      agent: 'Agent',
      human: 'Human',
    };
    return roleLabels[agentRole] || agentRole;
  }
  return 'Agent';
}

/**
 * 渲染进度状态标签
 */
function renderProgressTags(
  progress?: ProgressInfo,
  onStop?: () => void
): React.ReactNode {
  if (!progress || progress.status === 'idle') return null;

  return (
    <>
      {progress.status === 'thinking' && (
        <Tag color="blue" style={{ marginLeft: 8 }}>💭 思考中...</Tag>
      )}
      {progress.status === 'tool_use' && progress.toolName && (
        <Tag color="orange" style={{ marginLeft: 8 }}>🔧 执行: {progress.toolName}</Tag>
      )}
      {progress.status === 'generating' && (
        <Tag color="processing" style={{ marginLeft: 8 }}>生成中...</Tag>
      )}
      {onStop && (
        <Tooltip title="终止">
          <Button
            type="text"
            size="small"
            danger
            icon={<StopOutlined />}
            className="message-action-btn"
            onClick={onStop}
          />
        </Tooltip>
      )}
    </>
  );
}

/**
 * 过滤掉 a2a-handoff 交接块（已在调用日志面板中单独展示）
 */
function filterA2AHandoff(content: string): string {
  return content.replace(/<a2a-handoff>[\s\S]*?<\/a2a-handoff>/g, '').trim();
}

/**
 * 将纯文本内容转换为 contentBlocks（用于兼容旧消息）
 */
function contentToBlocks(content: string): MessageContentBlock[] {
  // 过滤 a2a-handoff 块
  const filteredContent = filterA2AHandoff(content);
  return [{
    id: `text-${Date.now()}`,
    type: 'text',
    content: filteredContent,
    timestamp: Date.now(),
  }];
}

/**
 * 单条聊天消息组件
 */
export const ChatMessage: React.FC<ChatMessageProps> = memo(({
  message,
  agentConfig,
  agentConfigs: _agentConfigs = [],
  projectPath: _projectPath,
  isStreaming = false,
  progress,
  onStop,
  onRetry,
  onOpenCodePanel: _onOpenCodePanel,
  onReplyClick,
  onQuestionSubmit,
}) => {
  const isUser = message.role === 'user';
  const isSystem = message.role === 'system';
  const agentRole = agentConfig?.role;

  const styleConfig = getStyleByRole(message.role, agentRole);
  const roleDisplay = getRoleDisplayName(message.role, agentRole);

  // 头像颜色
  const avatarColor = isUser ? '#52c41a' : 'var(--color-primary)';

  // 统一使用 contentBlocks，如果没有则从 content 转换
  // 同时过滤掉 a2a-handoff 块（已在调用日志面板中单独展示）
  const contentBlocks: MessageContentBlock[] = message.contentBlocks && message.contentBlocks.length > 0
    ? message.contentBlocks.map(block => {
        // 只过滤 text 类型的块
        if (block.type === 'text' && block.content) {
          return { ...block, content: filterA2AHandoff(block.content) };
        }
        return block;
      })
    : contentToBlocks(message.content);

  // 检查是否有产物和审查报告
  const hasArtifact = Boolean(message.metadata?.artifact);
  const hasReview = Boolean(message.metadata?.reviewReport);

  // 检查是否有回复引用
  const hasReplyTo = Boolean(message.replyTo && message.replyPreview);

  // 检查是否是悄悄话
  const isWhisper = message.visibility === 'whisper';
  const isRevealed = Boolean(isWhisper && message.revealedAt && new Date(message.revealedAt) <= new Date());

  // Token 使用统计
  const tokenUsage = message.tokenUsage;

  // 消息时间格式化
  const timestamp = message.createdAt
    ? new Date(message.createdAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
    : '';

  // 检查是否是人工任务卡片消息（通过 metadata.type 判断）
  if (message.metadata?.type === 'human_task') {
    const metadata = message.metadata;

    // 验证必需字段是否存在
    if (!metadata.taskId || !metadata.agentName || !metadata.status) {
      console.warn('Invalid human_task metadata - missing required fields:', metadata);
      // 继续渲染普通消息，而不是返回 null
    } else {
      // 验证枚举值是否有效
      const validStatuses: HumanTaskStatus[] = ['pending', 'completed', 'cancelled'];

      const status = metadata.status as HumanTaskStatus;

      if (!validStatuses.includes(status)) {
        console.warn('Invalid human_task status:', metadata.status);
      } else {
        // 所有验证通过，安全地构建 task 对象
        const task: HumanTask = {
          id: metadata.taskId as string,
          threadId: message.threadId,
          invocationId: (metadata.invocationId as string) || '',
          agentConfigId: (metadata.agentConfigId as string) || '',
          agentName: metadata.agentName as string,
          waitReason: message.content,
          projectId: (metadata.projectId as string) || '',
          projectName: (metadata.projectName as string) || '',
          threadName: (metadata.threadName as string) || '',
          status: status,
          createdAt: message.createdAt,
        };

        return (
          <div style={{ marginBottom: '16px' }}>
            <HumanTaskCard
              task={task}
              onExecute={() => {
                // TODO: 打开执行任务模态框 - 需要状态管理支持
                // 目前用户可以到 /tasks 页面执行任务
              }}
            />
          </div>
        );
      }
    }
  }

  // 系统消息特殊渲染
  if (isSystem) {
    const alertType = (message.metadata?.alertType as string) || 'info';
    return (
      <div
        className="chat-message chat-message-system"
        data-message-id={message.id}
        data-message-role={message.role}
        data-agent-id={message.agentId || ''}
        data-agent-name={message.agentName || ''}
        style={{ marginBottom: '16px' }}
      >
        <Alert
          type={alertType === 'error' ? 'error' : alertType === 'warning' ? 'warning' : 'info'}
          message={message.metadata?.title as string || '系统消息'}
          description={message.content}
          showIcon
          banner
        />
      </div>
    );
  }

  return (
    <div
      className={`chat-message ${isUser ? 'chat-message-user' : ''}`}
      data-message-id={message.id}
      data-message-role={message.role}
      data-agent-id={message.agentId || ''}
      data-agent-name={message.agentName || ''}
      style={{
        display: 'flex',
        flexDirection: isUser ? 'row-reverse' : 'row',
        alignItems: 'flex-start',
        marginBottom: '16px',
      }}
    >
      {/* 消息头像/图标 */}
      {isUser ? (
        <Avatar
          size={36}
          icon={<UserOutlined />}
          style={{
            backgroundColor: avatarColor,
            marginLeft: '12px',
          }}
        />
      ) : (
        <div
          style={{
            width: 36,
            height: 36,
            borderRadius: '50%',
            backgroundColor: avatarColor,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            marginRight: '12px',
            position: 'relative',
            overflow: 'visible',
          }}
        >
          <AgentTypeIcon
            requiresHuman={agentConfig?.requiresHuman || false}
            isSystem={agentConfig?.isSystem || false}
            size={20}
            iconColor="#fff"
          />
        </div>
      )}

      {/* 消息主体 */}
      <div
        className="chat-message-body"
        style={{
          maxWidth: '70%',
          flex: 1,
        }}
      >
        {/* 消息头部 */}
        {!isUser && (
          <div
            className="chat-message-header"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              marginBottom: '4px',
            }}
          >
            <span
              className="chat-message-name"
              style={{
                fontWeight: 500,
                color: '#262626',
                fontSize: '14px',
              }}
            >
              {agentConfig?.name || message.agentName || roleDisplay}
            </span>
            {timestamp && (
              <span
                className="chat-message-time"
                style={{
                  color: '#bfbfbf',
                  fontSize: '12px',
                }}
              >
                {timestamp}
              </span>
            )}
            {/* 进度状态标签 */}
            {renderProgressTags(progress, onStop)}
            {/* 重试按钮（仅非流式消息显示） */}
            {!isStreaming && onRetry && (
              <Tooltip title="重试">
                <Button
                  type="text"
                  size="small"
                  icon={<ReloadOutlined />}
                  className="message-action-btn"
                  onClick={onRetry}
                />
              </Tooltip>
            )}
          </div>
        )}

        {/* 内容块渲染 */}
        <div
          className={`chat-message-bubble ${styleConfig.radius}`}
          style={{
            backgroundColor: '#fff',
            border: isUser ? '1px solid #52c41a' : `1px solid ${styleConfig.color}20`,
            padding: '12px 16px',
            wordBreak: 'break-word',
            ...(styleConfig.font ? { fontFamily: 'monospace' } : {}),
          }}
        >
          {/* 悄悄话标签 */}
          {isWhisper && (
            <WhisperBadge
              revealedAt={message.revealedAt}
              isRevealed={isRevealed}
            />
          )}
          {/* 回复引用 */}
          {hasReplyTo && (
            <ReplyPill
              replyToAgentName={message.replyToAgentName}
              replyPreview={message.replyPreview!}
              onClick={onReplyClick && message.replyTo ? () => onReplyClick(message.replyTo!) : undefined}
            />
          )}
          <MessageContentRenderer
            blocks={contentBlocks}
            defaultExpanded={false}
            agentConfigs={_agentConfigs}
            onQuestionSubmit={onQuestionSubmit}
            filterWaitingQuestions={!isStreaming}
            messageInvocationId={message.metadata?.invocationId as string | undefined}
          />
          {isStreaming && (
            <span
              className="streaming-cursor"
              style={{
                display: 'inline-block',
                width: '8px',
                height: '16px',
                backgroundColor: styleConfig.color,
                marginLeft: '4px',
                animation: 'pulse 1s infinite',
              }}
            />
          )}
        </div>

        {/* Token 使用统计 */}
        {tokenUsage && (tokenUsage.inputTokens || tokenUsage.outputTokens) && (
          <div
            className="token-usage-badge"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              marginTop: '4px',
              fontSize: '11px',
              color: '#8c8c8c',
            }}
          >
            <ThunderboltOutlined style={{ fontSize: '12px' }} />
            {tokenUsage.inputTokens && <span>输入: {tokenUsage.inputTokens}</span>}
            {tokenUsage.outputTokens && <span>输出: {tokenUsage.outputTokens}</span>}
            {tokenUsage.totalTokens && <span>总计: {tokenUsage.totalTokens}</span>}
            {tokenUsage.estimatedCost && (
              <span style={{ color: '#faad14' }}>
                ~${tokenUsage.estimatedCost.toFixed(4)}
              </span>
            )}
          </div>
        )}

        {/* 产物卡片 */}
        {hasArtifact && (
          <Card
            size="small"
            className="artifact-card-in-message"
            style={{ marginTop: 12 }}
            title={
              <Space>
                <FileTextOutlined />
                <span>产物: {String((message.metadata?.artifact as Record<string, unknown>)?.name || '产物')}</span>
              </Space>
            }
          >
            <span style={{ color: '#8c8c8c' }}>
              {String((message.metadata?.artifact as Record<string, unknown>)?.description || '点击查看详情')}
            </span>
          </Card>
        )}

        {/* 审查报告卡片 */}
        {hasReview && (
          <Card
            size="small"
            className="review-card-in-message"
            style={{ marginTop: 12 }}
            title={
              <Space>
                <ExclamationCircleOutlined />
                <span>审查报告</span>
              </Space>
            }
          >
            <ReviewReport
              result={message.metadata?.reviewReport as any}
              issues={(message.metadata?.reviewIssues as ReviewIssue[]) || []}
            />
          </Card>
        )}

        {/* 用户消息时间戳 */}
        {isUser && timestamp && (
          <div
            style={{
              textAlign: 'right',
              color: '#bfbfbf',
              fontSize: '12px',
              marginTop: '4px',
            }}
          >
            {timestamp}
          </div>
        )}
      </div>
    </div>
  );
});

ChatMessage.displayName = 'ChatMessage';