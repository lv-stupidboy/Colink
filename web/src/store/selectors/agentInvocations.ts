import type { AgentInvocation } from '@/types';

export type InvocationStatus = 'pending' | 'running' | 'streaming' | 'completed' | 'failed' | 'cancelled' | 'interrupted';

export interface AgentLogItem {
  agentConfigId: string;
  agentName: string;
  recentStatus: InvocationStatus;
  lastInvokedAt: string;
  invocations: AgentInvocation[];
}

/**
 * 从 activeAgents 和 completedAgents 聚合出按 Agent 分组的数据
 * 按最近调用时间降序排序
 */
export function selectAgentLogList(
  activeAgents: AgentInvocation[],
  completedAgents: AgentInvocation[]
): AgentLogItem[] {
  const agentMap = new Map<string, AgentLogItem>();

  // 合并 active 和 completed
  const allInvocations = [...activeAgents, ...completedAgents];

  for (const inv of allInvocations) {
    const key = inv.agentConfigId || inv.agentName || inv.id;

    if (!agentMap.has(key)) {
      agentMap.set(key, {
        agentConfigId: inv.agentConfigId || '',
        agentName: inv.agentName || inv.role || 'Unknown',
        recentStatus: inv.status as InvocationStatus,
        lastInvokedAt: inv.startedAt || inv.createdAt,
        invocations: [],
      });
    }

    const entry = agentMap.get(key)!;
    entry.invocations.push(inv);

    // 更新最近状态和时间
    const invTime = new Date(inv.startedAt || inv.createdAt).getTime();
    const entryTime = new Date(entry.lastInvokedAt).getTime();
    if (invTime > entryTime) {
      entry.lastInvokedAt = inv.startedAt || inv.createdAt;
      entry.recentStatus = inv.status as InvocationStatus;
    }
  }

  // 按最近调用时间降序排序
  return Array.from(agentMap.values()).sort(
    (a, b) => new Date(b.lastInvokedAt).getTime() - new Date(a.lastInvokedAt).getTime()
  );
}