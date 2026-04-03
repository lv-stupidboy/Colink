// isdp/web/src/components/thread/ChatMessage.tsx
import React, { memo } from 'react';
import { Tag, Button, Tooltip, Alert, Card, Space } from 'antd';
import { StopOutlined, ReloadOutlined, FileTextOutlined, ExclamationCircleOutlined } from '@ant-design/icons';
import type { Message, AgentConfig, AgentRole, MessageRole, ReviewIssue, ToolEvent } from '@/types';
import type { FileChange } from '@/types/content';
import { getAgentStyle, AGENT_STYLES, USER_MESSAGE_STYLE, SYSTEM_MESSAGE_STYLE } from '@/config/agentStyles';
import { MessageContentEnhanced } from './MessageContentEnhanced';
import { ThinkingBlock, getThinkingContent } from './ThinkingBlock';
import { CliOutputBlock } from './CliOutputBlock';
import { toCliEvents } from '@/utils/toCliEvents';
import { ReviewReport } from '@/components/ReviewReport';
import { parseContentBlocks, shouldShowInPanel, shouldShowInBubble, parseCodeFiles } from '@/utils/contentDetector';
import { ContentCard, CodePreviewButton } from '.';
import './ChatMessage.css';

/**
 * 聊天消息组件
 * 支持品种样式、@提及高亮、思考块、文件路径链接、进度状态、产物卡片、审查报告、CLI输出块
 */

// 进度状态类型
export type ProgressStatus = 'thinking' | 'tool_use' | 'generating' | 'idle';

// 进度信息接口
export interface ProgressInfo {
  status: ProgressStatus;
  toolName?: string;
  toolInput?: Record<string, unknown>;
  thinkingText?: string;
}

