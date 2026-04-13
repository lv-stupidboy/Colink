// web/src/components/BlockingAlert/BlockingAlertModal.tsx

import React from 'react';
import { Modal, Tag, Space, Typography, Card, Button, Alert } from 'antd';
import {
  ClockCircleOutlined,
} from '@ant-design/icons';
import type { BlockingItem } from '@/types/blocking';
import './BlockingAlertModal.css';

const { Text, Paragraph } = Typography;

interface BlockingAlertModalProps {
  visible: boolean;
  blockingItem: BlockingItem | null;
  onConfirm: (item: BlockingItem) => void;
  onReject: (item: BlockingItem) => void;
  onSkip: () => void;
}

/** 阻塞类型配置 - 只有 schedule_end */
const BlockingTypeConfig = {
  icon: <ClockCircleOutlined />,
  color: 'green',
  title: 'Agent 执行完成',
  confirmText: '继续任务',
  rejectText: '查看结果',
};

export const BlockingAlertModal: React.FC<BlockingAlertModalProps> = ({
  visible,
  blockingItem,
  onConfirm,
  onReject,
  onSkip,
}) => {
  if (!blockingItem) return null;

  return (
    <Modal
      title={
        <Space>
          <Tag color={BlockingTypeConfig.color}>
            {BlockingTypeConfig.icon}
          </Tag>
          <span>{BlockingTypeConfig.title}</span>
        </Space>
      }
      open={visible}
      onCancel={onSkip}
      footer={
        <Space style={{ width: '100%', justifyContent: 'space-between' }}>
          <Button onClick={onSkip}>关闭</Button>
          <Space>
            <Button onClick={() => onReject(blockingItem)}>
              {BlockingTypeConfig.rejectText}
            </Button>
            <Button type="primary" onClick={() => onConfirm(blockingItem)}>
              {BlockingTypeConfig.confirmText}
            </Button>
          </Space>
        </Space>
      }
      width={500}
      centered
      className="blocking-alert-modal"
    >
      <Alert
        type="success"
        message={`Agent "${blockingItem.sourceAgentName}" 执行完成`}
        showIcon
        style={{ marginBottom: 16 }}
      />

      <Card size="small" title="执行结果" style={{ marginBottom: 12 }}>
        <Paragraph style={{ margin: 0 }}>
          {blockingItem.summary}
        </Paragraph>
        {blockingItem.details && blockingItem.details.length > 0 && (
          <Paragraph type="secondary" style={{ margin: '8px 0 0', fontSize: 12 }}>
            最后输出：{blockingItem.details[0]}
          </Paragraph>
        )}
      </Card>

      <Text type="secondary" style={{ fontSize: 12 }}>
        Agent 调度链已结束，请指示下一步操作
      </Text>
    </Modal>
  );
};

export default BlockingAlertModal;