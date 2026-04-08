import React from 'react';

export type InvocationStatus = 'pending' | 'running' | 'streaming' | 'completed' | 'failed' | 'cancelled' | 'interrupted';

interface Props {
  status: InvocationStatus;
}

const statusLabels: Record<InvocationStatus, string> = {
  pending: '等待',
  running: '运行',
  streaming: '输出',
  completed: '完成',
  failed: '失败',
  cancelled: '已取消',
  interrupted: '中断',
};

export const AgentStatusBadge: React.FC<Props> = ({ status }) => {
  return (
    <span className={`agent-status-badge ${status}`}>
      {statusLabels[status] || status}
    </span>
  );
};

export default AgentStatusBadge;