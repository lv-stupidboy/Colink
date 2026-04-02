import React, { useState, useEffect } from 'react';

interface DurationDisplayProps {
  startedAt?: string;
  completedAt?: string;
  isRunning?: boolean;
  compact?: boolean;
}

export const DurationDisplay: React.FC<DurationDisplayProps> = ({
  startedAt,
  completedAt,
  isRunning,
  compact = false
}) => {
  const [elapsed, setElapsed] = useState(0);

  useEffect(() => {
    if (!startedAt) return;

    const startTime = new Date(startedAt).getTime();

    if (completedAt) {
      // 已完成：显示总时长
      const endTime = new Date(completedAt).getTime();
      setElapsed(endTime - startTime);
      return;
    }

    if (!isRunning) return;

    // 运行中：实时计时
    const updateElapsed = () => {
      setElapsed(Date.now() - startTime);
    };

    updateElapsed();
    const timer = setInterval(updateElapsed, 1000);

    return () => clearInterval(timer);
  }, [startedAt, completedAt, isRunning]);

  if (!startedAt) return null;

  return (
    <span className={`agent-duration ${compact ? 'compact' : ''}`}>
      {formatDuration(elapsed, compact)}
    </span>
  );
};

const formatDuration = (ms: number, compact: boolean): string => {
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);

  if (compact) {
    // 紧凑格式：12s, 3m12s, 1h23m
    if (hours > 0) {
      return `${hours}h${minutes % 60}m`;
    }
    if (minutes > 0) {
      return `${minutes}m${seconds % 60}s`;
    }
    return `${seconds}s`;
  }

  // 常规格式：12s, 3m 12s, 1h 23m
  if (hours > 0) {
    return `${hours}h ${minutes % 60}m`;
  }
  if (minutes > 0) {
    return `${minutes}m ${seconds % 60}s`;
  }
  return `${seconds}s`;
};

export default DurationDisplay;