// web/src/utils/blockingDetector.ts

import type { BlockingItem } from '@/types/blocking';

/** 阻塞识别规则 */
export class BlockingDetector {

  /**
   * 检测调度结束
   * Agent 执行完成且没有调用下一个 agent
   */
  static detectScheduleEnd(
    invocationId: string,
    agentId: string,
    agentName: string,
    lastOutput?: string
  ): BlockingItem | null {
    // 调度结束阻塞项
    return {
      id: `schedule-end-${invocationId}-${Date.now()}`,
      type: 'schedule_end',
      sourceAgentId: agentId,
      sourceAgentName: agentName,
      summary: `Agent 执行完成，等待下一步指示`,
      details: lastOutput ? [lastOutput.slice(0, 100)] : undefined,
      timestamp: Date.now(),
      invocationId,
    };
  }
}