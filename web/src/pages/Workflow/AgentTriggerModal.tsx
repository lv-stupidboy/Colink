import React, { useState, useEffect } from 'react';
import { Modal, Form, Select, Input, Button, Empty, Typography, Space } from 'antd';
import { PlusOutlined, DeleteOutlined, RobotOutlined, CrownOutlined } from '@ant-design/icons';
import type { AgentConfig } from '@/types';

const { Text } = Typography;
const { Option } = Select;
const { TextArea } = Input;

// Agent 触发配置
interface AgentTrigger {
  toAgentId: string;
  triggerHint: string;
}

// 团队视图中的 Agent
interface TeamAgent {
  config: AgentConfig;
  triggers: AgentTrigger[];
}

interface AgentTriggerModalProps {
  visible: boolean;
  agent: TeamAgent | null;
  allAgents: AgentConfig[];
  onSave: (triggers: AgentTrigger[]) => Promise<void>;
  onClose: () => void;
}

const AgentTriggerModal: React.FC<AgentTriggerModalProps> = ({
  visible,
  agent,
  allAgents,
  onSave,
  onClose,
}) => {
  const [triggers, setTriggers] = useState<AgentTrigger[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (agent && visible) {
      setTriggers([...agent.triggers]);
    } else {
      setTriggers([]);
    }
  }, [agent, visible]);

  // 获取可选的目标 Agent（排除自己）
  const getAvailableTargets = () => {
    const currentAgentId = agent?.config.id;
    return allAgents.filter(a => a.id !== currentAgentId);
  };

  // 添加触发目标
  const handleAddTrigger = () => {
    setTriggers([...triggers, { toAgentId: '', triggerHint: '' }]);
  };

  // 删除触发目标
  const handleRemoveTrigger = (index: number) => {
    setTriggers(triggers.filter((_, i) => i !== index));
  };

  // 更新触发配置
  const handleUpdateTrigger = (index: number, field: keyof AgentTrigger, value: string) => {
    const updated = triggers.map((t, i) =>
      i === index ? { ...t, [field]: value } : t
    );
    setTriggers(updated);
  };

  // 保存
  const handleSave = async () => {
    // 验证
    const invalidTrigger = triggers.find(t => !t.toAgentId);
    if (invalidTrigger) {
      return;
    }

    setLoading(true);
    try {
      await onSave(triggers);
    } finally {
      setLoading(false);
    }
  };

  if (!agent) return null;

  const availableTargets = getAvailableTargets();

  return (
    <Modal
      title={
        <Space>
          {agent.config.isSystem ? (
            <CrownOutlined style={{ color: '#faad14' }} />
          ) : (
            <RobotOutlined style={{ color: 'var(--color-primary)' }} />
          )}
          <span>配置触发规则：{agent.config.name}</span>
        </Space>
      }
      open={visible}
      onCancel={onClose}
      onOk={handleSave}
      confirmLoading={loading}
      width={550}
      okText="保存"
      cancelText="取消"
    >
      <div style={{ marginBottom: 16 }}>
        <Text type="secondary">
          配置此 Agent 完成后，在什么条件下触发其他 Agent
        </Text>
      </div>

      {triggers.length === 0 ? (
        <Empty
          description="暂无触发配置"
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          style={{ padding: 20 }}
        >
          <Button type="dashed" icon={<PlusOutlined />} onClick={handleAddTrigger}>
            添加触发目标
          </Button>
        </Empty>
      ) : (
        <div>
          {triggers.map((trigger, index) => (
            <div
              key={index}
              style={{
                padding: 16,
                marginBottom: 12,
                border: '1px solid var(--ant-color-border)',
                borderRadius: 8,
                background: 'var(--ant-color-bg-container)',
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
                <Text strong>触发目标 {index + 1}</Text>
                <Button
                  type="text"
                  danger
                  size="small"
                  icon={<DeleteOutlined />}
                  onClick={() => handleRemoveTrigger(index)}
                >
                  删除
                </Button>
              </div>

              <Form.Item label="目标 Agent" style={{ marginBottom: 12 }}>
                <Select
                  value={trigger.toAgentId}
                  onChange={(value) => handleUpdateTrigger(index, 'toAgentId', value)}
                  placeholder="选择要触发的 Agent"
                  showSearch
                  optionFilterProp="children"
                >
                  {availableTargets.map(a => (
                    <Option key={a.id} value={a.id}>
                      {a.name} ({a.role})
                    </Option>
                  ))}
                </Select>
              </Form.Item>

              <Form.Item label="触发条件" style={{ marginBottom: 0 }}>
                <TextArea
                  value={trigger.triggerHint}
                  onChange={(e) => handleUpdateTrigger(index, 'triggerHint', e.target.value)}
                  placeholder="例如：当需要前端实现时"
                  rows={2}
                />
              </Form.Item>
            </div>
          ))}

          <Button
            type="dashed"
            icon={<PlusOutlined />}
            onClick={handleAddTrigger}
            style={{ width: '100%' }}
          >
            添加触发目标
          </Button>
        </div>
      )}
    </Modal>
  );
};

export default AgentTriggerModal;