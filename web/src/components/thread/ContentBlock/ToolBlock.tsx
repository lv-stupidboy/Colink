import React, { useState, useEffect, useRef, memo } from 'react';
import type { ToolUseBlock as ToolUseBlockType, ContentBlockStatus } from '@/types';
import './ContentBlock.css';

/**
 * 工具调用行组件
 * 只显示单行工具信息：状态图标 + 工具名 + 参数 + 耗时
 */

/** 工具摘要结构 */
interface ToolSummary {
  name: string;       // 工具名
  param: string;      // 关键参数摘要（截断后）
  paramFull?: string; // 完整参数（用于 tooltip）
}

/** 路径截断（保留文件名和关键目录） */
function truncatePath(path: string, maxLen = 50): string {
  if (!path) return '';
  if (path.length <= maxLen) return path;

  // 优先保留文件名（最后一个 / 后的内容）
  const parts = path.split('/');
  const fileName = parts.pop() || '';

  // 如果只剩文件名且超长，截断文件名
  if (parts.length === 0) {
    return fileName.length > maxLen ? `...${fileName.slice(-maxLen + 3)}` : fileName;
  }

  // 尝试保留：前两级目录 + 文件名
  if (parts.length >= 2 && fileName.length + parts[0].length + parts[1].length + 10 <= maxLen) {
    return `${parts[0]}/${parts[1]}/.../${fileName}`;
  }

  // 退化为只保留文件名
  return fileName.length > maxLen - 3 ? `...${fileName.slice(-maxLen + 3)}` : `.../${fileName}`;
}

/** 命令截断（保留命令名和关键参数） */
function truncateCommand(cmd: string, maxLen = 40): string {
  if (!cmd) return '';
  if (cmd.length <= maxLen) return cmd;

  // 尝试保留第一个单词（命令名）
  const firstWord = cmd.split(' ')[0];
  if (firstWord.length >= maxLen) {
    return `${firstWord.slice(0, maxLen - 3)}...`;
  }

  // 保留命令名 + 部分参数
  return `${cmd.slice(0, maxLen - 3)}...`;
}

/** 描述截断（保留核心意图） */
function truncateDescription(desc: string, maxLen = 25): string {
  if (!desc) return '';
  if (desc.length <= maxLen) return desc;
  return `${desc.slice(0, maxLen)}...`;
}

/** URL 截断（保留域名和路径关键部分） */
function truncateUrl(url: string, maxLen = 45): string {
  if (!url) return '';
  if (url.length <= maxLen) return url;

  try {
    const parsed = new URL(url);
    // 保留域名 + 路径前 20 字符
    const domain = parsed.hostname;
    const path = parsed.pathname.slice(0, 20);
    const result = `${domain}${path}${parsed.pathname.length > 20 ? '...' : ''}`;
    return result.length > maxLen ? result.slice(0, maxLen - 3) + '...' : result;
  } catch {
    // 解析失败，简单截断
    return `${url.slice(0, maxLen - 3)}...`;
  }
}

/** 搜索查询截断 */
function truncateQuery(query: string, maxLen = 30): string {
  if (!query) return '';
  if (query.length <= maxLen) return query;
  return `${query.slice(0, maxLen)}...`;
}

/** 问题截断 */
function truncateQuestion(question: string, maxLen = 20): string {
  if (!question) return '';
  if (question.length <= maxLen) return question;
  return `${question.slice(0, maxLen)}...`;
}

/** 默认摘要生成（提取首个关键参数） */
function generateDefaultSummary(toolName: string, input?: Record<string, unknown>): ToolSummary {
  const keys = ['file_path', 'command', 'pattern', 'url', 'query', 'path', 'content', 'name', 'id'];
  for (const key of keys) {
    const val = input?.[key];
    if (typeof val === 'string' && val.length > 0) {
      return {
        name: toolName,
        param: truncatePath(val, 40),
        paramFull: val,
      };
    }
  }
  return { name: toolName, param: '' };
}

