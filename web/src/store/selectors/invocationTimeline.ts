import type { AgentInvocation } from '@/types';

export type InvocationStatus = 'pending' | 'running' | 'streaming' | 'completed' | 'failed' | 'cancelled' | 'interrupted';

/**
 * 从 activeAgents 和 completedAgents 合并出时间线列表
 * 按调用时间倒序排列（最近的在上面）
 */
export function selectInvocationTimeline(
  activeAgents: AgentInvocation[],
  completedAgents: AgentInvocation[]
): AgentInvocation[] {
  // 合并所有调用
  const allInvocations = [...activeAgents, ...completedAgents];

  // 按 startedAt 倒序排列（最近的在上面）
  return allInvocations.sort((a, b) => {
    const timeA = new Date(a.startedAt || a.createdAt).getTime();
    const timeB = new Date(b.startedAt || b.createdAt).getTime();
    return timeB - timeA;
  });
}