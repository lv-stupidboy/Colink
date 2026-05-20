import React, { useState, useEffect, memo } from 'react';
import { useAppStore } from '@/store';
import type { MessageContentBlock, ToolUseBlock, ThinkingBlock as ThinkingBlockType, TextBlock as TextBlockType, RichBlock, AgentConfig, QuestionBlock, CardRichBlock, DiffRichBlock, ChecklistRichBlock } from '@/types';

// ✅ 条件日志：仅在开发环境且本地存储开启 debug 时输出
const DEBUG_LOGS = process.env.NODE_ENV === 'development' &&
  typeof window !== 'undefined' &&
  localStorage.getItem('debug_message_renderer') === 'true';
import ThinkingBlockComponent from './ThinkingBlock';
import ToolBlockComponent, { ToolCallRow } from './ToolBlock';
import TextBlockComponent from './TextBlock';
import QuestionBlockComponent from './QuestionBlock';
import { RichBlocks } from './RichBlocks';
import './ContentBlock.css';

interface MessageContentRendererProps {
  blocks: MessageContentBlock[];
  defaultExpanded?: boolean;
  agentConfigs?: AgentConfig[];
  onInteractiveAction?: (blockId: string, action: string, value?: string | string[]) => void;
  onQuestionSubmit?: (blockId: string, answers: Record<number, string | string[]>, invocationId: string) => void;
  // 是否过滤 waiting_user_input 状态的 question blocks
  // - true（默认）: 用于渲染已完成消息，跳过 waiting_user_input 状态的（由 StreamingMessage 渲染）
  // - false: 用于渲染 StreamingMessage 的内容，不跳过 waiting_user_input 状态的
  filterWaitingQuestions?: boolean;
  // 消息的 invocationId（从 metadata 获取），用于历史消息中 question block 缺少 invocationId 时作为备选
  messageInvocationId?: string;
}

/**
 * 消息内容渲染器
 *
 * 智能聚合渲染：
 * - 连续的 thinking 块合并为一个
 * - 连续的 tool_use 块聚合为一个工具面板
 * - text 块直接渲染
 * - rich 块按 richType 渲染不同组件
 *
 * Question blocks 渲染分工：
 * - 在 streamingContentBlocks 中的 → 由 StreamingMessage 渲染（filterWaitingQuestions=false）
 * - 不在 streamingContentBlocks 中的 success/failed 状态的 → 由已完成消息渲染（filterWaitingQuestions=true）
 */
