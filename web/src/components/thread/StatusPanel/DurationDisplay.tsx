import React, { useState, useEffect } from 'react';

// 解析带纳秒的 ISO 时间格式（如 2026-04-13T15:44:34.3872777+08:00）
// JavaScript Date 只支持毫秒精度，需要截断纳秒
const parseISOTime = (isoString?: string): Date | null => {
  if (!isoString) return null;

  try {
    // 处理带纳秒的格式：截断为毫秒精度
    let normalized = isoString;

    // 如果有小数点（纳秒），截断为毫秒（3位）
    const match = isoString.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})\.(\d+)(.*)$/);
    if (match) {
      const [, base, fractional, suffix] = match;
      const ms = (fractional.slice(0, 3) || '000').padEnd(3, '0');
      normalized = `${base}.${ms}${suffix}`;
    }

    const date = new Date(normalized);
    if (isNaN(date.getTime())) {
      return null;
    }
    return date;
  } catch {
    return null;
  }
};

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

    const startTime = parseISOTime(startedAt);
    if (!startTime) return;

    if (completedAt) {
      // 已完成：显示总时长
      const endTime = parseISOTime(completedAt);
      if (endTime) {
        setElapsed(endTime.getTime() - startTime.getTime());
      }
      return;
    }

    if (!isRunning) return;

    // 运行中：实时计时
    const updateElapsed = () => {
      setElapsed(Date.now() - startTime.getTime());
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