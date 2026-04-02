// isdp/web/src/pages/thread/InputArea.tsx
import React, { memo, useRef, useState, useCallback } from 'react';
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
  const [inputValue, setInputValue] = useState('');
  const [mentionListVisible, setMentionListVisible] = useState(false);
  const [mentionFilter, setMentionFilter] = useState('');
  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);

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

  const handleKeyPress = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }, [handleSend]);

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

  return (
    <div className="thread-input">
      <div style={{ position: 'relative', flex: 1 }}>
        <TextArea
          ref={inputRef}
          value={inputValue}
          onChange={handleInputChange}
          onKeyPress={handleKeyPress}
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
            {loadingContext ? (
              <div style={{ padding: 16, textAlign: 'center' }}>
                <Spin size="small" />
                <span style={{ marginLeft: 8 }}>加载中...</span>
              </div>
            ) : agentOptions.length === 0 ? (
              <div style={{ padding: 16, textAlign: 'center', color: '#999' }}>
                当前团队没有可用的 Agent
              </div>
            ) : (
              <List
                size="small"
                dataSource={filteredAgents}
                renderItem={(opt) => (
                  <List.Item
                    className="mention-list-item"
                    style={{ cursor: 'pointer', padding: '8px 12px' }}
                    onClick={() => selectMention(opt.id, opt.role as AgentRole, opt.name)}
                  >
                    <Space>
                      <Avatar size="small" icon={<RobotOutlined />} />
                      <span>{opt.label}</span>
                    </Space>
                  </List.Item>
                )}
              />
            )}
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