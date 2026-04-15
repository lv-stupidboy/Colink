// web/src/components/thread/MessageScrollIndicator.tsx
import React, { useEffect, useState, useCallback, RefObject } from 'react';
import { Avatar, Tooltip } from 'antd';
import { UserOutlined, CrownOutlined, RobotOutlined } from '@ant-design/icons';
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
      // 使用 requestAnimationFrame 节流
      requestAnimationFrame(updateIndicators);
    };

    container.addEventListener('scroll', handleScroll, { passive: true });
    return () => container.removeEventListener('scroll', handleScroll);
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
      // 默认跳转逻辑
      const element = document.querySelector(`[data-message-id="${messageId}"]`);
      if (element) {
        element.scrollIntoView({ behavior: 'smooth', block: 'center' });
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

  // 渲染指示器图标
  const renderIndicatorIcon = (indicator: IndicatorItem) => {
    const agentConfig = getAgentConfig(indicator.agentId, indicator.agentName);

    if (indicator.role === 'user') {
      return <UserOutlined style={{ color: 'var(--text-primary)' }} />;
    }

    if (indicator.role === 'system') {
      return <CrownOutlined style={{ color: '#faad14' }} />;
    }

    // Agent 角色
    if (agentConfig?.isSystem) {
      return <CrownOutlined style={{ color: '#faad14' }} />;
    }

    return <RobotOutlined style={{ color: 'var(--color-primary)' }} />;
  };

  // 获取显示名称
  const getDisplayName = (indicator: IndicatorItem) => {
    if (indicator.role === 'user') return '用户';
    if (indicator.role === 'system') return '系统';
    return indicator.agentName || 'Agent';
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
          title={getDisplayName(indicator)}
          placement="left"
        >
          <div
            className="indicator-item"
            style={{ top: indicator.y }}
            onClick={() => handleJump(indicator.messageId)}
          >
            <Avatar size={16} icon={renderIndicatorIcon(indicator)} />
          </div>
        </Tooltip>
      ))}
    </div>
  );
};

export default MessageScrollIndicator;