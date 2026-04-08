import React, { useMemo } from 'react';
import { CheckCircleOutlined, CloseCircleOutlined, LoadingOutlined } from '@ant-design/icons';
import type { ContentBlockStatus } from '@/types';
import './ContentBlock.css';

interface ToolCallRowProps {
  toolName: string;
  input?: Record<string, unknown>;
  status: ContentBlockStatus;
  duration?: number;
  startedAt?: number;
  isError?: boolean;
}

/**
 * 格式化执行时间
 */
function formatDuration(ms?: number): string {
  if (!ms) return '0.0s';
  if (ms < 1000) return `${ms}ms`;
  const seconds = ms / 1000;
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  return `${minutes}m${remainingSeconds.toFixed(0)}s`;
}

/**
 * 截断参数显示
 */
function formatInput(input?: Record<string, unknown>, maxLength = 60): string {
  if (!input || Object.keys(input).length === 0) return '';

  // 优先显示关键字段
  const priorityKeys = ['file_path', 'command', 'pattern', 'url', 'query', 'path'];
  for (const key of priorityKeys) {
    const value = input[key];
    if (typeof value === 'string' && value.length > 0) {
      const display = value.length > maxLength ? `${value.slice(0, maxLength)}...` : value;
      return `${key}: "${display}"`;
    }
  }

  // 没有优先字段，显示完整 JSON（截断）
  try {
    const json = JSON.stringify(input);
    return json.length > maxLength ? `${json.slice(0, maxLength)}...` : json;
  } catch {
    return '';
  }
}

/**
 * 工具调用单行显示组件
 *
 * 显示格式：
 * ○ Bash {"command": "npm run build"}    8.3s  ⏱️   ← 运行中
 * ✓ Read {"file_path": "src/App.tsx"}    0.2s       ← 成功
 * ✗ Grep {"pattern": "todo",...          0.5s       ← 失败
 */
const ToolCallRow: React.FC<ToolCallRowProps> = ({
  toolName,
  input,
  status,
  duration,
  startedAt,
  isError,
}) => {
  // 实时计算运行时间
  const [runningTime, setRunningTime] = React.useState(duration || 0);

  React.useEffect(() => {
    if (status !== 'streaming' || !startedAt) return;

    const updateTimer = () => {
      setRunningTime(Date.now() - startedAt);
    };

    updateTimer();
    const interval = setInterval(updateTimer, 100);
    return () => clearInterval(interval);
  }, [status, startedAt]);

  const displayDuration = status === 'streaming' ? runningTime : duration;
  const inputStr = formatInput(input);

  const statusIcon = useMemo(() => {
    switch (status) {
      case 'streaming':
        return (
          <span className="tool-call-status streaming">
            <LoadingOutlined spin />
          </span>
        );
      case 'success':
        return <CheckCircleOutlined className="tool-call-status success" />;
      case 'failed':
        return <CloseCircleOutlined className="tool-call-status failed" />;
      default:
        return null;
    }
  }, [status, isError]);

  return (
    <div className={`tool-call-row ${status}`}>
      <span className="tool-call-icon">
        {status === 'streaming' ? '○' : status === 'success' ? '✓' : '✗'}
      </span>
      <span className="tool-call-name">{toolName}</span>
      {inputStr && (
        <span className="tool-call-input">{inputStr}</span>
      )}
      <span className="tool-call-duration">{formatDuration(displayDuration)}</span>
      {status === 'streaming' && (
        <span className="tool-call-timer">⏱️</span>
      )}
      {statusIcon}
    </div>
  );
};

export default React.memo(ToolCallRow);