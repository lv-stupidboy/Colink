import React, { useState } from 'react';
import { Modal, Form, Select, Input, Button, Space, Tag, Typography, message, Divider } from 'antd';
import { TeamOutlined, QuestionCircleOutlined } from '@ant-design/icons';
import type { AgentConfig } from '@/types';
import { AgentRoleLabels } from '@/types';

const { TextArea } = Input;
const { Text } = Typography;

export interface MultiMentionModalProps {
  visible: boolean;
  agents: AgentConfig[];
  onCancel: () => void;
  onSubmit: (message: string) => void;
}

/**
 * MultiMentionModal 组件
 * 帮助用户快速起草多 Agent 讨论请求
 * 生成格式化的消息，如 "@AgentA @AgentB 请讨论以下问题：..."
 */
const MultiMentionModal: React.FC<MultiMentionModalProps> = ({
  visible,
  agents,
  onCancel,
  onSubmit,
}) => {
  const [form] = Form.useForm();
  const [selectedAgents, setSelectedAgents] = useState<string[]>([]);
  const [question, setQuestion] = useState('');
  const [context, setContext] = useState('');

  const handleSubmit = () => {
    if (selectedAgents.length === 0) {
      message.warning('请选择至少一个 Agent');
      return;
    }
    if (!question.trim()) {
      message.warning('请输入问题内容');
      return;
    }

    // 生成格式化的消息
    const mentions = selectedAgents.map(id => {
      const agent = agents.find(a => a.id === id);
      return agent ? `@${agent.name}` : '';
    }).filter(Boolean).join(' ');

    let formattedMessage = `${mentions} 请讨论以下问题：\n\n${question}`;
    if (context.trim()) {
      formattedMessage += `\n\n背景信息：\n${context}`;
    }

    onSubmit(formattedMessage);
    handleReset();
  };

  const handleReset = () => {
    form.resetFields();
    setSelectedAgents([]);
    setQuestion('');
    setContext('');
  };

  const handleCancel = () => {
    handleReset();
    onCancel();
  };

  // Agent 选项
  const agentOptions = agents.map(agent => ({
    label: `${agent.name} (${AgentRoleLabels[agent.role as keyof typeof AgentRoleLabels] || agent.role})`,
    value: agent.id,
  }));

  return (
    <Modal
      title={
        <Space>
          <TeamOutlined />
          <span>发起多 Agent 讨论</span>
        </Space>
      }
      open={visible}
      onCancel={handleCancel}
      footer={[
        <Button key="cancel" onClick={handleCancel}>
          取消
        </Button>,
        <Button key="submit" type="primary" onClick={handleSubmit}>
          生成消息
        </Button>,
      ]}
      width={600}
    >
      <Form form={form} layout="vertical">
        <Form.Item
          label="选择 Agent（1-3 个）"
          required
          help="选择需要参与讨论的 Agent"
        >
          <Select
            mode="multiple"
            placeholder="选择要召唤的 Agent"
            value={selectedAgents}
            onChange={setSelectedAgents}
            options={agentOptions}
            maxTagCount={3}
            optionFilterProp="label"
          />
        </Form.Item>

        <Form.Item label="问题内容" required>
          <TextArea
            placeholder="描述需要讨论的问题..."
            value={question}
            onChange={e => setQuestion(e.target.value)}
            rows={4}
            maxLength={5000}
            showCount
          />
        </Form.Item>

        <Form.Item label="背景信息（可选）">
          <TextArea
            placeholder="提供问题的背景信息，帮助 Agent 更好理解上下文..."
            value={context}
            onChange={e => setContext(e.target.value)}
            rows={3}
            maxLength={2000}
            showCount
          />
        </Form.Item>

        <Divider style={{ margin: '12px 0' }} />

        {/* 消息预览 */}
        <div style={{ marginBottom: 16 }}>
          <Text type="secondary">消息预览：</Text>
          <div
            style={{
              marginTop: 8,
              padding: 12,
              background: '#f5f5f5',
              borderRadius: 8,
              minHeight: 80,
            }}
          >
            {selectedAgents.length > 0 || question ? (
              <>
                {selectedAgents.map(id => {
                  const agent = agents.find(a => a.id === id);
                  return agent ? (
                    <Tag key={id} color="blue" style={{ marginBottom: 4 }}>
                      @{agent.name}
                    </Tag>
                  ) : null;
                })}
                {question && (
                  <div style={{ marginTop: 8 }}>
                    <Text>请讨论以下问题：</Text>
                    <div style={{ marginTop: 4, padding: '8px 12px', background: '#fff', borderRadius: 4 }}>
                      {question}
                    </div>
                  </div>
                )}
                {context && (
                  <div style={{ marginTop: 8 }}>
                    <Text type="secondary">背景信息：{context}</Text>
                  </div>
                )}
              </>
            ) : (
              <Text type="secondary">选择 Agent 并输入问题后，这里会显示消息预览</Text>
            )}
          </div>
        </div>

        <div style={{ background: '#e6f7ff', padding: 12, borderRadius: 8 }}>
          <Space>
            <QuestionCircleOutlined style={{ color: '#1890ff' }} />
            <Text type="secondary">
              生成消息后，Agent 会根据您的需求决定是否使用 multi_mention 工具召唤其他 Agent
            </Text>
          </Space>
        </div>
      </Form>
    </Modal>
  );
};

export default MultiMentionModal;