// web/src/types/blocking.ts

/** 阻塞类型 */
export type BlockingType = 'tool_confirm' | 'agent_question' | 'task_blocked';

/** 阻塞优先级（数值越大优先级越高） */
export const BlockingPriority: Record<BlockingType, number> = {
  tool_confirm: 3,    // 最高：工具执行前必须确认
  agent_question: 2,  // 中等：Agent需要信息才能继续
  task_blocked: 1,    // 最低：外部阻塞项
};

/** 阻塞项 */
export interface BlockingItem {
  id: string;                    // 唯一标识
  type: BlockingType;            // 阻塞类型
  priority: number;              // 优先级数值
  sourceAgentId: string;         // 来源 Agent ID
  sourceAgentName: string;       // 来源 Agent 名称
  summary: string;               // 一句话摘要
  details?: string[];            // 关键信息列表（最多展示3条）
  timestamp: number;             // 阻塞发生时间
  invocationId?: string;         // 关联的 invocation ID（用于工具确认）
  toolName?: string;             // 工具名称（仅 tool_confirm）
  toolInput?: Record<string, unknown>;  // 工具参数（仅 tool_confirm）
  question?: string;             // Agent 提问内容（仅 agent_question）
}