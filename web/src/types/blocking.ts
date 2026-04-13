// web/src/types/blocking.ts

/** 阻塞类型 - Agent执行完成且没有调用下一个agent */
export type BlockingType = 'schedule_end';

/** 阻塞项 */
export interface BlockingItem {
  id: string;                    // 唯一标识
  type: BlockingType;            // 阻塞类型
  sourceAgentId: string;         // 来源 Agent ID
  sourceAgentName: string;       // 来源 Agent 名称
  summary: string;               // 一句话摘要
  details?: string[];            // 关键信息列表（最多展示3条）
  timestamp: number;             // 阻塞发生时间
  invocationId?: string;         // 关联的 invocation ID
}