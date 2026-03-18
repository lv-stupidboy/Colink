import React from 'react';
import { Space, Button, Tooltip } from 'antd';
import {
  PauseCircleOutlined,
  PlayCircleOutlined,
  RedoOutlined,
  StopOutlined,
  StepForwardOutlined,
} from '@ant-design/icons';

interface InterventionControlsProps {
  onPause?: () => void;
  onResume?: () => void;
  onSkip?: () => void;
  onRetry?: () => void;
  onStop?: () => void;
  isPaused?: boolean;
  isRunning?: boolean;
  disabled?: boolean;
}

/**
 * 干预操作控制面板
 * 提供暂停、跳过、重做、终止等操作
 * 产物按钮已移至 ThreadView 中单独放置
 */
export const InterventionControls: React.FC<InterventionControlsProps> = ({
  onPause,
  onResume,
  onSkip,
  onRetry,
  onStop,
  isPaused = false,
  isRunning = false,
  disabled = false,
}) => {
  return (
    <div className="intervention-controls">
      <Space wrap size="small">
        <Tooltip title={isPaused ? '继续' : '暂停当前 Agent'}>
          <Button
            icon={isPaused ? <PlayCircleOutlined /> : <PauseCircleOutlined />}
            onClick={isPaused ? onResume : onPause}
            disabled={disabled || !isRunning}
            size="small"
          >
            {isPaused ? '继续' : '暂停'}
          </Button>
        </Tooltip>

        <Tooltip title="跳过当前 Agent，进入下一阶段">
          <Button
            icon={<StepForwardOutlined />}
            onClick={onSkip}
            disabled={disabled || !isRunning}
            size="small"
          >
            跳过
          </Button>
        </Tooltip>

        <Tooltip title="重做当前阶段">
          <Button
            icon={<RedoOutlined />}
            onClick={onRetry}
            disabled={disabled}
            size="small"
          >
            重做
          </Button>
        </Tooltip>

        <Tooltip title="终止当前任务">
          <Button
            icon={<StopOutlined />}
            onClick={onStop}
            disabled={disabled || !isRunning}
            danger
            size="small"
          >
            终止
          </Button>
        </Tooltip>
      </Space>
    </div>
  );
};

export default InterventionControls;
