import React from 'react';
import type { AgentInvocation } from '@/types';

interface Props {
  activeAgents: AgentInvocation[];
  completedAgents: AgentInvocation[];
}

export const SessionChainPanel: React.FC<Props> = ({ activeAgents, completedAgents }) => {
  const allAgents = [...completedAgents, ...activeAgents];

  if (allAgents.length === 0) {
    return null;
  }

  return (
    <div className="status-section">
      <div className="status-section-title">调用链</div>
      <div className="chain-timeline">
        {allAgents.map((agent) => (
          <div key={agent.id} className="chain-item">
            <span className={`chain-dot ${agent.status}`} />
            <div className="chain-content">
              <span className="chain-name">
                {agent.agentName || agent.role || agent.id.slice(0, 8)}
              </span>
            </div>
            <span className={`chain-status ${agent.status}`}>
              {agent.status === 'running' ? '运行' :
               agent.status === 'streaming' ? '输出' :
               agent.status === 'completed' ? '完成' : '失败'}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
};

export default SessionChainPanel;