/** 工具摘要生成（根据工具类型生成结构化摘要） */
function generateToolSummary(toolName: string, input?: Record<string, unknown>): ToolSummary {
  // 检查 input 是否有实际内容（非空对象）
  const hasInput = input && Object.keys(input).length > 0;

  switch (toolName) {
    case 'Read':
    case 'Write':
      const filePath = input?.file_path as string;
      if (filePath) {
        return { name: toolName, param: truncatePath(filePath), paramFull: filePath };
      }
      // 等待完整 input 数据时的占位文本
      return { name: toolName, param: hasInput ? '' : '...' };
    case 'Edit':
      const editPath = input?.file_path as string;
      if (editPath) {
        const param = `${truncatePath(editPath)}:${input?.old_string ? 'edit' : 'new'}`;
        return { name: toolName, param, paramFull: editPath };
      }
      return { name: toolName, param: hasInput ? '' : '...' };
    case 'Bash':
      const command = input?.command as string;
      if (command) {
        return { name: toolName, param: truncateCommand(command), paramFull: command };
      }
      // 等待完整 input 数据时的占位文本
      return { name: toolName, param: hasInput ? '' : '...' };
    case 'Grep':
      const pattern = input?.pattern as string;
      const grepPath = input?.path as string;
      if (pattern) {
        const pathPart = grepPath ? ` in ${truncatePath(grepPath, 30)}` : '';
        const param = `"${truncateQuery(pattern, 20)}"${pathPart}`;
        const paramFull = grepPath ? `"${pattern}" in ${grepPath}` : `"${pattern}"`;
        return { name: toolName, param, paramFull };
      }
      return { name: toolName, param: hasInput ? '' : '...' };
    case 'Glob':
      const globPattern = input?.pattern as string;
      if (globPattern) {
        return { name: toolName, param: globPattern, paramFull: globPattern };
      }
      return { name: toolName, param: hasInput ? '' : '...' };
    case 'Skill':
      return {
        name: toolName,
        param: input?.skill as string || 'unknown',
      };
    case 'Task':
      const taskDesc = input?.description as string;
      return {
        name: toolName,
        param: `@${input?.subagent_name || 'agent'} ${truncateDescription(taskDesc)}`,
        paramFull: taskDesc,
      };
    case 'NotebookEdit':
      const notebookPath = input?.notebook_path as string;
      return {
        name: toolName,
        param: `${truncatePath(notebookPath)}[${input?.cell_number}]`,
        paramFull: notebookPath,
      };
    case 'WebFetch':
      const url = input?.url as string;
      if (url) {
        return { name: toolName, param: truncateUrl(url), paramFull: url };
      }
      return { name: toolName, param: '' };
    case 'WebSearch':
      const query = input?.query as string;
      if (query) {
        return { name: toolName, param: `"${truncateQuery(query)}"`, paramFull: query };
      }
      return { name: toolName, param: '' };
    case 'AskUserQuestion':
      const questions = input?.questions;
      const firstQuestion = Array.isArray(questions) && questions.length > 0
        ? (questions[0] as { question?: string })?.question
        : undefined;
      return {
        name: 'Ask',
        param: truncateQuestion(firstQuestion || ''),
        paramFull: firstQuestion,
      };
    case 'TodoWrite':
      return {
        name: toolName,
        param: `${(input?.todos as unknown[])?.length || 0} items`,
      };
    case 'EnterPlanMode':
    case 'ExitPlanMode':
      return { name: toolName, param: '' };
    case 'CronCreate':
    case 'CronDelete':
    case 'CronList':
      return { name: toolName, param: 'cron job' };
    case 'ScheduleWakeup':
      return { name: toolName, param: `delay ${(input?.delaySeconds as number) || 0}s` };
    case 'EnterWorktree':
    case 'ExitWorktree':
      const worktreePath = input?.path as string || input?.name as string;
      return { name: toolName, param: truncatePath(worktreePath), paramFull: worktreePath };
    case 'Agent':
      const agentDesc = input?.description as string;
      return {
        name: toolName,
        param: `${input?.subagent_type || 'agent'}: ${truncateDescription(agentDesc)}`,
        paramFull: agentDesc,
      };
    default:
      return generateDefaultSummary(toolName, input);
  }
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

/** 状态图标 */
function StatusIcon({ status, color }: { status: ContentBlockStatus; color?: string }) {
  if (status === 'streaming') {
    return (
      <svg
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        stroke={color || '#1890ff'}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        style={{ animation: 'spin 1s linear infinite' }}
      >
        <path d="M21 12a9 9 0 1 1-6.219-8.56" />
      </svg>
    );
  }
  if (status === 'waiting_user_input') {
    // 等待用户输入：使用问号图标
    return (
      <svg
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        stroke={color || '#faad14'}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="12" cy="12" r="10" />
        <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" />
        <line x1="12" y1="17" x2="12.01" y2="17" />
      </svg>
    );
  }
  if (status === 'success') {
    return (
      <svg
        width="12"
        height="12"
        viewBox="0 0 24 24"
        fill="none"
        stroke="#52c41a"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <polyline points="20 6 9 17 4 12" />
      </svg>
    );
  }
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="#ff4d4f"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="12" r="10" />
      <line x1="15" y1="9" x2="9" y2="15" />
      <line x1="9" y1="9" x2="15" y2="15" />
    </svg>
  );
}

