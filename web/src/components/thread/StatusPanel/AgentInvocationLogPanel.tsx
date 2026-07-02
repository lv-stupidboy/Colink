import React, { useState, useMemo, useCallback } from 'react';
import { Modal, Input, Segmented, Empty } from 'antd';
import {
  FileTextOutlined,
  RightOutlined,
  FullscreenOutlined,
  SettingOutlined,
  MessageOutlined,
  FileOutlined,
  BranchesOutlined,
  UserOutlined,
  BookOutlined,
  SearchOutlined,
  CopyOutlined,
  CheckOutlined,
} from '@ant-design/icons';
import { useAppStore } from '@/store';
import { selectInvocationTimeline } from '@/store/selectors/invocationTimeline';
import { AgentStatusBadge, TimeDisplay, InvocationStatus } from './shared';
import { DurationDisplay } from './DurationDisplay';
import type { AgentInvocation } from '@/types';

// Layer Context 解析结果
// - system / conversation / artifacts / memory：静态语义块
// - a2aContext：A2A 场景专用（合并 <a2a-context> / <incremental-context> / <a2a-handoff-from-upstream>）
// - userInput：用户实际输入（<user> 或 <a2a_input>）
// - environment：解析出来仅用于剥离，不展示（用户诉求：太杂，无信息量）
// - remainingContent：其他未识别块
interface LayerContextInfo {
  hasLayers: boolean;
  systemPrompt: string;
  conversation: string;
  artifacts: string;
  memory: string;
  a2aContext: string;
  userInput: string;
  remainingContent: string;
}

/**
 * 解析 Layer XML 标签提取上下文分块
 *
 * 覆盖后端 prompt_builder.go 里所有已知标签，按语义归类：
 *   - <system>          → systemPrompt
 *   - <conversation>    → conversation
 *   - <artifacts>       → artifacts
 *   - <memory>          → memory
 *   - <environment>     → 剥离（不展示）
 *   - <a2a-context> / <incremental-context> / <a2a-handoff-from-upstream> → a2aContext（合并）
 *   - <user> / <a2a_input> → userInput（合并）
 */