interface ChatMessageProps {
  message: Message;
  agentConfig?: AgentConfig;
  agentConfigs?: AgentConfig[]; // 用于 @提及颜色匹配
  projectPath?: string;         // 用于文件路径链接
  isStreaming?: boolean;
  progress?: ProgressInfo;      // 进度状态
  toolEvents?: ToolEvent[];     // 工具事件列表
  onStop?: () => void;          // 终止回调
  onRetry?: () => void;         // 重试回调
  onOpenCodePanel?: (files: FileChange[]) => void; // 打开代码面板
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
      return agentRole ? getAgentStyle(agentRole) : AGENT_STYLES.custom;
    default:
      return AGENT_STYLES.custom;
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
      requirement: '需求分析师',
      architect: '架构师',
      developer: '开发者',
      reviewer: '评审者',
      testengineer: '测试工程师',
      devops: '运维工程师',
      fullstack_engineer: '全栈工程师',
      custom: '自定义',
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
 * 单条聊天消息组件
 */
export const ChatMessage: React.FC<ChatMessageProps> = memo(({
  message,
  agentConfig,
  agentConfigs = [],
  projectPath,
  isStreaming = false,
  progress,
  toolEvents = [],
  onStop,
  onRetry,
  onOpenCodePanel,
}) => {
  const isUser = message.role === 'user';
  const isSystem = message.role === 'system';
  const agentRole = agentConfig?.role;

  const styleConfig = getStyleByRole(message.role, agentRole);
  const roleDisplay = getRoleDisplayName(message.role, agentRole);
  const agentColor = styleConfig.color;

  // 检查是否有思考内容（从 metadata 或进度状态）
  const thinkingContent = getThinkingContent(message) || progress?.thinkingText;

  // 检查是否有产物和审查报告
  const hasArtifact = Boolean(message.metadata?.artifact);
  const hasReview = Boolean(message.metadata?.reviewReport);

  // 检查是否有工具事件
  const hasToolEvents = toolEvents.length > 0;

  // 解析内容块
  const contentBlocks = parseContentBlocks(message.content);

  // 消息时间格式化
  const timestamp = message.createdAt
    ? new Date(message.createdAt).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
    : '';

  // 系统消息特殊渲染
  if (isSystem) {
    const alertType = (message.metadata?.alertType as string) || 'info';
    return (
      <div className="chat-message chat-message-system" style={{ marginBottom: '16px' }}>
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
      style={{
        display: 'flex',
        flexDirection: isUser ? 'row-reverse' : 'row',
        alignItems: 'flex-start',
        marginBottom: '16px',
      }}
    >
      {/* 消息头像/图标 */}
      <div
        className="chat-message-avatar"
        style={{
          width: '36px',
          height: '36px',
          borderRadius: '50%',
          backgroundColor: isUser ? '#52c41a' : styleConfig.color,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: '#fff',
          fontSize: '14px',
          fontWeight: 500,
          marginLeft: isUser ? '12px' : 0,
          marginRight: isUser ? 0 : '12px',
        }}
      >
        {isUser ? 'U' : roleDisplay.slice(0, 2)}
      </div>

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
                color: 'var(--text-primary, #262626)',
                fontSize: '14px',
              }}
            >
              {agentConfig?.name || message.agentName || roleDisplay}
            </span>
            <span
              className="chat-message-role"
              style={{
                color: 'var(--text-secondary, #8c8c8c)',
                fontSize: '12px',
              }}
            >
              {roleDisplay}
            </span>
            {timestamp && (
              <span
                className="chat-message-time"
                style={{
                  color: 'var(--text-secondary, #bfbfbf)',
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

        {/* 思考块 - 仅在有实际思考内容时显示 */}
        {thinkingContent && !isUser && (
          <div style={{ marginBottom: '8px' }}>
            <ThinkingBlock
              content={thinkingContent}
              defaultExpanded={false}
              breedColor={agentColor}
            />
          </div>
        )}

        {/* 工具调用信息 */}
        {progress?.status === 'tool_use' && progress.toolInput && Object.keys(progress.toolInput).length > 0 && (
          <div style={{
            marginBottom: '8px',
            padding: '4px 8px',
            background: 'var(--bg-sidebar, #fafafa)',
            borderRadius: '4px',
            fontSize: '12px',
            color: 'var(--text-secondary, #666)',
          }}>
            {String(progress.toolInput.description || progress.toolInput.command || JSON.stringify(progress.toolInput).slice(0, 100))}
          </div>
        )}

        {/* 内容块渲染 */}
        <div
          className={`chat-message-bubble ${styleConfig.radius}`}
          style={{
            backgroundColor: 'var(--bg-container, #fff)',
            border: isUser ? '1px solid #52c41a' : `1px solid ${styleConfig.color}20`,
            padding: '12px 16px',
            wordBreak: 'break-word',
            ...(styleConfig.font ? { fontFamily: 'monospace' } : {}),
          }}
        >
          {contentBlocks.map((block, index) => {
            // 视觉内容：气泡内卡片
            if (shouldShowInBubble(block.type)) {
              return (
                <ContentCard
                  key={index}
                  type={block.type}
                  content={block.content}
                  title={block.filename}
                  language={block.language}
                />
              );
            }

            // 代码：预览入口按钮
            if (shouldShowInPanel(block.type) && onOpenCodePanel) {
              const files = parseCodeFiles(block);
              return (
                <CodePreviewButton
                  key={index}
                  files={files}
                  onClick={() => onOpenCodePanel(files)}
                />
              );
            }

            // 默认：使用增强的消息内容渲染
            return (
              <MessageContentEnhanced
                key={index}
                content={block.content}
                agentConfigs={agentConfigs}
                projectPath={projectPath}
              />
            );
          })}
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
            <span style={{ color: 'var(--text-secondary, #8c8c8c)' }}>
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

        {/* CLI 输出块 - 工具调用列表 */}
        {hasToolEvents && (
          <CliOutputBlock
            events={toCliEvents(toolEvents)}
            status={isStreaming ? 'streaming' : 'done'}
            breedColor={agentColor}
          />
        )}

        {/* 用户消息时间戳 */}
        {isUser && timestamp && (
          <div
            style={{
              textAlign: 'right',
              color: 'var(--text-secondary, #bfbfbf)',
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