/** 扳手图标 */
function WrenchIcon({ color }: { color?: string }) {
  return (
    <svg
      width="11"
      height="11"
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

/** 工具行属性 */
interface ToolCallRowProps {
  toolName: string;
  input?: Record<string, unknown>;
  output?: string;
  status: ContentBlockStatus;
  duration?: number;
  startedAt?: number;
  defaultExpanded?: boolean;
  accentColor?: string;
}

/** 单行工具调用组件 */
export const ToolCallRow: React.FC<ToolCallRowProps> = memo(({
  toolName,
  input,
  output,
  status,
  duration,
  startedAt,
  defaultExpanded = false,
  accentColor = '#7C3AED',
}) => {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const [runningTime, setRunningTime] = useState(duration || 0);
  const prevStatusRef = useRef(status);

  // 实时计算运行时间
  useEffect(() => {
    if (status !== 'streaming' || !startedAt) return;

    const updateTimer = () => {
      setRunningTime(Date.now() - startedAt);
    };

    updateTimer();
    const interval = setInterval(updateTimer, 100);
    return () => clearInterval(interval);
  }, [status, startedAt]);

  const displayDuration = status === 'streaming'
    ? formatDuration(runningTime)
    : formatDuration(duration);

  // 使用结构化摘要
  const summary = generateToolSummary(toolName, input);

  const hasDetail = (input && Object.keys(input).length > 0) || output;

  // streaming 时自动展开，完成后自动折叠
  useEffect(() => {
    // streaming 状态：自动展开
    if (status === 'streaming' && !expanded) {
      setExpanded(true);
    }
    // 从 streaming 变为非 streaming：自动折叠
    if (prevStatusRef.current === 'streaming' && status !== 'streaming') {
      setExpanded(false);
    }
    prevStatusRef.current = status;
  }, [status]);

  return (
    <div className="tool-call-row-container">
      <button
        type="button"
        className={`tool-call-row ${status}`}
        onClick={() => hasDetail && setExpanded(v => !v)}
        style={{ cursor: hasDetail ? 'pointer' : 'default' }}
      >
        {/* 状态图标 */}
        <span className="tool-call-icon">
          <StatusIcon status={status} color={accentColor} />
        </span>

        {/* 扳手图标 */}
        <WrenchIcon color={status === 'streaming' ? accentColor : undefined} />

        {/* 工具名称 */}
        <span
          className="tool-call-name"
          style={{ color: status === 'streaming' ? accentColor : undefined }}
        >
          {summary.name}
        </span>

        {/* 主要参数（带 tooltip） */}
        {summary.param && (
          <span
            className="tool-call-param"
            title={summary.paramFull || summary.param}
          >
            {summary.param}
          </span>
        )}

        {/* 耗时 */}
        {displayDuration && (
          <span className="tool-call-duration">
            {status === 'streaming' && (
              <span className="tool-call-timer">{displayDuration}</span>
            )}
            {status !== 'streaming' && displayDuration}
          </span>
        )}

        {/* 展开指示器 */}
        {hasDetail && <ChevronIcon expanded={expanded} />}
      </button>

      {/* 展开的详情 */}
      {expanded && hasDetail && (
        <div className="tool-call-detail">
          {input && Object.keys(input).length > 0 && (
            <div className="tool-call-detail-section">
              <div className="tool-call-detail-label">Input:</div>
              <pre>{JSON.stringify(input, null, 2)}</pre>
            </div>
          )}
          {output && (
            <div className="tool-call-detail-section">
              <div className="tool-call-detail-label">Output:</div>
              <pre>{output.length > 2000 ? `${output.slice(0, 2000)}...` : output}</pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
});

ToolCallRow.displayName = 'ToolCallRow';

/** 工具块属性 */
interface ToolBlockProps {
  block: ToolUseBlockType;
  defaultExpanded?: boolean;
}

/**
 * 单个工具块组件（包装器）
 * 用于单个工具调用场景，提供外壳
 */
const ToolBlock: React.FC<ToolBlockProps> = memo(({ block, defaultExpanded = false }) => {
  return (
    <div className="tool-block-single">
      <ToolCallRow
        toolName={block.toolName}
        input={block.input}
        output={block.output}
        status={block.status}
        duration={block.duration}
        startedAt={block.startedAt}
        defaultExpanded={defaultExpanded}
      />
    </div>
  );
});

ToolBlock.displayName = 'ToolBlock';

export default ToolBlock;