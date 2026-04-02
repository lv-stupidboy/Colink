// isdp/web/src/utils/toCliEvents.ts
import type { ToolEvent } from '@/types';

/**
 * CLI 事件类型
 */
export interface CliEvent {
  id: string;
  kind: 'tool_use' | 'tool_result' | 'text';
  timestamp: number;
  label?: string;       // 工具名称 + 主要参数
  detail?: string;      // 完整详情
  content?: string;     // 文本内容（仅 text 类型）
  status?: 'running' | 'success' | 'failed';
  duration?: number;
}

/**
 * 清理工具标签：移除 "catId → " 前缀
 * e.g. "opus → Read" → "Read", "opus → Bash" → "Bash"
 */
function cleanToolLabel(label: string): string {
  const arrowIdx = label.indexOf(' → ');
  return arrowIdx >= 0 ? label.slice(arrowIdx + 3) : label;
}

/**
 * 截断参数值
 */
function truncateArg(val: string, max = 60): string {
  if (val.length <= max) return val;
  return `${val.slice(0, max - 3)}...`;
}

/**
 * 从工具输入中提取主要参数的键名
 */
const PRIMARY_ARG_KEYS = ['file_path', 'command', 'pattern', 'url', 'query', 'prompt', 'path'] as const;

/**
 * 从工具输入 JSON 中提取主要参数
 * 用于在工具列表中显示简短描述
 */
function extractPrimaryArg(input?: Record<string, unknown>): string | undefined {
  if (!input) return undefined;

  // 优先提取已知的键
  for (const key of PRIMARY_ARG_KEYS) {
    const val = input[key];
    if (typeof val === 'string' && val.length > 0) {
      return truncateArg(val);
    }
  }

  // 没有已知键，取第一个字符串值
  for (const val of Object.values(input)) {
    if (typeof val === 'string' && val.length > 0 && val.length <= 80) {
      return truncateArg(val);
    }
  }

  return undefined;
}

/**
 * 将 ToolEvent[] 转换为 CliEvent[]
 * 统一工具事件格式，提取主要参数显示
 */
export function toCliEvents(toolEvents: ToolEvent[] | undefined): CliEvent[] {
  const events: CliEvent[] = [];

  if (!toolEvents || toolEvents.length === 0) {
    return events;
  }

  for (const te of toolEvents) {
    if (te.status === 'running') {
      // 工具调用中
      const toolName = te.name || cleanToolLabel(te.id);
      const primaryArg = extractPrimaryArg(te.input);
      events.push({
        id: te.id,
        kind: 'tool_use',
        timestamp: te.startedAt || Date.now(),
        label: primaryArg ? `${toolName} ${primaryArg}` : toolName,
        detail: te.input ? JSON.stringify(te.input, null, 2) : undefined,
        status: te.status,
        duration: te.duration,
      });
    } else {
      // 工具结果
      events.push({
        id: te.id,
        kind: 'tool_result',
        timestamp: te.completedAt || Date.now(),
        label: te.name,
        detail: te.output ? (te.output.length > 500 ? `${te.output.slice(0, 500)}...` : te.output) : undefined,
        status: te.status,
        duration: te.duration,
      });
    }
  }

  return events;
}

/**
 * 构建 CLI 输出摘要
 */
export function buildCliSummary(events: CliEvent[], status: 'streaming' | 'done' | 'failed'): string {
  const toolCount = events.filter(e => e.kind === 'tool_use').length;
  const timestamps = events.map(e => e.timestamp).filter(Boolean);

  let duration = '';
  if (timestamps.length >= 2 && status !== 'streaming') {
    const ms = Math.max(...timestamps) - Math.min(...timestamps);
    const s = Math.round(ms / 1000);
    duration = s < 60 ? `${s}s` : `${Math.floor(s / 60)}m${s % 60}s`;
    duration = ` · ${duration}`;
  }

  if (status === 'streaming') {
    const lastTool = [...events].reverse().find(e => e.kind === 'tool_use');
    return `CLI Output · streaming${lastTool ? ` · ${lastTool.label}...` : ''}`;
  }

  if (toolCount > 0) {
    return `CLI Output · ${status} · ${toolCount} tool${toolCount > 1 ? 's' : ''}${duration}`;
  }

  return `CLI Output · ${status}`;
}