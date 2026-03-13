import React from 'react';
import { Steps, Tag, Tooltip } from 'antd';
import {
  CheckCircleOutlined,
  LoadingOutlined,
  MinusOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import type { Phase, PhaseStatus } from '@/types';
import { PhaseLabels } from '@/types';

interface PhaseStep {
  phase: Phase;
  status: PhaseStatus;
  agent?: string;
  startTime?: string;
  endTime?: string;
}

interface EnhancedProgressProps {
  phases: PhaseStep[];
  currentPhase: Phase;
  collapsed?: boolean;
  onToggleCollapse?: () => void;
}

/**
 * 阶段状态标识
 * - 已完成 ✓ (绿色勾选)
 * - 运行中 ● (蓝色加载)
 * - 等待 - (灰色横线)
 * - 需确认 ⚠️ (橙色警告)
 */
const PhaseStatusIcon: React.FC<{ status: PhaseStatus }> = ({ status }) => {
  switch (status) {
    case 'completed':
      return <CheckCircleOutlined style={{ color: '#52c41a' }} />;
    case 'running':
      return <LoadingOutlined style={{ color: '#1890ff', fontSize: 16 }} />;
    case 'pending':
      return <MinusOutlined style={{ color: '#d9d9d9' }} />;
    case 'needs_review':
      return <ExclamationCircleOutlined style={{ color: '#faad14' }} />;
    default:
      return <MinusOutlined style={{ color: '#d9d9d9' }} />;
  }
};

/**
 * 增强的进度条组件
 * - 可折叠
 * - 显示每个阶段的状态
 * - 显示当前工作的 Agent
 */
export const EnhancedProgress: React.FC<EnhancedProgressProps> = ({
  phases,
  currentPhase,
  collapsed = false,
  onToggleCollapse,
}) => {
  const phaseOrder: Phase[] = [
    'requirement',
    'design',
    'development',
    'review',
    'test',
    'merge',
    'complete',
  ];

  const currentIndex = phaseOrder.indexOf(currentPhase);

  const steps = phaseOrder.map((phase, index) => {
    const phaseData = phases.find((p) => p.phase === phase);
    let stepStatus: 'finish' | 'process' | 'wait' | 'error' = 'wait';

    if (index < currentIndex) {
      stepStatus = 'finish';
    } else if (index === currentIndex) {
      stepStatus = 'process';
    }

    return {
      title: (
        <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
          <PhaseStatusIcon status={phaseData?.status || 'pending'} />
          <span>{PhaseLabels[phase]}</span>
        </div>
      ),
      description: phaseData?.agent ? `当前：${phaseData.agent}` : undefined,
      status: stepStatus,
    };
  });

  const content = (
    <div className="enhanced-progress">
      <Steps
        current={currentIndex}
        size="small"
        items={steps}
        style={{ fontSize: 12 }}
      />
    </div>
  );

  if (collapsed) {
    return (
      <div
        className="enhanced-progress-collapsed"
        onClick={onToggleCollapse}
        style={{
          padding: '8px 16px',
          background: '#f5f5f5',
          borderBottom: '1px solid #d9d9d9',
          cursor: 'pointer',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontWeight: 600 }}>当前阶段:</span>
          <Tag color="blue">{PhaseLabels[currentPhase]}</Tag>
        </div>
        <span style={{ fontSize: 12, color: '#999' }}>
          点击展开详情
        </span>
      </div>
    );
  }

  return (
    <div
      className="enhanced-progress-expanded"
      style={{
        padding: '12px 16px',
        background: '#fafafa',
        borderBottom: '1px solid #f0f0f0',
      }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
        <span style={{ fontWeight: 600, color: '#666' }}>开发流程</span>
        <Tooltip title="折叠">
          <span
            onClick={onToggleCollapse}
            style={{ cursor: 'pointer', color: '#1890ff', fontSize: 12 }}
          >
            收起 ▲
          </span>
        </Tooltip>
      </div>
      {content}
    </div>
  );
};

export default EnhancedProgress;