const parseLayerContext = (content: string): LayerContextInfo => {
  const extractTag = (tagName: string): string => {
    const pattern = `<${tagName}[^>]*>([\\s\\S]*?)</${tagName}>`;
    const regex = new RegExp(pattern, 'i');
    const match = content.match(regex);
    return match ? match[1].trim() : '';
  };

  // 拼接多个标签的内容（用两个换行分隔）
  const extractMerged = (tagNames: string[]): string => {
    const parts = tagNames
      .map((tag) => {
        const raw = extractTag(tag);
        return raw ? `<!-- ${tag} -->\n${raw}` : '';
      })
      .filter(Boolean);
    return parts.join('\n\n');
  };

  const systemPrompt = extractTag('system');
  const conversation = extractTag('conversation');
  const artifacts = extractTag('artifacts');
  const memory = extractTag('memory');
  const a2aContext = extractMerged(['a2a-context', 'incremental-context', 'a2a-handoff-from-upstream']);
  const userInput = extractMerged(['user', 'a2a_input']);

  const hasLayers: boolean = Boolean(
    systemPrompt || conversation || artifacts || memory || a2aContext || userInput
  );

  // 剥离所有已识别的标签（含 environment，虽然不展示但也要剥掉避免落入 remainingContent）
  let remainingContent = content;
  const strippedTags = [
    'system',
    'conversation',
    'artifacts',
    'memory',
    'environment',
    'a2a-context',
    'incremental-context',
    'a2a-handoff-from-upstream',
    'user',
    'a2a_input',
  ];
  strippedTags.forEach((tag) => {
    const pattern = `<${tag}[^>]*>[\\s\\S]*?</${tag}>`;
    remainingContent = remainingContent.replace(new RegExp(pattern, 'gi'), '');
  });
  remainingContent = remainingContent.trim();

  return {
    hasLayers,
    systemPrompt,
    conversation,
    artifacts,
    memory,
    a2aContext,
    userInput,
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
 * 格式化 Token 数量
 */
const formatTokens = (n: number): string => {
  if (n >= 1000000) return `${(n / 1000000).toFixed(1)}M`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`;
  return String(n);
};

// 状态过滤选项
type StatusFilter = 'all' | 'running' | 'completed' | 'failed';

/**
 * Layer 分块卡片组件
 * 支持外部强制展开，同时也支持用户单独切换
 */
const LayerCard: React.FC<{
  type: 'system' | 'conversation' | 'artifacts' | 'memory' | 'a2aContext' | 'userInput' | 'remaining';
  content: string;
  allExpanded?: boolean; // 全局展开状态（来自父组件）
}> = ({ type, content, allExpanded }) => {
  // 用户手动切换的状态（null 表示未手动切换，使用全局状态）
  const [userOverride, setUserOverride] = useState<boolean | null>(null);

  // 实际展开状态：优先使用用户手动设置的，否则使用全局状态
  const expanded = userOverride !== null ? userOverride : (allExpanded || false);

  const [copied, setCopied] = useState(false);

  const config = {
    system: { icon: <SettingOutlined />, label: 'System Prompt', color: 'purple' },
    conversation: { icon: <MessageOutlined />, label: 'Conversation History', color: 'blue' },
    artifacts: { icon: <FileOutlined />, label: 'Artifacts', color: 'orange' },
    memory: { icon: <BookOutlined />, label: 'Memory', color: 'teal' },
    a2aContext: { icon: <BranchesOutlined />, label: 'A2A 上下文', color: 'cyan' },
    userInput: { icon: <UserOutlined />, label: '用户输入', color: 'indigo' },
    remaining: { icon: <FileTextOutlined />, label: '其他内容', color: 'gray' },
  }[type];

  const handleCopy = useCallback(async (e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await navigator.clipboard.writeText(content);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('复制失败:', err);
    }
  }, [content]);

  const handleToggle = useCallback(() => {
    // 切换展开状态：如果当前是跟随全局状态，则切换到相反；如果已有手动状态，则切换
    const newExpanded = !expanded;
    setUserOverride(newExpanded);
  }, [expanded]);

  return (
    <div className={`layer-card layer-${config.color}`}>
      <div className="layer-card-header" onClick={handleToggle}>
        <div className="layer-card-title">
          {config.icon}
          <span>{config.label}</span>
          <span className="layer-card-size">{content.length} 字符</span>
        </div>
        <div className="layer-card-actions">
          <span
            className={`layer-copy-btn ${copied ? 'copied' : ''}`}
            onClick={handleCopy}
            title="复制"
          >
            {copied ? <CheckOutlined /> : <CopyOutlined />}
          </span>
          <RightOutlined className={`layer-expand-icon ${expanded ? 'expanded' : ''}`} />
        </div>
      </div>
      {expanded && (
        <div className="layer-card-content">
          <pre>{content}</pre>
        </div>
      )}
    </div>
  );
};

/**
 * 详情面板右侧组件
 */
const DetailPanel: React.FC<{ invocation: AgentInvocation | null }> = ({ invocation }) => {
  const [allExpanded, setAllExpanded] = useState(false);
  const [copiedAll, setCopiedAll] = useState(false);

  if (!invocation) {
    return (
      <div className="detail-panel-empty">
        <Empty description="选择左侧记录查看详情" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      </div>
    );
  }

  const content = invocation.fullPrompt || invocation.input || '';
  const layerInfo = parseLayerContext(content);

  const handleCopyAll = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(content);
      setCopiedAll(true);
      setTimeout(() => setCopiedAll(false), 2000);
    } catch (err) {
      console.error('复制失败:', err);
    }
  }, [content]);

  return (
    <div className="detail-panel">
      {/* 元信息区 */}
      <div className="detail-meta-section">
        <div className="detail-meta-row">
          <AgentStatusBadge status={invocation.status as InvocationStatus} />
          <span className="detail-agent-name">{getDisplayName(invocation)}</span>
          <TimeDisplay isoString={invocation.startedAt} />
          <DurationDisplay startedAt={invocation.startedAt} completedAt={invocation.completedAt} />
        </div>
        {/* Token 使用 */}
        {(invocation.inputTokens || invocation.outputTokens) && (
          <div className="detail-token-row">
            <span className="token-in">{formatTokens(invocation.inputTokens || 0)} ↓</span>
            <span className="token-out">{formatTokens(invocation.outputTokens || 0)} ↑</span>
            {invocation.costUsd && invocation.costUsd > 0 && (
              <span className="token-cost">${invocation.costUsd.toFixed(4)}</span>
            )}
          </div>
        )}
      </div>

      {/* 内容区域 */}
      <div className="detail-content-section">
        {content ? (
          <>
            {/* 操作栏 */}
            <div className="detail-actions-bar">
              <button className="action-btn" onClick={() => setAllExpanded(!allExpanded)}>
                {allExpanded ? '全部收起' : '全部展开'}
              </button>
              <button className={`action-btn ${copiedAll ? 'copied' : ''}`} onClick={handleCopyAll}>
                {copiedAll ? <><CheckOutlined /> 已复制</> : <><CopyOutlined /> 复制全部</>}
              </button>
            </div>

            {/* Layer 分块展示 —— 语义顺序：静态背景 → 动态上下文 → 用户输入 → 其他 */}
            {layerInfo.hasLayers ? (
              <div className="layer-cards-container">
                {layerInfo.systemPrompt && (
                  <LayerCard type="system" content={layerInfo.systemPrompt} allExpanded={allExpanded} />
                )}
                {layerInfo.conversation && (
                  <LayerCard type="conversation" content={layerInfo.conversation} allExpanded={allExpanded} />
                )}
                {layerInfo.artifacts && (
                  <LayerCard type="artifacts" content={layerInfo.artifacts} allExpanded={allExpanded} />
                )}
                {layerInfo.memory && (
                  <LayerCard type="memory" content={layerInfo.memory} allExpanded={allExpanded} />
                )}
                {layerInfo.a2aContext && (
                  <LayerCard type="a2aContext" content={layerInfo.a2aContext} allExpanded={allExpanded} />
                )}
                {layerInfo.userInput && (
                  <LayerCard type="userInput" content={layerInfo.userInput} allExpanded={allExpanded} />
                )}
                {/* environment 按用户诉求不再展示 */}
                {layerInfo.remainingContent && (
                  <LayerCard type="remaining" content={layerInfo.remainingContent} allExpanded={allExpanded} />
                )}
              </div>
            ) : (
              <div className="simple-content-block">
                <pre className="content-pre">{content}</pre>
              </div>
            )}
          </>
        ) : (
          <div className="empty-content">无内容</div>
        )}
      </div>
    </div>
  );
};

/**
 * Agent 调用日志面板（时间线列表）
 * 按调用时间倒序排列，最近的在最上面
 */
export const AgentInvocationLogPanel: React.FC = () => {
  const [expanded, setExpanded] = useState(true);

  // Modal 状态
  const [modalVisible, setModalVisible] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');

  // 从 Store 获取数据
  const activeAgents = useAppStore((state) => state.activeAgents);
  const completedAgents = useAppStore((state) => state.completedAgents);

  // 按时间排序的调用列表
  const timeline = useMemo(() => {
    return selectInvocationTimeline(activeAgents, completedAgents);
  }, [activeAgents, completedAgents]);

  // 过滤后的列表
  const filteredTimeline = useMemo(() => {
    let result = timeline;

    // 状态过滤
    if (statusFilter !== 'all') {
      result = result.filter(inv => inv.status === statusFilter);
    }

    // 搜索过滤
    if (searchQuery.trim()) {
      const query = searchQuery.toLowerCase();
      result = result.filter(inv => {
        const name = getDisplayName(inv).toLowerCase();
        const content = (inv.fullPrompt || inv.input || '').toLowerCase();
        return name.includes(query) || content.includes(query);
      });
    }

    return result;
  }, [timeline, statusFilter, searchQuery]);

  // 选中的调用
  const selectedInvocation = useMemo(() => {
    return timeline.find(inv => inv.id === selectedId) || null;
  }, [timeline, selectedId]);

  // 计算活跃数量
  const activeCount = activeAgents.length;

  // 打开 Modal 时默认选中第一条
  const handleOpenModal = useCallback(() => {
    setModalVisible(true);
    if (filteredTimeline.length > 0 && !selectedId) {
      setSelectedId(filteredTimeline[0].id);
    }
  }, [filteredTimeline, selectedId]);

  // 未展开时显示入口按钮
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
        {/* 标题栏 */}
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
              onClick={(e) => { e.stopPropagation(); handleOpenModal(); }}
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
              <div
                key={inv.id}
                className={`timeline-item ${selectedId === inv.id ? 'selected' : ''}`}
                onClick={() => setSelectedId(inv.id)}
              >
                <div className="timeline-main-row">
                  <AgentStatusBadge status={inv.status as InvocationStatus} />
                  <span className="timeline-agent-name">{getDisplayName(inv)}</span>
                  <TimeDisplay isoString={inv.startedAt} />
                  <DurationDisplay startedAt={inv.startedAt} completedAt={inv.completedAt} compact />
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {/* 全屏 Modal */}
      <Modal
        title="Agent 调用日志"
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        width={900}
        footer={null}
        className="invocation-log-modal-v2"
      >
        <div className="log-modal-layout">
          {/* 左侧列表 */}
          <div className="log-modal-sidebar">
            {/* 搜索 + 过滤 */}
            <div className="sidebar-header">
              <Input
                placeholder="搜索 Agent..."
                prefix={<SearchOutlined />}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                allowClear
                className="sidebar-search"
              />
              <Segmented
                options={[
                  { label: '全部', value: 'all' },
                  { label: '运行', value: 'running' },
                  { label: '完成', value: 'completed' },
                  { label: '失败', value: 'failed' },
                ]}
                value={statusFilter}
                onChange={(v) => setStatusFilter(v as StatusFilter)}
                size="small"
                className="sidebar-filter"
              />
            </div>

            {/* 列表 */}
            <div className="sidebar-list">
              {filteredTimeline.length === 0 ? (
                <Empty description="无匹配记录" image={Empty.PRESENTED_IMAGE_SIMPLE} />
              ) : (
                filteredTimeline.map(inv => (
                  <div
                    key={inv.id}
                    className={`sidebar-item ${selectedId === inv.id ? 'selected' : ''}`}
                    onClick={() => setSelectedId(inv.id)}
                  >
                    <div className="sidebar-item-status">
                      <span className={`status-dot ${inv.status}`} />
                    </div>
                    <div className="sidebar-item-content">
                      <span className="sidebar-item-name">{getDisplayName(inv)}</span>
                      <span className="sidebar-item-meta">
                        <TimeDisplay isoString={inv.startedAt} />
                        <DurationDisplay startedAt={inv.startedAt} completedAt={inv.completedAt} compact />
                      </span>
                    </div>
                    {(inv.inputTokens || inv.outputTokens) && (
                      <div className="sidebar-item-tokens">
                        <span>{formatTokens(inv.inputTokens || 0)}↓</span>
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>

            {/* 底部统计 */}
            <div className="sidebar-footer">
              <span>共 {filteredTimeline.length} 条记录</span>
            </div>
          </div>

          {/* 右侧详情 */}
          <DetailPanel invocation={selectedInvocation} />
        </div>
      </Modal>
    </>
  );
};

export default AgentInvocationLogPanel;