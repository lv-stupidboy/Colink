import React, { useState, useMemo } from 'react';
import { FileTextOutlined, RightOutlined, ExpandOutlined, CompressOutlined, CopyOutlined, CheckOutlined } from '@ant-design/icons';
import { useAppStore } from '@/store';
import { selectAgentLogList } from '@/store/selectors/agentInvocations';
import { AgentStatusBadge, TimeDisplay } from './shared';
import { DurationDisplay } from './DurationDisplay';

/**
 * Agent 调用日志面板
 * 两层结构：
 * 1. Agent 列表（名称 + 最近状态）
 * 2. 点击 Agent 后展示调用详情（时间、状态、输入内容、耗时）
 */
export const AgentInvocationLogPanel: React.FC = () => {
  const [expanded, setExpanded] = useState(false);
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [expandedPrompt, setExpandedPrompt] = useState<string | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  // 复制内容到剪贴板
  const handleCopy = async (e: React.MouseEvent, id: string, content: string) => {
    e.stopPropagation();
    try {
      await navigator.clipboard.writeText(content);
      setCopiedId(id);
      setTimeout(() => setCopiedId(null), 2000);
    } catch (err) {
      console.error('复制失败:', err);
    }
  };

  // 从 Store 获取数据
  const activeAgents = useAppStore((state) => state.activeAgents);
  const completedAgents = useAppStore((state) => state.completedAgents);

  // 聚合数据（按 Agent 分组，按最近调用时间排序）
  const agentLogList = useMemo(() => {
    return selectAgentLogList(activeAgents, completedAgents);
  }, [activeAgents, completedAgents]);

  // 找到选中的 Agent
  const selectedAgent = selectedAgentId
    ? agentLogList.find(
        (a) => a.agentConfigId === selectedAgentId || a.agentName === selectedAgentId
      )
    : null;

  // 未展开时显示入口按钮
  if (!expanded) {
    return (
      <div className="log-panel-trigger" onClick={() => setExpanded(true)}>
        <FileTextOutlined />
        <span>调用日志</span>
        {agentLogList.length > 0 && (
          <span className="log-panel-count">{agentLogList.length}</span>
        )}
      </div>
    );
  }

  // 展开后的面板
  return (
    <div className="status-section log-panel-content">
      {/* 标题栏 */}
      <div className="section-collapse-header" onClick={() => {
        setExpanded(false);
        setSelectedAgentId(null);
      }}>
        <FileTextOutlined />
        <span>调用日志</span>
        <span className="section-collapse-count">{agentLogList.length}</span>
      </div>

      {/* 第一层：Agent 列表 */}
      {!selectedAgent ? (
        <div className="agent-log-list">
          {agentLogList.length === 0 ? (
            <div className="idle-status">暂无调用记录</div>
          ) : (
            agentLogList.map((agent) => (
              <div
                key={agent.agentConfigId || agent.agentName}
                className="agent-log-item"
                onClick={() =>
                  setSelectedAgentId(agent.agentConfigId || agent.agentName)
                }
              >
                <span className="agent-name">{agent.agentName}</span>
                <AgentStatusBadge status={agent.recentStatus} />
                <RightOutlined style={{ fontSize: 10, color: '#9ca3af' }} />
              </div>
            ))
          )}
        </div>
      ) : (
        /* 第二层：调用详情 */
        <div className="invocation-detail">
          <div
            className="detail-header"
            onClick={() => setSelectedAgentId(null)}
          >
            <RightOutlined
              style={{ transform: 'rotate(180deg)', fontSize: 10 }}
            />
            <span>{selectedAgent.agentName}</span>
            <span className="invocation-count">
              {selectedAgent.invocations.length}次调用
            </span>
          </div>
          <div className="invocation-list">
            {selectedAgent.invocations.map((inv) => {
              const isPromptExpanded = expandedPrompt === inv.id;
              const hasFullPrompt = inv.fullPrompt && inv.fullPrompt.length > 0;

              return (
                <div key={inv.id} className="invocation-record">
                  <div className="invocation-meta">
                    <TimeDisplay isoString={inv.startedAt} />
                    <AgentStatusBadge status={inv.status as any} />
                    <DurationDisplay
                      startedAt={inv.startedAt}
                      completedAt={inv.completedAt}
                      compact
                    />
                  </div>
                  <div className="invocation-input">
                    <div className="invocation-input-header">
                      <span className="invocation-input-label">
                        {hasFullPrompt ? '完整提示词' : '用户输入'}
                      </span>
                      {hasFullPrompt && (
                        <>
                          <span
                            className="invocation-input-expand"
                            onClick={(e) => {
                              e.stopPropagation();
                              setExpandedPrompt(isPromptExpanded ? null : inv.id);
                            }}
                            title={isPromptExpanded ? '收起' : '展开'}
                          >
                            {isPromptExpanded ? <CompressOutlined /> : <ExpandOutlined />}
                          </span>
                          <span
                            className={`invocation-input-copy ${copiedId === inv.id ? 'copied' : ''}`}
                            onClick={(e) => handleCopy(e, inv.id, inv.fullPrompt || inv.input || '')}
                            title={copiedId === inv.id ? '已复制' : '复制内容'}
                          >
                            {copiedId === inv.id ? <CheckOutlined /> : <CopyOutlined />}
                          </span>
                        </>
                      )}
                    </div>
                    <pre className={isPromptExpanded ? 'expanded' : ''}>
                      {hasFullPrompt
                        ? (isPromptExpanded ? inv.fullPrompt : inv.fullPrompt?.slice(0, 300) + (inv.fullPrompt && inv.fullPrompt.length > 300 ? '...' : ''))
                        : (inv.input || '（无输入内容）')}
                    </pre>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
};

export default AgentInvocationLogPanel;