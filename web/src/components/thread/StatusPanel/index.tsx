import React, { useState } from 'react';
import { useAppStore } from '@/store';
import { AgentStatusCard } from './AgentStatusCard';
import { TokenUsage } from './TokenUsage';
import { AgentInvocationLogPanel } from './AgentInvocationLogPanel';
import { TaskProgressPanel } from './TaskProgressPanel';
import { MemoryEntriesPanel } from './MemoryEntriesPanel';
import { CopyOutlined, CheckOutlined } from '@ant-design/icons';
import './StatusPanel.css';

interface StatusPanelProps {
  width?: number;
  threadId?: string;
  projectPath?: string;
  memoryRefreshKey?: number;
}

export const StatusPanel: React.FC<StatusPanelProps> = ({ width = 320, threadId, projectPath, memoryRefreshKey = 0 }) => {
  const {
    activeAgents,
    agentUsage,
    completedAgents,
    agentTaskProgress,
    currentProject,
    currentWorkflowTemplate,
    debugProjectPath,
  } = useAppStore();
  const [copied, setCopied] = useState(false);

  // 计算调用统计
  const invocationStats = {
    total: activeAgents.length + completedAgents.length,
    running: activeAgents.length,
    completed: completedAgents.filter(a => a.status === 'completed').length,
    failed: completedAgents.filter(a => a.status === 'failed').length,
  };

  // 计算 Token 总计
  const totalUsage = Object.values(agentUsage).reduce(
    (acc, u) => ({
      input: acc.input + (u.inputTokens || 0),
      output: acc.output + (u.outputTokens || 0),
      cache: acc.cache + (u.cacheReadTokens || 0),
      cost: acc.cost + (u.costUsd || 0),
    }),
    { input: 0, output: 0, cache: 0, cost: 0 }
  );

  const copyThreadId = () => {
    if (threadId) {
      navigator.clipboard.writeText(threadId);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const displayThreadId = threadId ? threadId.slice(0, 8) : '—';

  return (
    <aside className="status-panel" style={{ width }}>
      {/* Thread ID + 调用统计 合并区块 */}
      <div className="status-section thread-info-section">
        <div className="thread-id-row">
          <span className="thread-id-label">Thread ID</span>
          <span className="thread-id-value">{displayThreadId}</span>
          {threadId && (
            <span className="thread-id-copy" onClick={copyThreadId}>
              {copied ? <CheckOutlined /> : <CopyOutlined />}
            </span>
          )}
        </div>
        <div className="message-grid compact">
          <div className="message-item">
            <span className="message-count">{invocationStats.total}</span>
            <span className="message-label">调用</span>
          </div>
          <div className="message-item">
            <span className="message-count running">{invocationStats.running}</span>
            <span className="message-label">运行</span>
          </div>
          <div className="message-item">
            <span className="message-count completed">{invocationStats.completed}</span>
            <span className="message-label">完成</span>
          </div>
          <div className="message-item">
            <span className="message-count failed">{invocationStats.failed}</span>
            <span className="message-label">失败</span>
          </div>
        </div>
      </div>

      {/* Agent 状态 */}
      <AgentStatusCard activeAgents={activeAgents} agentUsage={agentUsage} />

      {/* Agent 调用日志（合并历史参与） */}
      <AgentInvocationLogPanel />

      {/* 记忆模块 - 默认收起 */}
      <MemoryEntriesPanel
        refreshKey={memoryRefreshKey}
        scope={{
          teamId: currentWorkflowTemplate?.id || currentProject?.workflowTemplateId,
          teamName: currentWorkflowTemplate?.name,
          projectId: currentProject?.id,
          projectName: currentProject?.name,
          workspacePath: projectPath || currentProject?.localPath || debugProjectPath,
        }}
      />

      {/* 任务进度 - 仅在有任务时显示 */}
      {Object.keys(agentTaskProgress).length > 0 && (
        <TaskProgressPanel progress={agentTaskProgress} />
      )}

      {/* Token 统计 - 默认收起 */}
      <TokenUsage usage={agentUsage} totalUsage={totalUsage} defaultCollapsed />
    </aside>
  );
};

export default StatusPanel;
