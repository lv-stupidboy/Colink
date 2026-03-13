import React from 'react';
import { Modal, Space, Typography, Alert, Card, List, Tag, Button, Divider } from 'antd';
import {
  CheckCircleOutlined,
  FileTextOutlined,
  CodeOutlined,
  RocketOutlined,
  EditOutlined,
} from '@ant-design/icons';
import type { MergeCheckResult, ReviewIssue } from '@/types';

const { Text } = Typography;

/**
 * 检查点类型
 * PRD Section 5.3.5 - 人工检查点
 */
export type CheckpointType = 'requirement' | 'design' | 'review' | 'deploy';

/**
 * 检查点数据
 */
export interface CheckpointData {
  type: CheckpointType;
  title: string;
  summary: string;
  details?: string[];
  artifacts?: Array<{
    name: string;
    type: string;
    description?: string;
  }>;
  reviewResult?: MergeCheckResult;
  reviewIssues?: ReviewIssue[];
  canApprove?: boolean;
}

interface CheckpointConfirmProps {
  visible: boolean;
  data: CheckpointData | null;
  onConfirm: () => void;
  onReject: () => void;
  onModify?: () => void;
  loading?: boolean;
}

/**
 * 检查点确认组件
 * PRD Section 5.3.5 - 人工检查点
 *
 * 检查点类型：
 * - 需求确认：需求分析完成后
 * - 方案确认：架构设计完成后
 * - 代码合入：Review通过后
 * - 部署上线：测试通过后
 */
