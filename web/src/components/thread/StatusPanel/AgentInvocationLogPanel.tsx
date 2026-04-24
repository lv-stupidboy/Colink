import React, { useState, useMemo } from 'react';
import { Modal, Collapse } from 'antd';
import { FileTextOutlined, RightOutlined, ExpandOutlined, CompressOutlined, CopyOutlined, CheckOutlined, FullscreenOutlined, SwapOutlined, SettingOutlined, MessageOutlined, FileOutlined, EnvironmentOutlined } from '@ant-design/icons';
import { useAppStore } from '@/store';
import { selectAgentLogList } from '@/store/selectors/agentInvocations';
import { AgentStatusBadge, TimeDisplay } from './shared';
import { DurationDisplay } from './DurationDisplay';
import type { AgentInvocation } from '@/types';

// A2A 输入解析结果
interface A2AInputInfo {
  isA2A: boolean;
  triggerInfo: string;      // 触发者信息
  sessionStrategy: string;  // 会话策略类型
  originalRequest: string;  // 原始用户请求
  filteredOutput: string;   // 过滤后的前序输出
}

// Layer Context 解析结果
interface LayerContextInfo {
  hasLayers: boolean;
  systemPrompt: string;     // <system> Layer0
  conversation: string;     // <conversation> Layer1
  artifacts: string;        // <artifacts> Layer2
  environment: string;      // <environment> Layer3
  remainingContent: string; // 未匹配的剩余内容
}

// A2A Handoff 解析结果
interface A2AHandoffInfo {
  hasHandoff: boolean;
  what: string;
  why: string;
  tradeoff: string;
  openQuestions: string;
  nextAction: string;
}

/**
 * 解析 Layer XML 标签提取上下文分块
 */
const parseLayerContext = (content: string): LayerContextInfo => {
  const extractTag = (tagName: string): string => {
    // [\s\S] 匹配任意字符包括换行，在字符串模板中需要 \\s\\S
    const pattern = `<${tagName}[^>]*>([\\s\\S]*?)</${tagName}>`;
    const regex = new RegExp(pattern, 'i');
    const match = content.match(regex);
    return match ? match[1].trim() : '';
  };

  const systemPrompt = extractTag('system');
  const conversation = extractTag('conversation');
  const artifacts = extractTag('artifacts');
  const environment = extractTag('environment');

  const hasLayers: boolean = Boolean(systemPrompt || conversation || artifacts || environment);

  // 移除已提取的标签，保留剩余内容
  let remainingContent = content;
  ['system', 'conversation', 'artifacts', 'environment'].forEach(tag => {
    const pattern = `<${tag}[^>]*>[\\s\\S]*?</${tag}>`;
    remainingContent = remainingContent.replace(new RegExp(pattern, 'gi'), '');
  });
  remainingContent = remainingContent.trim();

  return {
    hasLayers,
    systemPrompt,
    conversation,
    artifacts,
    environment,
    remainingContent,
  };
};

/**
 * 解析 a2a-handoff 交接块
 */
