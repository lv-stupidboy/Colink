import React, { useState, useMemo } from 'react';
import { Modal, Collapse } from 'antd';
import { FileTextOutlined, RightOutlined, ExpandOutlined, CompressOutlined, CopyOutlined, CheckOutlined, FullscreenOutlined, SwapOutlined, SettingOutlined, MessageOutlined, FileOutlined, EnvironmentOutlined } from '@ant-design/icons';
import { useAppStore } from '@/store';
import { selectInvocationTimeline } from '@/store/selectors/invocationTimeline';
import { AgentStatusBadge, TimeDisplay } from './shared';
import { DurationDisplay } from './DurationDisplay';
import type { AgentInvocation } from '@/types';

// Layer Context 解析结果
interface LayerContextInfo {
  hasLayers: boolean;
  systemPrompt: string;
  conversation: string;
  artifacts: string;
  environment: string;
  remainingContent: string;
}

/**
 * 解析 Layer XML 标签提取上下文分块
 */
const parseLayerContext = (content: string): LayerContextInfo => {
  const extractTag = (tagName: string): string => {
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
const parseA2AHandoff = (content: string) => {
  const handoffMatch = content.match(/<a2a-handoff>([\s\S]*?)<\/a2a-handoff>/);
  if (!handoffMatch) return null;

  const handoffContent = handoffMatch[1];

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
 * 格式化 Token 数量
 */
const formatTokens = (n: number): string => {
  if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
  return String(n);
};

/**
 * 获取显示名称
 */
const getDisplayName = (inv: AgentInvocation): string => {
  if (inv.agentName && inv.agentName.trim()) return inv.agentName.trim();
  if (inv.role === 'agent') return 'Agent';
  if (inv.role) return inv.role;
  return inv.id.slice(0, 8);
};

/**
 * Agent 调用日志面板（时间线列表）
 * 按调用时间倒序排列，最近的在最上面
 */
export const AgentInvocationLogPanel: React.FC = () => {
  const [expanded, setExpanded] = useState(false);
  const [expandedPrompt, setExpandedPrompt] = useState<string | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  // Modal 状态
  const [modalState, setModalState] = useState<{
    visible: boolean;
    invocation: AgentInvocation | null;
  }>({ visible: false, invocation: null });

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

  // 按时间排序的调用列表
  const timeline = useMemo(() => {
    return selectInvocationTimeline(activeAgents, completedAgents);
  }, [activeAgents, completedAgents]);

  // 计算活跃数量
  const activeCount = activeAgents.length;

  // 折叠区块组件
  const CollapsibleInputBlock: React.FC<{ content: string; invocationId: string }> = ({ content }) => {
    const layerInfo = parseLayerContext(content);

    if (!content) {
      return <span className="empty-content">（无内容）</span>;
    }

    if (layerInfo.hasLayers) {
      const layerPanels = [
        { key: 'system', label: 'System Prompt', icon: <SettingOutlined />, content: layerInfo.systemPrompt, className: 'layer-system' },
        { key: 'conversation', label: 'Conversation History', icon: <MessageOutlined />, content: layerInfo.conversation, className: 'layer-conversation' },
        { key: 'artifacts', label: 'Artifacts', icon: <FileOutlined />, content: layerInfo.artifacts, className: 'layer-artifacts' },
        { key: 'environment', label: 'Environment', icon: <EnvironmentOutlined />, content: layerInfo.environment, className: 'layer-environment' },
      ].filter(p => p.content);

      if (layerInfo.remainingContent) {
        layerPanels.push({
          key: 'remaining', label: '其他内容', icon: <FileTextOutlined />, content: layerInfo.remainingContent, className: 'layer-remaining',
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

  // 渲染单条调用记录
  const renderInvocationItem = (inv: AgentInvocation) => {
    const isPromptExpanded = expandedPrompt === inv.id;
    const hasFullPrompt = inv.fullPrompt && inv.fullPrompt.length > 0;
    const handoffInfo = inv.output ? parseA2AHandoff(inv.output) : null;
    const usage = inv.inputTokens !== undefined || inv.outputTokens !== undefined ? inv : null;

    return (
      <div key={inv.id} className="timeline-item">
        {/* 状态行 */}
        <div className="timeline-status-row">
          <AgentStatusBadge status={inv.status as any} />
          <span className="timeline-agent-name">{getDisplayName(inv)}</span>
          <TimeDisplay isoString={inv.startedAt} />
          <DurationDisplay startedAt={inv.startedAt} completedAt={inv.completedAt} compact />
        </div>

        {/* Token 使用 */}
        {usage && (
          <div className="timeline-usage">
            <span>{formatTokens(usage.inputTokens || 0)}↓</span>
            <span>{formatTokens(usage.outputTokens || 0)}↑</span>
            {usage.costUsd !== undefined && usage.costUsd > 0 && (
              <span>${usage.costUsd.toFixed(4)}</span>
            )}
          </div>
        )}

        {/* A2A Handoff */}
        {handoffInfo && (
          <div className="handoff-card mini">
            <div className="handoff-card-header">
              <SwapOutlined style={{ marginRight: 6 }} />
              <span>A2A 交接</span>
            </div>
            <div className="handoff-card-content compact">
              {handoffInfo.what && (
                <div className="handoff-part">
                  <span className="handoff-label">What:</span>
                  <span className="handoff-value">{handoffInfo.what}</span>
                </div>
              )}
              {handoffInfo.nextAction && (
                <div className="handoff-part">
                  <span className="handoff-label">Next:</span>
                  <span className="handoff-value">{handoffInfo.nextAction}</span>
                </div>
              )}
            </div>
          </div>
        )}

        {/* 提示词区域 */}
        {hasFullPrompt && (
          <div className="timeline-prompt">
            <div className="timeline-prompt-header">
              <span className="timeline-prompt-label">提示词</span>
              <div className="timeline-prompt-actions">
                <span
                  className="prompt-action"
                  onClick={(e) => {
                    e.stopPropagation();
                    setExpandedPrompt(isPromptExpanded ? null : inv.id);
                  }}
                  title={isPromptExpanded ? '收起' : '展开'}
                >
                  {isPromptExpanded ? <CompressOutlined /> : <ExpandOutlined />}
                </span>
                <span
                  className={`prompt-action ${copiedId === inv.id ? 'copied' : ''}`}
                  onClick={(e) => handleCopy(e, inv.id, inv.fullPrompt || '')}
                  title={copiedId === inv.id ? '已复制' : '复制'}
                >
                  {copiedId === inv.id ? <CheckOutlined /> : <CopyOutlined />}
                </span>
              </div>
            </div>
            <pre className={isPromptExpanded ? 'expanded' : 'collapsed'}>
              {isPromptExpanded ? inv.fullPrompt : inv.fullPrompt?.slice(0, 200) + (inv.fullPrompt && inv.fullPrompt.length > 200 ? '...' : '')}
            </pre>
          </div>
        )}

        {/* 操作按钮 */}
        <div className="timeline-actions">
          <span className="detail-btn" onClick={() => setModalState({ visible: true, invocation: inv })}>
            查看详情
          </span>
        </div>
      </div>
    );
  };

  // 未展开时显示入口按钮
  if (!expanded) {
    return (
      <div className="log-panel-trigger" onClick={() => setExpanded(true)}>
        <FileTextOutlined />
        <span>调用日志</span>
        {timeline.length > 0 && (
          <span className="log-panel-count">
            {timeline.length}
            {activeCount > 0 && <span className="active-count">({activeCount} 运行中)</span>}
          </span>
        )}
        {timeline.length > 0 && (
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
  return (
    <>
      <div className="status-section log-panel-content">
        {/* 标题栏 */}
        <div className="section-collapse-header" onClick={() => setExpanded(false)}>
          <span>
            <FileTextOutlined />
            <span>调用日志</span>
            <span className="section-collapse-count">
              {timeline.length}
              {activeCount > 0 && <span className="active-dot">●{activeCount}</span>}
            </span>
          </span>
          <RightOutlined className="collapse-icon expanded" />
        </div>

        {/* 时间线列表 */}
        <div className="timeline-list">
          {timeline.length === 0 ? (
            <div className="idle-status">暂无调用记录</div>
          ) : (
            timeline.map(renderInvocationItem)
          )}
        </div>
      </div>

      {/* 详情 Modal */}
      <Modal
        title={modalState.invocation ? `调用详情 - ${getDisplayName(modalState.invocation)}` : 'Agent 调用日志'}
        open={modalState.visible}
        onCancel={() => setModalState({ visible: false, invocation: null })}
        width={modalState.invocation ? 600 : 800}
        footer={null}
        className={modalState.invocation ? 'invocation-detail-modal' : 'invocation-log-modal'}
      >
        {modalState.invocation ? (
          <>
            <div className="detail-meta">
              <AgentStatusBadge status={modalState.invocation.status as any} />
              <span className="detail-agent-name">{getDisplayName(modalState.invocation)}</span>
              <TimeDisplay isoString={modalState.invocation.startedAt} />
              <DurationDisplay startedAt={modalState.invocation.startedAt} completedAt={modalState.invocation.completedAt} />
            </div>
            <CollapsibleInputBlock content={modalState.invocation.fullPrompt || modalState.invocation.input || ''} invocationId={modalState.invocation.id} />
          </>
        ) : (
          <div className="modal-timeline">
            {timeline.map(inv => (
              <div key={inv.id} className="modal-timeline-item">
                <div className="modal-timeline-header">
                  <AgentStatusBadge status={inv.status as any} />
                  <span className="modal-timeline-name">{getDisplayName(inv)}</span>
                  <TimeDisplay isoString={inv.startedAt} />
                </div>
                <CollapsibleInputBlock content={inv.fullPrompt || inv.input || ''} invocationId={inv.id} />
              </div>
            ))}
          </div>
        )}
      </Modal>
    </>
  );
};

export default AgentInvocationLogPanel;