const MessageContentRenderer: React.FC<MessageContentRendererProps> = memo(({
  blocks,
  defaultExpanded = false,
  agentConfigs = [],
  onInteractiveAction,
  onQuestionSubmit,
  filterWaitingQuestions = true,
  messageInvocationId,
}) => {
  // 获取已提交的 question block IDs，用于过滤历史消息中的重复渲染
  const submittedQuestionBlockIds = useAppStore((s) => s.submittedQuestionBlockIds);

  if (!blocks || blocks.length === 0) {
    return null;
  }

  // 过滤 blocks：
  // - 当 filterWaitingQuestions=true 时（已完成消息）：
  //   跳过已提交的 question blocks（避免重复渲染）
  //   success/failed 状态的 question blocks 在历史消息中渲染（显示用户答案）
  //   waiting_user_input 状态的 question blocks 也渲染（让用户可以选择）
  // - 当 filterWaitingQuestions=false 时（StreamingMessage）：
  //   不跳过，直接渲染
  const filteredBlocks = filterWaitingQuestions
    ? blocks.filter((block) => {
        if (block.type === 'question') {
          // 已提交的 question block 直接过滤掉（避免重复渲染）
          if (submittedQuestionBlockIds.has(block.id)) {
            DEBUG_LOGS && console.log('[MessageContentRenderer] Filtering submitted question block:', block.id);
            return false;
          }
          // success/failed 状态的保留（显示用户答案）
          // waiting_user_input 状态的也保留（让用户可以选择）
        }
        return true;
      })
    : blocks;

  // 智能聚合块
  const aggregatedBlocks = aggregateBlocks(filteredBlocks);

  return (
    <div className="message-content-blocks">
      {aggregatedBlocks.map((block, index) => {
        switch (block.type) {
          case 'thinking':
            return (
              <ThinkingBlockComponent
                key={`thinking-${index}`}
                block={block as ThinkingBlockType}
                defaultExpanded={defaultExpanded}
              />
            );
          case 'tool_use_group':
            return (
              <ToolGroupBlock
                key={`tool-group-${index}`}
                tools={block.tools as ToolUseBlock[]}
                richBlocks={block.richBlocks as RichBlock[]}
                defaultExpanded={defaultExpanded}
              />
            );
          case 'question':
            // AskUserQuestion 工具使用内联组件，直接展示选项
            const qb = block as QuestionBlock;
            // 使用 question block 的 invocationId，如果没有则使用消息的 invocationId（备选）
            const effectiveInvocationId = qb.invocationId || messageInvocationId;

            // 交互启用逻辑：
            // 1. status 为 success/failed（已提交）：禁用，显示用户选择
            // 2. status 为 waiting_user_input 且 Agent 已完成：可点击
            // 3. status 为 waiting_user_input 且 Agent 正在执行：禁用，显示提示
            const isSubmitted = qb.status === 'success' || qb.status === 'failed';
            // 从 store 获取 Agent 执行状态
            const agentRunning = useAppStore.getState().isStreaming;
            const isInteractionEnabled = qb.status === 'waiting_user_input' && !agentRunning;

            DEBUG_LOGS && console.log('[MessageContentRenderer] question block:', { id: block.id, invocationId: qb.invocationId, messageInvocationId, effectiveInvocationId, status: qb.status, isInteractionEnabled, isSubmitted, agentRunning, hasOnQuestionSubmit: !!onQuestionSubmit });
            return (
              <QuestionBlockComponent
                key={block.id || `question-${index}`}
                block={block as QuestionBlock}
                onSubmit={(answers) => {
                  const questionBlock = block as QuestionBlock;
                  // 使用有效的 invocationId（block 的或消息的）
                  const submitInvocationId = questionBlock.invocationId || messageInvocationId;
                  DEBUG_LOGS && console.log('[MessageContentRenderer] onSubmit called:', { blockId: block.id, invocationId: submitInvocationId, answers });
                  if (onQuestionSubmit && submitInvocationId && block.id) {
                    onQuestionSubmit(block.id, answers, submitInvocationId);
                  } else {
                    DEBUG_LOGS && console.log('[MessageContentRenderer] onSubmit conditions not met:', { hasOnQuestionSubmit: !!onQuestionSubmit, hasInvocationId: !!submitInvocationId, hasBlockId: !!block.id });
                  }
                }}
                defaultExpanded={(block as QuestionBlock).status === 'waiting_user_input'}
                disabled={!isInteractionEnabled}
              />
            );
          case 'text':
            return (
              <TextBlockComponent
                key={`text-${index}`}
                block={block as TextBlockType}
                agentConfigs={agentConfigs}
              />
            );
          case 'rich':
            return (
              <RichBlocks
                key={`rich-${index}`}
                blocks={extractRichBlocks(block)}
                onInteractiveAction={onInteractiveAction}
              />
            );
          default:
            return null;
        }
      })}
    </div>
  );
});

/**
 * 从聚合块中提取富内容块数组
 */
function extractRichBlocks(block: MessageContentBlock | { type: 'tool_use_group'; tools: ToolUseBlock[] }): RichBlock[] {
  if (block.type === 'rich') {
    return [block as RichBlock];
  }
  if ('richBlocks' in block && Array.isArray(block.richBlocks)) {
    return block.richBlocks as RichBlock[];
  }
  return [];
}

/**
 * 聚合内容块
 * - 连续的 thinking 块合并（取最后一个，内容已由 Store 累积）
 * - 连续的 tool_use 块聚合为一个组（不含 stdout）
 * - text 块始终独立渲染，不聚合到 tool_use_group
 * - rich 块保持独立
 */
