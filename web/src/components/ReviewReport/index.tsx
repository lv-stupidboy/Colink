import React from 'react';
import { Card, Tag, Typography, List, Space, Badge, Collapse, Alert } from 'antd';
import {
  WarningOutlined,
  CheckCircleOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
import type { ReviewGrade, ReviewIssue, MergeCheckResult } from '@/types';

const { Panel } = Collapse;
const { Text } = Typography;

interface ReviewReportProps {
  result?: MergeCheckResult;
  issues?: ReviewIssue[];
}

/**
 * 审查报告组件
 * 展示 P1/P2/P3 分级的问题
 */
export const ReviewReport: React.FC<ReviewReportProps> = ({
  result,
  issues,
}) => {
  // 按等级分组问题
  const getIssuesByGrade = (grade: ReviewGrade) => {
    const allIssues = issues || result?.unresolved || [];
    return allIssues.filter((issue) => issue.grade === grade);
  };

  const getGradeColor = (grade: string) => {
    switch (grade) {
      case 'P1':
        return 'red';
      case 'P2':
        return 'orange';
      case 'P3':
        return 'blue';
      default:
        return 'default';
    }
  };

  const getDecisionAlert = () => {
    if (!result) return null;

    const config = {
      allow: {
        type: 'success' as const,
        message: '允许合并',
        description: '所有关键问题已解决，可以安全合并代码',
      },
      block: {
        type: 'error' as const,
        message: '阻止合并',
        description: '存在未解决的 P1 级别问题，必须先修复',
      },
      conditional: {
        type: 'warning' as const,
        message: '条件合并',
        description: '存在 P2/P3 级别问题，建议修复后可合并',
      },
    };

    const alertConfig = config[result.decision] || config.block;

    return (
      <Alert
        type={alertConfig.type}
        message={alertConfig.message}
        description={alertConfig.description}
        showIcon
        style={{ marginBottom: 16 }}
      />
    );
  };

  const renderIssueItem = (issue: ReviewIssue) => (
    <List.Item
      style={{ padding: '12px 0' }}
      actions={[
        <Badge
          key="status"
          count={issue.status === 'resolved' ? 0 : 1}
          style={{ backgroundColor: issue.status === 'resolved' ? '#52c41a' : '#ff4d4f' }}
        />,
      ]}
    >
      <List.Item.Meta
        avatar={
          <Tag color={getGradeColor(issue.grade)}>{issue.grade}</Tag>
        }
        title={
          <Space>
            <Text strong>{issue.description}</Text>
            {issue.status === 'resolved' && (
              <CheckCircleOutlined style={{ color: '#52c41a' }} />
            )}
          </Space>
        }
        description={
          <>
            {issue.file && (
              <Text type="secondary" style={{ fontSize: 12 }}>
                {issue.file}:{issue.line || '?'}
              </Text>
            )}
          </>
        }
      />
    </List.Item>
  );

  const p1Issues = getIssuesByGrade('P1');
  const p2Issues = getIssuesByGrade('P2');
  const p3Issues = getIssuesByGrade('P3');

  return (
    <Card
      title={
        <Space>
          <WarningOutlined />
          审查报告
        </Space>
      }
      size="small"
      className="review-report"
      style={{ margin: '16px 0' }}
    >
      {getDecisionAlert()}

      {result && (
        <Space style={{ marginBottom: 16 }} wrap>
          <Badge count={result.p1Issues} overflowCount={99}>
            <Tag color="red">P1 问题</Tag>
          </Badge>
          <Badge count={result.p2Issues} overflowCount={99}>
            <Tag color="orange">P2 问题</Tag>
          </Badge>
          <Badge count={result.p3Issues} overflowCount={99}>
            <Tag color="blue">P3 问题</Tag>
          </Badge>
          <Badge count={result.resolvedP1} overflowCount={99}>
            <Tag color="green">已解决 P1</Tag>
          </Badge>
        </Space>
      )}

      <Collapse defaultActiveKey={p1Issues.length > 0 ? ['p1'] : []}>
        {p1Issues.length > 0 && (
          <Panel
            header={
              <Space>
                <Tag color="red">P1</Tag>
                <Text strong>阻断性问题 ({p1Issues.length})</Text>
              </Space>
            }
            key="p1"
          >
            <List
              itemLayout="horizontal"
              dataSource={p1Issues}
              renderItem={renderIssueItem}
            />
          </Panel>
        )}

        {p2Issues.length > 0 && (
          <Panel
            header={
              <Space>
                <Tag color="orange">P2</Tag>
                <Text>重要问题 ({p2Issues.length})</Text>
              </Space>
            }
            key="p2"
          >
            <List
              itemLayout="horizontal"
              dataSource={p2Issues}
              renderItem={renderIssueItem}
            />
          </Panel>
        )}

        {p3Issues.length > 0 && (
          <Panel
            header={
              <Space>
                <Tag color="blue">P3</Tag>
                <Text>建议性问题 ({p3Issues.length})</Text>
              </Space>
            }
            key="p3"
          >
            <List
              itemLayout="horizontal"
              dataSource={p3Issues}
              renderItem={renderIssueItem}
            />
          </Panel>
        )}

        {p1Issues.length === 0 && p2Issues.length === 0 && p3Issues.length === 0 && (
          <Panel header="暂无问题" key="empty">
            <Alert
              message="未发现任何问题"
              description="所有审查项均已通过"
              type="success"
              showIcon
            />
          </Panel>
        )}
      </Collapse>

      {result?.recommendations && result.recommendations.length > 0 && (
        <div style={{ marginTop: 16 }}>
          <Text strong>改进建议：</Text>
          <List
            size="small"
            dataSource={result.recommendations}
            renderItem={(item) => (
              <List.Item>
                <Space>
                  <InfoCircleOutlined style={{ color: '#1890ff' }} />
                  <Text type="secondary">{item}</Text>
                </Space>
              </List.Item>
            )}
          />
        </div>
      )}
    </Card>
  );
};

export default ReviewReport;
