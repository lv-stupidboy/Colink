// isdp/web/src/components/thread/ThreadInput.tsx
import React, { memo, useState, useCallback, useRef, useEffect } from 'react';
import { Input, Button, Space, Card, List, Spin } from 'antd';
import { SendOutlined } from '@ant-design/icons';
import AgentTypeIcon from '@/components/AgentTypeIcon';
import type { AgentRole } from '@/types';

const { TextArea } = Input;

interface AgentOption {
  id: string;
  role: AgentRole;
  requiresHuman: boolean;
  isSystem?: boolean;
  name: string;
  label: string;
}

interface ThreadInputProps {
  placeholder: string;
  loadingContext: boolean;
  agentOptions: AgentOption[];
  onSend: (content: string) => void;
  disabled?: boolean;
  prefilledMention?: string;       // 预填入的 @mention 名称
  onPrefillConsumed?: () => void;  // 预填入被使用后的回调
}

/**
 * 独立的输入组件
 * 内部管理 inputValue 状态，避免每次输入触发父组件重渲染
 */
export const ThreadInput: React.FC<ThreadInputProps> = memo(({
  placeholder,
  loadingContext,
  agentOptions,
  onSend,
  disabled = false,
  prefilledMention,          // 新增
  onPrefillConsumed,         // 新增
}) => {
  const inputRef = useRef<any>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const [inputValue, setInputValue] = useState('');
  const [mentionListVisible, setMentionListVisible] = useState(false);
  const [mentionFilter, setMentionFilter] = useState('');
  const [highlightedIndex, setHighlightedIndex] = useState(0);
  const [showPrefillHint, setShowPrefillHint] = useState(false);  // 新增

  // 发送消息
  const handleSend = useCallback(() => {
    if (!inputValue.trim() || disabled) return;

    const content = inputValue.trim();
    setInputValue('');
    setMentionListVisible(false);
    onSend(content);
  }, [inputValue, disabled, onSend]);

  // 输入变化
  const handleInputChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const value = e.target.value;
    setInputValue(value);

    // 检测 @ 符号
    const lastAtIndex = value.lastIndexOf('@');
    if (lastAtIndex >= 0 && lastAtIndex === value.length - 1) {
      setMentionListVisible(true);
      setMentionFilter('');
      setHighlightedIndex(0);
    } else if (lastAtIndex >= 0 && value.indexOf(' ', lastAtIndex) === -1) {
      setMentionListVisible(true);
      setMentionFilter(value.substring(lastAtIndex + 1).toLowerCase());
      setHighlightedIndex(0);
    } else {
      setMentionListVisible(false);
    }
  }, []);

  // 选择 mention
  const selectMention = useCallback((name: string) => {
    const lastAtIndex = inputValue.lastIndexOf('@');
    if (lastAtIndex >= 0) {
      setInputValue(inputValue.substring(0, lastAtIndex) + '@' + name + ' ');
    }
    setMentionListVisible(false);
    inputRef.current?.focus();
  }, [inputValue]);

  // 过滤 Agent 列表
  const filteredAgents = agentOptions.filter(opt =>
    !mentionFilter ||
    opt.label.toLowerCase().includes(mentionFilter.toLowerCase()) ||
    opt.role.toLowerCase().includes(mentionFilter.toLowerCase())
  );

  // 当过滤列表变化时，重置高亮索引
  useEffect(() => {
    setHighlightedIndex(0);
  }, [mentionFilter]);

  // 滚动到高亮项
  useEffect(() => {
    if (mentionListVisible && listRef.current) {
      const items = listRef.current.querySelectorAll('.mention-list-item');
      if (items[highlightedIndex]) {
        items[highlightedIndex].scrollIntoView({ block: 'nearest' });
      }
    }
  }, [highlightedIndex, mentionListVisible]);

  // 自动填入 @mention（阻塞确认后触发）
  useEffect(() => {
    if (prefilledMention && inputRef.current) {
      // 检查是否在 agentOptions 中
      const agentExists = agentOptions.some(
        opt => opt.name === prefilledMention || opt.label.includes(prefilledMention)
      );

      if (agentExists) {
        // 自动填入 @mention
        setInputValue(`@${prefilledMention} `);
        inputRef.current.focus();
        setShowPrefillHint(true);

        // 3秒后隐藏提示
        setTimeout(() => setShowPrefillHint(false), 3000);

        // 通知父组件预填入已使用
        if (onPrefillConsumed) {
          onPrefillConsumed();
        }
      }
    }
  }, [prefilledMention, agentOptions, onPrefillConsumed]);

  // 键盘导航 - 上下键选择、Enter 确认、Escape 关闭
  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (mentionListVisible && filteredAgents.length > 0) {
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setHighlightedIndex(prev => prev > 0 ? prev - 1 : filteredAgents.length - 1);
        return;
      }
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setHighlightedIndex(prev => prev < filteredAgents.length - 1 ? prev + 1 : 0);
        return;
      }
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        const agent = filteredAgents[highlightedIndex];
        if (agent) {
          selectMention(agent.name);
        }
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        setMentionListVisible(false);
        return;
      }
    }

    // 正常发送
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }, [mentionListVisible, filteredAgents, highlightedIndex, selectMention, handleSend]);

  return (
    <div className="thread-input" style={{ display: 'flex', gap: '12px', padding: '12px 16px' }}>
      <div style={{ position: 'relative', flex: 1 }}>
        {/* 预填入提示 */}
        {showPrefillHint && (
          <div
            className="prefill-hint"
            style={{
              position: 'absolute',
              top: -28,
              left: 0,
              padding: '4px 8px',
              background: 'var(--color-primary-opacity-10, rgba(24, 144, 255, 0.1))',
              borderRadius: 4,
              fontSize: 12,
              color: 'var(--color-primary, #1890ff)',
              whiteSpace: 'nowrap',
            }}
          >
            已自动填入 @{prefilledMention}，可切换其他 Agent
          </div>
        )}

        <TextArea
          ref={inputRef}
          value={inputValue}
          onChange={handleInputChange}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          autoSize={{ minRows: 2, maxRows: 6 }}
          disabled={disabled}
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
                  renderItem={(opt, index) => {
                    const isHighlighted = index === highlightedIndex;

                    return (
                      <List.Item
                        className="mention-list-item"
                        style={{
                          cursor: 'pointer',
                          padding: '8px 12px',
                          backgroundColor: isHighlighted ? 'var(--color-primary-opacity-10, rgba(24, 144, 255, 0.1))' : 'transparent',
                          borderRadius: 4,
                          transition: 'background-color 0.15s ease',
                        }}
                        onClick={() => selectMention(opt.name)}
                        onMouseEnter={() => setHighlightedIndex(index)}
                      >
                        <Space>
                          <AgentTypeIcon
                            requiresHuman={opt.requiresHuman}
                            isSystem={opt.isSystem || false}
                            size={16}
                          />
                          <span>{opt.label}</span>
                        </Space>
                      </List.Item>
                    );
                  }}
                />
              )}
            </div>
          </Card>
        )}
      </div>
      <Space direction="vertical">
        <Button type="primary" icon={<SendOutlined />} onClick={handleSend} disabled={disabled || !inputValue.trim()}>
          发送
        </Button>
      </Space>
    </div>
  );
});

ThreadInput.displayName = 'ThreadInput';