function aggregateBlocks(blocks: MessageContentBlock[]): Array<MessageContentBlock | { type: 'tool_use_group'; tools: ToolUseBlock[]; richBlocks?: RichBlock[] }> {
  const result: Array<MessageContentBlock | { type: 'tool_use_group'; tools: ToolUseBlock[]; richBlocks?: RichBlock[] }> = [];
  let currentToolGroup: ToolUseBlock[] = [];
  let currentRichBlocks: RichBlock[] = [];

  for (const block of blocks) {
    if (block.type === 'tool_use') {
      // 累积 tool_use 块
      currentToolGroup.push(block as ToolUseBlock);
    } else if (block.type === 'text') {
      // text 块始终独立输出，不再聚合到 tool_use_group
      // 先输出累积的 tool_use 组
      if (currentToolGroup.length > 0) {
        result.push({
          type: 'tool_use_group',
          tools: currentToolGroup,
          richBlocks: currentRichBlocks.length > 0 ? currentRichBlocks : undefined,
        });
        currentToolGroup = [];
        currentRichBlocks = [];
      }
      result.push(block);
    } else if (block.type === 'rich') {
      // 累积 rich 块
      currentRichBlocks.push(block as RichBlock);
    } else {
      // 遇到非 tool_use/text/rich 块，先输出累积的块
      if (currentToolGroup.length > 0) {
        result.push({
          type: 'tool_use_group',
          tools: currentToolGroup,
          richBlocks: currentRichBlocks.length > 0 ? currentRichBlocks : undefined,
        });
        currentToolGroup = [];
        currentRichBlocks = [];
      }
      // 输出累积的 rich 块
      if (currentRichBlocks.length > 0) {
        for (const richBlock of currentRichBlocks) {
          result.push(richBlock);
        }
        currentRichBlocks = [];
      }
      result.push(block);
    }
  }

  // 处理末尾的累积块
  if (currentToolGroup.length > 0) {
    result.push({
      type: 'tool_use_group',
      tools: currentToolGroup,
      richBlocks: currentRichBlocks.length > 0 ? currentRichBlocks : undefined,
    });
  }
  if (currentRichBlocks.length > 0) {
    for (const richBlock of currentRichBlocks) {
      result.push(richBlock);
    }
  }

  return result;
}

/** 格式化执行时间 */
function formatDuration(ms?: number): string {
  if (!ms) return '';
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rem = s % 60;
  return rem > 0 ? `${m}m${rem}s` : `${m}m`;
}

/** Chevron 图标 */
function ChevronIcon({ expanded, color }: { expanded: boolean; color?: string }) {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke={color || '#8c8c8c'}
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      style={{
        transition: 'transform 0.15s',
        transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)',
      }}
    >
      <polyline points="9 18 15 12 9 6" />
    </svg>
  );
}

/** 扳手图标 */
function WrenchIcon({ color }: { color?: string }) {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke={color || '#8c8c8c'}
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" />
    </svg>
  );
}

/**
 * 工具组块组件
 * 三层折叠设计：
 * - 第 1 层：CLI Output Block 整体
 * - 第 2 层：tools 区（可独立折叠）
 * - 第 3 层：单个工具（点击展开看细节）
 */
interface ToolGroupBlockProps {
  tools: ToolUseBlock[];
  richBlocks?: RichBlock[];
  defaultExpanded?: boolean;
}

/** rich 块预览构建（从 richBlocks 提取摘要） */
function buildRichPreview(richBlocks?: RichBlock[], maxChars = 48): string {
  if (!richBlocks || richBlocks.length === 0) return '';

  for (const block of richBlocks) {
    if (block.richType === 'card') {
      const desc = (block as CardRichBlock).description;
      if (desc) {
        return desc.length > maxChars ? `${desc.slice(0, maxChars)}…` : desc;
      }
    }
    if (block.richType === 'diff') {
      const diff = block as DiffRichBlock;
      return `${diff.filename} (${diff.additions}+/${diff.deletions}-)`;
    }
    if (block.richType === 'checklist') {
      const list = block as ChecklistRichBlock;
      const done = list.items.filter(i => i.checked).length;
      return `${done}/${list.items.length} items`;
    }
  }
  return '';
}

