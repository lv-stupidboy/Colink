import React, { useState, useMemo, useCallback } from 'react';
import { Modal, Collapse } from 'antd';
import { FileTextOutlined, RightOutlined, FullscreenOutlined, SettingOutlined, MessageOutlined, FileOutlined, EnvironmentOutlined } from '@ant-design/icons';
import { useAppStore } from '@/store';
import { selectInvocationTimeline } from '@/store/selectors/invocationTimeline';
import { AgentStatusBadge, TimeDisplay } from './shared';
import { DurationDisplay } from './DurationDisplay';
import { TimelineItem } from './TimelineItem';
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
 * 获取显示名称
 */
const getDisplayName = (inv: AgentInvocation): string => {
  if (inv.agentName && inv.agentName.trim()) return inv.agentName.trim();
  if (inv.role === 'agent') return 'Agent';
  if (inv.role) return inv.role;
  return inv.id.slice(0, 8);
};

/**
 * 折叠区块组件
 */
const CollapsibleInputBlock: React.FC<{ content: string }> = ({ content }) => {
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

/**
 * Agent 调用日志面板（时间线列表）
 * 按调用时间倒序排列，最近的在最上面
 */
export const AgentInvocationLogPanel: React.FC = () => {
  const [expanded, setExpanded] = useState(true); // 默认展开

  // Modal 状态
  const [modalState, setModalState] = useState<{
    visible: boolean;
    invocation: AgentInvocation | null;
  }>({ visible: false, invocation: null });

  // 从 Store 获取数据
  const activeAgents = useAppStore((state) => state.activeAgents);
  const completedAgents = useAppStore((state) => state.completedAgents);

  // 按时间排序的调用列表（使用 useMemo 缓存）
  const timeline = useMemo(() => {
    return selectInvocationTimeline(activeAgents, completedAgents);
  }, [activeAgents, completedAgents]);

  // 计算活跃数量
  const activeCount = activeAgents.length;

  // 查看详情回调（使用 useCallback 缓存）
  const handleViewDetail = useCallback((inv: AgentInvocation) => {
    setModalState({ visible: true, invocation: inv });
  }, []);

  // 未展开时显示入口按钮（点击整个区域展开列表）
  if (!expanded) {
    return (
      <div className="log-panel-trigger" onClick={() => setExpanded(true)}>
        <FileTextOutlined />
        <span className="log-panel-title">调用日志</span>
        {timeline.length > 0 && (
          <span className="log-panel-count">
            {timeline.length}
            {activeCount > 0 && <span className="active-count">({activeCount} 运行中)</span>}
          </span>
        )}
        <RightOutlined className="expand-arrow" />
      </div>
    );
  }

  // 展开后的面板
  return (
    <>
      <div className="status-section log-panel-content">
        {/* 标题栏 - 整个区域可点击收起 */}
        <div className="section-collapse-header" onClick={() => setExpanded(false)}>
          <span>
            <FileTextOutlined />
            <span>调用日志</span>
            <span className="section-collapse-count">
              {timeline.length}
              {activeCount > 0 && <span className="active-dot">●{activeCount}</span>}
            </span>
            <RightOutlined className="collapse-icon expanded" style={{ marginLeft: 8 }} />
          </span>
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

        {/* 时间线列表 */}
        <div className="timeline-list">
          {timeline.length === 0 ? (
            <div className="idle-status">暂无调用记录</div>
          ) : (
            timeline.map(inv => (
              <TimelineItem
                key={inv.id}
                inv={inv}
                onViewDetail={handleViewDetail}
              />
            ))
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
            <CollapsibleInputBlock content={modalState.invocation.fullPrompt || modalState.invocation.input || ''} />
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
                <CollapsibleInputBlock content={inv.fullPrompt || inv.input || ''} />
              </div>
            ))}
          </div>
        )}
      </Modal>
    </>
  );
};

export default AgentInvocationLogPanel;