export const CheckpointConfirm: React.FC<CheckpointConfirmProps> = ({
  visible,
  data,
  onConfirm,
  onReject,
  onModify,
  loading,
}) => {
  if (!data) return null;

  const getCheckpointConfig = (type: CheckpointType) => {
    const configs = {
      requirement: {
        icon: <FileTextOutlined style={{ color: '#1890ff' }} />,
        color: '#1890ff',
        confirmText: '确认需求',
        rejectText: '需要修改',
        description: '请确认需求分析结果是否符合您的预期',
      },
      design: {
        icon: <CodeOutlined style={{ color: '#722ed1' }} />,
        color: '#722ed1',
        confirmText: '确认方案',
        rejectText: '需要调整',
        description: '请确认技术方案是否符合您的预期',
      },
      review: {
        icon: <CheckCircleOutlined style={{ color: '#52c41a' }} />,
        color: '#52c41a',
        confirmText: '放行',
        rejectText: '不能放行',
        description: '请确认代码审查结果，P1/P2 问题已清零',
      },
      deploy: {
        icon: <RocketOutlined style={{ color: '#13c2c2' }} />,
        color: '#13c2c2',
        confirmText: '确认部署',
        rejectText: '暂不部署',
        description: '请确认是否可以部署到生产环境',
      },
    };
    return configs[type];
  };

  const config = getCheckpointConfig(data.type);

  /**
   * 渲染需求确认内容
   */
  const renderRequirementContent = () => (
    <>
      <Alert
        type="info"
        message="需求确认"
        description={config.description}
        showIcon
        style={{ marginBottom: 16 }}
      />

      {data.details && data.details.length > 0 && (
        <Card size="small" title="需求摘要" style={{ marginBottom: 16 }}>
          <List
            dataSource={data.details}
            renderItem={(item, index) => (
              <List.Item>
                <Text>{index + 1}. {item}</Text>
              </List.Item>
            )}
          />
        </Card>
      )}

      {data.artifacts && data.artifacts.length > 0 && (
        <Card size="small" title="相关产物">
          <List
            dataSource={data.artifacts}
            renderItem={(item) => (
              <List.Item>
                <Space>
                  <FileTextOutlined />
                  <Text>{item.name}</Text>
                  {item.description && <Text type="secondary">- {item.description}</Text>}
                </Space>
              </List.Item>
            )}
          />
        </Card>
      )}
    </>
  );

  /**
   * 渲染方案确认内容
   */
  const renderDesignContent = () => (
    <>
      <Alert
        type="info"
        message="方案确认"
        description={config.description}
        showIcon
        style={{ marginBottom: 16 }}
      />

      {data.details && data.details.length > 0 && (
        <Card size="small" title="技术方案摘要" style={{ marginBottom: 16 }}>
          <List
            dataSource={data.details}
            renderItem={(item, index) => (
              <List.Item>
                <Text>{index + 1}. {item}</Text>
              </List.Item>
            )}
          />
        </Card>
      )}

      {data.artifacts && data.artifacts.length > 0 && (
        <Card size="small" title="设计产物">
          <List
            dataSource={data.artifacts}
            renderItem={(item) => (
              <List.Item>
                <Space>
                  <CodeOutlined />
                  <Text>{item.name}</Text>
                  <Tag>{item.type}</Tag>
                </Space>
              </List.Item>
            )}
          />
        </Card>
      )}
    </>
  );

  /**
   * 渲染审查放行内容
   * PRD Section 2.5.3 - Review分级标准
   */
  const renderReviewContent = () => {
    const issues = data.reviewIssues || [];

    const p1Issues = issues.filter((i) => i.grade === 'P1');
    const p2Issues = issues.filter((i) => i.grade === 'P2');
    const p3Issues = issues.filter((i) => i.grade === 'P3');

    const canApprove = p1Issues.length === 0 && p2Issues.length === 0;

    return (
      <>
        <Alert
          type={canApprove ? 'success' : 'warning'}
          message={canApprove ? '可以放行' : '存在问题，不能放行'}
          description={
            canApprove
              ? 'P1/P2 问题已清零，代码可以合入'
              : `P1/P2 问题未清零，必须修复后才能放行`
          }
          showIcon
          style={{ marginBottom: 16 }}
        />

        <Card size="small" title="审查结果" style={{ marginBottom: 16 }}>
          <Space direction="vertical" style={{ width: '100%' }}>
            <div>
              <Tag color="red">P1: {p1Issues.length}</Tag>
              <Tag color="orange">P2: {p2Issues.length}</Tag>
              <Tag color="blue">P3: {p3Issues.length}</Tag>
            </div>

            {issues.length > 0 && (
              <>
                <Divider style={{ margin: '8px 0' }} />
                <List
                  size="small"
                  dataSource={issues.slice(0, 5)}
                  renderItem={(issue) => (
                    <List.Item>
                      <Space>
                        <Tag color={issue.grade === 'P1' ? 'red' : issue.grade === 'P2' ? 'orange' : 'blue'}>
                          {issue.grade}
                        </Tag>
                        <Text ellipsis style={{ maxWidth: 300 }}>
                          {issue.description}
                        </Text>
                      </Space>
                    </List.Item>
                  )}
                />
                {issues.length > 5 && (
                  <Text type="secondary">...还有 {issues.length - 5} 个问题</Text>
                )}
              </>
            )}
          </Space>
        </Card>

        <Text type="secondary">
          放行条件：P1/P2 问题全部清零，审查员明确放行
        </Text>
      </>
    );
  };

  /**
   * 渲染部署确认内容
   */
  const renderDeployContent = () => (
    <>
      <Alert
        type="info"
        message="部署确认"
        description={config.description}
        showIcon
        style={{ marginBottom: 16 }}
      />

      {data.details && data.details.length > 0 && (
        <Card size="small" title="部署信息" style={{ marginBottom: 16 }}>
          <List
            dataSource={data.details}
            renderItem={(item, index) => (
              <List.Item>
                <Text>{index + 1}. {item}</Text>
              </List.Item>
            )}
          />
        </Card>
      )}

      <Text type="secondary">
        部署后代码将被推送到生产环境，请确保已完成充分测试
      </Text>
    </>
  );

  const renderContent = () => {
    switch (data.type) {
      case 'requirement':
        return renderRequirementContent();
      case 'design':
        return renderDesignContent();
      case 'review':
        return renderReviewContent();
      case 'deploy':
        return renderDeployContent();
      default:
        return null;
    }
  };

  // 审查放行时，如果有 P1/P2 问题，禁止确认
  const canConfirm = data.type === 'review'
    ? (data.reviewResult?.decision === 'allow' || (data.reviewIssues?.filter(i => i.grade === 'P1' || i.grade === 'P2').length || 0) === 0)
    : true;

  return (
    <Modal
      title={
        <Space>
          {config.icon}
          <span>{data.title}</span>
        </Space>
      }
      open={visible}
      onCancel={onReject}
      footer={
        <Space style={{ width: '100%', justifyContent: 'space-between' }}>
          <Space>
            {onModify && (
              <Button icon={<EditOutlined />} onClick={onModify}>
                修改内容
              </Button>
            )}
          </Space>
          <Space>
            <Button onClick={onReject}>{config.rejectText}</Button>
            <Button
              type="primary"
              onClick={onConfirm}
              disabled={!canConfirm}
              loading={loading}
              icon={<CheckCircleOutlined />}
            >
              {config.confirmText}
            </Button>
          </Space>
        </Space>
      }
      width={600}
    >
      {renderContent()}
    </Modal>
  );
};

export default CheckpointConfirm;