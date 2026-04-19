// 人工任务状态（简化版）
export type HumanTaskStatus = 'pending' | 'completed' | 'cancelled';

// 人工任务（简化版）
export interface HumanTask {
  id: string;
  threadId: string;
  invocationId: string;      // 关联的 invocation ID
  agentConfigId: string;     // Agent 角色配置 ID
  agentName: string;         // Agent 名称
  waitReason: string;        // 等待原因
  projectId: string;         // 项目 ID
  projectName: string;       // 项目名称
  threadName: string;        // 任务名称（Thread 名称）
  status: HumanTaskStatus;
  createdAt: string;
  completedAt?: string;      // 完成时间
}

// 提交人工任务请求（简化版）
export interface SubmitHumanTaskRequest {
  outputContent: string;
  outputFiles?: string[];
}

// 提交人工任务响应（简化版）
export interface SubmitHumanTaskResponse {
  success: boolean;
  triggered: boolean;
}