// web/src/components/thread/MessageScrollIndicator.tsx
import React, { useEffect, useState, useCallback, useRef, RefObject } from 'react';
import { Tooltip } from 'antd';
import { UserOutlined, RobotOutlined, CrownOutlined } from '@ant-design/icons';
import type { Message, AgentConfig } from '@/types';

interface IndicatorItem {
  messageId: string;
  role: 'user' | 'agent' | 'system';
  agentId?: string;
  agentName?: string;
  y: number;
}

interface MessageScrollIndicatorProps {
  messages: Message[];
  agentConfigs: AgentConfig[];
  containerRef: RefObject<HTMLDivElement>;
  onJumpToMessage?: (messageId: string) => void;
}

/**
 * 消息滚动指示器组件
 * 在滚动条轨道右侧显示角色头像，点击可跳转到对应消息
 */
const MessageScrollIndicator: React.FC<MessageScrollIndicatorProps> = ({
  messages,
  agentConfigs,
  containerRef,
  onJumpToMessage,
}) => {
  const [indicators, setIndicators] = useState<IndicatorItem[]>([]);
  const pendingRafRef = useRef<number | null>(null);

  // 计算指示器位置
  const updateIndicators = useCallback(() => {
    if (!containerRef.current) return;

    const container = containerRef.current;
    const containerHeight = container.clientHeight;
    const scrollHeight = container.scrollHeight;

    // 获取所有消息元素
    const messageElements = container.querySelectorAll('[data-message-id]');

    const newIndicators: IndicatorItem[] = [];

    messageElements.forEach((el) => {
      const messageId = el.getAttribute('data-message-id');
      const role = el.getAttribute('data-message-role') as 'user' | 'agent' | 'system';
      const agentId = el.getAttribute('data-agent-id');
      const agentName = el.getAttribute('data-agent-name');

      if (!messageId || !role) return;

      // 计算位置比例
      const element = el as HTMLElement;
      const ratio = element.offsetTop / scrollHeight;
      const y = ratio * containerHeight;

      newIndicators.push({
        messageId,
        role,
        agentId: agentId || undefined,
        agentName: agentName || undefined,
        y,
      });
    });

    setIndicators(newIndicators);
  }, [containerRef]);

  // 监听容器变化更新指示器
  useEffect(() => {
    updateIndicators();

    // 监听滚动事件更新位置
    const container = containerRef.current;
    if (!container) return;

    const handleScroll = () => {
      // 使用 requestAnimationFrame 节流，防止堆叠
      if (pendingRafRef.current !== null) {
        cancelAnimationFrame(pendingRafRef.current);
      }
      pendingRafRef.current = requestAnimationFrame(() => {
        updateIndicators();
        pendingRafRef.current = null;
      });
    };

    container.addEventListener('scroll', handleScroll, { passive: true });
    return () => {
      container.removeEventListener('scroll', handleScroll);
      if (pendingRafRef.current !== null) {
        cancelAnimationFrame(pendingRafRef.current);
      }
    };
  }, [containerRef, messages, updateIndicators]);

  // 消息变化时更新
  useEffect(() => {
    // 延迟更新，等待 DOM 渲染完成
    const timer = setTimeout(updateIndicators, 100);
    return () => clearTimeout(timer);
  }, [messages, updateIndicators]);

  // 跳转到消息
  const handleJump = useCallback((messageId: string) => {
    if (onJumpToMessage) {
      onJumpToMessage(messageId);
    } else {
      // 默认跳转逻辑：将消息首行对齐到可视区域顶部
      const element = document.querySelector(`[data-message-id="${messageId}"]`);
      if (element) {
        element.scrollIntoView({ behavior: 'smooth', block: 'start' });
      }
    }
  }, [onJumpToMessage]);

  // 获取角色配置
  const getAgentConfig = useCallback((agentId?: string, agentName?: string) => {
    if (agentId) {
      return agentConfigs.find((c) => c.id === agentId);
    }
    if (agentName) {
      return agentConfigs.find((c) => c.name === agentName);
    }
    return undefined;
  }, [agentConfigs]);

  // 统一的图标容器样式（模拟 Avatar 效果）
  const iconContainerStyle: React.CSSProperties = {
    width: 16,
    height: 16,
    borderRadius: '50%',
    backgroundColor: 'var(--bg-container)',
    border: '1px solid var(--border-color)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    position: 'relative',
  };

  // 人机交互角标样式（小圆点 + 人形图标）
  // 使用 CSS 变量适配深色模式
  const humanBadgeStyle: React.CSSProperties = {
    position: 'absolute',
    right: -2,
    bottom: -2,
    width: 8,
    height: 8,
    borderRadius: '50%',
    backgroundColor: 'var(--bg-container)',
    border: '1px solid var(--border-color)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    boxShadow: '0 1px 2px rgba(0,0,0,0.08)',
  };

  // 渲染指示器图标（小图标用于滚动条上）
  const renderSmallIcon = (indicator: IndicatorItem) => {
    const agentConfig = getAgentConfig(indicator.agentId, indicator.agentName);

    if (indicator.role === 'user') {
      return (
        <div style={iconContainerStyle}>
          <UserOutlined style={{ color: 'var(--text-primary)', fontSize: 10 }} />
        </div>
      );
    }

    if (indicator.role === 'system') {
      // 系统消息使用皇冠图标
      return (
        <div style={iconContainerStyle}>
          <CrownOutlined style={{ fontSize: 10, color: '#faad14' }} />
        </div>
      );
    }

    // Agent 角色
    const requiresHuman = agentConfig?.requiresHuman || false;
    const isSystem = agentConfig?.isSystem || false;

    return (
      <div style={iconContainerStyle}>
        {isSystem ? (
          <CrownOutlined style={{ fontSize: 10, color: '#faad14' }} />
        ) : (
          <RobotOutlined style={{ fontSize: 10, color: 'var(--color-primary)' }} />
        )}
        {/* 人机交互角标：小圆点内的人形图标 */}
        {requiresHuman && (
          <div style={humanBadgeStyle}>
            <UserOutlined style={{ fontSize: 5, color: 'var(--color-primary)' }} />
          </div>
        )}
      </div>
    );
  };

  // 统一的 Tooltip 图标容器样式
  const tooltipIconContainerStyle: React.CSSProperties = {
    width: 32,
    height: 32,
    borderRadius: '50%',
    backgroundColor: 'var(--bg-container)',
    border: '1px solid var(--border-color)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    position: 'relative',
    boxShadow: '0 2px 8px rgba(0,0,0,0.08)',
  };

  // Agent 专用的 Tooltip 图标容器（带主色调背景）
  const tooltipAgentIconStyle: React.CSSProperties = {
    width: 32,
    height: 32,
    borderRadius: '50%',
    backgroundColor: 'var(--color-primary)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    position: 'relative',
    boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
  };

  // 系统 Agent 专用的 Tooltip 图标容器（带金色背景）
  const tooltipSystemIconStyle: React.CSSProperties = {
    width: 32,
    height: 32,
    borderRadius: '50%',
    backgroundColor: '#faad14',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    position: 'relative',
    boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
  };

  // 人机交互 Tooltip 角标样式
  const tooltipHumanBadgeStyle: React.CSSProperties = {
    position: 'absolute',
    right: -4,
    bottom: -4,
    width: 14,
    height: 14,
    borderRadius: '50%',
    backgroundColor: 'var(--bg-container)',
    border: '1px solid var(--border-color)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    boxShadow: '0 1px 2px rgba(0,0,0,0.1)',
  };

  // 渲染 Tooltip 内容（大图标 + 名称）
  const renderTooltipContent = (indicator: IndicatorItem) => {
    const agentConfig = getAgentConfig(indicator.agentId, indicator.agentName);
    const displayName = getDisplayName(indicator);

    if (indicator.role === 'user') {
      return (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <div style={tooltipIconContainerStyle}>
            <UserOutlined style={{ color: 'var(--text-primary)', fontSize: 18 }} />
          </div>
          <span>{displayName}</span>
        </div>
      );
    }

    if (indicator.role === 'system') {
      return (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <div style={tooltipSystemIconStyle}>
            <CrownOutlined style={{ fontSize: 18, color: '#fff' }} />
          </div>
          <span>{displayName}</span>
        </div>
      );
    }

    // Agent 角色
    const requiresHuman = agentConfig?.requiresHuman || false;
    const isSystem = agentConfig?.isSystem || false;

    // 系统 Agent 使用金色背景，普通 Agent 使用主色调背景
    const containerStyle = isSystem ? tooltipSystemIconStyle : tooltipAgentIconStyle;

    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <div style={containerStyle}>
          {isSystem ? (
            <CrownOutlined style={{ fontSize: 18, color: '#fff' }} />
          ) : (
            <RobotOutlined style={{ fontSize: 18, color: '#fff' }} />
          )}
          {/* 人机交互角标 */}
          {requiresHuman && (
            <div style={tooltipHumanBadgeStyle}>
              <UserOutlined style={{ fontSize: 8, color: 'var(--color-primary)' }} />
            </div>
          )}
        </div>
        <span>{displayName}</span>
      </div>
    );
  };

  // 获取显示名称
  const getDisplayName = (indicator: IndicatorItem) => {
    if (indicator.role === 'user') return '用户';
    if (indicator.role === 'system') return '系统';

    // 优先使用 agentName
    if (indicator.agentName && indicator.agentName.trim()) {
      return indicator.agentName;
    }

    // 从 agentConfigs 中查找名字
    const config = getAgentConfig(indicator.agentId, indicator.agentName);
    if (config?.name) {
      return config.name;
    }

    return 'Agent';
  };

  // 消息为空时不渲染
  if (messages.length === 0) {
    return null;
  }

  return (
    <div className="message-scroll-indicators">
      {indicators.map((indicator) => (
        <Tooltip
          key={indicator.messageId}
          title={renderTooltipContent(indicator)}
          placement="left"
        >
          <div
            className="indicator-item"
            style={{ top: indicator.y }}
            onClick={() => handleJump(indicator.messageId)}
          >
            {renderSmallIcon(indicator)}
          </div>
        </Tooltip>
      ))}
    </div>
  );
};

export default MessageScrollIndicator;