import React from 'react';
import { RobotOutlined, StopOutlined } from '@ant-design/icons';
import type { AgentInvocation } from '@/types';
import { DurationDisplay } from './DurationDisplay';
import { useAppStore } from '@/store';

interface Props {
  activeAgents: AgentInvocation[];
}

const statusLabels: Record<string, string> = {
  pending: '等待',
  running: '运行',
  streaming: '输出',
  completed: '完成',
  failed: '失败',
  cancelled: '已取消',
  interrupted: '完成', // AskUserQuestion 等待用户输入，显示为完成
};

export const AgentStatusCard: React.FC<Props> = ({ activeAgents }) => {
  const { cancelAgent } = useAppStore();

  const handleCancel = async (agentId: string) => {
    try {
      await cancelAgent(agentId);
    } catch (err) {
      console.error('Failed to cancel agent:', err);
    }
  };

  return (
    <div className="status-section">
      <div className="status-section-title">
        <RobotOutlined />
        当前调用
      </div>
      {activeAgents.length === 0 ? (
        <div className="idle-status">空闲</div>
      ) : (
        <div className="agent-list">
          {activeAgents.map(agent => (
            <div key={agent.id} className="agent-item">
              <div className="agent-header">
                <span className={`agent-dot ${agent.status}`} />
                <span className="agent-name">{agent.agentName || agent.role || agent.id.slice(0, 8)}</span>
                <span className={`agent-status-badge ${agent.status}`}>
                  {statusLabels[agent.status] || agent.status}
                </span>
                {(agent.status === 'running' || agent.status === 'streaming') && (
                  <span
                    className="agent-cancel-btn"
                    onClick={() => handleCancel(agent.id)}
                    title="取消执行"
                  >
                    <StopOutlined />
                  </span>
                )}
              </div>
              <div className="agent-meta">
                <DurationDisplay
                  startedAt={agent.startedAt}
                  completedAt={agent.completedAt}
                  isRunning={agent.status === 'running' || agent.status === 'streaming'}
                />
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default AgentStatusCard;