const parseA2AHandoff = (content: string): A2AHandoffInfo | null => {
  const handoffMatch = content.match(/<a2a-handoff>([\s\S]*?)<\/a2a-handoff>/);
  if (!handoffMatch) return null;

  const handoffContent = handoffMatch[1];

  // 提取各部分内容
  const extractPart = (header: string): string => {
    const idx = handoffContent.indexOf(header);
    if (idx === -1) return '';
    const start = idx + header.length;
    const nextPart = handoffContent.slice(start).indexOf('### ');
    if (nextPart !== -1) {
      return handoffContent.slice(start, start + nextPart).trim();
    }
    return handoffContent.slice(start).trim();
  };

  return {
    hasHandoff: true,
    what: extractPart('### What'),
    why: extractPart('### Why'),
    tradeoff: extractPart('### Tradeoff'),
    openQuestions: extractPart('### Open Questions'),
    nextAction: extractPart('### Next Action'),
  };
};

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

  // 统一 Modal state（合并 visible 和 selectedInvocation）
  interface ModalState {
    visible: boolean;
    invocation: AgentInvocation | null;  // null = 全屏模式，非 null = 单条详情
  }
  const [modalState, setModalState] = useState<ModalState>({ visible: false, invocation: null });

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

  // 解析 A2A 输入信息
  const parseA2AInput = (fullPrompt: string): A2AInputInfo | null => {
    const a2aMatch = fullPrompt.match(/<a2a_input>([\s\S]*?)<\/a2a_input>/);
    if (!a2aMatch) return null;

    const a2aContent = a2aMatch[1];

    const triggerMatch = a2aContent.match(/\*\*来自\*\*:\s*(.+)/);
    const strategyMatch = a2aContent.match(/\*\*类型\*\*:\s*(.+)/);
    const requestMatch = a2aContent.match(/## 原始请求\s+([\s\S]*?)---/);
    const outputMatch = a2aContent.match(/## 前序分析[\s\S]*?\n\n([\s\S]*?)---/);

    return {
      isA2A: true,
      triggerInfo: triggerMatch?.[1]?.trim() || '',
      sessionStrategy: strategyMatch?.[1]?.trim() || '',
      originalRequest: requestMatch?.[1]?.trim() || '',
      filteredOutput: outputMatch?.[1]?.trim() || '',
    };
  };

  // 折叠区块独立组件
  interface CollapsibleInputBlockProps {
    content: string;
    invocationId: string;
  }

  const CollapsibleInputBlock: React.FC<CollapsibleInputBlockProps> = ({ content }) => {
    const a2aInfo = parseA2AInput(content);
    const layerInfo = parseLayerContext(content);

    if (!content) {
      return <span className="empty-content">（无内容）</span>;
    }

    // Layer 分块显示（优先）
    if (layerInfo.hasLayers) {
      const layerPanels = [
        {
          key: 'system',
          label: 'System Prompt',
          icon: <SettingOutlined />,
          content: layerInfo.systemPrompt,
          className: 'layer-system',
        },
        {
          key: 'conversation',
          label: 'Conversation History',
          icon: <MessageOutlined />,
          content: layerInfo.conversation,
          className: 'layer-conversation',
        },
        {
          key: 'artifacts',
          label: 'Artifacts',
          icon: <FileOutlined />,
          content: layerInfo.artifacts,
          className: 'layer-artifacts',
        },
        {
          key: 'environment',
          label: 'Environment',
          icon: <EnvironmentOutlined />,
          content: layerInfo.environment,
          className: 'layer-environment',
        },
      ].filter(p => p.content);

      // 添加剩余内容（如果有）
      if (layerInfo.remainingContent) {
        layerPanels.push({
          key: 'remaining',
          label: '其他内容',
          icon: <FileTextOutlined />,
          content: layerInfo.remainingContent,
          className: 'layer-remaining',
        });
      }

      return (
        <div className="layer-context-block">
          <Collapse
            defaultActiveKey={[]}
            size="small"
            items={layerPanels.map(p => ({
              key: p.key,
              label: (
                <span className="layer-panel-label">
                  {p.icon}
                  <span>{p.label}</span>
                  <span className="layer-panel-size">{p.content.length} 字符</span>
                </span>
              ),
              children: <pre className="collapsed-content">{p.content}</pre>,
              className: p.className,
            }))}
          />
        </div>
      );
    }

    // A2A 输入格式解析
    if (a2aInfo) {
      const panels = [
        { key: 'trigger', label: '触发者信息', content: a2aInfo.triggerInfo },
        { key: 'strategy', label: '会话策略', content: a2aInfo.sessionStrategy },
        { key: 'request', label: '原始请求', content: a2aInfo.originalRequest },
        { key: 'output', label: '前序输出摘要', content: a2aInfo.filteredOutput },
      ].filter(p => p.content);
      return (
        <Collapse
          defaultActiveKey={[]}
          size="small"
          items={panels.map(p => ({
            key: p.key,
            label: p.label,
            children: <pre className="collapsed-content">{p.content}</pre>,
          }))}
        />
      );
    }

    return (
      <Collapse
        defaultActiveKey={[]}
        size="small"
        items={[
          { key: 'input', label: '输入内容', children: <pre className="collapsed-content">{content}</pre> },
        ]}
      />
    );
  };

  // 未展开时显示入口按钮
  if (!expanded) {
    return (
      <div className="log-panel-trigger" onClick={() => setExpanded(true)}>
        <FileTextOutlined />
        <span>调用日志</span>
        {agentLogList.length > 0 && (
          <span className="log-panel-count">{agentLogList.length}</span>
        )}
        {agentLogList.length > 0 && (
          <FullscreenOutlined
            className="fullscreen-btn"
            onClick={(e) => {
              e.stopPropagation();
              setModalState({ visible: true, invocation: null });
            }}
            title="全屏查看"
          />
        )}
      </div>
    );
  }

  // 展开后的面板
  const panelContent = (
    <div className="status-section log-panel-content">
      {/* 标题栏 - 整行可点击收起 */}
      <div
        className="section-collapse-header"
        onClick={() => {
          setExpanded(false);
          setSelectedAgentId(null);
        }}
      >
        <span>
          <FileTextOutlined />
          <span>调用日志</span>
          <span className="section-collapse-count">{agentLogList.length}</span>
        </span>
        <FullscreenOutlined
          className="fullscreen-btn"
          onClick={(e) => {
            e.stopPropagation();
            setModalState({ visible: true, invocation: null });
          }}
          title="全屏查看"
        />
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
              // 解析 A2A Handoff（从输出中提取）
              const handoffInfo = inv.output ? parseA2AHandoff(inv.output) : null;

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
                  {/* A2A Handoff 交接信息卡片 */}
                  {handoffInfo && (
                    <div className="handoff-card">
                      <div className="handoff-card-header">
                        <SwapOutlined style={{ marginRight: 6 }} />
                        <span>A2A 交接信息</span>
                      </div>
                      <div className="handoff-card-content">
                        {handoffInfo.what && (
                          <div className="handoff-part">
                            <span className="handoff-label">What:</span>
                            <span className="handoff-value">{handoffInfo.what}</span>
                          </div>
                        )}
                        {handoffInfo.why && (
                          <div className="handoff-part">
                            <span className="handoff-label">Why:</span>
                            <span className="handoff-value">{handoffInfo.why}</span>
                          </div>
                        )}
                        {handoffInfo.tradeoff && (
                          <div className="handoff-part">
                            <span className="handoff-label">Tradeoff:</span>
                            <span className="handoff-value">{handoffInfo.tradeoff}</span>
                          </div>
                        )}
                        {handoffInfo.openQuestions && (
                          <div className="handoff-part">
                            <span className="handoff-label">Open Questions:</span>
                            <span className="handoff-value">{handoffInfo.openQuestions}</span>
                          </div>
                        )}
                        {handoffInfo.nextAction && (
                          <div className="handoff-part">
                            <span className="handoff-label">Next Action:</span>
                            <span className="handoff-value">{handoffInfo.nextAction}</span>
                          </div>
                        )}
                      </div>
                    </div>
                  )}
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
                  {/* 查看详情按钮 */}
                  <div className="invocation-record-actions">
                    <span
                      className="detail-btn"
                      onClick={() => setModalState({ visible: true, invocation: inv })}
                    >
                      查看详情
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );

  return (
    <>
      {panelContent}

      {/* 统一 Modal：全屏模式或单条详情 */}
      <Modal
        title={modalState.invocation ? `调用详情 - ${modalState.invocation.agentName}` : 'Agent 调用日志'}
        open={modalState.visible}
        onCancel={() => setModalState({ visible: false, invocation: null })}
        width={modalState.invocation ? 600 : 800}
        footer={null}
        className={modalState.invocation ? 'invocation-detail-modal' : 'invocation-log-modal'}
      >
        {modalState.invocation ? (
          // 单条详情模式
          <>
            <div className="detail-meta">
              <TimeDisplay isoString={modalState.invocation.startedAt} />
              <AgentStatusBadge status={modalState.invocation.status as any} />
              <DurationDisplay startedAt={modalState.invocation.startedAt} completedAt={modalState.invocation.completedAt} />
            </div>
            <CollapsibleInputBlock content={modalState.invocation.fullPrompt || modalState.invocation.input || ''} invocationId={modalState.invocation.id} />
          </>
        ) : (
          // 全屏模式：全部 Agent 日志列表
          <div className="modal-log-list">
            {agentLogList.length === 0 ? (
              <div className="idle-status">暂无调用记录</div>
            ) : (
              agentLogList.map(agent => (
                <div key={agent.agentConfigId} className="modal-agent-section">
                  <div className="modal-agent-header">{agent.agentName}</div>
                  {agent.invocations.map(inv => (
                    <div key={inv.id} className="modal-invocation-item">
                      <div className="modal-invocation-meta">
                        <TimeDisplay isoString={inv.startedAt} />
                        <AgentStatusBadge status={inv.status as any} />
                      </div>
                      <CollapsibleInputBlock content={inv.fullPrompt || inv.input || ''} invocationId={inv.id} />
                    </div>
                  ))}
                </div>
              ))
            )}
          </div>
        )}
      </Modal>
    </>
  );
};

export default AgentInvocationLogPanel;