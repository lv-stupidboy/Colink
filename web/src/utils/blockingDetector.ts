// web/src/utils/blockingDetector.ts

import type { BlockingItem } from '@/types/blocking';
import { BlockingPriority } from '@/types/blocking';
import type { MessageContentBlock } from '@/types';

/** 需要确认的敏感工具列表 */
const SENSITIVE_TOOLS = [
  'Bash',
  'Write',
  'Edit',
  'execute_bash_command',
  'run_code',
];

/** 阻塞识别规则 */
export class BlockingDetector {

  /**
   * 检测 Agent 主动提问
   * 匹配输出中的问句关键词
   */
  static detectAgentQuestion(
    content: string,
    invocationId: string,
    agentId: string,
    agentName: string
  ): BlockingItem | null {
    // 问句关键词模式
    const questionPatterns = [
      /请问[：:\s]*(.{10,100})/,
      /请确认[：:\s]*(.{10,100})/,
      /需要您[：:\s]*(.{10,100})/,
      /请您[：:\s]*(.{10,100})/,
      /是否[？\s]*(.{10,100})/,
      /请选择[：:\s]*(.{10,100})/,
      /请回答[：:\s]*(.{10,100})/,
    ];

    for (const pattern of questionPatterns) {
      const match = content.match(pattern);
      if (match) {
        const question = match[1]?.trim() || match[0];
        return {
          id: `question-${invocationId}-${Date.now()}`,
          type: 'agent_question',
          priority: BlockingPriority.agent_question,
          sourceAgentId: agentId,
          sourceAgentName: agentName,
          summary: `Agent提问：${question.slice(0, 50)}${question.length > 50 ? '...' : ''}`,
          question: question,
          timestamp: Date.now(),
          invocationId,
        };
      }
    }
    return null;
  }

  /**
   * 检测工具确认需求
   * 识别敏感工具的 streaming 状态 tool_use
   */
  static detectToolConfirm(
    block: MessageContentBlock,
    invocationId: string,
    agentId: string,
    agentName: string
  ): BlockingItem | null {
    if (block.type !== 'tool_use') {
      return null;
    }

    // 检查是否是 streaming 状态（未完成）
    if ('status' in block && block.status !== 'streaming') {
      return null;
    }

    const toolName = block.toolName || '';

    // 检查是否是敏感工具
    const isSensitive = SENSITIVE_TOOLS.some(t =>
      toolName.toLowerCase().includes(t.toLowerCase())
    );

    if (!isSensitive) {
      return null;
    }

    // 提取关键参数摘要
    const inputSummary = this.extractToolInputSummary(
      'input' in block ? (block.input as Record<string, unknown>) || {} : {}
    );

    return {
      id: `tool-${block.toolId || Date.now()}`,
      type: 'tool_confirm',
      priority: BlockingPriority.tool_confirm,
      sourceAgentId: agentId,
      sourceAgentName: agentName,
      summary: `工具确认：${toolName}`,
      details: inputSummary,
      timestamp: Date.now(),
      invocationId,
      toolName,
      toolInput: 'input' in block ? block.input as Record<string, unknown> : undefined,
    };
  }

  /**
   * 检测任务阻塞
   * 匹配阻塞关键词
   */
  static detectTaskBlocked(
    content: string,
    invocationId: string,
    agentId: string,
    agentName: string
  ): BlockingItem | null {
    const blockedPatterns = [
      /阻塞[：:\s]*(.{10,100})/,
      /等待[：:\s]*(.{10,100})/,
      /暂停[：:\s]*(.{10,100})/,
      /需要处理[：:\s]*(.{10,100})/,
      /待办[：:\s]*(.{10,100})/,
    ];

    for (const pattern of blockedPatterns) {
      const match = content.match(pattern);
      if (match) {
        const reason = match[1]?.trim() || match[0];
        return {
          id: `blocked-${invocationId}-${Date.now()}`,
          type: 'task_blocked',
          priority: BlockingPriority.task_blocked,
          sourceAgentId: agentId,
          sourceAgentName: agentName,
          summary: `任务阻塞：${reason.slice(0, 50)}${reason.length > 50 ? '...' : ''}`,
          details: [reason],
          timestamp: Date.now(),
          invocationId,
        };
      }
    }
    return null;
  }

  /**
   * 提取工具输入参数摘要（最多3条关键信息）
   */
  private static extractToolInputSummary(input: Record<string, unknown>): string[] {
    const summary: string[] = [];
    const priorityKeys = ['file_path', 'path', 'command', 'code', 'content', 'url'];

    for (const key of priorityKeys) {
      if (input[key] !== undefined && input[key] !== null) {
        const value = String(input[key]);
        const truncated = value.length > 60 ? value.slice(0, 60) + '...' : value;
        summary.push(`${key}: ${truncated}`);
      }
    }

    return summary.slice(0, 3);
  }
}