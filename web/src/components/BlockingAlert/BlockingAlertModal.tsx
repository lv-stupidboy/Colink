// web/src/components/BlockingAlert/BlockingAlertModal.tsx

import React from 'react';
import { Modal, Tag, Space, Typography, Card, List, Button, Alert } from 'antd';
import {
  QuestionCircleOutlined,
  ToolOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import type { BlockingItem, BlockingType } from '@/types/blocking';
import './BlockingAlertModal.css';

const { Text, Paragraph } = Typography;

interface BlockingAlertModalProps {
  visible: boolean;
  blockingItem: BlockingItem | null;
  onConfirm: (item: BlockingItem) => void;
  onReject: (item: BlockingItem) => void;
  onSkip: () => void;
}

/** 阻塞类型配置 */
const BlockingTypeConfig: Record<BlockingType, {
  icon: React.ReactNode;
  color: string;
  title: string;
  confirmText: string;
  rejectText: string;
}> = {
  tool_confirm: {
    icon: <ToolOutlined />,
    color: 'orange',
    title: '工具执行确认',
    confirmText: '确认执行',
    rejectText: '拒绝执行',
  },
  agent_question: {
    icon: <QuestionCircleOutlined />,
    color: 'blue',
    title: 'Agent 需要您的回答',
    confirmText: '去回答',
    rejectText: '稍后处理',
  },
  task_blocked: {
    icon: <WarningOutlined />,
    color: 'red',
    title: '任务阻塞提醒',
    confirmText: '去处理',
    rejectText: '暂不处理',
  },
};

export const BlockingAlertModal: React.FC<BlockingAlertModalProps> = ({
  visible,
  blockingItem,
  onConfirm,
  onReject,
  onSkip,
}) => {
  if (!blockingItem) return null;

  const config = BlockingTypeConfig[blockingItem.type];

  /** 渲染工具确认内容 */
  const renderToolConfirmContent = () => (
    <>
      <Alert
        type="warning"
        message={`Agent "${blockingItem.sourceAgentName}" 正在请求执行敏感操作`}
        showIcon
        style={{ marginBottom: 16 }}
      />

      <Card size="small" title="工具信息" style={{ marginBottom: 12 }}>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Text strong>工具名称：{blockingItem.toolName}</Text>
          {blockingItem.details && blockingItem.details.length > 0 && (
            <>
              <Text type="secondary">参数摘要：</Text>
              <List
                size="small"
                dataSource={blockingItem.details}
                renderItem={(item) => (
                  <List.Item style={{ padding: '4px 0', border: 'none' }}>
                    <Text code style={{ fontSize: 12 }}>{item}</Text>
                  </List.Item>
                )}
              />
            </>
          )}
        </Space>
      </Card>

      <Text type="secondary" style={{ fontSize: 12 }}>
        认后将执行此操作，拒绝将终止本次工具调用
      </Text>
    </>
  );

  /** 渲染 Agent 提问内容 */
  const renderAgentQuestionContent = () => (
    <>
      <Alert
        type="info"
        message={`Agent "${blockingItem.sourceAgentName}" 有问题需要您回答`}
        showIcon
        style={{ marginBottom: 16 }}
      />

      <Card size="small" title="问题内容" style={{ marginBottom: 12 }}>
        <Paragraph style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
          {blockingItem.question || blockingItem.summary}
        </Paragraph>
      </Card>

      <Text type="secondary" style={{ fontSize: 12 }}>
        请在输入框中回复，输入框已自动填入 @{blockingItem.sourceAgentName}
      </Text>
    </>
  );

  /** 渲染任务阻塞内容 */
  const renderTaskBlockedContent = () => (
    <>
      <Alert
        type="error"
        message={`Agent "${blockingItem.sourceAgentName}" 遇到阻塞`}
        showIcon
        style={{ marginBottom: 16 }}
      />

      <Card size="small" title="阻塞原因" style={{ marginBottom: 12 }}>
        <Paragraph style={{ margin: 0 }}>
          {blockingItem.summary}
        </Paragraph>
        {blockingItem.details && blockingItem.details.length > 0 && (
          <List
            size="small"
            dataSource={blockingItem.details}
            renderItem={(item) => (
              <List.Item style={{ padding: '4px 0', border: 'none' }}>
                <Text>{item}</Text>
              </List.Item>
            )}
            style={{ marginTop: 8 }}
          />
        )}
      </Card>

      <Text type="secondary" style={{ fontSize: 12 }}>
        请处理阻塞项后再继续任务
      </Text>
    </>
  );

  const renderContent = () => {
    switch (blockingItem.type) {
      case 'tool_confirm':
        return renderToolConfirmContent();
      case 'agent_question':
        return renderAgentQuestionContent();
      case 'task_blocked':
        return renderTaskBlockedContent();
      default:
        return null;
    }
  };

  return (
    <Modal
      title={
        <Space>
          <Tag color={config.color}>
            {config.icon}
          </Tag>
          <span>{config.title}</span>
        </Space>
      }
      open={visible}
      onCancel={onSkip}
      footer={
        <Space style={{ width: '100%', justifyContent: 'space-between' }}>
          <Button onClick={onSkip}>关闭</Button>
          <Space>
            <Button onClick={() => onReject(blockingItem)}>
              {config.rejectText}
            </Button>
            <Button type="primary" onClick={() => onConfirm(blockingItem)}>
              {config.confirmText}
            </Button>
          </Space>
        </Space>
      }
      width={500}
      centered
      className="blocking-alert-modal"
    >
      {renderContent()}
    </Modal>
  );
};

export default BlockingAlertModal;