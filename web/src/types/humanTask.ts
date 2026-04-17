// 人工任务状态
export type HumanTaskStatus = 'pending' | 'in_progress' | 'completed' | 'rejected' | 'failed';

// 人工任务类型
export type HumanTaskType = 'task_dispatch' | 'review' | 'confirm';

// 人工任务
export interface HumanTask {
  id: string;
  threadId: string;
  roleConfigId: string;
  roleName: string;
  taskType: HumanTaskType;
  taskContent: string;
  expectedOutput: string;
  sourceAgentId: string;
  sourceAgentName: string;
  status: HumanTaskStatus;
  submittedAt?: string;
  submittedBy?: string;
  outputContent?: string;
  outputFiles?: string[];
  targetAgentId?: string;
  createdAt: string;
  updatedAt: string;
}

// 提交人工任务请求
export interface SubmitHumanTaskRequest {
  outputContent: string;
  outputFiles?: string[];
}

// 提交人工任务响应
export interface SubmitHumanTaskResponse {
  success: boolean;
  nextAgent?: {
    id: string;
    name: string;
  };
  triggered: boolean;
}