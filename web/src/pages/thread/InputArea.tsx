// isdp/web/src/pages/thread/InputArea.tsx
import React, { memo, useRef, useState, useCallback, useEffect } from 'react';
import { Input, Button, Space, Card, List, Avatar, Spin } from 'antd';
import { SendOutlined, RobotOutlined } from '@ant-design/icons';
import type { AgentRole } from '@/types';

const { TextArea } = Input;

interface AgentOption {
  id: string;
  role: AgentRole;
  name: string;
  label: string;
}

interface InputAreaProps {
  placeholder: string;
  loadingContext: boolean;
  agentOptions: AgentOption[];
  onSend: (content: string, skipAgentTrigger?: boolean) => void;
  onSpawnAgent: (role: AgentRole, input: string, configId?: string) => void;
}

/**
 * 输入区域组件
 * 只管理本地输入状态，不订阅全局 store
 */
export const InputArea: React.FC<InputAreaProps> = memo(({
  placeholder,
  loadingContext,
  agentOptions,
  onSend,
  onSpawnAgent,
}) => {
  const inputRef = useRef<any>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const [inputValue, setInputValue] = useState('');
  const [mentionListVisible, setMentionListVisible] = useState(false);
  const [mentionFilter, setMentionFilter] = useState('');
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [highlightedIndex, setHighlightedIndex] = useState(0);

  const handleSend = useCallback(() => {
    if (!inputValue.trim()) return;

    const content = inputValue.trim();
    setInputValue('');
    setMentionListVisible(false);

    // 检查是否是 @mention 命令
    const mentionMatch = content.match(/^@(\S+)\s*(.*)/);
    if (mentionMatch) {
      const agentName = mentionMatch[1].toLowerCase();
      const input = mentionMatch[2] || content;

      if (selectedAgentId) {
        onSend(content, true);
        onSpawnAgent('custom', input, selectedAgentId);
        setSelectedAgentId(null);
        return;
      }

      const agentByName = agentOptions.find(opt =>
        opt.name.toLowerCase() === agentName ||
        opt.label.toLowerCase() === agentName
      );
      if (agentByName) {
        onSend(content, true);
        onSpawnAgent('custom', input, agentByName.id);
        setSelectedAgentId(null);
        return;
      }

      // 未找到 Agent，正常发送
      onSend(content);
      setSelectedAgentId(null);
    } else {
      onSend(content);
      setSelectedAgentId(null);
    }
  }, [inputValue, selectedAgentId, agentOptions, onSend, onSpawnAgent]);

  const handleInputChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const value = e.target.value;
    setInputValue(value);

    // 检测 @ 符号
    const lastAtIndex = value.lastIndexOf('@');
    if (lastAtIndex >= 0 && lastAtIndex === value.length - 1) {
      setMentionListVisible(true);
      setMentionFilter('');
      setSelectedAgentId(null);
    } else if (lastAtIndex >= 0 && value.indexOf(' ', lastAtIndex) === -1) {
      setMentionListVisible(true);
      setMentionFilter(value.substring(lastAtIndex + 1).toLowerCase());
      setSelectedAgentId(null);
    } else {
      setMentionListVisible(false);
    }
  }, []);

  const selectMention = useCallback((agentId: string, _agentRole: AgentRole, label: string) => {
    const lastAtIndex = inputValue.lastIndexOf('@');
    if (lastAtIndex >= 0) {
      setInputValue(inputValue.substring(0, lastAtIndex) + '@' + label + ' ');
    }
    setSelectedAgentId(agentId);
    setMentionListVisible(false);
    inputRef.current?.focus();
  }, [inputValue]);

  const filteredAgents = agentOptions.filter(opt =>
    !mentionFilter ||
    opt.label.toLowerCase().includes(mentionFilter.toLowerCase()) ||
    opt.role.toLowerCase().includes(mentionFilter.toLowerCase())
  );

  // 当过滤列表变化时，重置高亮索引
  useEffect(() => {
    setHighlightedIndex(0);
  }, [filteredAgents.length]);

  // 滚动到高亮项
  useEffect(() => {
    if (mentionListVisible && listRef.current) {
      const items = listRef.current.querySelectorAll('.mention-list-item');
      if (items[highlightedIndex]) {
        items[highlightedIndex].scrollIntoView({ block: 'nearest' });
      }
    }
  }, [highlightedIndex, mentionListVisible]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    // 如果 mention 列表可见，处理上下键和 Enter 选择
    if (mentionListVisible && filteredAgents.length > 0) {
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setHighlightedIndex(prev =>
          prev > 0 ? prev - 1 : filteredAgents.length - 1
        );
        return;
      }
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setHighlightedIndex(prev =>
          prev < filteredAgents.length - 1 ? prev + 1 : 0
        );
        return;
      }
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        const agent = filteredAgents[highlightedIndex];
        if (agent) {
          selectMention(agent.id, agent.role as AgentRole, agent.name);
        }
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setMentionListVisible(false);
        return;
      }
    }

    // 正常发送逻辑
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }, [mentionListVisible, filteredAgents, highlightedIndex, selectMention, handleSend]);

  return (
    <div className="thread-input">
      <div style={{ position: 'relative', flex: 1 }}>
        <TextArea
          ref={inputRef}
          value={inputValue}
          onChange={handleInputChange}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          autoSize={{ minRows: 2, maxRows: 6 }}
        />
        {mentionListVisible && (
          <Card
            size="small"
            className="mention-dropdown"
            style={{
              position: 'absolute',
              bottom: '100%',
              left: 0,
              marginBottom: 4,
              minWidth: 200,
              zIndex: 1000,
            }}
          >
            <div ref={listRef}>
              {loadingContext ? (
                <div style={{ padding: 16, textAlign: 'center' }}>
                  <Spin size="small" />
                  <span style={{ marginLeft: 8 }}>加载中...</span>
                </div>
              ) : agentOptions.length === 0 ? (
                <div style={{ padding: 16, textAlign: 'center', color: '#999' }}>
                  当前团队没有可用的 Agent
                </div>
              ) : filteredAgents.length === 0 ? (
                <div style={{ padding: 16, textAlign: 'center', color: '#999' }}>
                  没有匹配的 Agent
                </div>
              ) : (
                <List
                  size="small"
                  dataSource={filteredAgents}
                  renderItem={(opt, index) => (
                    <List.Item
                      className="mention-list-item"
                      style={{
                        cursor: 'pointer',
                        padding: '8px 12px',
                        backgroundColor: index === highlightedIndex ? 'var(--color-primary-opacity-10)' : 'transparent',
                        borderRadius: '4px',
                      }}
                      onClick={() => selectMention(opt.id, opt.role as AgentRole, opt.name)}
                      onMouseEnter={() => setHighlightedIndex(index)}
                    >
                      <Space>
                        <Avatar size="small" icon={<RobotOutlined />} />
                        <span>{opt.label}</span>
                      </Space>
                    </List.Item>
                  )}
                />
              )}
            </div>
          </Card>
        )}
      </div>
      <Space direction="vertical">
        <Button type="primary" icon={<SendOutlined />} onClick={handleSend}>
          发送
        </Button>
      </Space>
    </div>
  );
});

InputArea.displayName = 'InputArea';