const ToolGroupBlock: React.FC<ToolGroupBlockProps> = memo(({ tools, richBlocks, defaultExpanded = false }) => {
  if (tools.length === 0) return null;

  // 单个工具：直接用单行显示
  if (tools.length === 1) {
    return <ToolBlockComponent block={tools[0]} defaultExpanded={defaultExpanded} />;
  }

  // 多个工具：三层折叠显示
  const anyStreaming = tools.some(t => t.status === 'streaming');
  const anyFailed = tools.some(t => t.status === 'failed');
  const totalDuration = tools.reduce((sum, t) => sum + (t.duration || 0), 0);

  // rich 预览
  const richPreview = buildRichPreview(richBlocks);

  // 第 1 层：整体折叠状态
  const [blockExpanded, setBlockExpanded] = useState(defaultExpanded);
  // 第 2 层：tools 区折叠状态
  const [toolsCollapsed, setToolsCollapsed] = useState(true);

  const userInteracted = React.useRef(false);

  // 完成后自动折叠（除非用户操作过）
  const prevStreamingRef = React.useRef(anyStreaming);
  useEffect(() => {
    if (prevStreamingRef.current && !anyStreaming) {
      if (!userInteracted.current) {
        setBlockExpanded(false);
        setToolsCollapsed(true);
      }
    }
    prevStreamingRef.current = anyStreaming;
  }, [anyStreaming]);

  const handleToggleBlock = () => {
    userInteracted.current = true;
    setBlockExpanded(v => !v);
  };

  const handleToggleTools = () => {
    userInteracted.current = true;
    setToolsCollapsed(v => !v);
  };

  // 状态文本和颜色
  const statusText = anyStreaming ? 'running' : anyFailed ? 'failed' : 'completed';
  const accentColor = '#7C3AED';

  // 摘要行预览（折叠时显示）
  const previewDisplay = richPreview && !blockExpanded ? ` · ${richPreview}` : '';

  return (
    <div className="tool-block-wrapper" style={{ marginTop: 8 }}>
      {/* 第 1 层 Header - CLI Output Block 整体 */}
      <button
        type="button"
        onClick={handleToggleBlock}
        className="tool-block-header"
        aria-expanded={blockExpanded}
        aria-controls="cli-output-body"
      >
        <ChevronIcon expanded={blockExpanded} color={accentColor} />
        <WrenchIcon color="#6B7280" />
        <span className="tool-block-label">CLI Output</span>
        <span className="tool-block-status" style={{ color: anyStreaming ? accentColor : anyFailed ? '#ff4d4f' : '#52c41a' }}>
          · {statusText}
        </span>
        <span className="tool-block-duration-badge">{tools.length} tools</span>
        {totalDuration > 0 && (
          <span className="tool-block-duration-badge">{formatDuration(totalDuration)}</span>
        )}
        {previewDisplay && (
          <span className="tool-block-preview">{previewDisplay}</span>
        )}
      </button>

      {/* 第 2 层 Body - tools 区 */}
      {blockExpanded && (
        <div className="tool-block-body" id="cli-output-body">
          {/* Tools 区折叠按钮 */}
          <div className="tool-block-summary">
            <button
              type="button"
              onClick={handleToggleTools}
              className="tool-block-summary-btn"
              aria-expanded={!toolsCollapsed}
            >
              <ChevronIcon expanded={!toolsCollapsed} color="#6B7280" />
              <span>{toolsCollapsed ? '▶' : '▼'} {tools.length} tools</span>
              <span className="tool-block-summary-collapsed">
                {toolsCollapsed ? '(collapsed)' : '(expanded)'}
              </span>
            </button>
          </div>

          {/* 第 3 层：工具列表 */}
          {!toolsCollapsed && (
            <div className="tool-block-list">
              {tools.map((tool, index) => (
                <ToolCallRow
                  key={tool.id || `tool-${index}`}
                  toolName={tool.toolName}
                  input={tool.input}
                  output={tool.output}
                  status={tool.status}
                  duration={tool.duration}
                  startedAt={tool.startedAt}
                  defaultExpanded={false}
                  accentColor={accentColor}
                />
              ))}
            </div>
          )}

          {/* rich 块渲染 */}
          {richBlocks && richBlocks.length > 0 && (
            <div className="cli-output-rich">
              <RichBlocks blocks={richBlocks} onInteractiveAction={undefined} />
            </div>
          )}
        </div>
      )}
    </div>
  );
});

ToolGroupBlock.displayName = 'ToolGroupBlock';

export default MessageContentRenderer;